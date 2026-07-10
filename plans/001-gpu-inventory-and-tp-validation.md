# Plan 001: GPU inventory + TP validation before serve

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 970d1db..HEAD -- internal/commands/doctor.go internal/commands/serve.go internal/commands/run.go internal/config/config.go internal/engine/`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: bug / correctness
- **Planned at**: commit `970d1db`, 2026-07-10

## Why this matters

Hermes defaults `--tp` (tensor parallel size) to **4** and always forwards it to sglang/vllm. On 1–2 GPU hosts (common cloud shapes), the engine fails late with OOM/NCCL errors. Doctor claims to report GPU count but uses a broken nvidia-smi query (`--query-gpu=count`), so operators cannot trust the check. Fixing inventory + validating TP before launch turns multi-GPU failures into clear preflight errors.

## Current state

### Broken GPU count (`internal/commands/doctor.go`)

```go
// checkGPUs around lines 209–234
result := execx.RunWithTimeout(ctx.Ctx, doctorProbeTimeout, "nvidia-smi",
    "--query-gpu=count", "--format=csv,noheader")
// ...
lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
count := len(lines)
```

`count` is not a valid per-GPU nvidia-smi field in the usual query API. Correct approach: query a real per-GPU field (e.g. `index` or `name`) and count non-empty lines, or parse `nvidia-smi -L`.

### Default TP = 4 with no validation

- `internal/config/config.go` `DefaultServeConfig()` sets `TP: 4`
- `internal/commands/serve.go` flag: `tp := fs.Int("tp", 4, "Tensor parallel size")`
- `internal/commands/run.go` flag: same default 4
- Engines pass through blindly:
  - vLLM: `"--tensor-parallel-size", strconv.Itoa(cfg.TP)` (`internal/engine/vllm.go`)
  - SGLang: `"--tp-size", strconv.Itoa(cfg.TP)` (`internal/engine/sglang.go`)

### Conventions to match

- Pure helpers with table-driven tests (see `internal/engine/args_test.go`, `internal/config/config_test.go`).
- Errors wrapped with `fmt.Errorf("...: %w", err)` or clear `fmt.Errorf` messages for user-facing validation.
- Commands parse flags then validate early and return error (DESIGN.md command pattern).
- GP-1: new package must not create illegal import edges. Prefer a leaf package under `internal/` that only imports stdlib + `execx` if it shells out. `config` must not import other internal packages.

### Product constraints

- Single-host only (PRODUCT_SENSE non-goal: not a cluster scheduler).
- Honest output: fail early with a readable reason (PRODUCT_SENSE core belief #4).

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Tests | `go test -race ./...` | all packages pass |
| Vet | `go vet ./...` | exit 0 |
| Full gate | `just check` | "All checks passed" |
| Focused tests | `go test -race ./internal/gpu/ ./internal/config/ ./internal/commands/` | pass (commands may still be thin until plan 005) |

## Scope

**In scope** (the only files you should create/modify):

- `internal/gpu/gpu.go` (create) — pure parse + optional live probe
- `internal/gpu/gpu_test.go` (create)
- `internal/config/config.go` — safer default TP; optional pure `ValidateTP`
- `internal/config/config_test.go` — update default TP expectation; test ValidateTP if placed here
- `internal/commands/doctor.go` — use `gpu` package for count
- `internal/commands/serve.go` — validate TP vs GPU inventory before start
- `internal/commands/run.go` — same validation
- `plans/README.md` — status row only

**Out of scope**:

- `--cuda-devices` / env injection (plan 003)
- Port checks (plan 002)
- Daemon readiness (plan 004)
- Pipeline parallel, dtype, mem-fraction, auto-TP
- Changing engine ServeCommand flag names
- Bash `hermes.sh` parity
- Wiring the unused huh form in `internal/ui/tui/form.go`

## Git workflow

- Branch: `advisor/001-gpu-inventory-tp-validation` (or continue on current work branch if operator directs)
- Commit style from recent history: `fix: ...` / `feat: ...` (example: `fix: process lifecycle, exit codes, fd leak, arg parsing + seed tests`)
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Create `internal/gpu` with pure parsing + probe

Create package `internal/gpu` with:

1. **Pure** `CountFromQueryOutput(stdout string) int`  
   - Input: stdout of `nvidia-smi --query-gpu=index,name,memory.total --format=csv,noheader`  
   - Count non-empty trimmed lines.  
   - Empty string → 0.

2. **Pure** `CountFromListOutput(stdout string) int` (optional helper) for `nvidia-smi -L` lines matching `GPU \d+:` if you prefer that path — pick **one** primary query and test it thoroughly.

3. **Live** `Count(ctx context.Context) (int, error)` using `execx.RunWithTimeout` (10s timeout, same spirit as `doctorProbeTimeout`):
   - If `nvidia-smi` missing (`execx.CommandExists` false): return `(0, ErrUnavailable)` where `ErrUnavailable` is a package-level sentinel `errors.New("nvidia-smi not available")`.
   - If command fails: return error wrapping stderr.
   - If success: return `CountFromQueryOutput(result.Stdout), nil`.
   - If count is 0 with successful command: return `(0, nil)` (caller decides fail vs warn).

Recommended query (real per-GPU fields):

```text
nvidia-smi --query-gpu=index,name,memory.total --format=csv,noheader
```

**Do not** use `--query-gpu=count`.

Keep GP-1: `internal/gpu` may import `context`, stdlib, and `internal/execx` only.

**Verify**: `go test -race ./internal/gpu/` — write tests in Step 2 first if TDD, or immediately after.

### Step 2: Table-driven tests for pure parsing

In `internal/gpu/gpu_test.go`, cover:

| Case | Input | Want |
|------|-------|------|
| empty | `""` | 0 |
| one GPU | `"0, NVIDIA A100, 40960 MiB\n"` | 1 |
| two GPUs | two CSV lines | 2 |
| trailing newline / blank line | multi-line with blank | ignore blanks |
| whitespace-only line | ignored | |

Do **not** require a real GPU for unit tests. Optionally skip live `Count` test if `nvidia-smi` absent:

```go
if !execx.CommandExists("nvidia-smi") {
    t.Skip("nvidia-smi not available")
}
```

**Verify**: `go test -race ./internal/gpu/ -v` → all pass.

### Step 3: TP validation helper + safer default

In `internal/config/config.go`:

1. Change `DefaultServeConfig().TP` from `4` to **`1`** (safe single-GPU default for production).
2. Add pure validation (either on config package or gpu package — prefer **config** for TP rules so engines stay free of GPU probing):

```go
// ValidateTP checks tensor-parallel size against available GPUs.
// gpuCount < 0 means "unknown" (probe failed); only tp < 1 is rejected then.
// gpuCount >= 0 requires 1 <= tp <= gpuCount.
func ValidateTP(tp, gpuCount int) error {
    if tp < 1 {
        return fmt.Errorf("tensor parallel size must be >= 1, got %d", tp)
    }
    if gpuCount >= 0 && tp > gpuCount {
        return fmt.Errorf("tensor parallel size %d exceeds available GPUs (%d)", tp, gpuCount)
    }
    return nil
}
```

Update `internal/config/config_test.go`:

- `TestDefaultServeConfig` expects `TP == 1`
- Add `TestValidateTP` table cases: tp=0 fail; tp=1 gpu=0 fail; tp=4 gpu=2 fail; tp=2 gpu=2 ok; tp=2 gpu=-1 ok (unknown)

**Verify**: `go test -race ./internal/config/` → pass.

### Step 4: Fix doctor GPU count

In `checkGPUs` (`internal/commands/doctor.go`):

- Replace the broken nvidia-smi invocation with `gpu.Count(ctx.Ctx)` (or run the same query and pure parse).
- On `ErrUnavailable` / missing smi: StatusFail or StatusSkipped consistent with existing nvidia-smi check (prefer **StatusSkipped** or Fail with clear message if you want strictness — match existing style: fail when no GPUs, skip when cannot query).
- Message still like `"%d GPU(s) available"`.

Keep doctorProbeTimeout behavior: `gpu.Count` should use a timeout internally.

**Verify**: `go test -race ./internal/commands/ ./internal/gpu/` (commands may have no tests yet — package must compile).  
`go vet ./...` → exit 0.

### Step 5: Validate on `serve` and `run` before launch

In both `Serve` (after building `cfg`) and `Run` (after building `serveCfg` / before `startEngine`):

```go
gpuCount, err := gpu.Count(ctx.Ctx)
if err != nil {
    // Probe failed: treat as unknown (-1), still reject tp < 1
    gpuCount = -1
    ctx.Logger.Warn("could not query GPU count; skipping TP vs GPU check", "error", err)
}
if err := config.ValidateTP(cfg.TP, gpuCount); err != nil {
    return err
}
```

Also change flag defaults for `--tp` in `serve.go` and `run.go` from `4` to `1` so CLI flags match `DefaultServeConfig`.

User-facing error example:

```text
tensor parallel size 4 exceeds available GPUs (1)
```

Do **not** start the engine process if validation fails.

**Verify**:

```bash
go test -race ./...
go vet ./...
just build   # or: CGO_ENABLED=0 go build -o bin/hermes ./cmd/hermes
```

Manual smoke (if no GPU in CI, skip):  
`./bin/hermes serve --model x --tp 0` → non-zero exit, error about tp >= 1.

### Step 6: Update engine unit tests if they hardcode assumptions

`internal/engine/engine_test.go` already uses explicit `TP: 2` etc. — should still pass. If any test relied on default config TP=4, update it.

**Verify**: `go test -race ./internal/engine/ ./internal/config/ ./internal/gpu/` → pass.

## Test plan

- `internal/gpu/gpu_test.go` — pure parse cases listed above.
- `internal/config/config_test.go` — default TP=1 + ValidateTP table.
- No integration test that launches real engines in this plan.
- Model test style after `internal/engine/args_test.go` (table / simple asserts, no external frameworks).

## Done criteria

- [ ] `go test -race ./...` exits 0
- [ ] `go vet ./...` exits 0
- [ ] `just check` passes (or lint + test + build equivalent)
- [ ] `grep -n 'query-gpu=count' internal/commands/doctor.go` returns nothing
- [ ] Default TP is 1 in `config.DefaultServeConfig` and serve/run flags
- [ ] `config.ValidateTP` (or equivalent) rejects `tp > gpuCount` when count known
- [ ] serve/run call validation before `startEngine` / process start
- [ ] No files outside the in-scope list modified (`git status`)
- [ ] `plans/README.md` status for 001 set to DONE

## STOP conditions

- Code at cited locations no longer matches excerpts (drift).
- Adding `internal/gpu` would violate depguard / GP-1 (imports commands/app/ui).
- nvidia-smi on the executor machine behaves radically differently and pure tests cannot encode output — still implement pure parser against documented CSV samples; do not block on live GPU.
- A step's verification fails twice after a reasonable fix attempt.
- Fix seems to require changing sglang/vllm CLI flags beyond TP.

## Maintenance notes

- Plan 003 will refine validation when `--cuda-devices` limits visible GPUs — leave a clear `ValidateTP` API that accepts an explicit `gpuCount`.
- Reviewers should check: default TP change is intentional product change (safer); document in commit message.
- Follow-up not in this plan: auto-select TP from GPU count.
