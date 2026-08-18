package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/execx"
)

const llamaCppProbeTimeout = 10 * time.Second

type LlamaCppEngine struct{}

func (e *LlamaCppEngine) Name() string {
	return "llamacpp"
}

func (e *LlamaCppEngine) Profile() RuntimeProfile {
	return RuntimeProfile{Kind: RuntimeNative}
}

func (e *LlamaCppEngine) CheckInstalled(ctx context.Context) (bool, string, error) {
	if !execx.CommandExists("llama-server") {
		return false, "", nil
	}

	versionResult := execx.RunWithTimeout(ctx, llamaCppProbeTimeout, "llama-server", "--version")
	if versionResult.ExitCode != 0 {
		return false, "", fmt.Errorf("llama-server --version failed: %s", commandError(versionResult))
	}

	helpResult := execx.RunWithTimeout(ctx, llamaCppProbeTimeout, "llama-server", "--help")
	help := helpResult.Stdout + "\n" + helpResult.Stderr
	if helpResult.ExitCode != 0 && strings.TrimSpace(help) == "" {
		return false, "", fmt.Errorf("llama-server --help failed: %s", commandError(helpResult))
	}
	for _, flag := range []string{"--model", "--hf-repo", "--model-url", "--host", "--port", "--gpu-layers"} {
		if !strings.Contains(help, flag) {
			return false, "", fmt.Errorf("llama-server is installed but does not support required flag %s", flag)
		}
	}

	version := strings.TrimSpace(versionResult.Stdout)
	if version == "" {
		version = strings.TrimSpace(versionResult.Stderr)
	}
	return true, version, nil
}

func (e *LlamaCppEngine) Install(ctx context.Context) error {
	installed, _, err := e.CheckInstalled(ctx)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}
	return fmt.Errorf("llama-server is not installed; install llama.cpp and ensure llama-server is on PATH")
}

func (e *LlamaCppEngine) ServeCommand(cfg config.ServeConfig) (string, []string) {
	args := make([]string, 0, 10+len(cfg.ExtraArgs))
	switch {
	case cfg.Model != "":
		args = append(args, "--model", cfg.Model)
	case cfg.HFRepo != "":
		args = append(args, "--hf-repo", cfg.HFRepo)
	case cfg.ModelURL != "":
		args = append(args, "--model-url", cfg.ModelURL)
	}
	args = append(args, "--host", cfg.Host, "--port", fmt.Sprint(cfg.Port))
	if cfg.GPULayers >= 0 {
		args = append(args, "--gpu-layers", fmt.Sprint(cfg.GPULayers))
	}
	args = append(args, cfg.ExtraArgs...)
	return "llama-server", args
}

func commandError(result execx.Result) string {
	if result.Stderr != "" {
		return result.Stderr
	}
	if result.Stdout != "" {
		return result.Stdout
	}
	if result.Err != nil {
		return result.Err.Error()
	}
	return "unknown error"
}
