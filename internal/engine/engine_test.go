package engine

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/svngoku/hermes-cli/internal/config"
)

func TestGet(t *testing.T) {
	if _, ok := Get(config.EngineSGLang).(*SGLangEngine); !ok {
		t.Errorf("Get(sglang) did not return *SGLangEngine")
	}
	if _, ok := Get(config.EngineVLLM).(*VLLMEngine); !ok {
		t.Errorf("Get(vllm) did not return *VLLMEngine")
	}
	if _, ok := Get(config.EngineLlamaCpp).(*LlamaCppEngine); !ok {
		t.Errorf("Get(llamacpp) did not return *LlamaCppEngine")
	}
	if e := Get(config.Engine("nope")); e != nil {
		t.Errorf("Get(unknown) = %v, want nil", e)
	}
}

func TestLlamaCppCheckInstalled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	binary := filepath.Join(dir, "llama-server")
	script := `#!/bin/sh
case "$1" in
  --version) echo "llama.cpp test-build" ;;
  --help) echo "--model --hf-repo --model-url --host --port --gpu-layers" ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	installed, version, err := (&LlamaCppEngine{}).CheckInstalled(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !installed || !strings.Contains(version, "test-build") {
		t.Errorf("CheckInstalled() = %t, %q", installed, version)
	}
}

func TestLlamaCppCheckInstalledRejectsUnsupportedBinary(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "llama-server")
	script := "#!/bin/sh\n[ \"$1\" = --version ] && echo old && exit 0\necho --model\n"
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	installed, _, err := (&LlamaCppEngine{}).CheckInstalled(context.Background())
	if err == nil || installed {
		t.Fatalf("CheckInstalled() = %t, %v; want unsupported error", installed, err)
	}
}

func TestVLLMServeCommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.ServeConfig{
		Engine: config.EngineVLLM,
		Model:  "Qwen/Qwen3-8B",
		TP:     2,
		Host:   "0.0.0.0",
		Port:   8000,
	}
	bin, args := (&VLLMEngine{}).ServeCommand(cfg)
	if bin != "vllm" {
		t.Errorf("bin = %q, want vllm", bin)
	}
	want := []string{"serve", "Qwen/Qwen3-8B",
		"--host", "0.0.0.0", "--port", "8000",
		"--tensor-parallel-size", "2", "--trust-remote-code"}
	if !slices.Equal(args, want) {
		t.Errorf("args = %#v, want %#v", args, want)
	}
}

func TestVLLMServeCommandExtraArgsQuoted(t *testing.T) {
	cfg := config.ServeConfig{
		Engine:    config.EngineVLLM,
		Model:     "m",
		TP:        1,
		Host:      "127.0.0.1",
		Port:      30000,
		ExtraArgs: []string{"--reasoning-parser", "qwen3", "--chat-template", "a b"},
	}
	_, args := (&VLLMEngine{}).ServeCommand(cfg)
	for _, want := range []string{"--reasoning-parser", "qwen3", "--chat-template", "a b"} {
		if !slices.Contains(args, want) {
			t.Errorf("args %#v missing %q", args, want)
		}
	}
}

func TestVLLMServeCommandUsesConfiguredVenv(t *testing.T) {
	cfg := config.ServeConfig{
		Model:    "m",
		TP:       1,
		Host:     "127.0.0.1",
		Port:     30000,
		VenvPath: "/opt/hermes/venv",
	}
	bin, args := (&VLLMEngine{}).ServeCommand(cfg)
	if bin != "/opt/hermes/venv/bin/vllm" {
		t.Errorf("bin = %q", bin)
	}
	if len(args) == 0 || args[0] != "serve" {
		t.Errorf("args = %#v", args)
	}
}

func TestSGLangServeCommandIncludesExtraArgs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	cfg := config.ServeConfig{
		Engine:    config.EngineSGLang,
		Model:     "m",
		TP:        4,
		Host:      "0.0.0.0",
		Port:      30000,
		ExtraArgs: []string{"--mem-fraction-static", "0.8"},
	}
	bin, args := (&SGLangEngine{}).ServeCommand(cfg)
	if bin != "python3" {
		t.Errorf("bin = %q, want python3", bin)
	}
	if !slices.Contains(args, "sglang.launch_server") {
		t.Errorf("args %#v missing launch module", args)
	}
	if !slices.Contains(args, "--mem-fraction-static") || !slices.Contains(args, "0.8") {
		t.Errorf("args %#v did not include extra args", args)
	}
}

func TestVLLMServeCommandUsesDefaultVenv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vllmBin := filepath.Join(home, "vllm-env", "bin")
	if err := os.MkdirAll(vllmBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vllmBin, "vllm"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	bin, args := (&VLLMEngine{}).ServeCommand(config.ServeConfig{
		Model: "m", TP: 1, Host: "127.0.0.1", Port: 30000,
	})
	if bin != filepath.Join(vllmBin, "vllm") {
		t.Errorf("bin = %q, want default venv binary", bin)
	}
	if len(args) == 0 || args[0] != "serve" {
		t.Errorf("args = %#v", args)
	}
}

func TestSGLangServeCommandUsesDefaultVenv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	venvBin := filepath.Join(home, "sglang-env", "bin")
	if err := os.MkdirAll(venvBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(venvBin, "python"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	bin, args := (&SGLangEngine{}).ServeCommand(config.ServeConfig{
		Model: "m", TP: 1, Host: "127.0.0.1", Port: 30000,
	})
	if bin != filepath.Join(venvBin, "python") {
		t.Errorf("bin = %q, want default venv python", bin)
	}
	if !slices.Contains(args, "sglang.launch_server") {
		t.Errorf("args = %#v", args)
	}
}

// writeFakePython installs a fake python3 that "creates" venvs by copying
// itself, answers pip commands, and reports version 9.9.9 for import probes.
func writeFakePython(t *testing.T, binDir string) {
	t.Helper()
	script := `#!/bin/sh
# Self-contained: PATH only contains the fake bin dir during tests.
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
	if err := os.WriteFile(filepath.Join(binDir, "python3"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestSGLangInstallStreamsIntoDedicatedVenv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binDir := t.TempDir()
	writeFakePython(t, binDir)
	t.Setenv("PATH", binDir)

	var out bytes.Buffer
	if err := (&SGLangEngine{}).Install(context.Background(), &out, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "sglang-env", "bin", "python")); err != nil {
		t.Errorf("venv python missing: %v", err)
	}
	for _, want := range []string{"$ ", "-m venv", "-m pip install -U pip", "sglang[all]"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}

	installed, version, err := (&SGLangEngine{}).CheckInstalled(context.Background())
	if err != nil || !installed || version != "9.9.9" {
		t.Errorf("CheckInstalled() = %t, %q, %v", installed, version, err)
	}
}

func TestVLLMInstallUsesPip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binDir := t.TempDir()
	writeFakePython(t, binDir)
	t.Setenv("PATH", binDir)

	var out bytes.Buffer
	if err := (&VLLMEngine{}).Install(context.Background(), &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "pip install -U vllm") {
		t.Errorf("output missing vLLM pip install:\n%s", out.String())
	}
	if strings.Contains(out.String(), "--torch-backend") {
		t.Errorf("output contains uv-only torch backend flag:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(home, "vllm-env", "bin", "python")); err != nil {
		t.Errorf("venv python missing: %v", err)
	}
}

func TestLlamaCppServeCommandFallsBackToLocalBin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0o755); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
case "$1" in
  --version) echo "local-build" ;;
  --help) echo "--model --hf-repo --model-url --host --port --gpu-layers" ;;
esac
`
	if err := os.WriteFile(filepath.Join(localBin, "llama-server"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	bin, _ := (&LlamaCppEngine{}).ServeCommand(config.ServeConfig{
		Model: "/models/m.gguf", Host: "127.0.0.1", Port: 8080, GPULayers: -1,
	})
	if bin != filepath.Join(localBin, "llama-server") {
		t.Errorf("bin = %q, want ~/.local/bin fallback", bin)
	}

	installed, version, err := (&LlamaCppEngine{}).CheckInstalled(context.Background())
	if err != nil || !installed || version != "local-build" {
		t.Errorf("CheckInstalled() = %t, %q, %v", installed, version, err)
	}
}

// installFakeLlamaCppToolchain puts fake git/cmake/make binaries on PATH and a
// fake llama-server template referenced by FAKE_LLAMA_SERVER.
func installFakeLlamaCppToolchain(t *testing.T) (home, binDir string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	binDir = t.TempDir()

	fakeServer := filepath.Join(binDir, "fake-llama-server")
	serverScript := `#!/bin/sh
case "$1" in
  --version) echo "test-build" ;;
  --help) echo "--model --hf-repo --model-url --host --port --gpu-layers" ;;
esac
`
	if err := os.WriteFile(fakeServer, []byte(serverScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_LLAMA_SERVER", fakeServer)

	tools := map[string]string{
		"git": `#!/bin/sh
# Self-contained: PATH only contains the fake bin dir during tests.
if [ "$1" = "clone" ]; then
  /bin/mkdir -p "$3/.git"
  exit 0
fi
if [ "$1" = "-C" ]; then
  exit 0
fi
exit 1
`,
		"cmake": `#!/bin/sh
if [ "$1" = "-S" ]; then
  /bin/mkdir -p "$4/bin"
  exit 0
fi
if [ "$1" = "--build" ]; then
  /bin/cp "$FAKE_LLAMA_SERVER" "$2/bin/llama-server"
  /bin/cp "$FAKE_LLAMA_SERVER" "$2/bin/llama-cli"
  /bin/chmod +x "$2/bin/llama-server" "$2/bin/llama-cli"
  exit 0
fi
exit 1
`,
		"make": "#!/bin/sh\nexit 0\n",
	}
	for name, script := range tools {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", binDir)
	return home, binDir
}

func TestLlamaCppInstallBuildsFromSource(t *testing.T) {
	home, _ := installFakeLlamaCppToolchain(t)

	var out bytes.Buffer
	if err := (&LlamaCppEngine{}).Install(context.Background(), &out, &out); err != nil {
		t.Fatal(err)
	}

	installed := filepath.Join(home, ".local", "bin", "llama-server")
	info, err := os.Stat(installed)
	if err != nil {
		t.Fatalf("installed binary missing: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("installed binary is not executable: %v", info.Mode())
	}
	for _, want := range []string{"git clone", "cmake -S", "-DCMAKE_BUILD_TYPE=Release", "cmake --build", "not on PATH"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "-DGGML_CUDA=ON") {
		t.Errorf("CUDA flag unexpected without nvidia-smi:\n%s", out.String())
	}

	ok, version, err := (&LlamaCppEngine{}).CheckInstalled(context.Background())
	if err != nil || !ok || version != "test-build" {
		t.Errorf("CheckInstalled() = %t, %q, %v", ok, version, err)
	}
}

func TestLlamaCppInstallEnablesCUDAWhenNvidiaPresent(t *testing.T) {
	_, binDir := installFakeLlamaCppToolchain(t)
	for _, tool := range []string{"nvidia-smi", "nvcc"} {
		if err := os.WriteFile(filepath.Join(binDir, tool), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var out bytes.Buffer
	if err := (&LlamaCppEngine{}).Install(context.Background(), &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "-DGGML_CUDA=ON") {
		t.Errorf("output missing CUDA flag:\n%s", out.String())
	}
}

func TestLlamaCppInstallRequiresNVCCForCUDA(t *testing.T) {
	_, binDir := installFakeLlamaCppToolchain(t)
	if err := os.WriteFile(filepath.Join(binDir, "nvidia-smi"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := (&LlamaCppEngine{}).Install(context.Background(), &out, &out)
	if err == nil || !strings.Contains(err.Error(), "nvcc is required") {
		t.Fatalf("Install() error = %v", err)
	}
}

func TestLlamaCppInstallRequiresBuildTools(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())

	var out bytes.Buffer
	err := (&LlamaCppEngine{}).Install(context.Background(), &out, &out)
	if err == nil || !strings.Contains(err.Error(), "required to build llama.cpp") {
		t.Fatalf("Install() error = %v", err)
	}
}

func TestLlamaCppServeCommands(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	tests := []struct {
		name string
		cfg  config.ServeConfig
		want []string
	}{
		{
			name: "local model and gpu layers",
			cfg: config.ServeConfig{
				Model: "/models/test.gguf", Host: "127.0.0.1", Port: 8080, GPULayers: 99,
				ExtraArgs: []string{"--ctx-size", "4096"},
			},
			want: []string{"--model", "/models/test.gguf", "--host", "127.0.0.1", "--port", "8080", "--gpu-layers", "99", "--ctx-size", "4096"},
		},
		{
			name: "hugging face repo",
			cfg:  config.ServeConfig{HFRepo: "owner/model:Q4_K_M", Host: "0.0.0.0", Port: 30000, GPULayers: -1},
			want: []string{"--hf-repo", "owner/model:Q4_K_M", "--host", "0.0.0.0", "--port", "30000"},
		},
		{
			name: "model URL",
			cfg:  config.ServeConfig{ModelURL: "https://example.com/model.gguf", Host: "0.0.0.0", Port: 30000, GPULayers: -1},
			want: []string{"--model-url", "https://example.com/model.gguf", "--host", "0.0.0.0", "--port", "30000"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin, args := (&LlamaCppEngine{}).ServeCommand(tt.cfg)
			if bin != "llama-server" {
				t.Errorf("bin = %q, want llama-server", bin)
			}
			if !slices.Equal(args, tt.want) {
				t.Errorf("args = %#v, want %#v", args, tt.want)
			}
		})
	}
}
