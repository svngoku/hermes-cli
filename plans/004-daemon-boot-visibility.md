# Plan 004: Daemon boot crash visibility

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 970d1db..HEAD -- internal/commands/serve.go internal/commands/run.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MED (process lifecycle)
- **Depends on**: plans/002-port-preflight.md (port must be free before detach)
- **Category**: reliability
- **Planned at**: commit `970d1db`, 2026-07-10

## Why this matters

`hermes serve --daemon` returns success immediately after `cmd.Start()`. If the engine dies during model load (wrong TP, OOM, bad weights), the operator sees "Daemon started" and only discovers failure later via `status` or a manual log read. Product belief: **show the real crash reason immediately**. Multi-GPU misconfig often fails in the first 30–120s of boot — exactly when detach hides the failure.

## Current state

### Daemon path returns after Start

```go
// internal/commands/serve.go ~106–122
if cfg.Daemon {
    // ...
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("failed to start daemon: %w", err)
    }
    recordDaemon(ctx, cfg, cmd.Process.Pid)
    fmt.Fprintln(ctx.Stdout, ui.Ok(fmt.Sprintf("Daemon started (pid=%d)", cmd.Process.Pid)))
    // ...
    return nil
}
```

### Run pipeline already waits for readiness (when not failing early)

`run.go` uses `waitForReadiness` against `http://127.0.0.1:port` after start, then optionally verify. That is good for `run`, but:

- If the process dies during wait, readiness just times out with a generic message (does not surface process exit / log tail).
- `serve --daemon` has **no** equivalent wait.

### Logging

Daemon stdout/stderr go to `cfg.LogFile` when set (default global `./hermes.log` via AppContext). Crash reasons are often in that file.

### Conventions

- Keep daemon processes on `context.Background()` so they survive CLI exit (existing `startEngine` behavior — do not regress).
- Process group kill already used in `stopProcess` / `Stop` — preserve.
- Stream honest errors to `ctx.Stdout` via `ui.Fail` / returned `error`.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Tests | `go test -race ./internal/commands/` | pass |
| Full gate | `just check` | All checks passed |

## Scope

**In scope**:

- `internal/commands/serve.go` — daemon boot wait / early-exit handling
- `internal/commands/run.go` — improve failure path when process dies during readiness
- `internal/commands/boot.go` (create) — shared helpers: wait for healthy-or-dead, tail log
- `internal/commands/boot_test.go` (create) — pure helpers + fake process where feasible
- `plans/README.md` — status

**Out of scope**:

- Full process supervisor / auto-restart
- `hermes logs` / `hermes restart` commands (deferred finding 10)
- Changing pidfile schema
- Windows support
- Increasing default readiness timeout product-wide without a flag

## Git workflow

- Branch: `advisor/004-daemon-boot-visibility`
- Commit: `fix: surface engine crash during daemon boot wait`
- Do NOT push unless instructed.

## Steps

### Step 1: Add boot-wait helper

Create `internal/commands/boot.go` with something equivalent to:

```go
// waitForBoot watches an already-started process until:
//   - HTTP readiness succeeds on baseURL, or
//   - the process exits, or
//   - timeout elapses.
// On process exit, returns an error that includes the last log lines when logPath is set.
func waitForBoot(ctx context.Context, cmd *exec.Cmd, baseURL string, timeout time.Duration, logPath string) error
```

Behavior:

1. Start a goroutine (or use existing pattern from `waitForServer`) that waits on `cmd.Wait()` and sends exit error to a channel. **Important**: only one `Wait` may run — if boot succeeds, do **not** leave a double-Wait for later. For pure daemon detach after successful boot, the process continues; you must **not** call `Wait` in a way that reaps the process if the CLI will exit and leave the daemon running.

**Critical design constraint (read carefully):**

- For `serve --daemon`, after successful boot Hermes **exits** while the child keeps running. Therefore you **must not** call `cmd.Wait()` on the success path in a way that blocks forever, but you **also** need to detect early death.
- Correct pattern:
  1. `cmd.Start()`
  2. Poll HTTP readiness on an interval (e.g. 1–2s).
  3. Between polls, check if the process is still alive with `pidfile.Alive(cmd.Process.Pid)` or non-blocking wait:
     - Use `syscall.Wait4` with `WNOHANG` **or**
     - `cmd.ProcessState` after a short `Wait` only if exited.
  - Simpler portable approach used by many CLIs:
    - Run `cmd.Wait()` in a goroutine always.
    - On readiness success: **do not** kill the process; return nil from boot wait while the Wait goroutine remains — when the CLI process exits, the child is in its own process group (`Setpgid: true`) and survives. The unreaped Wait goroutine dies with the CLI — **this can create a zombie briefly** on some systems if the CLI exits without reaping.

**Preferred clean approach for daemon:**

1. Poll readiness + `pidfile.Alive(pid)` without calling `Wait` on the success path.
2. If `!Alive(pid)` before ready: read log tail, return error (process already reaped by init or still a zombie — try `cmd.Wait()` once to collect exit code if possible).
3. If ready: return nil **without** Wait (daemon continues; orphan reaping is OS's job when CLI exits — with `Setpgid`, child is not killed on parent exit).

If `cmd.Wait()` was never called, the child is not reaped by this parent; that is OK for daemon mode (same as current code which also never Waits on daemon success).

Implement alive check using existing `pidfile.Alive`.

Readiness: reuse logic from `waitForReadiness` in `run.go` — **extract** shared function into `boot.go` (e.g. move `waitForReadiness` there) so serve and run share one implementation.

Default boot timeout for `serve --daemon`: **120 seconds** (flag `--boot-timeout` seconds, default 120).  
`run` already has `--readiness-timeout` (default 300) — keep it; on failure, add log tail.

### Step 2: Log tail helper

```go
// tailFile returns the last maxBytes of path for crash context. Best-effort.
func tailFile(path string, maxBytes int64) string
```

- If path empty or unreadable: return "".
- Read last ~8KiB (8192) by default.
- Include in error: `fmt.Errorf("engine exited before becoming ready: %w\n--- log tail (%s) ---\n%s", err, path, tail)`.

**Verify**: unit test with a temp file larger than maxBytes.

### Step 3: Wire `serve --daemon`

After successful `Start` + `recordDaemon`:

```go
base := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
// Always probe loopback for health even if bind host is 0.0.0.0
if err := waitForBoot(ctx.Ctx, cmd, base, time.Duration(bootTimeout)*time.Second, cfg.LogFile); err != nil {
    _ = pidfile.Remove(cfg.Port)
    // best-effort: if still alive somehow, leave stop to operator; if dead, record already removed
    return err
}
fmt.Fprintln(ctx.Stdout, ui.Ok(... Daemon started ...))
```

Add flag:

```go
bootTimeout := fs.Int("boot-timeout", 120, "Seconds to wait for daemon readiness before giving up")
```

If boot fails: return non-nil error so exit code is non-zero.

**Note on health endpoints**: same as run — try `/v1/models` and `/health`.

### Step 4: Improve `run` failure message

In `waitForReadiness` (now shared), when timeout hits, check `Alive(pid)`:

- If dead: return error with log tail (pass pid + log path into wait helper).
- If alive but not ready: keep timeout error but mention log path.

`run` currently starts with `Daemon: true` always for the serveCfg — keep that; still Wait on foreground via `waitForServer` when `!*daemon`.

### Step 5: Tests

Without real GPUs/engines:

1. `tailFile` tests (temp files).
2. `waitForBoot` with a fake process:
   - Start `exec.Command("sleep", "60")` with Setpgid, point readiness at a local `httptesttestServer` that returns 200 on `/health` immediately → waitForBoot returns nil quickly; kill sleep afterward.
   - Start `exec.Command("false")` or `sleep 0` that exits immediately, no server → waitForBoot returns error quickly (alive becomes false).
   - Start `sleep 60`, no HTTP server, short timeout → timeout error.

Clean up processes in tests (`syscall.Kill(-pid, SIGTERM)`).

**Verify**: `go test -race ./internal/commands/ -count=1` → pass.

### Step 6: Full gate

```bash
just check
```

## Test plan

- `boot_test.go` as above.
- Do not require nvidia-smi or model downloads.
- Race detector on: careful with goroutines and HTTP client timeouts.

## Done criteria

- [ ] `serve --daemon` waits for readiness or process death up to `--boot-timeout` (default 120s)
- [ ] On early process death, error includes log tail when log file is set
- [ ] On success, CLI still exits and leaves engine running (Setpgid preserved; Background ctx preserved)
- [ ] `run` readiness failures distinguish dead process vs timeout when possible
- [ ] Port preflight from plan 002 still runs before start
- [ ] `go test -race ./...` and `just check` pass
- [ ] `plans/README.md` 004 → DONE

## STOP conditions

- Cannot detect process death without breaking daemon detach (zombie/reap issues) — stop and report design options rather than silently calling `Wait` on the success path in a way that blocks daemon lifetime.
- Drift: `run` no longer sets `Daemon: true` for background engine during pipeline.
- Verification fails twice.
- Need to change pidfile format for this to work — report first.

## Maintenance notes

- Reviewers: pay special attention to process lifecycle and no regression of TD-4 (daemon killed on CLI exit).
- Follow-up: `hermes logs --port` to stream log file from pidfile record.
- Boot timeout may need to be higher for huge models; flag covers this.
