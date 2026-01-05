# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Project Overview

Hermes CLI is a GPU inference server launcher for sglang and vllm, built with Go and the Charm ecosystem.

## Tech Stack

- **Language**: Go 1.24+
- **CLI**: Custom subcommand router (no Cobra)
- **UI**: Charm ecosystem (Bubble Tea, Bubbles, Lip Gloss, Huh)
- **Logging**: charmbracelet/log

## Commands

```bash
make build        # Build binary to bin/hermes
make test         # Run tests
make lint         # Run go vet
go vet ./...      # Type check
```

## Repository Layout

```
cmd/hermes/         # Main entry point
internal/
  app/              # AppContext, global config
  commands/         # Command implementations (doctor, install, serve, verify, studio, run)
  config/           # Typed config structs
  engine/           # Engine interface (sglang, vllm)
  execx/            # Process execution helpers
  ui/               # Lip Gloss styles
  ui/tui/           # Bubble Tea components (spinner, steps, forms)
```

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

