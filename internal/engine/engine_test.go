package engine

import (
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
	cfg := config.ServeConfig{
		Engine: config.EngineVLLM,
		Model:  "Qwen/Qwen3-8B",
		TP:     2,
		Host:   "0.0.0.0",
		Port:   8000,
	}
	bin, args := (&VLLMEngine{}).ServeCommand(cfg)
	if bin != "uv" {
		t.Errorf("bin = %q, want uv", bin)
	}
	want := []string{"run", "vllm", "serve", "Qwen/Qwen3-8B",
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
	cfg := config.ServeConfig{
		Engine:    config.EngineSGLang,
		Model:     "m",
		TP:        4,
		Host:      "0.0.0.0",
		Port:      30000,
		ExtraArgs: []string{"--mem-fraction-static", "0.8"},
	}
	bin, args := (&SGLangEngine{}).ServeCommand(cfg)
	if bin != "uv" {
		t.Errorf("bin = %q, want uv", bin)
	}
	if !slices.Contains(args, "sglang.launch_server") {
		t.Errorf("args %#v missing launch module", args)
	}
	if !slices.Contains(args, "--mem-fraction-static") || !slices.Contains(args, "0.8") {
		t.Errorf("args %#v did not include extra args", args)
	}
}

func TestLlamaCppServeCommands(t *testing.T) {
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
