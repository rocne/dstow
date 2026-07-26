# Session-start prompt

Copy-paste this to open the evaluation session. Not part of the pack's
content — it exists so the prompt is durable and editable rather than
retyped.

---

```
You are evaluating and planning a port of dstow from Go to Rust. This is an
evaluation, not an implementation. Begin work immediately — this message is
your authorization to start; you do not need to ask.

READ FIRST, BEFORE ANY CODE
  dev/port-rust/README.md, then the rest of that directory.

It is a briefing pack: the codebase indexed, every third-party dependency
traced to its consumers, every capability matched to a Rust approach, the CLI
and distribution ecosystems researched, the open questions isolated.

THE PACK IS A SEED, NOT AN AUTHORITY.
  It was written by Opus 5 — a less capable model than you — to save you
  discovery cost, not to hand you conclusions. Three tiers, and the difference
  matters:

  - Rocne's instructions bind: the task, the hard constraints below, the
    rulings and the stated motivation the README records as his. Those are
    not up for revision.
  - Mechanical observations are reliable: line counts, exported surfaces, the
    symbol-level dependency trace, measured sizes, what the surveyed crates
    are. Verified against the tree. Reuse them rather than re-deriving.
  - Everything else is one model's judgment and is not authoritative: every
    recommendation, every crate choice, the ranking of options, the verdicts
    on the engine question, the framing of what matters, the difficulty
    estimates, and the shape of the open questions themselves. Some of it is
    probably wrong.

  Argue with the third tier in writing. Where you reach a different
  conclusion, say so and say why. Agreeing with the pack throughout would be
  a surprising result, not a satisfying one.

HARD CONSTRAINTS
  - Write no Rust. Not a line, not a sketch, not a Cargo.toml.
  - Change no existing code.
  - Create no wayfinder map and no tickets. Rocne has ruled this out.
  - File no GitHub issues.
  - Your deliverable is documentation.

Rulings already made are listed in the README under "Rulings already made".
Treat them as settled inputs and do not re-open them.

WHAT THE PACK THINKS DOMINATES
  Observed fact: dstow is ~13k lines of Go over a 374-line seam onto gostow —
  a separate, stdlib-only, ~5.4k-line Go reimplementation of GNU Stow 2.4.1,
  also Rocne's, checked out at ~/git/rocne/gostow (read it; do not modify it).
  Rocne's ruling: the existing Rust stow projects are inadequate and are not
  an option, so no third party carries that role.

  The pack's judgment, which you may reject: what happens to the engine is
  therefore the largest decision in the evaluation, and worth a
  correspondingly large share of your budget. If you think something else
  dominates, say so — that itself would be a finding.

  The motivation is stated in the README and it is the ranking function for
  every option: Rust is preferred for agentic coding — a compiler that makes
  bug classes structurally impossible. Human ergonomics are explicitly
  discounted. Rank options by that, not by convenience.

SECONDARY CHARGE
  dev/port-rust/07-incidental-findings.md asks you to flag problems you notice
  in the existing Go code as you read it. Read that file before you start
  reading code. It is a side effort and must not displace the evaluation.

DELIVERABLE
  Markdown in dev/port-rust/. Shape and structure are yours to choose. Work on
  a branch. Do not merge anything. Whether it becomes a PR is Rocne's call —
  ask when you are done.

DECISIONS
  Where something is genuinely Rocne's to decide rather than yours to
  recommend, do not decide it in-session. Flag it, state the options and your
  lean, and carry on with everything that does not depend on the answer.
```
