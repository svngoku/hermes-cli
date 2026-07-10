# Plan 005: Command-layer characterization tests

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 970d1db..HEAD -- internal/commands/ internal/config/ internal/gpu/`
> Confirm plans 001–004 APIs exist before writing tests against them. If a helper name differs slightly but behavior matches, test the actual API.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: LOW
- **Depends on**: plans 001, 002, 003, 004 (test the final preflight surface)
- **Category**: tests
- **Planned at**: commit `970d1db`, 2026-07-10

## Why this matters

QUALITY_SCORE grades `internal/commands` at C+ with **no command-level tests**. Plans 001–004 add multi-GPU validation, port preflight, CUDA devices, and daemon boot logic — all high-value and easy to regress. Characterization tests lock behavior so production multi-GPU CLI stays consistent under future edits.

## Current state

```text
go test ./...
?   github.com/svngoku/hermes-cli/cmd/hermes          [no test files]
?   github.com/svngoku/hermes-cli/internal/app        [no test files]
?   github.com/svngoku/hermes-cli/internal/commands   [no test files]  # may have port/boot tests after 002/004
ok  ... config, engine, execx, pidfile
```

Existing exemplars:

- `internal/pidfile/pidfile_test.go` — temp dir override pattern
- `internal/engine/engine_test.go` — ServeCommand arg assertions
- `internal/execx/runner_test.go` — process helpers

### Conventions

- Tests live beside code: `internal/commands/*_test.go`
- Stdlib `testing` only (no testify)
- Prefer testing **pure or injectable** logic; avoid requiring NVIDIA GPUs or network model downloads
- For commands that need I/O: use `AppContext` with buffer stdout/stderr and cancelable context

`AppContext` (`internal/app/context.go`) fields of interest:

```go
type AppContext struct {
    Ctx       context.Context
    Cancel    context.CancelFunc
    Logger    *log.Logger
    Stdout    io.Writer
    Stderr    io.Writer
    // ...
}
```

Build a test helper:

```go
func testApp(t *testing.T) *app.AppContext {
    t.Helper()
    ctx, cancel := context.WithCancel(context.Background())
    t.Cleanup(cancel)
    var buf bytes.Buffer
    logger := log.New(&buf) // or log.NewWithOptions
    return &app.AppContext{
        Ctx:    ctx,
        Cancel: cancel,
        Logger: logger,
        Stdout: &buf,
        Stderr: &buf,
    }
}
```

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Package tests | `go test -race ./internal/commands/ -count=1` | pass |
| Full suite | `go test -race ./...` | pass |
| Full gate | `just check` | All checks passed |

## Scope

**In scope**:

- `internal/commands/*_test.go` files as needed (serve validation, doctor report pure parts, port, boot, flag parse errors)
- Small extractions **only if required for testability** (e.g. export nothing new if helpers are already package-private in `commands`)
- `plans/README.md` — status

**Out of scope**:

- Full E2E against real vLLM/SGLang
- Rewriting production code for coverage theater (minimal seams only)
- `cmd/hermes` main tests (optional tiny dispatch test allowed if easy; not required)
- UI/lipgloss snapshot tests

## Git workflow

- Branch: `advisor/005-command-tests`
- Commit: `test: characterize serve preflight and doctor GPU paths`
- Do NOT push unless instructed.

## Steps

### Step 1: Inventory helpers after 001–004

Confirm these exist (names may vary slightly):

| Helper | From plan |
|--------|-----------|
| `gpu.Count` / parse helpers | 001 |
| `config.ValidateTP` | 001 |
| `assertPortAvailable` | 002 |
| `gpu.ParseCUDADevices` | 003 |
| `waitForBoot` / `tailFile` | 004 |

If pure helpers already have tests in their packages, **do not duplicate** — focus on command wiring and error paths still untested.

**Verify**: `go test -race ./internal/gpu/ ./internal/config/ ./internal/commands/`

### Step 2: Flag validation tests (no process start)

Test that command functions return errors **before** needing engines, by passing invalid flags:

1. `Serve(ctx, []string{})` or without `--model` → error containing `model`
2. `Serve(ctx, []string{"--model", "m", "--engine", "nope"})` → invalid engine
3. `Serve(ctx, []string{"--model", "m", "--tp", "0"})` → TP validation error (plan 001)
4. `Serve(ctx, []string{"--model", "m", "--cuda-devices", "0,x"})` → invalid devices (plan 003)
5. `Run(ctx, []string{"--model", "m"})` without engine → error
6. `Stop(ctx, []string{})` without port/all → error

These will still try GPU count / port checks for some cases. To keep tests hermetic:

- For TP>GPU tests: use `--cuda-devices 0` with `--tp 2` → must fail validation without needing nvidia-smi (plan 003 makes count=1 from list).
- For port busy: use `assertPortAvailable` tests from 002; add one Serve test only if you can inject host/port to a busy listener **and** pass TP/devices so validation gets that far. If Serve always probes GPUs and warns only, OK.

If `Serve` always calls `gpu.Count` and that is slow/missing, ensure unknown GPU path does not fail TP=1.

**Verify**: `go test -race ./internal/commands/ -run 'Serve|Run|Stop' -count=1`

### Step 3: Doctor report pure logic

Extract **only if needed**: summary exit code selection from check statuses is currently inline in `Doctor`. Prefer testing via:

- `Doctor(ctx, []string{"--json"})` when nvidia-smi absent — should still produce JSON and structured exit via `ExitError` (may be code 3).

Or extract:

```go
func summarizeDoctor(checks []CheckResult, strict bool) (summary string, exitCode int)
```

and table-test it. **Extraction is allowed** and preferred for hermetic tests.

Cases:

| checks | strict | exit |
|--------|--------|------|
| all OK | false | 0 |
| one warn | false | 0 |
| one warn | true | 2 |
| one fail | false | 3 |

**Verify**: `go test -race ./internal/commands/ -run Doctor -v`

### Step 4: Daemon Stop/Status with pidfile override

Mirror `pidfile` tests: the pidfile package uses `dirOverride`. From `commands` tests you cannot set unexported `dirOverride` in another package.

Options (pick one):

A. Export a test hook in pidfile: `func SetDirForTest(t *testing.T, dir string)` that sets override + cleanup — **small production change, acceptable**.  
B. Only unit-test pure formatters in daemon.go (`truncate`, `formatStatusJSON`).

Prefer **A** if Stop/Status need coverage; implement:

```go
// SetDirForTest redirects the daemon record directory. For tests only.
func SetDirForTest(dir string) (restore func()) {
    prev := dirOverride
    dirOverride = dir
    return func() { dirOverride = prev }
}
```

in `pidfile` (file `pidfile/testing.go` or in pidfile.go). Then test Stop with a fake record + non-alive pid (prunes), without killing real processes.

**Verify**: `go test -race ./internal/pidfile/ ./internal/commands/ -count=1`

### Step 5: Boot helper tests (if not already from 004)

Ensure 004's boot tests exist; if thin, add the httptest + sleep cases described in plan 004.

### Step 6: Full gate + coverage sanity

```bash
go test -race ./... -count=1
just check
```

Optional: `go test ./internal/commands/ -cover` — aim for meaningful paths, not a percentage fetish. Locking validation errors is enough.

## Test plan (checklist)

- [ ] Invalid serve/run flags return errors without hanging
- [ ] `--cuda-devices` + excessive `--tp` fails deterministically
- [ ] Doctor summarize exit codes (extracted or via --json)
- [ ] Port helper still covered
- [ ] Boot/tail helpers covered
- [ ] Stop/status with temp pid dir (if hook added)

## Done criteria

- [ ] `internal/commands` has tests that pass under `-race`
- [ ] At least the multi-GPU validation regression (`tp` > device list length) is covered
- [ ] `go test -race ./...` passes
- [ ] `just check` passes
- [ ] No production behavior changes except tiny test hooks (pidfile dir override export) if needed
- [ ] `plans/README.md` 005 → DONE

## STOP conditions

- Plans 001–004 not present — STOP and implement them first (or reduce this plan to only testing whatever landed).
- Testing requires downloading models or real GPU — do not; narrow the test.
- Large refactor of commands package requested by coverage tools — STOP, keep characterization minimal.

## Maintenance notes

- Reviewers: reject flaky tests that sleep longer than ~3s without t.Deadline awareness.
- When adding new serve flags, extend the invalid-flag table here.
- QUALITY_SCORE can mark TD-1 closer to done after this lands.
