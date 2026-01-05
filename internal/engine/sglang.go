package engine

import (
	"context"
	"fmt"
	"strconv"

	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/execx"
)

type SGLangEngine struct{}

func (e *SGLangEngine) Name() string {
	return "sglang"
}

func (e *SGLangEngine) CheckInstalled(ctx context.Context) (bool, string, error) {
	result := execx.Run(ctx, "uv", "run", "python", "-c", "import sglang; print(sglang.__version__)")
	if result.ExitCode == 0 {
		return true, result.Stdout, nil
	}

	result = execx.Run(ctx, "python", "-c", "import sglang; print(sglang.__version__)")
	if result.ExitCode == 0 {
		return true, result.Stdout, nil
	}

	return false, "", nil
}

func (e *SGLangEngine) Install(ctx context.Context) error {
	result := execx.Run(ctx, "uv", "pip", "install", "-U", "sglang>=0.4")
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to install sglang: %s", result.Stderr)
	}
	return nil
}

func (e *SGLangEngine) ServeCommand(cfg config.ServeConfig) (string, []string) {
	args := []string{
		"run", "python", "-m", "sglang.launch_server",
		"--model-path", cfg.Model,
		"--trust-remote-code",
		"--tp-size", strconv.Itoa(cfg.TP),
		"--host", cfg.Host,
		"--port", strconv.Itoa(cfg.Port),
	}
	return "uv", args
}
