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
  dev/port-rust/README.md, then the numbered files 01 through 07 in order.

That directory is a briefing pack built for you: the codebase indexed, every
third-party dependency traced to its consumers, every capability matched to a
Rust approach, the CLI and distribution ecosystems researched, and the open
questions isolated. It exists so you spend your budget on judgment rather than
discovery. Do not re-derive what it already establishes.

Do challenge it. Its recommendations are inputs, not conclusions, and it
closes with a table of every claim in it that is not a verified observation.
If something in it is wrong, say so plainly — that is a useful result.

HARD CONSTRAINTS
  - Write no Rust. Not a line, not a sketch, not a Cargo.toml.
  - Change no existing code.
  - Create no wayfinder map and no tickets. Rocne has ruled this out.
  - File no GitHub issues.
  - Your deliverable is documentation.

Rulings already made are listed in the README under "Rulings already made".
Treat them as settled inputs and do not re-open them.

WHAT DOMINATES
  dstow is ~13k lines of Go over a 374-line seam onto gostow — a separate,
  stdlib-only, ~5.4k-line Go reimplementation of GNU Stow 2.4.1, also Rocne's,
  checked out at ~/git/rocne/gostow (read it; do not modify it). No Rust crate
  fills that role and the existing Rust stow projects are ruled out. What
  happens to the engine is the largest decision in the evaluation. Spend your
  budget accordingly.

  The motivation is stated in the README and it is the ranking function for
  every option: Rust is preferred for agentic coding — a compiler that makes
  bug classes structurally impossible. Human ergonomics are explicitly
  discounted. Rank options by that, not by convenience.

SECONDARY CHARGE
  dev/port-rust/07-incidental-findings.md asks you to flag problems you notice
  in the existing Go code as you read it. Read that file before you start
  reading code. It is a side effort and must not displace the evaluation.

DELIVERABLE
  Markdown in dev/port-rust/, numbered from 10 so it sorts after the pack.
  Work on a branch. Do not merge anything. Whether it becomes a PR is Rocne's
  call — ask when you are done.

DECISIONS
  Where something is genuinely Rocne's to decide rather than yours to
  recommend, do not decide it in-session. Flag it, state the options and your
  lean, and carry on with everything that does not depend on the answer.
```
