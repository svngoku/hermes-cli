package engine

import (
	"context"
	"io"
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
	return RuntimeProfile{Kind: RuntimePythonVenv, RequiresNVIDIA: true, SupportsTensorParallel: true}
}

func (e *SGLangEngine) venvDir() string {
	return defaultVenvPath("sglang")
}

func (e *SGLangEngine) CheckInstalled(ctx context.Context) (bool, string, error) {
	if python, ok := venvExecutable(e.venvDir(), "python"); ok {
		result := execx.Run(ctx, python, "-c", "import sglang; print(sglang.__version__)")
		if result.ExitCode == 0 {
			return true, result.Stdout, nil
		}
	}

	// A system Python installation is launchable by ServeCommand too.
	if python, err := python3(); err == nil {
		result := execx.Run(ctx, python, "-c", "import sglang; print(sglang.__version__)")
		if result.ExitCode == 0 {
			return true, result.Stdout, nil
		}
	}

	return false, "", nil
}

func (e *SGLangEngine) Install(ctx context.Context, stdout, stderr io.Writer) error {
	return installPythonEngine(ctx, stdout, stderr, e.venvDir(), "sglang[all]")
}

func (e *SGLangEngine) ServeCommand(cfg config.ServeConfig) (string, []string) {
	binary := "python3"
	if python, err := python3(); err == nil {
		binary = python
	}
	var args []string
	switch venvDir := cfg.VenvPath; {
	case venvDir != "":
		binary = filepath.Join(venvDir, "bin", "python")
		args = nil
	default:
		if python, ok := venvExecutable(e.venvDir(), "python"); ok {
			binary = python
			args = nil
		}
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
