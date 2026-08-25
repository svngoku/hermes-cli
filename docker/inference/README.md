# CUDA 13 Inference Image

One CUDA 13.0.2 image contains isolated vLLM, SGLang, and llama.cpp runtimes.
Each container launches exactly one engine; do not run multiple servers against
the same GPUs from one container.

## Host

- Linux NVIDIA host with Docker 20.10+ and NVIDIA Container Toolkit.
- NVIDIA driver **580.95.05 or newer** for CUDA 13.0 Update 2. Forward
  compatibility on older data-center drivers has feature and PTX limits.
- A GPU supported by the selected engine. The default llama.cpp build covers
  CUDA architectures 80, 86, 89, 90, 100, 103, and 120.
- At least 16–32 GiB of shared memory for tensor-parallel workloads.

## Build

From the repository root:

```bash
docker buildx build --load \
  -t hermes-inference:cuda13 \
  docker/inference
```

The defaults pin vLLM, SGLang, and llama.cpp versions. vLLM uses its official
cu129 wheel on the CUDA 13 base; SGLang uses its cu130 wheel index. Override
versions when upgrading:

```bash
docker buildx build --load -t hermes-inference:cuda13 \
  --build-arg VLLM_VERSION='0.27.1' \
  --build-arg SGLANG_VERSION='0.5.18' \
  --build-arg LLAMA_CPP_REF='f280b26983ad0fdb705a0d9ebf0503e76f2899b0' \
  --build-arg LLAMA_CUDA_ARCHITECTURES='80;90;100' \
  docker/inference
```

For reproducible production builds, also pass digest-pinned
`CUDA_DEVEL_IMAGE` and `CUDA_RUNTIME_IMAGE` values and use a hash-locked Python
resolution. Expect a large image and a long first build; this artifact favors a
lab or single GPU node over fleet pull efficiency.

## Run

The examples bind the API to localhost, scope the container to GPU 0, and use a
private shared-memory segment. Use an authenticated reverse proxy or the
selected engine's API-key option before exposing the API on a network.

vLLM:

```bash
docker run --rm --gpus '"device=0"' --shm-size=32g \
  --security-opt=no-new-privileges -p 127.0.0.1:8000:8000 \
  -e ENGINE=vllm -e MODEL=Qwen/Qwen3-32B -e HF_TOKEN \
  -v hf-cache:/models/hf -v vllm-cache:/models/vllm \
  hermes-inference:cuda13 --tensor-parallel-size 1
```

SGLang:

```bash
docker run --rm --gpus '"device=0"' --shm-size=32g \
  --security-opt=no-new-privileges -p 127.0.0.1:8000:8000 \
  -e ENGINE=sglang -e MODEL=Qwen/Qwen3-32B -e HF_TOKEN \
  -v hf-cache:/models/hf \
  hermes-inference:cuda13 --tp-size 1
```

llama.cpp:

```bash
docker run --rm --gpus '"device=0"' --shm-size=16g \
  --security-opt=no-new-privileges -p 127.0.0.1:8000:8000 \
  -e ENGINE=llamacpp \
  -e MODEL='/models/gguf/qwen3-32b.Q4_K_M.gguf' \
  -e N_GPU_LAYERS=99 -e CTX=8192 \
  -v "$PWD/models:/models/gguf:ro" \
  hermes-inference:cuda13
```

`MODEL` is required. `ENGINE` defaults to `vllm`. `HOST` and `PORT` default to
`0.0.0.0` and `8000`. Arguments after the image name are passed unchanged to
the selected server. Reserve `--ipc=host` and `--gpus all` for trusted,
dedicated multi-GPU hosts.

Named cache volumes inherit the image's non-root ownership. Bind-mounted model
files may be read-only, but `/models/cache`, `/models/hf`, and `/models/vllm`
must be writable by container UID 10001.

## Scale

Scale containers, not processes inside this image. Safetensors/FP8 models belong
on vLLM or SGLang replicas; GGUF models belong on llama.cpp replicas. Split this
into thin engine-specific targets only when image pull time or attack surface is
measurably costly.

## Upstream References

- [vLLM GPU installation](https://docs.vllm.ai/en/stable/getting_started/installation/gpu/)
- [vLLM Docker deployment](https://docs.vllm.ai/en/stable/deployment/docker/)
- [SGLang installation](https://docs.sglang.ai/get_started/install.html)
- [llama.cpp CUDA Dockerfile](https://github.com/ggml-org/llama.cpp/blob/master/.devops/cuda.Dockerfile)
- [CUDA 13.0 Update 2 release notes](https://docs.nvidia.com/cuda/archive/13.0.2/cuda-toolkit-release-notes/index.html)
