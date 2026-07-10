package commands

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestTailFileEmptyUnreadableAndInvalidLimit(t *testing.T) {
	emptyPath := filepath.Join(t.TempDir(), "empty.log")
	if err := os.WriteFile(emptyPath, nil, 0600); err != nil {
		t.Fatalf("write empty log: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		maxBytes int64
	}{
		{name: "empty", path: emptyPath, maxBytes: 10},
		{name: "unreadable", path: filepath.Join(t.TempDir(), "missing.log"), maxBytes: 10},
		{name: "zero limit", path: emptyPath, maxBytes: 0},
		{name: "negative limit", path: emptyPath, maxBytes: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tailFile(tt.path, tt.maxBytes); got != "" {
				t.Errorf("tailFile() = %q, want empty", got)
			}
		})
	}
}

func TestTailFileSmallFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "small.log")
	const content = "short log"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	if got := tailFile(path, 100); got != content {
		t.Errorf("tailFile() = %q, want %q", got, content)
	}
}

func TestTailFileReturnsExactSuffix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.log")
	const content = "0123456789abcdefghijklmnopqrstuvwxyz"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	const maxBytes = 11
	want := content[len(content)-maxBytes:]
	if got := tailFile(path, maxBytes); got != want {
		t.Errorf("tailFile() = %q, want exact suffix %q", got, want)
	}
}

func TestWaitForBootHealthyLeavesProcessRunning(t *testing.T) {
	var modelsCalled atomic.Bool
	var healthCalled atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			modelsCalled.Store(true)
			http.NotFound(w, r)
		case "/health":
			healthCalled.Store(true)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := startBootTestCommand(t, "sleep 30")
	cleaned := false
	t.Cleanup(func() {
		if !cleaned {
			terminateAndReap(cmd)
		}
	})

	started := time.Now()
	if err := waitForBoot(context.Background(), cmd, server.URL, time.Second, ""); err != nil {
		t.Fatalf("waitForBoot() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Errorf("waitForBoot() took %s, want under 500ms", elapsed)
	}
	if !modelsCalled.Load() || !healthCalled.Load() {
		t.Errorf("readiness probes called models=%t health=%t, want both true", modelsCalled.Load(), healthCalled.Load())
	}
	if err := syscall.Kill(cmd.Process.Pid, 0); err != nil {
		t.Fatalf("healthy process is not alive after waitForBoot: %v", err)
	}

	stopProcess(cmd)
	_ = cmd.Wait()
	cleaned = true
}

func TestWaitForBootReportsEarlyExitAndLogTail(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "engine.log")
	const marker = "BOOT_FAILURE_MARKER"
	if err := os.WriteFile(logPath, []byte("engine crashed: "+marker+"\n"), 0600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	cmd := startBootTestCommand(t, "exit 17")
	started := time.Now()
	err := waitForBoot(context.Background(), cmd, unreachableBootURL(t), 2*time.Second, logPath)
	if err == nil {
		t.Fatal("waitForBoot() error = nil, want early-exit error")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("waitForBoot() took %s, want prompt early-exit detection", elapsed)
	}
	for _, want := range []string{"exited during boot", "exit status 17", marker} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("waitForBoot() error = %q, want substring %q", err, want)
		}
	}
}

func TestWaitForBootTimeoutIncludesLogTail(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "engine.log")
	const marker = "TIMEOUT_LOG_MARKER"
	if err := os.WriteFile(logPath, []byte(marker), 0600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	cmd := startBootTestCommand(t, "sleep 30")
	t.Cleanup(func() { terminateAndReap(cmd) })

	err := waitForBoot(context.Background(), cmd, unreachableBootURL(t), 120*time.Millisecond, logPath)
	if err == nil {
		t.Fatal("waitForBoot() error = nil, want timeout error")
	}
	for _, want := range []string{"timed out after 120ms", logPath, marker} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("waitForBoot() error = %q, want substring %q", err, want)
		}
	}
}

func TestWaitForBootContextCancellation(t *testing.T) {
	cmd := startBootTestCommand(t, "sleep 30")
	t.Cleanup(func() { terminateAndReap(cmd) })

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	err := waitForBoot(ctx, cmd, unreachableBootURL(t), 2*time.Second, "")
	if err == nil {
		t.Fatal("waitForBoot() error = nil, want cancellation error")
	}
	if !strings.Contains(err.Error(), "server boot canceled: context canceled") {
		t.Errorf("waitForBoot() error = %q, want contextual cancellation", err)
	}
}

func startBootTestCommand(t *testing.T, script string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start test process: %v", err)
	}
	return cmd
}

func unreachableBootURL(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve unreachable address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close reserved listener: %v", err)
	}
	return "http://" + address
}
