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
--tp              Tensor parallel size (default: 4)
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

## What's Included

- **LLM Engines**: SGLang / vLLM with configurable tensor parallelism
- **vLLM-Studio**: Model lifecycle management, chat UI, recipe saving
- **Web Interfaces**:
  - vLLM-Studio controller: `http://localhost:8000`
  - Optional frontend dev server

## Logs

Server logs and TUI output go to `./hermes.log`. Follow with:

```bash
tail -f hermes.log
```

## Quick Examples

```bash
# Basic: sglang + studio (default)
./hermes.sh --model meta-llama/Llama-2-7b

# vLLM with studio + frontend
./hermes.sh --engine vllm --model mistralai/Mistral-7B-v0.1 --frontend

# No studio
./hermes.sh --engine sglang --model openchat/openchat-3.5 --studio 0

# Daemon mode (background)
./hermes.sh --model model-id --daemon
tail -f hermes.log
```

## References

- [SGLang](https://github.com/sgl-project/sglang)
- [vLLM](https://github.com/vllm-project/vllm)
- [vLLM-Studio](https://github.com/0xSero/vllm-studio)
