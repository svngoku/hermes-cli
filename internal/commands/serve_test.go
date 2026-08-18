package commands

import (
	"os"
	"path/filepath"
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

func TestValidateLlamaCppServeConfig(t *testing.T) {
	modelPath := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(modelPath, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	testCtx := newTestAppContext(t)
	cfg := config.ServeConfig{
		Engine:    config.EngineLlamaCpp,
		Model:     modelPath,
		GPULayers: -1,
		TP:        1,
		Host:      "127.0.0.1",
		Port:      8080,
	}
	if err := validateServeConfig(testCtx.AppContext, &cfg); err != nil {
		t.Fatalf("validateServeConfig() error = %v", err)
	}
	if !filepath.IsAbs(cfg.Model) {
		t.Errorf("validated model = %q, want absolute path", cfg.Model)
	}
}

func TestValidateLlamaCppModelSources(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.ServeConfig
		wantText string
	}{
		{
			name:     "missing source",
			cfg:      config.ServeConfig{},
			wantText: "exactly one",
		},
		{
			name:     "multiple sources",
			cfg:      config.ServeConfig{HFRepo: "owner/repo", ModelURL: "https://example.com/model.gguf"},
			wantText: "exactly one",
		},
		{
			name:     "invalid repo",
			cfg:      config.ServeConfig{HFRepo: "repo"},
			wantText: "owner/repository",
		},
		{
			name:     "URL credentials",
			cfg:      config.ServeConfig{ModelURL: "https://user@example.com/model.gguf"},
			wantText: "must not contain",
		},
		{
			name:     "URL query",
			cfg:      config.ServeConfig{ModelURL: "https://example.com/model.gguf?token=x"},
			wantText: "must not contain",
		},
		{
			name:     "invalid URL scheme",
			cfg:      config.ServeConfig{ModelURL: "file:///tmp/model.gguf"},
			wantText: "HTTP(S)",
		},
		{
			name:     "private URL",
			cfg:      config.ServeConfig{ModelURL: "http://127.0.0.1/model.gguf"},
			wantText: "public host",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLlamaCppModelSource(&tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.wantText) {
				t.Errorf("validateLlamaCppModelSource() error = %v, want %q", err, tt.wantText)
			}
		})
	}
}

func TestValidateLlamaCppExtraArgs(t *testing.T) {
	for _, arg := range []string{"--model", "--model=x.gguf", "-hf", "-hfr=owner/repo", "--port=8080", "-ngl"} {
		t.Run(arg, func(t *testing.T) {
			if err := validateLlamaCppExtraArgs([]string{arg}); err == nil {
				t.Errorf("validateLlamaCppExtraArgs(%q) error = nil", arg)
			}
		})
	}
	if err := validateLlamaCppExtraArgs([]string{"--ctx-size", "4096"}); err != nil {
		t.Errorf("validateLlamaCppExtraArgs() error = %v", err)
	}
}

func TestValidateLlamaCppSkipsGPUCapacity(t *testing.T) {
	t.Setenv("CUDA_VISIBLE_DEVICES", "")
	testCtx := newTestAppContext(t)
	cfg := config.ServeConfig{
		Engine:    config.EngineLlamaCpp,
		HFRepo:    "owner/repo",
		GPULayers: -1,
		TP:        1,
	}
	if err := validateServeConfig(testCtx.AppContext, &cfg); err != nil {
		t.Fatalf("validateServeConfig() error = %v", err)
	}
	cfg.TP = 2
	if err := validateServeConfig(testCtx.AppContext, &cfg); err == nil {
		t.Fatal("validateServeConfig() error = nil for llamacpp TP=2")
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

func TestValidateTensorParallelInheritedCUDADevices(t *testing.T) {
	tests := []struct {
		name      string
		inherited string
		tp        int
		wantErr   bool
	}{
		{name: "one inherited GPU cannot satisfy TP2", inherited: "0", tp: 2, wantErr: true},
		{name: "two inherited GPUs satisfy TP2", inherited: "0,1", tp: 2},
		{name: "empty inherited visibility means zero GPUs", inherited: "", tp: 1, wantErr: true},
		{name: "whitespace inherited visibility means zero GPUs", inherited: " \t ", tp: 1, wantErr: true},
		{name: "disabled inherited visibility means zero GPUs", inherited: "-1", tp: 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CUDA_VISIBLE_DEVICES", tt.inherited)
			testCtx := newTestAppContext(t)
			cfg := config.ServeConfig{TP: tt.tp}

			err := validateTensorParallel(testCtx.AppContext, cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTensorParallel() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTensorParallelUnsupportedInheritedCUDADevices(t *testing.T) {
	t.Setenv("CUDA_VISIBLE_DEVICES", "GPU-uuid")
	testCtx := newTestAppContext(t)

	if err := validateTensorParallel(testCtx.AppContext, config.ServeConfig{TP: 2}); err != nil {
		t.Fatalf("validateTensorParallel() error = %v, want nil for unsupported inherited format", err)
	}
	if !strings.Contains(testCtx.logs.String(), "unsupported inherited CUDA_VISIBLE_DEVICES") {
		t.Errorf("logs = %q, want unsupported inherited CUDA_VISIBLE_DEVICES warning", testCtx.logs)
	}
}

func TestValidateTensorParallelExplicitCUDADevicesOverrideEnvironment(t *testing.T) {
	t.Setenv("CUDA_VISIBLE_DEVICES", "0")
	testCtx := newTestAppContext(t)
	cfg := config.ServeConfig{
		TP:          2,
		CUDADevices: "2,3",
	}

	if err := validateTensorParallel(testCtx.AppContext, cfg); err != nil {
		t.Fatalf("validateTensorParallel() error = %v, want explicit devices to override inherited environment", err)
	}
}
