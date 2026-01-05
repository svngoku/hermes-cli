package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/execx"
)

type VLLMEngine struct{}

func (e *VLLMEngine) Name() string {
	return "vllm"
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
	result := execx.Run(ctx, "uv", "pip", "install", "-U", "vllm>=0.6")
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to install vllm: %s", result.Stderr)
	}
	return nil
}

func (e *VLLMEngine) ServeCommand(cfg config.ServeConfig) (string, []string) {
	args := []string{
		"run", "vllm", "serve", cfg.Model,
		"--host", cfg.Host,
		"--port", strconv.Itoa(cfg.Port),
		"--tensor-parallel-size", strconv.Itoa(cfg.TP),
		"--trust-remote-code",
	}
	if cfg.ExtraArgs != "" {
		args = append(args, strings.Fields(cfg.ExtraArgs)...)
	}
	return "uv", args
}
