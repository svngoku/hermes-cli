#!/usr/bin/env bash
set -euo pipefail

: "${HERMES_VERSION:=0.1.0}"
: "${LOG_FILE:=./hermes.log}"
: "${LOG_LEVEL:=INFO}"          # DEBUG|INFO|WARN|ERROR
: "${FORCE_COLOR:=0}"           # 1 to force
: "${NO_COLOR_OVERRIDE:=0}"     # 1 to disable
: "${SPINNER_DELAY:=0.12}"

_is_tty() { [[ -t 1 ]]; }
_has() { command -v "$1" >/dev/null 2>&1; }
_ts() { date +"%Y-%m-%d %H:%M:%S"; }

_should_color() {
  if [[ "${FORCE_COLOR}" == "1" ]]; then return 0; fi
  if [[ -n "${NO_COLOR:-}" ]]; then return 1; fi
  if [[ "${NO_COLOR_OVERRIDE}" == "1" ]]; then return 1; fi
  _is_tty
}

_init_colors() {
  if _should_color && _has tput; then
    RESET="$(tput sgr0 || true)"
    BOLD="$(tput bold || true)"
    DIM="$(tput dim || true)"

    RED="$(tput setaf 1 || true)"
    GREEN="$(tput setaf 2 || true)"
    YELLOW="$(tput setaf 3 || true)"
    BLUE="$(tput setaf 4 || true)"
    MAGENTA="$(tput setaf 5 || true)"
    CYAN="$(tput setaf 6 || true)"
    GRAY="$(tput setaf 7 || true)"
  else
    RESET=""; BOLD=""; DIM=""
    RED=""; GREEN=""; YELLOW=""; BLUE=""; MAGENTA=""; CYAN=""; GRAY=""
  fi
}

_level_rank() {
  case "$1" in
    DEBUG) echo 10 ;;
    INFO)  echo 20 ;;
    WARN)  echo 30 ;;
    ERROR) echo 40 ;;
    *)     echo 20 ;;
  esac
}

_should_log() {
  local want="$(_level_rank "${LOG_LEVEL}")"
  local got="$(_level_rank "$1")"
  [[ "$got" -ge "$want" ]]
}

_log_file_line() {
  local level="$1"; shift
  echo "$(_ts) [$level] $*" >> "$LOG_FILE" 2>/dev/null || true
}

_log_console() {
  local level="$1"; shift
  local msg="$*"

  _log_file_line "$level" "$msg"
  _should_log "$level" || return 0

  case "$level" in
    DEBUG) printf "%s%s[%s]%s %s\n" "${DIM}" "${GRAY}" "$level" "${RESET}" "$msg" ;;
    INFO)  printf "%s%s[%s]%s %s\n" "${BLUE}" "${BOLD}" "$level" "${RESET}" "$msg" ;;
    WARN)  printf "%s%s[%s]%s %s\n" "${YELLOW}" "${BOLD}" "$level" "${RESET}" "$msg" ;;
    ERROR) printf "%s%s[%s]%s %s\n" "${RED}" "${BOLD}" "$level" "${RESET}" "$msg" ;;
    *)     printf "[%s] %s\n" "$level" "$msg" ;;
  esac
}

log_debug() { _log_console DEBUG "$*"; }
log_info()  { _log_console INFO  "$*"; }
log_warn()  { _log_console WARN  "$*"; }
log_error() { _log_console ERROR "$*"; }

die() { log_error "$*"; exit 1; }

cols() {
  if _has tput && _is_tty; then tput cols 2>/dev/null || echo 80
  else echo 80
  fi
}

hr() {
  local c; c="$(cols)"
  printf "%s\n" "$(printf '%*s' "$c" '' | tr ' ' '-')"
}

center() {
  local text="$*"
  local c; c="$(cols)"
  local pad=$(( (c - ${#text}) / 2 ))
  (( pad < 0 )) && pad=0
  printf "%*s%s\n" "$pad" "" "$text"
}

banner() {
  clear 2>/dev/null || true
  
  # Try to find banner.txt from SCRIPT_DIR or relative to this lib
  local banner_file=""
  if [[ -n "${SCRIPT_DIR:-}" ]]; then
    banner_file="${SCRIPT_DIR}/assets/banner.txt"
  fi
  
  if [[ -z "$banner_file" ]] || [[ ! -f "$banner_file" ]]; then
    # Fallback if not found
    hr
    printf "%s" "${MAGENTA}${BOLD}"
    center "HERMES"
    printf "%s" "${RESET}"
  else
    # Display the fancy ASCII art banner
    printf "%s%s" "${MAGENTA}${BOLD}"
    cat "$banner_file"
    printf "%s" "${RESET}"
  fi
  
  center "RunPod LLM Serving TUI  •  v${HERMES_VERSION}"
  hr
}

step() {
  # step "Title"
  printf "%s%s▶%s %s%s%s\n" "${CYAN}" "${BOLD}" "${RESET}" "${BOLD}" "$*" "${RESET}"
}

ok()    { printf "%s%s[ OK ]%s %s\n"    "${GREEN}" "${BOLD}" "${RESET}" "$*"; _log_file_line INFO  "[OK] $*"; }
warn()  { printf "%s%s[WARN]%s %s\n"    "${YELLOW}" "${BOLD}" "${RESET}" "$*"; _log_file_line WARN  "[WARN] $*"; }
fail()  { printf "%s%s[FAIL]%s %s\n"    "${RED}" "${BOLD}" "${RESET}" "$*"; _log_file_line ERROR "[FAIL] $*"; }

# --- spinner (PID-based) ---
_spinner_pid=""
_spinner_msg=""

_spinner_start() {
  _spinner_msg="$1"
  _log_file_line INFO "[SPIN] ${_spinner_msg}"

  if _has tput && _is_tty; then tput civis 2>/dev/null || true; fi

  (
    local sp='|/-\'
    local i=0
    while :; do
      printf "\r%s%s⏳%s %s %s" "${MAGENTA}${BOLD}" "" "${RESET}" "${_spinner_msg}" "${sp:i++%${#sp}:1}"
      sleep "${SPINNER_DELAY}"
    done
  ) &
  _spinner_pid="$!"
  disown || true
}

_spinner_stop() {
  local status="${1:-0}"
  if [[ -n "${_spinner_pid}" ]]; then
    kill "${_spinner_pid}" >/dev/null 2>&1 || true
    wait "${_spinner_pid}" >/dev/null 2>&1 || true
    _spinner_pid=""
  fi

  # clear spinner line + print result
  printf "\r\033[2K" 2>/dev/null || true

  if _has tput && _is_tty; then tput cnorm 2>/dev/null || true; fi

  if [[ "$status" -eq 0 ]]; then
    ok "${_spinner_msg}"
  else
    fail "${_spinner_msg}"
  fi
}

run_task() {
  # run_task "Message" "bash -lc '...'"
  local msg="$1"; shift
  local cmd="$*"
  _spinner_start "$msg"
  set +e
  bash -lc "$cmd" >>"$LOG_FILE" 2>&1
  local rc=$?
  set -e
  _spinner_stop "$rc"
  return "$rc"
}

run_stream() {
  # Use when you WANT to see live output, also logs it.
  local msg="$1"; shift
  local cmd="$*"
  log_info "Running (stream): $msg :: $cmd"
  step "$msg"
  bash -lc "$cmd" 2>&1 | tee -a "$LOG_FILE"
}

prompt() {
  local q="$1"
  local d="${2:-}"
  local ans=""
  if [[ -n "$d" ]]; then
    printf "%s%s?%s %s [%s]: " "${CYAN}" "${BOLD}" "${RESET}" "$q" "$d"
  else
    printf "%s%s?%s %s: " "${CYAN}" "${BOLD}" "${RESET}" "$q"
  fi
  read -r ans
  [[ -z "$ans" && -n "$d" ]] && ans="$d"
  echo "$ans"
}

tui_init() {
  mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true
  touch "$LOG_FILE" 2>/dev/null || true
  _init_colors
}
