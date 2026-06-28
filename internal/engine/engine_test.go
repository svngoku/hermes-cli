package engine

import (
	"slices"
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
	if e := Get(config.Engine("nope")); e != nil {
		t.Errorf("Get(unknown) = %v, want nil", e)
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
		ExtraArgs: `--reasoning-parser qwen3 --chat-template "a b"`,
	}
	_, args := (&VLLMEngine{}).ServeCommand(cfg)
	for _, want := range []string{"--reasoning-parser", "qwen3", "--chat-template", "a b"} {
		if !slices.Contains(args, want) {
			t.Errorf("args %#v missing %q", args, want)
		}
	}
}

func TestSGLangServeCommandIncludesExtraArgs(t *testing.T) {
	cfg := config.ServeConfig{
		Engine:    config.EngineSGLang,
		Model:     "m",
		TP:        4,
		Host:      "0.0.0.0",
		Port:      30000,
		ExtraArgs: "--mem-fraction-static 0.8",
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
