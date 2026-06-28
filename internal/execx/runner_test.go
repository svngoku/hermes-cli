package execx

import (
	"context"
	"testing"
	"time"
)

func TestRunSuccess(t *testing.T) {
	res := Run(context.Background(), "echo", "hello")
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	if res.Stdout != "hello" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "hello")
	}
	if res.Err != nil {
		t.Errorf("Err = %v, want nil", res.Err)
	}
}

func TestRunNonZeroExit(t *testing.T) {
	res := Run(context.Background(), "false")
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", res.ExitCode)
	}
	if res.Err == nil {
		t.Error("Err = nil, want non-nil")
	}
}

func TestRunMissingCommand(t *testing.T) {
	res := Run(context.Background(), "hermes-no-such-binary-xyz")
	if res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", res.ExitCode)
	}
}

func TestRunWithTimeout(t *testing.T) {
	start := time.Now()
	res := RunWithTimeout(context.Background(), 100*time.Millisecond, "sleep", "5")
	if time.Since(start) > 2*time.Second {
		t.Errorf("RunWithTimeout did not cancel promptly: took %s", time.Since(start))
	}
	if res.ExitCode == 0 {
		t.Errorf("ExitCode = 0, want non-zero (timed out)")
	}
}

func TestCommandExists(t *testing.T) {
	if !CommandExists("echo") {
		t.Error("CommandExists(echo) = false, want true")
	}
	if CommandExists("hermes-no-such-binary-xyz") {
		t.Error("CommandExists(nonexistent) = true, want false")
	}
}
