# 07 — Standing request: flag what you find in the Go code

**A secondary charge for the evaluating session, requested by Rocne.**

## The ask

While you read this codebase for the port evaluation, **keep a running list of
problems you notice in the existing Go implementation** and hand it back as
part of your output. Structural, logical, architectural, documentation drift —
anything. Rocne wants to be made aware of what you see.

This is a **side effort**. It must not displace or slow the primary
evaluation. If a finding would take real investigation to confirm, note it as
unconfirmed and move on rather than chasing it.

## Why you specifically, and why now

You will be reading ~18.5k lines of production Go closely, asking of each part
*what does this actually guarantee, and how would a type system express it?*
That posture surfaces defects that ordinary code review does not — and this
project has direct evidence for it:

- [#131](https://github.com/rocne/dstow/issues/131) and
  [#141](https://github.com/rocne/dstow/issues/141) each found real bugs by
  **driving documented behavior** rather than reading code. #141's `/verify`
  pass corrected four plausible-and-wrong doc drafts and surfaced two
  shipped defects ([#145](https://github.com/rocne/dstow/issues/145),
  [#146](https://github.com/rocne/dstow/issues/146)), both since fixed.
- The map names a recurring pattern: **"the gap between tickets."** A ticket
  builds a detector and defers the caller contract; consuming tickets each
  assume someone else wired it. It has happened at least twice
  ([#139](https://github.com/rocne/dstow/issues/139),
  [#151](https://github.com/rocne/dstow/issues/151)). Seams are where to look.

Your read is a fresh pair of eyes over the whole system at once, which no
single ticket has ever had.

## Rules

1. **Flag, don't fix.** This session is evaluation and planning; it writes no
   code. That includes not fixing "obvious" one-liners. The precedent is
   [#141](https://github.com/rocne/dstow/issues/141), which filed what it
   found and fixed none of it.
2. **Don't file GitHub issues.** Capture findings in your deliverable with
   enough detail that filing is mechanical. Whether any of it becomes a
   ticket is Rocne's call.
3. **Separate confirmed from suspected**, explicitly. A confirmed finding
   names the file and line and states the failure concretely — *given this
   input or state, this wrong thing happens*. A suspicion says so and says
   what would settle it. **Do not inflate suspicions into findings**; a
   wrong finding costs more trust than a missed one.
4. **Check the known list below before reporting.** Re-reporting a tracked
   issue is noise.
5. **No severity theatre.** If something is cosmetic, say cosmetic.

## What counts

Everything, but these categories are the likely yield:

- **Correctness and logic** — wrong results, unhandled cases, silent
  fallbacks, error paths that swallow information.
- **Spec drift** — code disagreeing with `dev/DESIGN.md`,
  `dev/REQUIREMENTS.md`, `CONTEXT.md`, or an ADR. The specs are binding; where
  they and the code disagree, that is a finding either way.
- **Shipped-documentation drift** — `docs/**` is **binary content** (ADR 0003):
  it is what `--help` and `dstow manual` print. Text that misdescribes real
  behavior is a user-facing defect, not a docs nit. This category has the
  richest history in the repo.
- **Structural smells** — duplicated ownership, leaky seams, a module reaching
  past its boundary, a rule enforced in two places that could disagree. The
  architecture (DESIGN §8) is explicit about ownership; violations are
  findable against it.
- **Gaps between tickets** — a capability built and never wired to its caller.
  See above; this is the repo's known recurring failure mode.
- **★ Guarantees carried by discipline rather than by the compiler.** The most
  valuable category for you specifically, because it is **dual-purpose**:
  it is a defect-risk list *and* direct input to the port design. Every place
  the Go code depends on a convention holding — "only `ui` touches streams,"
  "gostow types stop at `engine`," "every typed error is mapped in
  `classifyExit`," "`Apply` and `Expected` must build options identically" —
  is a place where Rust could make the rule structural instead. Note both
  facts: *is the discipline currently holding?* and *what would enforce it?*

## Already known — do not re-report

Open issues covering code quality and behavior. Check here first.

| Issue | Gist |
|---|---|
| [#124](https://github.com/rocne/dstow/issues/124) | `status`'s per-link observation failures claim occupied; `check` has an explicit unobservable class |
| [#125](https://github.com/rocne/dstow/issues/125) | `engine.mapConflictKind` falls back to a zero value on an unknown gostow conflict kind |
| [#126](https://github.com/rocne/dstow/issues/126) | `list --json` repo row's `root` field brushes against a retired CONTEXT.md term |
| [#127](https://github.com/rocne/dstow/issues/127) | Pre-Go-1.21 idiom (hand-rolled `max`/`contains`/`sortedKeys`, `sort.Slice`) on a Go 1.26 module |
| [#161](https://github.com/rocne/dstow/issues/161) | Systemic guard against `Args`-arity / `Use`-operand drift (C1-class) |
| [#172](https://github.com/rocne/dstow/issues/172) | Should `info` expose `metadata-dir` at repo/package scope? |
| [#184](https://github.com/rocne/dstow/issues/184) | `info -f` field vocabulary diverges from config keys; far-off unknown field gets no guidance |
| [#185](https://github.com/rocne/dstow/issues/185) | No `--json` on the mutating verbs or `--dry-run` |
| [#180](https://github.com/rocne/dstow/issues/180) | Paths verified by design/code reading only, never exercised — `repo update`/`upgrade` against a real remote, stowrc supplement diffing, non-RE2 refusal, installer runtime contract |
| [#73](https://github.com/rocne/dstow/issues/73) | Exit-code consistency for declined and not-found cases |
| [#72](https://github.com/rocne/dstow/issues/72) | `repo add owner/name -y` proceeds on the github guess in scripts |

**Prior audits — do not redo:**

- `dev/audit/2026-07-20-code-quality-audit.md` — a code-quality read whose
  findings became #124–#127.
- [Wayfinder map: 2026-07-22 correctness audit (#158)](https://github.com/rocne/dstow/issues/158)
  — **closed, complete**. A docs-vs-code reading plus an isolated-sandbox
  pass; all findings (#147–#157, #165) shipped. Worth skimming for what
  ground is already covered.
- An agent-experience audit against v0.6.0 produced #184 and #185.

**Note the shape of what those audits did *not* cover**, since it is where
fresh yield is likeliest: they were reading-and-sandbox passes. Nothing has
yet examined the codebase as a *whole system* with the question "what
invariants does this rely on that nothing enforces?" — which is exactly the
question a port evaluation asks anyway.

## Seeded findings

Noticed during preparation of this pack, not investigated further. Included so
you neither re-derive nor re-report them. Both are **confirmed observations,
unassessed severity**.

1. **`editDistance` is implemented twice** — `internal/config/native.go:215`
   and `internal/ops/info.go:286`, backing `config.didYouMean` and
   `ops.nearestField` respectively. Two owners of one algorithm that two
   user-facing suggestion surfaces depend on, and
   [#157](https://github.com/rocne/dstow/issues/157) already established a
   shared rule for them (distance gate ≤ 2). They can drift. Cosmetic today;
   structurally the wrong shape. Note that
   [#127](https://github.com/rocne/dstow/issues/127) covers *adjacent*
   hand-rolled-helper cleanup but does **not** name this one.

2. **`internal/ignore` refuses `!` and `//` twice** — once at config parse and
   again in `Compile`, the second described in the package doc as "an
   invariant guard." This is deliberate and documented, not a defect; flagged
   only because it is a textbook instance of the ★ category above (a rule held
   by belt-and-braces runtime checks that a type could carry instead), and
   therefore useful port-design input.

## Output shape

Whatever fits your deliverable. A separate section or file is fine. What
matters is that each entry carries: **where** (file:line), **what** (the
concrete failure or smell), **confidence** (confirmed / suspected), and — for
★ items — **what would enforce it in Rust**.

If you find nothing, say so plainly. That is a real result about a codebase
that has already been through two audits.
