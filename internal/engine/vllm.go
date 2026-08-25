package engine

import (
	"context"
	"io"
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
	return RuntimeProfile{Kind: RuntimePythonVenv, RequiresNVIDIA: true, SupportsTensorParallel: true}
}

func (e *VLLMEngine) venvDir() string {
	return defaultVenvPath("vllm")
}

func (e *VLLMEngine) CheckInstalled(ctx context.Context) (bool, string, error) {
	if python, ok := venvExecutable(e.venvDir(), "python"); ok {
		result := execx.Run(ctx, python, "-c", "import vllm; print(vllm.__version__)")
		if result.ExitCode == 0 {
			return true, result.Stdout, nil
		}
	}

	// A PATH installation is launchable by ServeCommand too.
	result := execx.Run(ctx, "vllm", "--version")
	if result.ExitCode == 0 {
		return true, result.Stdout, nil
	}

	return false, "", nil
}

func (e *VLLMEngine) Install(ctx context.Context, stdout, stderr io.Writer) error {
	return installPythonEngine(ctx, stdout, stderr, e.venvDir(), "vllm")
}

func (e *VLLMEngine) ServeCommand(cfg config.ServeConfig) (string, []string) {
	binary := "vllm"
	var args []string
	switch venvDir := cfg.VenvPath; {
	case venvDir != "":
		binary = filepath.Join(venvDir, "bin", "vllm")
		args = nil
	default:
		if vllm, ok := venvExecutable(e.venvDir(), "vllm"); ok {
			binary = vllm
			args = nil
		}
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
