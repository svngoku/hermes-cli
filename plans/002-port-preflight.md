# Plan 002: Port preflight before serve / run

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 970d1db..HEAD -- internal/commands/serve.go internal/commands/run.go docs/RELIABILITY.md`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none (merge-friendly after 001 if both touch serve/run)
- **Category**: reliability
- **Planned at**: commit `970d1db`, 2026-07-10

## Why this matters

`docs/RELIABILITY.md` lists "port in use" as a handled failure mode ("Refuse to start, tell operator the port"), but the Go `serve`/`run` paths never check. Operators get late engine bind errors instead of a clear Hermes preflight failure. Production multi-GPU boxes often run several endpoints; port collisions are common.

## Current state

- `internal/commands/serve.go` `runServe` → `startEngine` → `cmd.Start()` with no listen probe.
- `internal/commands/run.go` starts the engine the same way.
- `docs/RELIABILITY.md` failure table claims port pre-flight exists.
- Bash legacy only checked studio port via `lsof` (`hermes.sh`), not the engine port.

### Conventions

- Side effects at edges: a small helper is fine in `internal/commands` or a tiny leaf package. Prefer **stdlib only** `net.Listen` probe — no new deps.
- User-facing errors: plain `fmt.Errorf` returned to main (prints + exit 1).
- Match existing command structure: validate after flag parse, before process start.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Tests | `go test -race ./internal/commands/ ./...` | pass |
| Full gate | `just check` | All checks passed |
| Manual | bind a port then `hermes serve ... --port <busy>` | clear error, no engine start |

## Scope

**In scope**:

- `internal/commands/port.go` (create) — `assertPortAvailable(host string, port int) error`
- `internal/commands/port_test.go` (create)
- `internal/commands/serve.go` — call preflight in `runServe` before `startEngine`
- `internal/commands/run.go` — call preflight before `startEngine`
- `plans/README.md` — status only

**Out of scope**:

- Changing default host/port values
- TCP health checks of already-running Hermes daemons (that is `status`)
- IPv6-only edge cases beyond what `net.Listen` naturally covers
- Docs rewrite of RELIABILITY.md (optional one-line fix only if you touch it; not required)

## Git workflow

- Branch: `advisor/002-port-preflight`
- Commit style: `fix: refuse serve when port is already bound`
- Do NOT push unless instructed.

## Steps

### Step 1: Implement `assertPortAvailable`

Create `internal/commands/port.go`:

```go
package commands

import (
    "fmt"
    "net"
)

// assertPortAvailable tries to bind host:port briefly. If the bind fails,
// the port is treated as unavailable for a new engine process.
func assertPortAvailable(host string, port int) error {
    if port <= 0 || port > 65535 {
        return fmt.Errorf("invalid port: %d", port)
    }
    addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
    // When host is 0.0.0.0 or empty, Listen on TCP should claim the port on all interfaces.
    ln, err := net.Listen("tcp", addr)
    if err != nil {
        return fmt.Errorf("port %d is not available on %s: %w", port, host, err)
    }
    _ = ln.Close()
    return nil
}
```

Notes:

- Use the **same host** the operator will bind (`cfg.Host`), so `127.0.0.1` vs `0.0.0.0` behavior matches the engine.
- Close the listener immediately so the engine can bind.

**Verify**: package builds: `go test -race ./internal/commands/ -c -o /dev/null` or run tests after Step 2.

### Step 2: Unit tests without external engines

`internal/commands/port_test.go`:

1. **Available port**: listen on `127.0.0.1:0` to get a free port, close it, then `assertPortAvailable("127.0.0.1", freePort)` → nil.
2. **Busy port**: `ln, _ := net.Listen("tcp", "127.0.0.1:0")`; parse port; keep open; `assertPortAvailable("127.0.0.1", port)` → error; then close.
3. **Invalid port**: `0` and `70000` → error.

Pattern: stdlib testing only (like `internal/pidfile/pidfile_test.go`).

**Verify**: `go test -race ./internal/commands/ -run Port -v` → pass.

### Step 3: Wire into serve and run

In `runServe` (`serve.go`), after printing config summary and **before** `startEngine`:

```go
if err := assertPortAvailable(cfg.Host, cfg.Port); err != nil {
    return err
}
```

In `Run` (`run.go`), before `startEngine(ctx, serveCfg)`:

```go
if err := assertPortAvailable(serveCfg.Host, serveCfg.Port); err != nil {
    return err
}
```

Do not call this inside `startEngine` only — keep process construction separate from validation if possible, but either is acceptable as long as both serve and run paths are covered.

**Verify**:

```bash
go test -race ./...
go vet ./...
just build
```

Optional manual: `python3 -m http.server 30000` in another terminal, then `./bin/hermes serve --model x --port 30000` → error about port, process exits non-zero.

## Test plan

- `port_test.go` cases above.
- No need to mock engines.
- Model after existing simple tests in `internal/execx/runner_test.go`.

## Done criteria

- [ ] `assertPortAvailable` exists and is tested (available, busy, invalid)
- [ ] `serve` and `run` call it before starting the engine
- [ ] `go test -race ./...` passes
- [ ] `just check` passes
- [ ] No out-of-scope files modified
- [ ] `plans/README.md` 002 → DONE

## STOP conditions

- Drift in serve/run startup flow after plan 001/004 merges — re-read and place preflight immediately before process start.
- Need for root privileges or raw sockets — should not; plain `net.Listen` only.
- Verification fails twice.

## Maintenance notes

- Race: another process can grab the port between preflight and engine bind (TOCTOU). Acceptable for a CLI; do not build a retry loop unless product asks.
- Plan 004 daemon path must keep this preflight so daemons never detach on a busy port.
