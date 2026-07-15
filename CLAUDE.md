# dstow

## Session start

- **Reading a handoff or any other context file is NOT permission or instruction to start work** — unless the file itself says so explicitly. "Begin work immediately" is explicit. "Next high-priority task that MUST be completed urgently or else there will be catastrophic consequences" is a description, not an instruction to begin.
- **The wayfinder skill is explicitly user-invoked.** Never substitute its flow manually or pre-claim tickets.
- **Tell the user which GitHub issue number is the current wayfinder map** when orienting the session.
- **Read operations on issues to orient the session** — especially determining the map issue — **are allowed** before any go-ahead. Read operations done as research in pursuance of executing some task fall under "don't start without permission."

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues (github.com/rocne/dstow) via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default labels, named as-is: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
