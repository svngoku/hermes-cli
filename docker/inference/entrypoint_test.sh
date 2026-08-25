#!/usr/bin/env bash
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

expect_failure() {
  local expected=$1
  shift
  if output=$("$@" 2>&1); then
    echo "expected command to fail" >&2
    exit 1
  fi
  [[ "$output" == *"$expected"* ]] || {
    printf 'expected %q in %q\n' "$expected" "$output" >&2
    exit 1
  }
}

expect_failure "MODEL is required" env -i PATH="$PATH" \
  ENGINE=vllm "$here/entrypoint.sh"
expect_failure "ENGINE must be vllm|sglang|llamacpp" \
  env -i PATH="$PATH" ENGINE=unknown MODEL=test "$here/entrypoint.sh"

cat >"$tmp/llama-server" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$@" >"$CAPTURE"
FAKE
chmod +x "$tmp/llama-server"

env -i CAPTURE="$tmp/args" PATH="$tmp:$PATH" ENGINE=llamacpp \
  MODEL="/models/model with spaces.gguf" "$here/entrypoint.sh" --verbose

cat >"$tmp/want" <<'EXPECTED'
-m
/models/model with spaces.gguf
--host
0.0.0.0
--port
8000
-ngl
99
-c
8192
--verbose
EXPECTED

diff -u "$tmp/want" "$tmp/args"
echo "inference entrypoint checks passed"
