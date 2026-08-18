# References — Library & Tool Docs

> Pointers to upstream documentation for key dependencies. Keep links current;
> verify versions against `go.mod` and engine version floors.

## Inference Engines

- **vLLM** — https://docs.vllm.ai/ (PyPI: https://pypi.org/project/vllm/)
  - Current floor in code: `vllm>=0.8`
  - Reasoning: use `--reasoning-parser <name>`; `--enable-reasoning` is deprecated.
- **SGLang** — https://docs.sglang.io/ (PyPI: https://pypi.org/project/sglang/)
  - Current floor in code: `sglang>=0.5`; requires Python 3.10+.
- **llama.cpp server** — https://github.com/ggml-org/llama.cpp/tree/master/tools/server
  - Preinstalled `llama-server`; Hermes supports local, Hugging Face, and URL GGUF sources.
- **vLLM-Studio** — https://github.com/0xSero/vllm-studio

## Tooling

- **uv** — https://docs.astral.sh/uv/
- **Go** — https://go.dev/doc/

## Charm Ecosystem (UI)

- **Bubble Tea** — https://github.com/charmbracelet/bubbletea
- **Bubbles** — https://github.com/charmbracelet/bubbles
- **Lip Gloss** — https://github.com/charmbracelet/lipgloss
- **Huh** — https://github.com/charmbracelet/huh
- **log** — https://github.com/charmbracelet/log
