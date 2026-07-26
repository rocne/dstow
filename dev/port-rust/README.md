# Rust port — evaluation briefing pack

**Prepared 2026-07-26 for a Fable 5 evaluation-and-planning session.**

## What this is

A pre-built context corpus so the evaluating session spends its budget on
*judgment*, not on discovery. Every file here is legwork already done: the
codebase indexed, every third-party dependency traced to its consumers, every
conceptual capability matched against a Rust approach, the CLI/distribution
ecosystem researched, and the genuinely open questions isolated.

## What the evaluation is

**Evaluate and plan a port of dstow (and, necessarily, gostow) from Go to
Rust.** Output is *documentation* — an evaluation and a plan. Explicitly:

- **No implementation.** Not a line of Rust.
- **No wayfinder map, no tickets.** Rocne has ruled this out for now. Prose
  documents in the working tree are the deliverable; whether they get committed
  is a later call.
- **Nothing is decided.** This pack takes no position on whether the port
  should happen. Recommendations inside it are inputs, not conclusions.

**Plus one secondary charge:** as you read the Go code, flag any problems you
notice in it — structural, logical, drift, anything — and hand the list back.
Side effort, must not displace the evaluation. Rules and the
already-known list are in [`07-incidental-findings.md`](07-incidental-findings.md);
read it *before* you start reading code.

## The one structural fact to absorb first

**This is a two-project port.** dstow is ~13k lines of Go over a **seam** onto
[gostow](https://github.com/rocne/gostow) — a separate, stdlib-only, ~5.4k-line
Go reimplementation of GNU Stow 2.4.1, also Rocne's. dstow's entire deploy path
(conflict detection, tree folding, dot-prefix translation, ignore resolution,
adopt, `.stowrc` parsing) lives there.

There is no Rust crate that fills gostow's role, and the existing Rust
stow projects are **ruled out** (below). Porting dstow without an answer for
gostow is not a plan. See [`03-concept-map.md`](03-concept-map.md) § *The
engine question* — it is the largest single decision in the evaluation.

## The motivation — stated, and load-bearing

**Rocne's determination, 2026-07-26, from prior research outside this project:**

> Rust is the preferable language for **agentic coding** — type safety, and a
> compiler that makes whole classes of bug structurally impossible rather than
> merely detectable. **Human ergonomic considerations are explicitly
> discounted**; the question is only what is best for the agent, and what is
> best for the agent is a language that gives clear feedback and makes problems
> impossible.

This is not background colour. It is the **ranking function** for every option
in this pack, and several recommendations change under it:

- **Compile-time beats test-time, always.** Where a design choice moves a
  failure from "a test catches it" to "the build catches it," that is the
  choice — even at a cost in machinery. This reverses the usual default of
  preferring the simpler runtime approach. It directly re-ranks the help-text
  design ([`04`](04-rust-cli-practices.md) § *Help text comes from `docs/`*)
  toward `build.rs`, and the clap derive-vs-builder question toward derive.
- **Exhaustiveness is a feature purchase, not a nicety.** The exit-code map
  becoming a compiler-checked `match` over a `thiserror` enum
  ([`04`](04-rust-cli-practices.md) § *Error handling*) is not a minor
  ergonomic gain — it is a direct instance of the motivation. This codebase has
  been bitten twice by "a detector was built and the caller contract was
  deferred" ([#139](https://github.com/rocne/dstow/issues/139),
  [#151](https://github.com/rocne/dstow/issues/151)); both are the shape of bug
  the motivation targets.
- **Making invalid states unrepresentable is the point**, not a side effect.
  The observation that `ops`'s struct-plus-status-code result types collapse
  into enums-with-payload ([`03`](03-concept-map.md)) is a *primary* benefit
  under this framing, not an incidental line-count win.
- **Runtime dynamism is a cost.** Anything constructed at startup rather than
  declared at compile time — the `manual` command tree, runtime help
  extraction, `dyn Trait` seams — is spending the exact currency the port is
  being undertaken to acquire. Not disqualifying, but it needs to earn its
  place rather than being chosen by default.
- **Ergonomic arguments carry no weight.** Where this pack cites developer
  convenience for or against an option, discount it. Where it cites
  compiler-enforced correctness, weight it heavily.

**One thing worth stating plainly**, because it affects how much the port has
to carry: the strongest agentic-coding assets dstow already has are **not**
language-dependent. The specification corpus (REQUIREMENTS, DESIGN, CONTEXT,
the ADRs) and the ~1:1 test-to-production ratio are what make this codebase
navigable and safely modifiable today, and **both survive a port unchanged**.
Rust's guarantees compound with them; they do not replace them. A plan should
treat the port as *adding a compiler to an already-specified system* — which
is a much better position than porting into a spec vacuum, and is a large part
of why this port is tractable at all.

## Rulings already made — do not re-open these

Recorded 2026-07-26, after Rocne reviewed the first draft of this pack. They
narrow the evaluation; treat them as settled inputs.

1. **The release path is out of scope.** An upstream workstream in
   `rocne/release-ci` is already accommodating Rust rather than Go. Do not
   plan around it, cost it, or treat it as a blocker or dependency. What was
   Q8 is closed. See [`05`](05-distribution-and-musl.md).
2. **The existing Rust stow projects are inadequate and are not an option.**
   `rustow`, `new-stow`, `stow-rs`, `rstow` were surveyed and rejected; the
   survey is retained in [`03`](03-concept-map.md) as the record of *why*, not
   as a live option. **Whatever the engine becomes, dstow owns it.**
3. **Bundling or embedding the Go gostow binary has been characterized** —
   raised by Rocne, analysed in [`03`](03-concept-map.md) § *Option 3 in
   detail*. Verdict offered: weak as a destination, genuinely useful as a
   sequencing phase. Not a settled ruling; an option with its costs named.

Everything still genuinely open lives in
[`06-open-questions.md`](06-open-questions.md).

## Reading order

| File | What it answers |
|---|---|
| [`01-codebase-index.md`](01-codebase-index.md) | What exists, package by package, in both repos. Sizes, roles, exported surfaces, dependency graph, port-difficulty notes. |
| [`02-dependency-map.md`](02-dependency-map.md) | Every third-party library, what it is used for, exactly where, and the Rust candidates. |
| [`03-concept-map.md`](03-concept-map.md) | Every *capability* — including ones that are not a Go package — matched to a Rust crate or approach. Includes the engine question. |
| [`04-rust-cli-practices.md`](04-rust-cli-practices.md) | Ecosystem research: CLI framework, error/exit-code modelling, streams and color, testing, and how the docs-driven help machinery translates. |
| [`05-distribution-and-musl.md`](05-distribution-and-musl.md) | Build/release/signing pipeline, packaging, the installer, and the musl target constraint. |
| [`06-open-questions.md`](06-open-questions.md) | What this pack could not settle — the decision points the evaluation must actually reason about. |
| [`07-incidental-findings.md`](07-incidental-findings.md) | **A second, smaller charge**: flag problems you notice in the existing Go code as you read it. Read this one before you start reading code, not after. |

## The design corpus (read these, not the code, for *intent*)

dstow is unusually well-specified. The binding documents, in precedence order:

| Document | Lines | Binds |
|---|---|---|
| [`dev/REQUIREMENTS.md`](../REQUIREMENTS.md) | 338 | **Behavior.** What dstow does, language-agnostic. Survives a port unchanged. |
| [`dev/DESIGN.md`](../DESIGN.md) | 1054 | **Design.** CLI surface, config schema, ledger format, output rules. §8 is *Go* architecture and is the only major section a port rewrites. |
| [`CONTEXT.md`](../../CONTEXT.md) | 228 | **Vocabulary.** The ubiquitous language. Survives a port unchanged. |
| [`dev/adr/`](../adr/) | 3 ADRs | Ledger-is-an-index; no-dependency-concept; embedded-docs-are-binary-content. All survive. |

A rough but useful frame: **REQUIREMENTS + CONTEXT + ADRs + DESIGN §1–§7, §9–§11
are language-neutral and port intact. DESIGN §8 is the part being replaced.**
The port is therefore far more constrained — and far more tractable — than a
13k-line rewrite normally is: the target behavior is written down, and the
test suites encode it.

Also useful:

- [`docs/`](../../docs/) — 43 markdown files, **shipped inside the binary** and
  the source of every `--help` string. Content survives a port; the embedding
  and extraction mechanism is Go-specific. See ADR 0003.
- [`dev/audit/2026-07-20-code-quality-audit.md`](../audit/2026-07-20-code-quality-audit.md)
  — an independent quality read of the Go codebase.

## Provenance and confidence

Everything in this pack was read from the working tree at `783c9d8`
(v0.6.2) or verified against live sources during preparation, except where a
line is explicitly marked *unverified* or *recall*. Where a claim is an
inference rather than an observation, it says so.
