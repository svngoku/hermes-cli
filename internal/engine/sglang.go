package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/execx"
)

type SGLangEngine struct{}

func (e *SGLangEngine) Name() string {
	return "sglang"
}

func (e *SGLangEngine) Profile() RuntimeProfile {
	return RuntimeProfile{Kind: RuntimeUVPython, RequiresNVIDIA: true, SupportsTensorParallel: true}
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
	result := execx.Run(ctx, "uv", "pip", "install", "-U", "sglang>=0.5")
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to install sglang: %s", result.Stderr)
	}
	return nil
}

func (e *SGLangEngine) CheckInstalledIn(ctx context.Context, venvPath string) (bool, string, error) {
	result := execx.Run(ctx, filepath.Join(venvPath, "bin", "python"), "-c", "import sglang; print(sglang.__version__)")
	return result.ExitCode == 0, result.Stdout, nil
}

func (e *SGLangEngine) InstallIn(ctx context.Context, venvPath string) error {
	result := execx.Run(ctx, "uv", "pip", "install", "--python", filepath.Join(venvPath, "bin", "python"), "-U", "sglang>=0.5")
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to install sglang: %s", result.Stderr)
	}
	return nil
}

func (e *SGLangEngine) ServeCommand(cfg config.ServeConfig) (string, []string) {
	binary := "uv"
	args := []string{"run", "python"}
	if cfg.VenvPath != "" {
		binary = filepath.Join(cfg.VenvPath, "bin", "python")
		args = nil
	}
	args = append(args,
		"-m", "sglang.launch_server",
		"--model-path", cfg.Model,
		"--trust-remote-code",
		"--tp-size", strconv.Itoa(cfg.TP),
		"--host", cfg.Host,
		"--port", strconv.Itoa(cfg.Port),
	)
	args = append(args, cfg.ExtraArgs...)
	return binary, args
}
