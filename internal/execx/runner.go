package execx

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

func Run(ctx context.Context, name string, args ...string) Result {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		exitCode = -1
	}

	return Result{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: exitCode,
		Err:      err,
	}
}

// RunWithTimeout runs a command with a bounded duration so probes (e.g. a hung
// nvidia-smi) cannot block the CLI indefinitely.
func RunWithTimeout(ctx context.Context, timeout time.Duration, name string, args ...string) Result {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return Run(tctx, name, args...)
}

func RunWithStreaming(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
