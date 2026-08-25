# syntax=docker/dockerfile:1.7

ARG CUDA_DEVEL_IMAGE=nvidia/cuda:13.0.2-devel-ubuntu22.04
ARG CUDA_RUNTIME_IMAGE=nvidia/cuda:13.0.2-runtime-ubuntu22.04

FROM ${CUDA_DEVEL_IMAGE} AS builder

ARG UV_VERSION=0.8.22
ARG VLLM_VERSION=0.27.1
ARG SGLANG_VERSION=0.5.18
ARG LLAMA_CPP_REF=f280b26983ad0fdb705a0d9ebf0503e76f2899b0
ARG LLAMA_CUDA_ARCHITECTURES="80;86;89;90;100;103;120"
ARG DEBIAN_FRONTEND=noninteractive

ENV PIP_DISABLE_PIP_VERSION_CHECK=1 \
    UV_LINK_MODE=copy

RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential ca-certificates cmake git ninja-build \
      python3 python3-dev python3-pip python3-venv \
    && rm -rf /var/lib/apt/lists/* \
    && python3 -m pip install --no-cache-dir "uv==${UV_VERSION}"

RUN --mount=type=cache,target=/root/.cache/uv \
    uv venv --python /usr/bin/python3 /opt/vllm \
    && arch="$(uname -m)" \
    && uv pip install --python /opt/vllm/bin/python \
      "https://github.com/vllm-project/vllm/releases/download/v${VLLM_VERSION}/vllm-${VLLM_VERSION}%2Bcu129-cp38-abi3-manylinux_2_28_${arch}.whl" \
      --extra-index-url https://download.pytorch.org/whl/cu129

RUN --mount=type=cache,target=/root/.cache/uv \
    uv venv --python /usr/bin/python3 /opt/sglang \
    && uv pip install --python /opt/sglang/bin/python \
      --prerelease=allow \
      --index-strategy unsafe-best-match \
      --extra-index-url https://docs.sglang.ai/whl/cu130/ \
      "sglang==${SGLANG_VERSION}"

WORKDIR /src
RUN git init llama.cpp \
    && cd llama.cpp \
    && git remote add origin https://github.com/ggml-org/llama.cpp.git \
    && git fetch --depth 1 origin "${LLAMA_CPP_REF}" \
    && git checkout --detach FETCH_HEAD \
    && cmake -S . -B build -G Ninja \
      -DCMAKE_BUILD_TYPE=Release \
      -DCMAKE_CUDA_ARCHITECTURES="${LLAMA_CUDA_ARCHITECTURES}" \
      -DGGML_BACKEND_DL=ON \
      -DGGML_CUDA=ON \
      -DGGML_NATIVE=OFF \
      -DLLAMA_BUILD_EXAMPLES=OFF \
      -DLLAMA_BUILD_SERVER=ON \
      -DLLAMA_BUILD_TESTS=OFF \
      -DLLAMA_CURL=OFF \
    && cmake --build build --target llama-server --parallel \
    && install -Dm755 build/bin/llama-server /opt/llama/bin/llama-server \
    && mkdir -p /opt/llama/lib \
    && find build -name '*.so*' -exec cp -P {} /opt/llama/lib/ \;

FROM ${CUDA_RUNTIME_IMAGE} AS runtime

LABEL org.opencontainers.image.source="https://github.com/svngoku/hermes-cli" \
      org.opencontainers.image.description="CUDA 13 inference runtimes for vLLM, SGLang, and llama.cpp"

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      bash ca-certificates curl libgomp1 libnuma1 libopenmpi3 python3 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 inference \
    && install -d -o inference -g inference \
      /models/cache /models/gguf /models/hf /models/vllm

COPY --from=builder /opt/vllm /opt/vllm
COPY --from=builder /opt/sglang /opt/sglang
COPY --from=builder /opt/llama/bin/llama-server /usr/local/bin/llama-server
COPY --from=builder /opt/llama/lib/ /usr/local/bin/
COPY --chown=inference:inference entrypoint.sh /usr/local/bin/inference-entrypoint
RUN echo /usr/local/bin >/etc/ld.so.conf.d/llama.conf && ldconfig

ENV ENGINE=vllm \
    HF_HOME=/models/hf \
    HOME=/home/inference \
    LLAMA_CACHE=/models/gguf \
    NVIDIA_DRIVER_CAPABILITIES=compute,utility \
    NVIDIA_VISIBLE_DEVICES=all \
    PORT=8000 \
    PYTHONDONTWRITEBYTECODE=1 \
    VLLM_CACHE_ROOT=/models/vllm \
    XDG_CACHE_HOME=/models/cache

USER inference
EXPOSE 8000
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=30s --timeout=5s --start-period=5m --retries=3 \
  CMD curl --fail --silent --output /dev/null "http://127.0.0.1:${PORT:-8000}/health"

ENTRYPOINT ["/usr/local/bin/inference-entrypoint"]
