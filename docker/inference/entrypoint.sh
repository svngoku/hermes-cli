#!/usr/bin/env bash
set -euo pipefail

engine=${ENGINE:-vllm}
model=${MODEL:-}
host=${HOST:-0.0.0.0}
port=${PORT:-8000}

if [[ -z "$model" ]]; then
  echo "MODEL is required" >&2
  exit 64
fi

case "$engine" in
  vllm)
    exec /opt/vllm/bin/vllm serve "$model" \
      --host "$host" --port "$port" "$@"
    ;;
  sglang)
    exec /opt/sglang/bin/python -m sglang.launch_server \
      --model-path "$model" --host "$host" --port "$port" "$@"
    ;;
  llamacpp)
    exec llama-server -m "$model" \
      --host "$host" --port "$port" \
      -ngl "${N_GPU_LAYERS:-99}" -c "${CTX:-8192}" "$@"
    ;;
  *)
    echo "ENGINE must be vllm|sglang|llamacpp" >&2
    exit 64
    ;;
esac
