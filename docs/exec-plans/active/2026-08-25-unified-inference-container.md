---
title: "Unified CUDA inference container"
area: "docker, docs"
status: in_progress
risk: medium
created: 2026-08-25
updated: 2026-08-25
author: agent
issue: hermes-cli-qcj
---

# Plan: Unified CUDA Inference Container

## Intent

Offer a repeatable CUDA 13.0.2 deployment path that avoids host installation and
keeps vLLM, SGLang, and llama.cpp dependencies isolated. One image may contain
all three runtimes, but each container launches exactly one engine.

## Acceptance Criteria

- [ ] A multi-stage Dockerfile builds isolated vLLM/SGLang environments and a
      CUDA-enabled `llama-server`.
- [x] The entrypoint requires a model, dispatches one engine, and forwards extra
      arguments without shell evaluation.
- [x] The runtime uses a non-root user, persistent `/models` caches, and a basic
      process-readiness health check.
- [x] Documentation includes build, host prerequisites, and per-engine run examples.
- [x] A deterministic shell check exercises dispatcher validation and argument forwarding.
- [x] Quality gate: `just check` passes.

## Approach

```mermaid
flowchart TD
    build[CUDA 13.0.2 builder] --> vllm[/opt/vllm venv]
    build --> sglang[/opt/sglang venv]
    build --> llama[static llama-server]
    vllm --> runtime[CUDA 13.0.2 runtime]
    sglang --> runtime
    llama --> runtime
    runtime --> entry[dispatcher entrypoint]
    entry --> one{one ENGINE}
    one --> serve[OpenAI-compatible server on :8000]
```

1. Add `docker/inference/` with the image, dispatcher, focused tests, and usage notes.
2. Add the shell check to `Justfile` so the existing quality gate catches regressions.
3. Link the product spec and plan from their indexes and add one README pointer.
4. Validate shell syntax/tests, Dockerfile parsing when Docker is available, and
   the repository `just check` gate.

## Non-Goals

- Changing current Go engine install or launch code while its streamed-install
  plan is already in progress.
- Running all engines concurrently, adding a supervisor, or routing requests.
- Adding Compose, Kubernetes, or registry-publish workflows before the image is
  proven on target GPU hardware.
- Optimizing into engine-specific thin tags.

## Risks & Mitigations

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Python wheels do not support CUDA 13.0.2 together | Medium | Keep separate venvs and make package versions build arguments. |
| Runtime misses a builder library | Medium | Install explicit runtime libraries; build llama.cpp without shared project libraries. |
| Image is too large for fleet use | High | Document lab/single-node scope; split targets only when pull cost matters. |
| GPU image cannot be built in normal CI | High | Keep a deterministic dispatcher test and document required GPU smoke tests. |

## Progress Log

| Date | Update |
|------|--------|
| 2026-08-25 | Created; issue hermes-cli-qcj. |
| 2026-08-25 | Added the image, dispatcher, tests, and usage docs; earlier static/local checks passed. Full image build and further local testing were intentionally deferred per operator direction. |
| 2026-08-25 | Static research/review replaced GPU auto-detection with pinned CUDA-compatible installs, pinned llama.cpp, copied its shared libraries, isolated test environments, and tightened run guidance. |
