# Hermes — GPU Inference Server Launcher

A beautiful CLI for launching and monitoring LLM serving engines (SGLang, vLLM) on GPU infrastructure.

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
make build
./bin/hermes --help
```

### Requirements

- Go 1.24+
- NVIDIA GPU with `nvidia-smi`
- Python 3.8+ (for engine installation)

## Commands

| Command | Description |
|---------|-------------|
| `hermes doctor` | Check GPU, CUDA, and system requirements |
| `hermes install` | Install inference engines (sglang, vllm) |
| `hermes serve` | Start inference server |
| `hermes verify` | Verify server is responding |
| `hermes studio` | Launch vllm-studio controller |
| `hermes run` | Run full pipeline (doctor → install → serve → verify) |

## Quick Start

```bash
# Check system requirements
hermes doctor

# Install engines
hermes install --install both

# Start server
hermes serve --engine sglang --model meta-llama/Llama-3-8B --tp 4

# Or run the full pipeline
hermes run --engine vllm --model mistralai/Mistral-7B-v0.1 --tp 4
```

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
```

### Serve

```bash
# Start sglang server
hermes serve --engine sglang --model meta-llama/Llama-3-8B --tp 4

# Start vllm server with custom port
hermes serve --engine vllm --model mistralai/Mistral-7B-v0.1 --port 8080

# Daemon mode (background)
hermes serve --engine vllm --model Qwen/Qwen3-8B --daemon

# With extra engine arguments
hermes serve --engine vllm --model Qwen/Qwen3-8B --extra-args "--enable-reasoning --reasoning-parser qwen3"
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
hermes run --engine sglang --model meta-llama/Llama-3-8B --tp 4

# Daemon mode
hermes run --engine vllm --model Qwen/Qwen3-8B --daemon

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
  engine/                # Engine interface (sglang, vllm)
  execx/                 # Process execution helpers
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
./hermes.sh --engine sglang --model meta-llama/Llama-3-8B --tp 4
```

## Development

```bash
make build     # Build binary
make test      # Run tests
make lint      # Run go vet
make check     # Run all checks
```

## References

- [SGLang](https://github.com/sgl-project/sglang)
- [vLLM](https://github.com/vllm-project/vllm)
- [vLLM-Studio](https://github.com/0xSero/vllm-studio)
- [Charm](https://charm.sh) — Bubble Tea, Lip Gloss, Huh, Log
