# Plan 003: Minimal multi-GPU — `--cuda-devices` flag

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 970d1db..HEAD -- internal/config/config.go internal/commands/serve.go internal/commands/run.go internal/engine/ internal/gpu/`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: plans/001-gpu-inventory-and-tp-validation.md
- **Category**: direction / feature (minimal multi-GPU)
- **Planned at**: commit `970d1db`, 2026-07-10

## Why this matters

Real multi-GPU hosts often pin a subset of GPUs (`CUDA_VISIBLE_DEVICES=0,1`). The legacy bash launcher set a hardcoded `CUDA_VISIBLE_DEVICES=0,1,2,3`; the Go CLI never sets it. Operators must wrap Hermes in shell env hacks, and TP validation (plan 001) would otherwise count **all** system GPUs while the process only sees a subset — still producing late failures. A first-class `--cuda-devices` flag keeps Hermes engine-agnostic and multi-GPU-safe.

## Current state

### ServeConfig (no device field)

```go
// internal/config/config.go
type ServeConfig struct {
    Engine    Engine
    Model     string
    TP        int
    Host      string
    Port      int
    Daemon    bool
    ExtraArgs string
    LogFile   string
}
```

### Process start does not set env

```go
// internal/commands/serve.go startEngine
cmd := exec.CommandContext(execCtx, cmdName, cmdArgs...)
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
// no cmd.Env
```

### Bash legacy (do not port blindly)

```bash
# hermes.sh
export CUDA_VISIBLE_DEVICES="${CUDA_VISIBLE_DEVICES:-0,1,2,3}"
```

Go should **not** hardcode `0,1,2,3`. Default remains: inherit environment (empty flag = do not override).

### After plan 001

Expect `config.ValidateTP` and `gpu.Count` to exist. This plan must count devices from `--cuda-devices` when set.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Tests | `go test -race ./internal/config/ ./internal/commands/ ./internal/engine/` | pass |
| Full gate | `just check` | All checks passed |

## Scope

**In scope**:

- `internal/config/config.go` — add `CUDADevices string` to `ServeConfig`
- `internal/config/config_test.go` — only if you add pure parsers/validators here
- `internal/gpu/devices.go` and/or extend `gpu` package — pure parse of device list
- `internal/gpu/devices_test.go` (or gpu_test.go)
- `internal/commands/serve.go` — flag `--cuda-devices`, wire into cfg, env on process
- `internal/commands/run.go` — same flag and wiring
- `internal/commands/serve.go` `startEngine` — apply `cmd.Env` when devices set
- TP validation call sites — use device-list length when flag set
- `plans/README.md` — status

**Out of scope**:

- Pipeline parallel, data parallel, expert parallel flags
- dtype / max-model-len / mem-fraction
- Auto-TP from GPU count
- Changing engine `ServeCommand` to inject device flags (env is enough and engine-agnostic)
- Persisting devices into pidfile (nice-to-have; only add if trivial and tested — default **skip**)

## Git workflow

- Branch: `advisor/003-cuda-devices`
- Commit: `feat: add --cuda-devices for multi-GPU pinning`
- Do NOT push unless instructed.

## Steps

### Step 1: Pure device-list parsing

In `internal/gpu` (preferred) add:

```go
// ParseCUDADevices parses a CUDA_VISIBLE_DEVICES-style list.
// Accepts "0", "0,1", "0, 1, 2". Rejects empty tokens and non-integer IDs.
// Returns the count of devices and the normalized comma-joined string (no spaces).
func ParseCUDADevices(s string) (count int, normalized string, err error)
```

Rules:

- Empty string → `(0, "", nil)` meaning "unset" (caller treats as inherit).
- Whitespace trimmed around tokens.
- Each token must match `^\d+$` (integer GPU index). Reject `GPU-uuid` form in this minimal plan (STOP if product later needs UUIDs — do not invent).
- Duplicates: reject or allow? **Reject duplicates** with clear error.
- Normalized form: `strings.Join(ids, ",")` with no spaces.

Tests:

| Input | Result |
|-------|--------|
| `""` | count 0, normalized "", err nil |
| `"0"` | 1, `"0"`, nil |
| `"0,1"` | 2, `"0,1"`, nil |
| `"0, 1, 2"` | 3, `"0,1,2"`, nil |
| `"0,1,0"` | error (duplicate) |
| `"0,a"` | error |
| `","` | error |

**Verify**: `go test -race ./internal/gpu/ -run CUDA -v` → pass.

### Step 2: Extend ServeConfig + flags

Add field:

```go
CUDADevices string // empty = do not override process environment
```

In `serve` and `run` FlagSets:

```go
cudaDevices := fs.String("cuda-devices", "", "CUDA_VISIBLE_DEVICES list (e.g. 0,1). Empty inherits env")
```

Copy into `cfg.CUDADevices` / `serveCfg.CUDADevices`.

If non-empty, parse immediately after flags:

```go
n, normalized, err := gpu.ParseCUDADevices(*cudaDevices)
if err != nil {
    return fmt.Errorf("invalid --cuda-devices: %w", err)
}
cfg.CUDADevices = normalized
// n is visible GPU count for TP validation when set
```

### Step 3: TP validation uses visible device count

Replace/adjust the post-001 validation block roughly as:

```go
gpuCount := -1 // unknown
if cfg.CUDADevices != "" {
    n, _, err := gpu.ParseCUDADevices(cfg.CUDADevices)
    if err != nil {
        return err
    }
    gpuCount = n
} else {
    if c, err := gpu.Count(ctx.Ctx); err == nil {
        gpuCount = c
    } else {
        ctx.Logger.Warn("could not query GPU count; skipping TP vs GPU check", "error", err)
    }
}
if err := config.ValidateTP(cfg.TP, gpuCount); err != nil {
    return err
}
```

Also: if `CUDADevices` set and `n == 0` after parse — parse already rejects empty tokens; empty flag means unset.

**Verify**: unit-test the parse; full serve path tested in plan 005. At minimum compile + config/gpu tests.

### Step 4: Inject env in `startEngine`

In `startEngine` after creating `cmd`:

```go
if cfg.CUDADevices != "" {
    // Preserve existing environment; override/set CUDA_VISIBLE_DEVICES.
    cmd.Env = append(os.Environ(), "CUDA_VISIBLE_DEVICES="+cfg.CUDADevices)
}
```

Import `"os"` if not already present in `serve.go`.

Print in the serve summary when set:

```go
if cfg.CUDADevices != "" {
    fmt.Fprintln(ctx.Stdout, ui.Info(fmt.Sprintf("CUDA:   %s", cfg.CUDADevices)))
}
```

**Do not** set `CUDA_VISIBLE_DEVICES` when flag is empty (inherit parent env — production scripts may already export it).

**Verify**:

```bash
go test -race ./...
go vet ./...
just check
```

Optional: with `--debug`, logger already dumps args; env need not be logged (SECURITY: avoid noisy env dumps). Do not log full `os.Environ()`.

### Step 5: Engine tests unchanged

`ServeCommand` stays pure (no env). No change required in `internal/engine/*` unless a test constructs ServeConfig and needs the new field zero value.

**Verify**: `go test -race ./internal/engine/` → pass.

## Test plan

- `ParseCUDADevices` table tests (required).
- Optional pure test for "validation count prefers device list length" if you extract a small helper `VisibleGPUCount(cfg, probed int) int`.
- No live multi-GPU test required in CI.

## Done criteria

- [ ] `--cuda-devices` on `serve` and `run`
- [ ] Non-empty value sets `CUDA_VISIBLE_DEVICES` on the engine process only
- [ ] Empty value does not override environment
- [ ] TP validation uses device-list length when flag set
- [ ] Invalid lists rejected before process start
- [ ] `go test -race ./...` and `just check` pass
- [ ] No out-of-scope multi-GPU flags added
- [ ] `plans/README.md` 003 → DONE

## STOP conditions

- Plan 001 not landed (no `ValidateTP` / `gpu.Count`) — implement 001 first or STOP.
- Requirement appears for UUID device IDs — report; do not half-implement.
- Temptation to hardcode `0,1,2,3` like bash — do not.
- Verification fails twice.

## Maintenance notes

- Pidfile could later store `CUDADevices` for `status` display; deferred.
- Reviewers: confirm we never strip unrelated env vars (always `append(os.Environ(), ...)`).
- Follow-up: `hermes status` showing devices; auto-TP.
