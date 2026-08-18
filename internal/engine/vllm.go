package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/execx"
)

type VLLMEngine struct{}

func (e *VLLMEngine) Name() string {
	return "vllm"
}

func (e *VLLMEngine) Profile() RuntimeProfile {
	return RuntimeProfile{Kind: RuntimeUVPython, RequiresNVIDIA: true, SupportsTensorParallel: true}
}

func (e *VLLMEngine) CheckInstalled(ctx context.Context) (bool, string, error) {
	result := execx.Run(ctx, "uv", "run", "python", "-c", "import vllm; print(vllm.__version__)")
	if result.ExitCode == 0 {
		return true, result.Stdout, nil
	}

	result = execx.Run(ctx, "python", "-c", "import vllm; print(vllm.__version__)")
	if result.ExitCode == 0 {
		return true, result.Stdout, nil
	}

	result = execx.Run(ctx, "vllm", "--version")
	if result.ExitCode == 0 {
		return true, result.Stdout, nil
	}

	return false, "", nil
}

func (e *VLLMEngine) Install(ctx context.Context) error {
	result := execx.Run(ctx, "uv", "pip", "install", "-U", "vllm>=0.8")
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to install vllm: %s", result.Stderr)
	}
	return nil
}

func (e *VLLMEngine) CheckInstalledIn(ctx context.Context, venvPath string) (bool, string, error) {
	result := execx.Run(ctx, filepath.Join(venvPath, "bin", "python"), "-c", "import vllm; print(vllm.__version__)")
	return result.ExitCode == 0, result.Stdout, nil
}

func (e *VLLMEngine) InstallIn(ctx context.Context, venvPath string) error {
	result := execx.Run(ctx, "uv", "pip", "install", "--python", filepath.Join(venvPath, "bin", "python"), "-U", "vllm>=0.8")
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to install vllm: %s", result.Stderr)
	}
	return nil
}

func (e *VLLMEngine) ServeCommand(cfg config.ServeConfig) (string, []string) {
	binary := "uv"
	args := []string{"run", "vllm"}
	if cfg.VenvPath != "" {
		binary = filepath.Join(cfg.VenvPath, "bin", "vllm")
		args = nil
	}
	args = append(args,
		"serve", cfg.Model,
		"--host", cfg.Host,
		"--port", strconv.Itoa(cfg.Port),
		"--tensor-parallel-size", strconv.Itoa(cfg.TP),
		"--trust-remote-code",
	)
	args = append(args, cfg.ExtraArgs...)
	return binary, args
}
