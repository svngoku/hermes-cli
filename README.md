# Hermes — RunPod LLM Serving TUI

A beautiful, zero-dependency Bash TUI for launching and monitoring LLM serving engines (SGLang, vLLM) on RunPod.

## Features

- 🎨 **Branded header** with terminal colors
- ⏳ **Spinner feedback** for long-running tasks (installs, downloads)
- 📋 **Structured logging** to file + console
- 🎯 **Step-by-step guidance** with visual indicators
- 💾 **Portable** — uses only `tput`, `curl`, no external ASCII tools
- 🔧 **Environment-aware** — respects `NO_COLOR`, TTY detection

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
--engine        sglang | vllm
--model         HF repo id or local path
--tp            Tensor parallel size (default: 4)
--host          Bind host (default: 0.0.0.0)
--port          Bind port (default: 30000)
--install       sglang|vllm|both|none (default: both)
--no-verify     Skip request verification
--daemon        Keep server running in background
--log-file      Log file path (default: ./hermes.log)
--debug         Verbose logging
--no-color      Disable colors
--force-color   Force colors even if NO_COLOR=1
```

## Environment

- Requires: `bash 4+`, `tput`, `curl`, `nvidia-smi`
- Tested: Ubuntu 22.04+ with CUDA 12+

## Logs

Server logs and TUI output go to `./hermes.log`. Follow with:

```bash
tail -f hermes.log
```

## References

- [SGLang](https://github.com/sgl-project/sglang)
- [vLLM](https://github.com/vllm-project/vllm)
