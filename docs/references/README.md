# References — Library & Tool Docs

> Pointers to upstream documentation for key dependencies. Keep links current;
> verify versions against `go.mod` and engine version floors.

## Inference Engines

- **vLLM** — https://docs.vllm.ai/ (PyPI: https://pypi.org/project/vllm/)
  - Installed via `pip install -U vllm` into `~/vllm-env`.
  - Reasoning: use `--reasoning-parser <name>`; `--enable-reasoning` is deprecated.
- **SGLang** — https://docs.sglang.io/ (PyPI: https://pypi.org/project/sglang/)
  - Installed via `pip install -U "sglang[all]"` into `~/sglang-env`; requires Python 3.10+.
- **llama.cpp server** — https://github.com/ggml-org/llama.cpp/tree/master/tools/server
  - Built from source (`git clone` + CMake, `-DGGML_CUDA=ON` when NVIDIA and `nvcc` are available)
    into `~/.local/bin`; Hermes supports local, Hugging Face, and URL GGUF sources.
  - Build docs: https://github.com/ggml-org/llama.cpp/blob/master/docs/build.md
- **vLLM-Studio** — https://github.com/0xSero/vllm-studio

## Tooling

- **Go** — https://go.dev/doc/

## Charm Ecosystem (UI)

- **Bubble Tea** — https://github.com/charmbracelet/bubbletea
- **Bubbles** — https://github.com/charmbracelet/bubbles
- **Lip Gloss** — https://github.com/charmbracelet/lipgloss
- **Huh** — https://github.com/charmbracelet/huh
- **log** — https://github.com/charmbracelet/log
