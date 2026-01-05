# Hermes — RunPod LLM Serving TUI

A beautiful, zero-dependency Bash TUI for launching and monitoring LLM serving engines (SGLang, vLLM) on RunPod.

## Features

- 🎨 **Branded header** with terminal colors
- ⏳ **Spinner feedback** for long-running tasks (installs, downloads)
- 📋 **Structured logging** to file + console
- 🎯 **Step-by-step guidance** with visual indicators
- 💾 **Portable** — uses only `tput`, `curl`, no external ASCII tools
- 🔧 **Environment-aware** — respects `NO_COLOR`, TTY detection
- 🎛️ **Integrated vLLM-Studio** — model lifecycle management & chat UI

## Usage

```bash
chmod +x hermes.sh lib/hermes_tui.sh

./hermes.sh \
  --engine sglang \
  --model IQuestLab/IQuest-Coder-V1-40B-Loop-Instruct \
  --tp 4 \
  --port 30000 \
  --debug
```

## Options

```
--engine          sglang | vllm
--model           HF repo id or local path
--tp              Tensor parallel size (default: 4, tune for your GPUs)
--host            Bind host (default: 0.0.0.0)
--port            Bind port (default: 30000)
--install         sglang|vllm|both|none (default: both)
--no-verify       Skip request verification
--daemon          Keep server running in background
--studio          Launch vllm-studio controller (default: 1)
--studio-port     vllm-studio controller port (default: 8000)
--frontend        Launch vllm-studio frontend (default: 0)
--log-file        Log file path (default: ./hermes.log)
--debug           Verbose logging
--no-color        Disable colors
--force-color     Force colors even if NO_COLOR=1
```

## Environment

- Requires: `bash 4+`, `tput`, `curl`, `nvidia-smi`
- Tested: Ubuntu 22.04+ with CUDA 12+
- For reasoning features (Qwen3, DeepSeek R1): Use system vLLM with `--install none`

## What's Included

- **LLM Engines**: SGLang / vLLM with configurable tensor parallelism
- **Optional vLLM-Studio**: Model lifecycle management, chat UI, recipe saving
  - Install separately: `pip install -e git+https://github.com/0xSero/vllm-studio.git#egg=vllm-studio`
  - Run: `vllm-studio --port 8000`

## Logs

Server logs and TUI output go to `./hermes.log`. Follow with:

```bash
tail -f hermes.log
```

## Quick Examples

```bash
# Basic: sglang (supports Llama, Qwen, Mistral, etc.)
./hermes.sh --model meta-llama/Llama-2-7b

# vLLM: broader model support
./hermes.sh --engine vllm --model mistralai/Mistral-7B-v0.1

# vLLM with Qwen3 reasoning parser (use system vLLM)
./hermes.sh --engine vllm --model Qwen/Qwen3-8B --install none --extra-args "--reasoning-parser qwen3"

# vLLM with DeepSeek R1 reasoning parser (use system vLLM)
./hermes.sh --engine vllm --model Qwen/Qwen3-8B --install none --extra-args "--reasoning-parser deepseek_r1"

# Disable optional studio instructions
./hermes.sh --engine vllm --model model-id --studio 0

# Daemon mode (background)
./hermes.sh --model model-id --daemon
tail -f hermes.log
```

### Model Compatibility

**SGLang** (better for latency):
- ✅ Llama 2/3, Qwen, Mistral, CodeLlama
- ❌ Custom/new architectures

**vLLM** (broader support):
- ✅ Most HF models (Llama, Qwen, Mistral, custom architectures)
- ✅ Better for new/experimental models

If you get `no SGlang implementation` error, switch to vLLM engine.

## vLLM-Studio Setup (Optional)

After hermes server is running, optionally add model management UI:

```bash
# Clone and install vllm-studio
git clone https://github.com/0xSero/vllm-studio
cd vllm-studio
pip install -e .

# In separate terminal, launch controller (default port: 8000)
vllm-studio --port 8000

# If port 8000 is in use, use different port
vllm-studio --port 8001

# Optionally launch frontend dev server (in another terminal)
cd frontend && npm install && npm run dev
```

### Configure port in hermes
```bash
# Use custom port for studio
./hermes.sh --engine vllm --model Qwen/Qwen3-8B --studio-port 8001
```

## References

- [SGLang](https://github.com/sgl-project/sglang)
- [vLLM](https://github.com/vllm-project/vllm)
- [vLLM-Studio](https://github.com/0xSero/vllm-studio)
