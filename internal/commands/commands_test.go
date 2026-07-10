package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/svngoku/hermes-cli/internal/app"
)

type testAppContext struct {
	*app.AppContext
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	logs   *bytes.Buffer
}

func newTestAppContext(t *testing.T) *testAppContext {
	t.Helper()

	commandCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	logs := &bytes.Buffer{}

	return &testAppContext{
		AppContext: &app.AppContext{
			Ctx:     commandCtx,
			Cancel:  cancel,
			Logger:  log.New(logs),
			Stdout:  stdout,
			Stderr:  stderr,
			LogFile: "",
		},
		stdout: stdout,
		stderr: stderr,
		logs:   logs,
	}
}

func TestCommandValidation(t *testing.T) {
	tests := []struct {
		name    string
		command func(*app.AppContext, []string) error
		args    []string
		want    string
	}{
		{
			name:    "serve requires model",
			command: Serve,
			want:    "--model is required",
		},
		{
			name:    "serve rejects invalid engine",
			command: Serve,
			args:    []string{"--model", "m", "--engine", "nope"},
			want:    "invalid engine",
		},
		{
			name:    "serve rejects tensor parallel lower bound",
			command: Serve,
			args:    []string{"--model", "m", "--cuda-devices", "0", "--tp", "0"},
			want:    "tensor parallel size must be at least 1",
		},
		{
			name:    "serve rejects tensor parallel size above selected GPUs",
			command: Serve,
			args:    []string{"--model", "m", "--cuda-devices", "0", "--tp", "2"},
			want:    "exceeds available GPU count 1",
		},
		{
			name:    "serve rejects invalid CUDA devices",
			command: Serve,
			args:    []string{"--model", "m", "--cuda-devices", "0,x"},
			want:    "invalid --cuda-devices",
		},
		{
			name:    "serve rejects nonpositive boot timeout",
			command: Serve,
			args:    []string{"--model", "m", "--boot-timeout", "0"},
			want:    "--boot-timeout must be greater than zero",
		},
		{
			name:    "run requires engine",
			command: Run,
			args:    []string{"--model", "m"},
			want:    "--engine and --model are required",
		},
		{
			name:    "run rejects nonpositive readiness timeout",
			command: Run,
			args:    []string{"--engine", "vllm", "--model", "m", "--readiness-timeout", "0"},
			want:    "--readiness-timeout must be greater than zero",
		},
		{
			name:    "stop requires port or all",
			command: Stop,
			want:    "--port is required (or use --all)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx := newTestAppContext(t)
			err := tt.command(testCtx.AppContext, tt.args)
			if err == nil {
				t.Fatalf("command error = nil, want substring %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("command error = %q, want substring %q", err, tt.want)
			}
		})
	}
}
