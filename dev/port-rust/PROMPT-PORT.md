# Port-kickoff session prompt

Copy-paste this to open the session that **starts the Rust port** — the
successor to [`PROMPT.md`](PROMPT.md), which opened the *evaluation* session
and whose constraints (write no Rust, create no map or tickets) applied to
that session only and are superseded by this one.

---

```
You are starting the Rust port of dstow. The evaluation is done and the
decisions are made; your job is to set up the work, not to re-litigate it.
This message is your authorization to create the wayfinder map, file
tickets, create the engine repo, and write code.

PRECONDITION — check before anything else
  Decision 1 rules the port starts after v1. Verify dstow v1.0.0 has been
  released (a v1.x tag / release exists). If it has not, stop and say so —
  do not start the port early on your own judgment.

READ FIRST
  dev/port-rust/08-evaluation.md — the evaluation and plan. §6.1 holds
  Rocne's six recorded decisions: they are SETTLED RULINGS, not
  recommendations. Do not reopen them. The load-bearing ones:

  - Engine: a Rust stow clone in its OWN repo, a sibling of gostow —
    library-first (the semantic subset dstow consumes, per 08 §2.1),
    with the full stow clone (CLI, parity) as the committed later
    destination. dstow depends on the library.
  - Acceptance: behavioral equivalence with the Go v1 except an explicit,
    maintained divergence list (seeded in 08 §6, decision 4).
  - macOS: XDG lanes on every platform, no migration machinery — the
    pre-v1 Go behavior is not owed compatibility on macOS state location.
  - Completion: clap_complete (decided-retractable; the counter is
    recorded in 08 §6.1 item 5).

  Then: 09-incidental-findings.md §3 (the discipline-vs-compiler ledger is
  port-design input), and the pack (01–07) as reference material. The
  briefing pack's judgments were already audited by the evaluation — where
  08 disagrees with 01–07, 08 wins.

FIRST JOBS, IN ORDER
  1. Re-check the findings issues (#195–#206) for what landed between v1
     and now; anything fixed in Go changes the port baseline.
  2. Create the wayfinder map for the port from 08 §7's phases, tickets
     per dev/agents/issue-tracker.md conventions. The map is the work
     queue; 08 §7 is its source, not its substitute.
  3. Create the sibling engine repo (name is Rocne's call — ask) and the
     dstow cargo workspace scaffolding, with CI (cargo test / clippy /
     fmt) wired per the develop-with-CI/CD standing rule.
  4. Port internal/name first — the calibration piece (08 §7 phase 1) —
     and prove the e2e harness swap (the one go-build line in
     test/run-e2e.sh).

STANDING RULES THAT BIND HERE
  - Branch + PR for everything; CI green is the done-gate.
  - The testing charter carries over verbatim: tests assert intended
    behavior from REQUIREMENTS/DESIGN, never contrived-green.
  - Help/docs assertions are content, never bytes (the #96/#141 rulings;
    snapshot tooling's byte-pinning is the named trap).
  - Where something is genuinely Rocne's to decide, flag it with options
    and a lean; do not decide it in-session.
```
