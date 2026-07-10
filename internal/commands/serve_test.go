package commands

import (
	"strings"
	"testing"

	"github.com/svngoku/hermes-cli/internal/config"
)

func TestEnvironmentWithCUDADevices(t *testing.T) {
	environ := []string{
		"PATH=/test/bin",
		"CUDA_VISIBLE_DEVICES=7",
		"OTHER=value",
		"CUDA_VISIBLE_DEVICES=8,9",
	}

	got := environmentWithCUDADevices(environ, "2,3")
	want := []string{
		"PATH=/test/bin",
		"OTHER=value",
		"CUDA_VISIBLE_DEVICES=2,3",
	}

	if len(got) != len(want) {
		t.Fatalf("environmentWithCUDADevices() = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("environmentWithCUDADevices()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStartEngineCUDAEnvironment(t *testing.T) {
	t.Setenv("HERMES_COMMANDS_PARENT_ENV", "retained")
	t.Setenv("CUDA_VISIBLE_DEVICES", "7")

	testCtx := newTestAppContext(t)
	cfg := config.ServeConfig{
		Engine:      config.EngineVLLM,
		Model:       "m",
		TP:          1,
		Host:        "127.0.0.1",
		Port:        12345,
		CUDADevices: "2,3",
		LogFile:     "",
	}

	cmd, logFile, err := startEngine(testCtx.AppContext, cfg)
	if err != nil {
		t.Fatalf("startEngine() error = %v", err)
	}
	if logFile != nil {
		t.Fatal("startEngine() log file is non-nil for empty LogFile")
	}

	var cudaEntries int
	var retainedParent bool
	for _, entry := range cmd.Env {
		switch {
		case strings.HasPrefix(entry, "CUDA_VISIBLE_DEVICES="):
			cudaEntries++
			if entry != "CUDA_VISIBLE_DEVICES=2,3" {
				t.Errorf("startEngine() CUDA environment = %q, want %q", entry, "CUDA_VISIBLE_DEVICES=2,3")
			}
		case entry == "HERMES_COMMANDS_PARENT_ENV=retained":
			retainedParent = true
		}
	}
	if cudaEntries != 1 {
		t.Errorf("startEngine() CUDA environment entry count = %d, want 1", cudaEntries)
	}
	if !retainedParent {
		t.Error("startEngine() environment did not retain known parent entry")
	}
}

func TestStartEngineEmptyCUDADevicesUsesNaturalInheritance(t *testing.T) {
	testCtx := newTestAppContext(t)
	cfg := config.ServeConfig{
		Engine:  config.EngineVLLM,
		Model:   "m",
		TP:      1,
		Host:    "127.0.0.1",
		Port:    12345,
		LogFile: "",
	}

	cmd, logFile, err := startEngine(testCtx.AppContext, cfg)
	if err != nil {
		t.Fatalf("startEngine() error = %v", err)
	}
	if logFile != nil {
		t.Fatal("startEngine() log file is non-nil for empty LogFile")
	}
	if cmd.Env != nil {
		t.Errorf("startEngine() Env = %q, want nil for natural inheritance", cmd.Env)
	}
}
