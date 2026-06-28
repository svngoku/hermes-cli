# PRODUCT_SENSE — Why Hermes CLI Exists

> Purpose, beliefs, and non-goals. Read before adding features so scope stays honest.

---

## 1. Product Purpose

Hermes CLI makes launching and monitoring LLM inference servers (SGLang, vLLM)
on GPU infrastructure a single, legible command. It removes the friction of
hand-managing Python virtualenvs, engine flags, and health checks.

```mermaid
journey
    title Operator journey: from bare GPU box to serving model
    section Diagnose
      Run hermes doctor: 4: Operator
      See GPU/CUDA/Python status: 5: Operator
    section Install
      hermes install --install vllm: 4: Operator
      uv sets up venv + engine: 5: CLI
    section Serve
      hermes serve --engine vllm --model ...: 5: Operator
      Health check passes: 5: CLI
    section Verify
      hermes verify --port 8000: 5: Operator
```

## 2. Core Beliefs

1. **One command, predictable outcome.** The operator should not need to
   remember engine-specific incantations.
2. **The environment is the hard part.** Most failures are GPU/CUDA/venv issues;
   `doctor` and clear crash reasons matter more than fancy features.
3. **Engine-agnostic by design.** SGLang and vLLM are interchangeable behind one
   interface; the CLI should never leak engine specifics into shared flows.
4. **Honest output.** Show the real crash reason immediately; never silently
   redirect failures into a log the operator has to hunt for.
5. **Minimal dependencies.** A static Go binary that shells out to `uv` beats a
   heavyweight framework.

## 3. Non-Goals

- **Not** an inference engine — it launches sglang/vllm, it does not serve tokens.
- **Not** a model registry or downloader beyond what the engines already do.
- **Not** a cluster scheduler — single-host orchestration only.
- **Not** a GUI — terminal-first, scriptable.
- **Not** tied to one cloud — assumes a local NVIDIA GPU host.

## 4. Primary Users

- ML engineers spinning up inference on a fresh GPU instance.
- Researchers comparing SGLang vs vLLM on the same model.
- Ops automating model serving in scripts/CI.
