package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallLlamaCppCheckDoesNotRequireUV(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "llama-server")
	script := `#!/bin/sh
case "$1" in
  --version) echo "test-build" ;;
  --help) echo "--model --hf-repo --model-url --host --port --gpu-layers" ;;
esac
`
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	testCtx := newTestAppContext(t)

	if err := Install(testCtx.AppContext, []string{"--install", "llamacpp", "--check"}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if !strings.Contains(testCtx.stdout.String(), "llamacpp: test-build") {
		t.Errorf("stdout = %q", testCtx.stdout)
	}
}

func TestInstallLlamaCppMissingReturnsManualInstruction(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	testCtx := newTestAppContext(t)

	err := Install(testCtx.AppContext, []string{"--install", "llamacpp"})
	if err == nil || !strings.Contains(err.Error(), "ensure llama-server is on PATH") {
		t.Fatalf("Install() error = %v", err)
	}
}
