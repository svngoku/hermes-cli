#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export SCRIPT_DIR
# shellcheck source=lib/hermes_tui.sh
source "${SCRIPT_DIR}/lib/hermes_tui.sh"

ENGINE=""
MODEL=""
HOST="0.0.0.0"
PORT="30000"
TP="4"
INSTALL="both"     # sglang|vllm|both|none
VERIFY="1"         # 1|0
READINESS="1"      # 1|0 - check server readiness
DAEMON="0"
VENV_DIR=".venv"
STUDIO="1"         # 1|0 - launch vllm-studio controller
STUDIO_PORT="8000"
FRONTEND="0"       # 1|0 - launch vllm-studio frontend
EXTRA_ARGS=""      # Additional engine-specific args

usage() {
  banner
  cat <<EOF

Usage:
  ./hermes.sh --engine {sglang|vllm} --model <hf_repo_or_path> [options]

Options:
  --engine        sglang | vllm
  --model         HF repo id or local path
  --tp            Tensor parallel size (default: 4)
  --host          Bind host (default: 0.0.0.0)
  --port          Bind port (default: 30000)
  --install       sglang|vllm|both|none (default: both)
  --extra-args    Additional engine flags (e.g., '--enable-reasoning --reasoning-parser deepseek_r1')
  --no-verify     Skip request verification
  --no-readiness  Skip server readiness check
  --daemon        Keep server running in background
  --studio        Launch vllm-studio controller (default: 1)
  --studio-port   vllm-studio controller port (default: 8000)
  --frontend      Launch vllm-studio frontend (default: 0)
  --log-file      Log file path (default: ./hermes.log)
  --debug         Verbose logging
  --no-color      Disable colors
  --force-color   Force colors even if NO_COLOR=1
EOF
}

tui_init

while [[ $# -gt 0 ]]; do
  case "$1" in
    --engine) ENGINE="${2:-}"; shift 2 ;;
    --model)  MODEL="${2:-}"; shift 2 ;;
    --tp)     TP="${2:-}"; shift 2 ;;
    --host)   HOST="${2:-}"; shift 2 ;;
    --port)   PORT="${2:-}"; shift 2 ;;
    --install) INSTALL="${2:-}"; shift 2 ;;
    --extra-args) EXTRA_ARGS="${2:-}"; shift 2 ;;
    --no-verify) VERIFY="0"; shift ;;
    --no-readiness) READINESS="0"; shift ;;
    --daemon) DAEMON="1"; shift ;;
    --studio) STUDIO="${2:-1}"; shift 2 ;;
    --studio-port) STUDIO_PORT="${2:-}"; shift 2 ;;
    --frontend) FRONTEND="1"; shift ;;
    --log-file) LOG_FILE="${2:-}"; shift 2 ;;
    --debug) LOG_LEVEL="DEBUG"; shift ;;
    --no-color) NO_COLOR_OVERRIDE="1"; shift ;;
    --force-color) FORCE_COLOR="1"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown arg: $1"; usage; exit 2 ;;
  esac
done

trap 'die "Unexpected error on line $LINENO (see $LOG_FILE)"' ERR

banner
log_info "Log file: $LOG_FILE"
log_info "SGLang repo: https://github.com/sgl-project/sglang"
log_info "vLLM repo  : https://github.com/vllm-project/vllm"
hr

step "Doctor checks"
if command -v nvidia-smi >/dev/null 2>&1; then
  run_stream "GPU visibility (nvidia-smi)" "nvidia-smi"
  ok "GPU detected"
else
  die "nvidia-smi not found; GPU not visible."
fi

if command -v nvcc >/dev/null 2>&1; then
  run_stream "CUDA compiler (nvcc --version)" "nvcc --version | sed -n '1,6p'"
  ok "CUDA toolchain detected"
else
  warn "nvcc not found (runtime-only image is fine)."
fi

step "Ensure uv"
if command -v uv >/dev/null 2>&1; then
  ok "uv present: $(uv --version || true)"
else
  run_task "Installing uv" "curl -LsSf https://astral.sh/uv/install.sh | sh"
  export PATH="$HOME/.local/bin:$PATH"
  command -v uv >/dev/null 2>&1 || die "uv install failed."
fi

step "Python env + engine install"
run_task "Creating venv ($VENV_DIR)" "rm -rf \"$VENV_DIR\" && uv venv \"$VENV_DIR\""

case "$INSTALL" in
  none) warn "Skipping engine install (--install none)." ;;
  sglang) run_task "Installing sglang (uv pip)" "uv pip install -U 'sglang>=0.4'" ;;
  vllm)   run_task "Installing vllm (uv pip)"   "uv pip install -U 'vllm>=0.6'" ;;
  both)   run_task "Installing sglang+vllm (uv pip)" "uv pip install -U 'sglang>=0.4' 'vllm>=0.6'" ;;
  *) die "Invalid --install: $INSTALL" ;;
esac

if [[ "$STUDIO" == "1" ]]; then
  log_info "vllm-studio: install from source or use --studio 0 to disable"
  warn "Skipping vllm-studio (install from repo: pip install -e git+https://github.com/0xSero/vllm-studio.git#egg=vllm-studio)"
fi

if [[ -z "$ENGINE" ]]; then ENGINE="$(prompt "Choose engine (sglang/vllm)" "sglang")"; fi
if [[ -z "$MODEL" ]]; then MODEL="$(prompt "Model (HF repo or local path)" "IQuestLab/IQuest-Coder-V1-40B-Loop-Instruct")"; fi

step "Serve"
if [[ -n "${HUGGING_FACE_HUB_TOKEN:-}" ]]; then
  run_task "Hugging Face login (token)" "uv run huggingface-cli login --token \"$HUGGING_FACE_HUB_TOKEN\" --add-to-git-credential || true"
fi

export CUDA_VISIBLE_DEVICES="${CUDA_VISIBLE_DEVICES:-0,1,2,3}"
# Disable hf_transfer if not available (optional optimization, not required)
export HF_HUB_ENABLE_HF_TRANSFER=0

log_info "Serving: engine=$ENGINE model=$MODEL tp=$TP host=$HOST port=$PORT"

step "Starting $ENGINE server (output below)..."
hr

if [[ "$ENGINE" == "sglang" ]]; then
  bash -lc "uv run python -m sglang.launch_server \
    --model-path \"$MODEL\" \
    --trust-remote-code \
    --tp-size \"$TP\" \
    --host \"$HOST\" \
    --port \"$PORT\"" 2>&1 | tee -a "$LOG_FILE" &
elif [[ "$ENGINE" == "vllm" ]]; then
  # Use system vllm if --install none, otherwise use uv venv
  if [[ "$INSTALL" == "none" ]]; then
    cmd="vllm serve"
  else
    cmd="uv run vllm serve"
  fi
  bash -lc "$cmd \"$MODEL\" \
    --host \"$HOST\" \
    --port \"$PORT\" \
    --tensor-parallel-size \"$TP\" \
    --trust-remote-code \
    $EXTRA_ARGS" 2>&1 | tee -a "$LOG_FILE" &
else
  die "Invalid engine: $ENGINE"
fi

SERVER_PID="$!"
hr
ok "Server started (pid=$SERVER_PID, watching output above)"
sleep 3  # Give server time to initialize

# vllm-studio controller (manual setup required)
if [[ "$STUDIO" == "1" ]]; then
  step "vLLM-Studio (manual setup required)"
  log_info "To launch vllm-studio controller:"
  log_info "  cd vllm-studio && pip install -e . && vllm-studio --port $STUDIO_PORT"
  if [[ "$FRONTEND" == "1" ]]; then
    log_info "To launch frontend:"
    log_info "  cd vllm-studio/frontend && npm install && npm run dev"
  fi
  ok "vLLM-Studio instructions provided"
fi

BASE="http://127.0.0.1:${PORT}"

if [[ "$READINESS" == "1" ]]; then
  step "Readiness"
  
  # Brief wait for initialization
  sleep 2
  
  # Check if server process is still alive
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    fail "Server process died (pid=$SERVER_PID)"
    log_error "Server crash detected. Last 20 lines of log:"
    log_error ""
    tail -20 "$LOG_FILE" | sed 's/^/  /'
    log_error ""
    die "Server failed to start - check log above"
  fi
  
  run_task "Waiting for server readiness" "for i in \$(seq 1 180); do \
    if curl -sf \"${BASE}/v1/models\" >/dev/null 2>&1; then exit 0; fi; \
    if curl -sf \"${BASE}/health\" >/dev/null 2>&1; then exit 0; fi; \
    sleep 2; \
  done; echo ''; echo 'Timeout waiting for server at \${BASE}'; echo ''; grep -i 'error\|exception\|valueerror\|runtimeerror' \"${LOG_FILE}\" | tail -5; exit 1"
else
  log_info "Readiness check skipped (--no-readiness)"
fi

if [[ "$VERIFY" == "1" ]]; then
  step "Verify requests"
  run_task "GET /v1/models" "curl -sf \"${BASE}/v1/models\" | head -c 600"
  run_task "POST /v1/chat/completions" "curl -sf \"${BASE}/v1/chat/completions\" \
    -H \"Content-Type: application/json\" \
    -H \"Authorization: Bearer DUMMY\" \
    -d '{\"model\":\"default\",\"messages\":[{\"role\":\"user\",\"content\":\"Return OK\"}],\"max_tokens\":8,\"temperature\":0}' | head -c 1200"
else
  warn "Verification skipped."
fi

hr
ok "Hermes is operational: ${BASE}"
log_info "Tail logs: tail -f $LOG_FILE"
if [[ "$STUDIO" == "1" ]]; then
  log_info "vLLM-Studio: http://${HOST}:${STUDIO_PORT}"
fi
hr

if [[ "$DAEMON" == "1" ]]; then
  ok "Daemon mode: leaving servers in background."
  exit 0
fi

log_info "Foreground mode: Ctrl+C to stop."
trap 'kill $SERVER_PID 2>/dev/null; exit 0' SIGINT
wait "$SERVER_PID"
