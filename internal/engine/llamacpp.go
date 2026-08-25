package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/svngoku/hermes-cli/internal/config"
	"github.com/svngoku/hermes-cli/internal/execx"
)

const (
	llamaCppProbeTimeout = 10 * time.Second
	llamaCppRepoURL      = "https://github.com/ggml-org/llama.cpp"
)

type LlamaCppEngine struct{}

func (e *LlamaCppEngine) Name() string {
	return "llamacpp"
}

func (e *LlamaCppEngine) Profile() RuntimeProfile {
	return RuntimeProfile{Kind: RuntimeNative}
}

// llamaServerBinary resolves the llama-server binary, preferring PATH and
// falling back to the Hermes install location (~/.local/bin).
func llamaServerBinary() (string, bool) {
	if path, err := exec.LookPath("llama-server"); err == nil {
		return path, true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	candidate := filepath.Join(home, ".local", "bin", "llama-server")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate, true
	}
	return "", false
}

func (e *LlamaCppEngine) CheckInstalled(ctx context.Context) (bool, string, error) {
	binary, ok := llamaServerBinary()
	if !ok {
		return false, "", nil
	}

	versionResult := execx.RunWithTimeout(ctx, llamaCppProbeTimeout, binary, "--version")
	if versionResult.ExitCode != 0 {
		return false, "", fmt.Errorf("llama-server --version failed: %s", commandError(versionResult))
	}

	helpResult := execx.RunWithTimeout(ctx, llamaCppProbeTimeout, binary, "--help")
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

// CheckLlamaCppBuildPrerequisites validates the tools used by Install. CUDA is
// required when an NVIDIA GPU is visible so the install cannot silently produce
// a CPU-only server on a GPU host.
func CheckLlamaCppBuildPrerequisites() error {
	for _, tool := range []string{"git", "cmake", "make"} {
		if !execx.CommandExists(tool) {
			return fmt.Errorf("%s is required to build llama.cpp; on Ubuntu: sudo apt install -y git cmake build-essential libcurl4-openssl-dev", tool)
		}
	}
	if execx.CommandExists("nvidia-smi") && !execx.CommandExists("nvcc") {
		return fmt.Errorf("nvcc is required for a CUDA llama.cpp build; install the NVIDIA CUDA toolkit")
	}
	return nil
}

// Install builds llama.cpp from source, mirroring the documented Ubuntu flow
// without sudo: clone into ~/llama.cpp, build with CMake (CUDA enabled when an
// NVIDIA GPU is present), and install binaries into ~/.local/bin.
func (e *LlamaCppEngine) Install(ctx context.Context, stdout, stderr io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("locate home directory: %w", err)
	}
	srcDir := filepath.Join(home, "llama.cpp")
	buildDir := filepath.Join(srcDir, "build")
	binDir := filepath.Join(home, ".local", "bin")

	if err := CheckLlamaCppBuildPrerequisites(); err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(srcDir, ".git")); err == nil {
		fmt.Fprintln(stdout, "Updating existing llama.cpp checkout: "+srcDir)
		if err := runStep(ctx, stdout, stderr, "git", "-C", srcDir, "pull", "--ff-only"); err != nil {
			fmt.Fprintln(stdout, "warning: git pull failed; building the existing checkout")
		}
	} else {
		if _, err := os.Stat(srcDir); err == nil {
			return fmt.Errorf("%s exists but is not a llama.cpp git clone; move it aside and retry", srcDir)
		}
		if err := runStep(ctx, stdout, stderr, "git", "clone", llamaCppRepoURL, srcDir); err != nil {
			return err
		}
	}

	configureArgs := []string{"-S", srcDir, "-B", buildDir, "-DCMAKE_BUILD_TYPE=Release"}
	if execx.CommandExists("nvidia-smi") {
		fmt.Fprintln(stdout, "NVIDIA GPU detected: enabling CUDA build (-DGGML_CUDA=ON)")
		configureArgs = append(configureArgs, "-DGGML_CUDA=ON")
	}
	if err := runStep(ctx, stdout, stderr, "cmake", configureArgs...); err != nil {
		return err
	}
	if err := runStep(ctx, stdout, stderr, "cmake", "--build", buildDir, "--config", "Release", "-j", strconv.Itoa(runtime.NumCPU())); err != nil {
		return err
	}

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", binDir, err)
	}
	for _, name := range []string{"llama-server", "llama-cli"} {
		dst := filepath.Join(binDir, name)
		if err := copyExecutable(filepath.Join(buildDir, "bin", name), dst); err != nil {
			return fmt.Errorf("install %s: %w", name, err)
		}
		fmt.Fprintln(stdout, "installed "+dst)
	}

	if !execx.CommandExists("llama-server") {
		fmt.Fprintf(stdout, "warning: %s is not on PATH; add it with: export PATH=\"%s:$PATH\"\n", binDir, binDir)
	}
	return nil
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (e *LlamaCppEngine) ServeCommand(cfg config.ServeConfig) (string, []string) {
	binary := "llama-server"
	if !execx.CommandExists(binary) {
		if resolved, ok := llamaServerBinary(); ok {
			binary = resolved
		}
	}
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
	return binary, args
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
