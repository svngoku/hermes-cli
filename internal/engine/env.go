package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/svngoku/hermes-cli/internal/execx"
)

// defaultVenvPath returns the per-engine virtual environment directory
// (~/<name>-env), matching the documented install layout.
func defaultVenvPath(name string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, name+"-env")
}

// venvExecutable returns the path to name inside the venv's bin directory when
// it exists on disk.
func venvExecutable(venvDir, name string) (string, bool) {
	if venvDir == "" {
		return "", false
	}
	path := filepath.Join(venvDir, "bin", name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", false
	}
	return path, true
}

// runStep prints the exact command being run and streams its output so the
// operator can follow long installs.
func runStep(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) error {
	fmt.Fprintf(stdout, "\n$ %s %s\n", name, strings.Join(args, " "))
	if err := execx.RunWithStreaming(ctx, stdout, stderr, name, args...); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// python3 locates a Python 3 interpreter for venv creation.
func python3() (string, error) {
	for _, name := range []string{"python3", "python"} {
		if execx.CommandExists(name) {
			return name, nil
		}
	}
	return "", fmt.Errorf("python3 is required; on Ubuntu: sudo apt install -y python3 python3-venv")
}

// installPythonEngine creates the engine's dedicated virtual environment (when
// missing) and installs the given pip requirements into it, streaming all
// output. pipArgs are appended to `pip install -U`.
func installPythonEngine(ctx context.Context, stdout, stderr io.Writer, venvDir string, pipArgs ...string) error {
	if venvDir == "" {
		return fmt.Errorf("could not locate home directory for the virtual environment")
	}
	python, err := python3()
	if err != nil {
		return err
	}
	if _, ok := venvExecutable(venvDir, "python"); !ok {
		if err := runStep(ctx, stdout, stderr, python, "-m", "venv", venvDir); err != nil {
			return fmt.Errorf("create virtual environment: %w (on Ubuntu: sudo apt install -y python3-venv)", err)
		}
	}
	venvPython := filepath.Join(venvDir, "bin", "python")
	if err := runStep(ctx, stdout, stderr, venvPython, "-m", "pip", "install", "-U", "pip"); err != nil {
		return err
	}
	installArgs := append([]string{"-m", "pip", "install", "-U"}, pipArgs...)
	return runStep(ctx, stdout, stderr, venvPython, installArgs...)
}
