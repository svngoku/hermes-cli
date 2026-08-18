package commands

import (
	"strings"
	"testing"

	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/settingsstore"
)

func TestSetupNonInteractiveSavesUserDefaults(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	testCtx := newTestAppContext(t)

	err := Setup(testCtx.AppContext, []string{
		"--non-interactive",
		"--install=false",
		"--engine", "llamacpp",
		"--hf-repo", "owner/repository:Q4_K_M",
		"--scope", "user",
		"--gpu-layers", "24",
	})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	settings, found, err := settingsstore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !found || settings.Engine != config.EngineLlamaCpp || settings.HFRepo != "owner/repository:Q4_K_M" {
		t.Errorf("settings = %#v, found = %t", settings, found)
	}
	if settings.GPULayers == nil || *settings.GPULayers != 24 || settings.TP != 1 {
		t.Errorf("settings = %#v", settings)
	}
	if !strings.Contains(testCtx.stdout.String(), "Configuration saved") {
		t.Errorf("stdout = %q", testCtx.stdout)
	}
}

func TestResolveRunInstallMode(t *testing.T) {
	mode, err := resolveRunInstallMode(config.EngineLlamaCpp, "")
	if err != nil || mode != config.InstallLlamaCpp {
		t.Errorf("resolveRunInstallMode(llamacpp, empty) = %q, %v", mode, err)
	}
	if _, err := resolveRunInstallMode(config.EngineLlamaCpp, config.InstallBoth); err == nil {
		t.Fatal("resolveRunInstallMode(llamacpp, both) error = nil")
	}
	mode, err = resolveRunInstallMode(config.EngineVLLM, "")
	if err != nil || mode != config.InstallVLLM {
		t.Errorf("resolveRunInstallMode(vllm, empty) = %q, %v", mode, err)
	}
}

func TestSetupAutoTPUsesSelectedCUDADevices(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	testCtx := newTestAppContext(t)

	err := Setup(testCtx.AppContext, []string{
		"--non-interactive",
		"--install=false",
		"--engine", "vllm",
		"--model", "owner/model",
		"--scope", "user",
		"--cuda-devices", "2,3",
	})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	settings, _, err := settingsstore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.TP != 2 {
		t.Errorf("TP = %d, want 2", settings.TP)
	}
	if settings.CUDADevices == nil || *settings.CUDADevices != "2,3" {
		t.Errorf("CUDA devices = %#v", settings.CUDADevices)
	}
	if settings.VenvPath == "" {
		t.Error("VenvPath is empty")
	}
}
