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

func TestInstallLlamaCppMissingBuildTools(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	testCtx := newTestAppContext(t)

	err := Install(testCtx.AppContext, []string{"--install", "llamacpp"})
	if err == nil || !strings.Contains(err.Error(), "required to build llama.cpp") {
		t.Fatalf("Install() error = %v", err)
	}
}

// fakePython answers venv creation, pip installs, and import probes.
// Self-contained: PATH only contains the fake bin dir during tests.
const fakePython = `#!/bin/sh
if [ "$1" = "-m" ] && [ "$2" = "venv" ]; then
  /bin/mkdir -p "$3/bin"
  /bin/cp "$0" "$3/bin/python"
  exit 0
fi
if [ "$1" = "-m" ] && [ "$2" = "pip" ]; then
  echo "pip: $*"
  exit 0
fi
if [ "$1" = "-c" ]; then
  echo "9.9.9"
  exit 0
fi
exit 0
`

func TestInstallSGLangStreamsIntoDedicatedVenv(t *testing.T) {
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "python3"), []byte(fakePython), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	testCtx := newTestAppContext(t)

	if err := Install(testCtx.AppContext, []string{"--install", "sglang"}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	for _, want := range []string{"Streaming live output", "-m venv", "sglang[all]", "sglang installed: 9.9.9"} {
		if !strings.Contains(testCtx.stdout.String(), want) {
			t.Errorf("stdout missing %q:\n%s", want, testCtx.stdout)
		}
	}

	state, err := loadState()
	if err != nil {
		t.Fatal(err)
	}
	if !state.SGLangInstalled || state.SGLangVersion != "9.9.9" {
		t.Errorf("state = %#v", state)
	}
}

func TestInstallAllIncludesLlamaCpp(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	testCtx := newTestAppContext(t)

	// No python3/git/cmake available: sglang fails first, proving all three
	// engines were selected (llamacpp would never need python3).
	err := Install(testCtx.AppContext, []string{"--install", "all"})
	if err == nil || !strings.Contains(err.Error(), "python3 is required") {
		t.Fatalf("Install() error = %v", err)
	}
	if !strings.Contains(testCtx.stdout.String(), "llamacpp: not installed") {
		t.Errorf("stdout = %q", testCtx.stdout)
	}
}
