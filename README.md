# Hermes — GPU Inference Server Launcher

A CLI for launching and monitoring SGLang, vLLM, and llama.cpp inference servers.

Built with Go and the [Charm](https://charm.sh) ecosystem for delightful terminal UX.

## Features

- 🎨 **Beautiful TUI** with Lip Gloss styling and Bubble Tea components
- ⏳ **Spinner feedback** for long-running tasks
- 📋 **Structured logging** with charmbracelet/log (text/JSON/logfmt)
- 🎯 **Step-by-step pipeline** with visual indicators
- 🔧 **No framework bloat** — lightweight subcommand router (no Cobra)
- 🎛️ **Interactive mode** with huh forms for guided setup

## Installation

### From Source

```bash
git clone https://github.com/svngoku/hermes-cli.git
cd hermes-cli
just build
./bin/hermes --help
```

### Requirements

- Go 1.24+
- NVIDIA GPU with `nvidia-smi` for SGLang/vLLM
- Python 3.10+ for SGLang/vLLM installation
- Preinstalled `llama-server` for llama.cpp (CPU, Metal, or CUDA)
- [`just`](https://github.com/casey/just) (task runner, for development)

## Commands

| Command | Description |
|---------|-------------|
| `hermes setup` | Detect hardware, prepare an engine, and save defaults |
| `hermes doctor` | Check GPU, CUDA, and system requirements |
| `hermes install` | Install or check inference engines |
| `hermes serve` | Start inference server |
| `hermes verify` | Verify server is responding |
| `hermes studio` | Launch vllm-studio controller |
| `hermes run` | Run full pipeline (doctor → install → serve → verify) |
| `hermes stop` | Stop a background engine daemon |
| `hermes status` | List recorded engine daemons |

## Quick Start

```bash
# Guided GPU-host setup, installation, and saved defaults
hermes setup

# Uses saved defaults
hermes run
```

`setup` can also be automated, for example:

```bash
hermes setup --non-interactive --scope user --engine vllm \
  --model Qwen/Qwen3-8B
```

User defaults are stored in the platform config directory. Project defaults
are stored in `.hermes.json`, override user defaults, and can be selected with
`hermes setup --scope project`. Explicit command flags override saved values.
SGLang/vLLM setup uses one managed environment under the user cache directory
by default, so later `run` and `serve` commands use the same installation.

## Usage Examples

### Doctor (System Check)

```bash
# Human-readable output
hermes doctor

# JSON output for automation
hermes doctor --json

# Strict mode (fail on warnings)
hermes doctor --strict
```

### Install Engines

```bash
# Install both engines
hermes install --install both

# Install only sglang
hermes install --install sglang

# Check installation status without changes
hermes install --check

# Validate a preinstalled llama-server
hermes install --install llamacpp --check
```

### Serve

```bash
# Start sglang server
hermes serve --engine sglang --model meta-llama/Llama-3-8B --tp 1

# Start vllm server with custom port
hermes serve --engine vllm --model mistralai/Mistral-7B-v0.1 --port 8080

# Daemon mode (background)
hermes serve --engine vllm --model Qwen/Qwen3-8B --daemon

# Multi-GPU: select specific devices and set tensor parallel
hermes serve --engine vllm --model Qwen/Qwen3-8B --cuda-devices 0,1,2,3 --tp 4

# With extra engine arguments
hermes serve --engine vllm --model Qwen/Qwen3-8B --extra-args "--reasoning-parser qwen3"

# llama.cpp with a local GGUF
hermes serve --engine llamacpp --model ./model.gguf --gpu-layers 99

# llama.cpp can also resolve GGUF from Hugging Face or a public URL
hermes serve --engine llamacpp --hf-repo owner/model-GGUF:Q4_K_M
hermes serve --engine llamacpp --model-url https://models.example/model.gguf
```

### Daemon management

Background engines are recorded in `~/.cache/hermes/daemons` and can be listed
and stopped after the launching CLI has exited:

```bash
hermes status              # list recorded daemons and their health
hermes stop --port 30000   # stop a specific daemon
hermes stop --all          # stop every recorded daemon
```

### Verify

```bash
# Check server health
hermes verify --host 127.0.0.1 --port 30000

# JSON output
hermes verify --json

# Include chat completion test
hermes verify --chat
```

### Run (Full Pipeline)

```bash
# Complete pipeline: doctor → install → serve → verify
hermes run --engine sglang --model meta-llama/Llama-3-8B --tp 1

# Daemon mode
hermes run --engine vllm --model Qwen/Qwen3-8B --daemon

# Multi-GPU with CUDA device selection
hermes run --engine vllm --model Qwen/Qwen3-8B --cuda-devices 0,1 --tp 2

# Skip verification
hermes run --engine sglang --model mymodel --no-verify
```

## Global Flags

All commands support these flags:

```
--log-file      Log file path (default: ./hermes.log)
--debug         Enable debug logging
--no-color      Disable colored output
--force-color   Force colored output
```

## Architecture

```
cmd/hermes/main.go       # Entry point with subcommand router
internal/
  app/                   # AppContext, global config, Charm logger
  commands/              # Command implementations
  config/                # Typed config structs
  engine/                # Engine interface (sglang, vllm, llamacpp)
  execx/                 # Process execution helpers
  gpu/                   # GPU inventory (nvidia-smi) + CUDA device parsing
  pidfile/               # Daemon registry (PID records)
  settingsstore/         # User/project config persistence
  ui/                    # Lip Gloss styles
  ui/tui/                # Bubble Tea components (spinner, steps, forms)
```

## Model Compatibility

**SGLang** (better for latency):
- ✅ Llama 2/3, Qwen, Mistral, CodeLlama
- ❌ Custom/new architectures

**vLLM** (broader support):
- ✅ Most HF models (Llama, Qwen, Mistral, custom architectures)
- ✅ Better for new/experimental models

**llama.cpp** (native, cross-platform):
- ✅ GGUF models on CPU, Metal, or CUDA
- ✅ Local files, Hugging Face GGUF repositories, and public HTTP(S) URLs
- ⚠️ Base GGUF models may not include a usable chat template; use `verify --chat` explicitly

## API Examples

Once the server is running:

```bash
# List models
curl http://localhost:30000/v1/models

# Chat completion
curl -X POST http://localhost:30000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "default",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 128
  }'
```

## Legacy Bash Script

The original Bash implementation is still available:

```bash
./hermes.sh --engine sglang --model meta-llama/Llama-3-8B --tp 1
```

## Development

```bash
just build     # Build binary
just test      # Run tests
just lint      # Run go vet
just check     # Run all checks
```

## References

- [SGLang](https://github.com/sgl-project/sglang)
- [vLLM](https://github.com/vllm-project/vllm)
- [llama.cpp](https://github.com/ggml-org/llama.cpp)
- [vLLM-Studio](https://github.com/0xSero/vllm-studio)
- [Charm](https://charm.sh) — Bubble Tea, Lip Gloss, Huh, Log
