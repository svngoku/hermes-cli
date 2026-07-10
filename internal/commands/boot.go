package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const (
	bootLogTailBytes = 8192
	bootPollInterval = 50 * time.Millisecond
	bootRetryDelay   = 100 * time.Millisecond
	bootProbeTimeout = time.Second
)

// tailFile returns up to maxBytes from the end of path. It is best-effort so
// boot errors are never hidden by a missing or unreadable log file.
func tailFile(path string, maxBytes int64) string {
	if path == "" || maxBytes <= 0 {
		return ""
	}

	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.Size() <= 0 {
		return ""
	}

	size := info.Size()
	start := int64(0)
	if size > maxBytes {
		start = size - maxBytes
		size = maxBytes
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return ""
	}

	data, err := io.ReadAll(io.LimitReader(file, size))
	if err != nil {
		return ""
	}
	return string(data)
}

func probeReadiness(ctx context.Context, client *http.Client, base string) bool {
	for _, endpoint := range []string{"/v1/models", "/health"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+endpoint, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return true
		}
	}
	return false
}

// pollProcessExit checks and reaps an exited root process without blocking. It
// never reaps a running process.
func pollProcessExit(cmd *exec.Cmd) (bool, syscall.WaitStatus, error) {
	var status syscall.WaitStatus
	if cmd == nil || cmd.Process == nil {
		return true, status, fmt.Errorf("process was not started")
	}

	for {
		pid, err := syscall.Wait4(cmd.Process.Pid, &status, syscall.WNOHANG, nil)
		switch {
		case err == nil && pid == 0:
			return false, status, nil
		case err == nil:
			return true, status, nil
		case errors.Is(err, syscall.EINTR):
			continue
		case errors.Is(err, syscall.ECHILD):
			return true, status, nil
		default:
			return false, status, err
		}
	}
}

func processExitError(status syscall.WaitStatus, logPath string) error {
	message := "server exited during boot"
	switch {
	case status.Exited():
		message = fmt.Sprintf("%s with exit status %d", message, status.ExitStatus())
	case status.Signaled():
		message = fmt.Sprintf("%s from signal %s", message, status.Signal())
	}
	return bootErrorWithTail(message, logPath)
}

func bootErrorWithTail(message, logPath string) error {
	tail := tailFile(logPath, bootLogTailBytes)
	if tail == "" {
		return errors.New(message)
	}
	return fmt.Errorf("%s\nlog tail (%s):\n%s", message, logPath, tail)
}

// waitForBoot waits for readiness or an early root-process exit. Successful
// readiness intentionally leaves cmd unreaped so a daemon can outlive the CLI.
func waitForBoot(ctx context.Context, cmd *exec.Cmd, base string, timeout time.Duration, logPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if cmd == nil || cmd.Process == nil {
		return bootErrorWithTail("cannot wait for server boot: process was not started", logPath)
	}

	probeCtx, cancelProbes := context.WithCancel(ctx)
	defer cancelProbes()

	client := &http.Client{Timeout: bootProbeTimeout}
	probeResults := make(chan bool, 1)
	startProbe := func() {
		go func() {
			probeResults <- probeReadiness(probeCtx, client, strings.TrimRight(base, "/"))
		}()
	}

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	processTicker := time.NewTicker(bootPollInterval)
	defer processTicker.Stop()

	var retryTimer *time.Timer
	var retry <-chan time.Time
	startProbe()

	for {
		select {
		case ready := <-probeResults:
			if ready {
				exited, status, err := pollProcessExit(cmd)
				if err != nil {
					return bootErrorWithTail(fmt.Sprintf("failed to inspect server process during boot: %v", err), logPath)
				}
				if exited {
					return processExitError(status, logPath)
				}
				return nil
			}
			retryTimer = time.NewTimer(bootRetryDelay)
			retry = retryTimer.C
		case <-retry:
			retryTimer.Stop()
			retryTimer = nil
			retry = nil
			startProbe()
		case <-processTicker.C:
			exited, status, err := pollProcessExit(cmd)
			if err != nil {
				return bootErrorWithTail(fmt.Sprintf("failed to inspect server process during boot: %v", err), logPath)
			}
			if exited {
				return processExitError(status, logPath)
			}
		case <-ctx.Done():
			return bootErrorWithTail(fmt.Sprintf("server boot canceled: %v", ctx.Err()), logPath)
		case <-timeoutTimer.C:
			return bootErrorWithTail(
				fmt.Sprintf("server boot timed out after %s (log: %s)", timeout, logPath),
				logPath,
			)
		}
	}
}

// terminateAndReap stops a running process group after a failed boot. The
// initial nonblocking wait avoids signaling a PID that was already reaped.
func terminateAndReap(cmd *exec.Cmd) {
	exited, _, err := pollProcessExit(cmd)
	if exited || err != nil {
		return
	}

	stopProcess(cmd)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
		return
	case <-timer.C:
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Process.Kill()
		<-done
	}
}
