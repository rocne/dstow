# 06 — Open questions

What this pack could not settle. These are the decision points the evaluation
has to actually reason about — everything else in the pack is either an
observation or a well-supported recommendation.

Nothing here is ranked by importance except Q1, which genuinely dominates.

---

## Q1 — The engine. What happens to gostow?

**The dominant question, and now a narrower one.** No Rust crate fills
gostow's role, and **Rocne has ruled the existing projects inadequate — not an
option to evaluate** (2026-07-26). So no third party carries the conformance
guarantee: whatever the engine becomes, dstow owns it.

What remains ([`03`](03-concept-map.md)): port gostow to Rust; write a native
Rust engine against the conformance oracle; keep gostow in Go out-of-process
(characterized there — weak as a destination, useful as sequencing); drop stow
compatibility; or don't port. These differ by roughly a factor of two in total
project size and by a large margin in what they preserve.

Sub-questions the evaluation should separate out:

- Is gostow's **GNU Stow 2.4.1 conformance guarantee** a v1 product promise
  that must survive, or a Go-era implementation detail? DESIGN §3.6 and the
  whole `.stowrc` compatibility layer say the former; that is worth
  re-confirming rather than assuming.
- Can the **differential conformance harness** be re-pointed at a Rust engine?
  This pack believes yes in principle (it compares two binaries' observable
  behavior over fixture trees) but has not examined the harness closely enough
  to cost the work.
- Does a port of gostow belong to **this** evaluation, or is it a separate
  project that dstow's port depends on? gostow is a standalone released
  product with its own users, not a vendored library.

## ~~Q2 — What is the motivation?~~ — **answered**

**Rocne, 2026-07-26: Rust is preferable for agentic coding** — type safety and
a compiler that makes bug classes structurally impossible. Human ergonomics
explicitly discounted; only what serves the agent counts. Stated in full in the
README, where its consequences for ranking are worked through.

What remains open is not *why* but *whether the benefit clears the cost* — and
that is Q1 and Q3, not a separate question. Note the baseline the cost is
measured against: dstow is at v0.6.2, feature-complete, 41 of 42 map tickets
closed, one HITL acceptance walk from v1. **The Go implementation is not in
distress.** The port is an improvement to a working system, so the plan should
be honest about what is being spent and what is being bought.

## Q3 — Relationship to v1

Three shapes, and they are very different projects:

1. **After v1.** Ship the Go v1, then port. Clean baseline, a stable behavioral
   contract to port *against*, and the e2e suite as the acceptance test.
2. **Instead of v1.** v1 ships in Rust. Deletes the Go work's release but
   avoids maintaining two implementations through a transition.
3. **Alongside.** Both exist for a period. Doubles maintenance for as long as
   it lasts, and raises a which-one-is-canonical question at every bug.

Note that the map's destination is *"v1.0.0 actively releasable at a button
press"* and it is one HITL ticket from that. Whatever is decided, the
evaluation should say plainly what happens to
[the acceptance walk (#52)](https://github.com/rocne/dstow/issues/52).

## Q4 — XDG on macOS: fix the collision, or preserve compatibility?

`etcetera`'s XDG strategy gives distinct config and state lanes on **every**
platform, which dissolves [issue #181](https://github.com/rocne/dstow/issues/181)
and its shipped carve-out entirely — the `config → ledger` dependency edge,
the DESIGN §5 exception, and the `files.md` lane-collision documentation all
become unnecessary.

But it **relocates the ledger on macOS**, so anyone who installed the Go dstow
has a migration. The `directories`/`dirs` crates would instead reproduce the
Go behavior byte-for-byte, carve-out included.

This is a user-visible behavior decision, not a crate preference. Note that
Rocne recorded #181's fix as *decided, not derived* and **retractable**, with
"relocate state on macOS" preserved as the counter-argument — so this
question is already anticipated and the doorway is open.

## Q5 — Where does help text come from at build time?

Three designs, differing in **where drift is caught** ([`04`](04-rust-cli-practices.md)):

1. **Runtime extraction into clap's builder** — closest to today; drift caught
   by tests.
2. **`build.rs` generating the strings at compile time** — a missing or
   malformed tag becomes a build failure.
3. **A proc macro** — same, with more machinery.

**The stated motivation ranks these**: (2) or (3) over (1), because moving a
failure from test-time to build-time is exactly what the port is being
undertaken to buy. That does not settle it — (2) and (3) still differ, and the
runtime-constructed `manual` tree may force some dynamism regardless — but the
usual "prefer the simpler runtime approach" default does **not** apply here.

Related: **does the `docs/`-is-binary-content rule (ADR 0003) survive?** It
should — it is about product semantics, not language — but its *enforcement*
(`docs-release-guard.yml`) is path-and-commit-type based and would carry over
unchanged, so this is likely a confirm-and-move-on.

Also related: [the docs-driven CLI help infrastructure spin-out
(#138)](https://github.com/rocne/dstow/issues/138) is an existing post-v1
plan to extract this machinery as a library for other cobra CLIs. **A Rust
port interacts with that plan directly** — either it invalidates it, forks it
into two libraries, or reframes it. The evaluation should say which.

## Q6 — Dynamic completion: accept the maturity gap, or change the design?

`clap_complete`'s `CompleteEnv` is behind an `unstable-dynamic` feature, has
an explicitly unstable interface, requires that nothing writes to stdout
before it runs, and recommends regenerating shell code on shell startup
rather than installing completion files — which changes `install.sh`'s job.

Options: accept it and follow the recommended pattern; ship static completion
only and drop A20's dynamic package/repo-name completion; or keep dynamic
completion but implement it directly rather than through `clap_complete`.

## Q7 — Is this one port or two repos' worth of work?

Follows from Q1 but is a scoping question in its own right. gostow is a
separate released product with its own installer, packages, man page,
conformance suite, and Homebrew cask. If it ports, that is a second project
with its own release pipeline and its own users.

## ~~Q8 — `rocne/release-ci`~~ — **closed, not a question**

**Ruled by Rocne, 2026-07-26.** An upstream workstream in `rocne/release-ci`
is already accommodating Rust. The release path is **out of scope** for this
evaluation: do not plan around it, cost it, or treat it as a blocker.

Retained as a numbered heading so the answer travels with the question.

## Q9 — musl: confirm the tentative direction, and verify the toolchain path

Rocne's direction is tentative and pending judgment. Two things would firm it up:

- **Confirm the framing.** Rust+musl is not new ambition — it *recovers* the
  fully-static property that `CGO_ENABLED=0` already gives the Go build for
  free. That reframing may make the decision easy.
- **Verify the untested combination.** GoReleaser's Rust builder documents
  `-gnu`/`-darwin` triples as defaults and does **not** mention musl. This
  pack could not verify *"GoReleaser + Rust builder + musl targets + nfpm +
  cosign, end to end."* It is plausible (targets are configurable and
  cargo-zigbuild supports musl) but it is the single most load-bearing
  unverified claim in this pack. **A small prototype would settle it cheaply
  and should probably come before the plan is finalized.**

Sub-item, already characterized in [`05`](05-distribution-and-musl.md) and not
really open: `~user` expansion must parse `/etc/passwd` directly rather than
use a `getpwnam`-backed crate — which is exactly what the CGO-free Go build
already does.

## Q10 — What is the acceptance criterion for "the port is done"?

The pack's strong suggestion is **the existing e2e suite** — 11 POSIX-sh
exercisers driving an installed binary in Docker, entirely language-agnostic,
usable from day one of the port. Plus the 26 `--help` outputs, which have
been repeatedly treated as a behavioral baseline (issues #141, #183 both
asserted "all 26 unchanged from HEAD").

Open part: is **behavioral equivalence with v0.6.2/v1.0.0 the bar**, or is the
port allowed to fix things along the way? Q4's macOS relocation is already one
deliberate divergence. A stated policy — "equivalent except for an explicit
divergence list" — would prevent the question recurring per-decision.

---

## Assumptions in this pack that a reader should not inherit blindly

Flagged so they are not mistaken for verified facts.

| Claim | Status |
|---|---|
| GoReleaser Rust builder works with musl + nfpm + cosign end to end | **Unverified.** Plausible; not tested. See Q9. |
| clap's `hide(true)` completes visible children of a hidden parent (cobra v1.10.1 does — verified Go-side at #130) | **Unverified for clap.** |
| The gostow conformance harness can be re-pointed at a non-Go engine | **Inference**, from reading its structure. Not costed. |
| `ops` will shrink in Rust via enums-with-payload | **Judgment**, not measurement. |
| `serde_ignored` covers `md.Undecoded()`'s *unknown-key* role | High confidence. The *misplaced*-key half is dstow's own matrix logic either way. |
| Six transitive Go dependencies disappear | Verified from `go.mod` and a consumer trace. |
| The e2e suite runs unchanged against a non-Go binary | **Verified.** `procure/local.sh` copies a pre-built binary to `~/.local/bin` and chmods it — no toolchain assumption. `Dockerfile.local` is stock `ubuntu:24.04` (glibc, which runs a static musl binary fine). The *only* Go reference in the whole harness is one line in `test/run-e2e.sh`: `GOOS=linux GOARCH=amd64 go build …`. Swap that line and 992 lines of e2e carry over. |
