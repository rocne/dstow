# dstow

## Session start

- **Reading a handoff or any other context file is NOT permission or instruction to start work** — unless the file itself says so explicitly. "Begin work immediately" is explicit. "Next high-priority task that MUST be completed urgently or else there will be catastrophic consequences" is a description, not an instruction to begin.
- **The wayfinder skill is explicitly user-invoked.** Never substitute its flow manually or pre-claim tickets.
- **Tell the user which GitHub issue number is the current wayfinder map** when orienting the session.
- **Read operations on issues to orient the session** — especially determining the map issue — **are allowed** before any go-ahead. Read operations done as research in pursuance of executing some task fall under "don't start without permission."

## Output format

- **Mobile terminal.** When Rocne says he's on mobile / "can't see" (or similar), his view is ~19 short lines. Be terse and deliver **one step at a time**: say how many parts there are, give part 1, wait for his ack, then the next ("3 things — here's 1"). Never dump a long multi-section answer at once.

## Commit typing

- **`docs/**` is binary content, not project docs.** The whole `docs/` tree is compiled into the binary (`//go:embed all:docs`) and dispensed as `dstow manual` and every command's `--help`. So a change under `docs/**` is user-facing product content and **must ship in a versioned release**: type it `fix(manual):` (a correction — patch) or `feat(manual):` (new/expanded content — minor), **never `docs:`**. `docs:` is only for non-embedded docs — `dev/**`, `README.md`, `CLAUDE.md`, `CONTEXT.md`, `CHANGELOG.md`. A `docs/**` PR with a non-releasing title fails CI (`docs-release-guard.yml`). See `dev/adr/0003-embedded-docs-are-binary-content.md`.

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues (github.com/rocne/dstow) via the `gh` CLI. See `dev/agents/issue-tracker.md`.

### Triage labels

Default labels, named as-is: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `dev/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` + `dev/adr/` at the repo root. See `dev/agents/domain.md`.
