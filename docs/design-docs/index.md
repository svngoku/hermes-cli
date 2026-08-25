# Design Docs — ADR Catalogue

> Architectural Decision Records. One file per significant decision.
> Newest first.

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| — | _No ADRs recorded yet_ | — | — |

---

## How to add an ADR

Create `docs/design-docs/NNNN-short-title.md` with:

```markdown
# ADR-NNNN: <title>

- **Status**: proposed | accepted | superseded by ADR-XXXX
- **Date**: YYYY-MM-DD
- **Context**: what forces are at play
- **Decision**: what we chose
- **Consequences**: tradeoffs accepted
```

Then add a row to the table above.

## Candidate decisions to record

- Why a custom subcommand router instead of Cobra.
- Why per-engine `python3 -m venv` + `pip` installs over uv/conda.
- Engine interface boundary (`ServeCommand` returns args, does not run).
