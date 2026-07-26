# 08 — Evaluation and plan

**Fable 5, 2026-07-26.** The evaluating session's output over the briefing pack
(01–07), the design corpus, all of dstow's production Go, and gostow's engine,
stowrc, getopt, and conformance harness — read closely, at `783c9d8` (dstow
v0.6.2) and gostow `v0.4.0`'s working tree. The pack's third tier (judgments)
was treated as argued-with, not inherited; every disagreement is stated
explicitly in §3. The companion file
[`09-incidental-findings.md`](09-incidental-findings.md) carries the
secondary charge's output.

---

## 1. Verdict summary

**The port is technically sound, unusually well-conditioned, and worth doing —
engine included — if dstow is expected to keep evolving.** Three sentences of
support, then the honest caveat:

- The behavioral contract is written down (REQUIREMENTS, DESIGN §1–7/9–11,
  CONTEXT, the ADRs) and executable (17.5k lines of tests, 992 lines of
  language-agnostic e2e, gostow's differential fixtures). A port is a
  translation against a frozen spec, not a reconstruction.
- Every third-party dependency has a clean Rust answer, several strictly
  better; the systems mechanisms (flock, atomic rename, subprocess with
  controlled env, symlink inspection) are std or near-std.
- The engine question — which I agree dominates — is **smaller than the pack
  costs it**, because dstow consumes a narrow, quirk-reduced subset of gostow
  and its product promise is semantic, not byte-parity. §2 reframes it.

**The caveat, stated plainly (Q2's residue):** the Go implementation is not in
distress. v0.6.2 is feature-complete, one HITL walk from v1, with a ~1:1
test ratio and a spec corpus that already delivers most of what makes a
codebase safely modifiable by agents. What the port buys is the *compiler*
layer on top: exhaustive matches over the exit map, enums-with-payload where
Go carries struct-plus-status conventions, typed seams where today a magic
string or an annotation holds the line
([`09`](09-incidental-findings.md) §3 is effectively the receipts). That
purchase pays rent only across **future changes**. If v1 is near-terminal
maintenance, the port buys little; if the §10 doorways (hook drop-ins, the
property store, aliasing, pickers) are a real v2 roadmap, it pays every time
an agent touches the code. My read of the doorways says the roadmap is real,
so: **port, after v1** (decision 1, §6).

---

## 2. The engine question, reframed

The pack frames the engine choice as "port gostow (~5.4k lines, roughly
doubles the port) / write fresh / bundle the Go binary / drop compat / don't
port." Reading both codebases changes the frame in two load-bearing ways.

### 2.1 dstow consumes a subset, and the subset excludes the delicate part

dstow's standing option law (`internal/engine/engine.go` options(), A14) pins
every engine call to: `FixQuirks: true`, `NoGlobalIgnoreFile: true`,
`Log: io.Discard`, `Verbosity` 0, never `Compat`, never `Defer`/`Override`
(stowrc's `--override`/`--defer` are warn-and-ignore at C19, so no pattern
ever reaches the engine). Hold that against what gostow actually spends its
care on, and a large fraction of gostow falls outside what dstow can ever
exercise:

| gostow machinery | Needed by dstow's engine? |
|---|---|
| Parity-pinned log bytes: `logOp`, debug lines, `Gerund`, the RMDIR-colon quirk, the two opendir wordings | **No** — the log is discarded; dstow reports from `Result` only |
| `errnoText` Perl-`$!` emulation | **Mostly no** — it reaches dstow users only inside fatal/conflict prose, as dstow output, not as a parity contract |
| Compat mode (`unstow_contents` target-tree walk, reverse dot-translation) | **No** — never set |
| Defer/override matching | **No** — never populated |
| The PL-04 wrong-directory guard asymmetry | **No** — FixQuirks is always on; dstow gets the corrected behavior |
| gostow's CLI, getopt-driven argv parsing, mangen, its own ui | **No** — pack already established this |
| The planner (plan-then-disk predicates, task cancellation), folding/unfolding, conflict detection + `ConflictKind`, dot-translation, ignore resolution (regex chain, file sources, builtin defaults, `IgnoreFunc`), `Expected`/`Owner`, `CompilePattern`/`IgnoreAnchor` | **Yes** — this is the real engine |
| stowrc: shellwords, the Getopt::Long dialect, post-parse expansion, `Result` | **Yes** — `.stowrc` quirk-faithful parsing is a §3.6 product promise, and the dialect (bundling, permute, auto-abbrev, `:+`) is exactly what makes it faithful |

dstow's own promise (§3.6, REQUIREMENTS §4.3) is **semantic**: same tree
meaning, `.stowrc` honored with stow's quirks, adoptable by a stow user
without their tree changing meaning. It is *gostow's* promise — a separate
product's — that is byte-for-byte. The byte-parity layer is precisely the
delicate, transcription-heavy part of gostow, and **dstow's port does not
need it**.

Measured against the tree: the needed subset is roughly `engine.go` (1,008) +
`tasks.go` (382) + `ignore.go` (301) + `introspect.go` (140) + `dotfiles.go`
+ `move.go` + `paths.go` (~250) + `stowrc/` (518) + `internal/getopt` (311)
≈ **2.9k production lines**, not 5.4k — and the lines that drop out are the
ones with the highest fidelity risk per line. The pack's "roughly doubles the
port" overstates it; more importantly it mis-locates the *kind* of work.

### 2.2 The conformance harness does not re-point the way the pack assumed

The pack's inference — "the harness compares two binaries' observable
behavior; re-pointing it at a Rust engine is a translation of the harness but
not of the method" — is wrong in a detail that matters, and it was flagged as
unverified, so this is the check. Verified by reading
`internal/conformance/{gostow,differential,oracle}.go`:

1. **The harness drives binaries through stow's CLI.** `GostowPath` builds
   `./cmd/gostow`, names it `stow`, and `AssertSameAsOracle` compares
   stdout, stderr, exit code, and tree of full CLI invocations. Re-pointing
   it verbatim at a Rust engine requires the Rust side to ship a
   **stow-compatible CLI shim** — which drags argv parity (getopt bundling,
   usage bytes, verbosity output) right back into scope. That is the
   opposite of §2.1's scoping.
2. **The library surfaces dstow leans on hardest are unreachable through the
   oracle by definition.** Real stow has no `Expected`, no `Owner`, no
   `IgnoreFunc`, no `NoGlobalIgnoreFile`. The differential oracle can never
   cover them; gostow covers them with unit tests
   (`introspect_test.go`, `ignorefunc_test.go`).

So the verification instrument for a dstow-scoped engine port is
**necessarily a new comparator over the old corpus**, not the harness as-is:

- **Reuse the fixtures and the method, replace the comparison.** A small
  driver on each side (Go linking gostow; Rust linking the new crate)
  materializes each fixture case, runs the library call, and emits a
  normalized report — resulting tree, planned tasks, conflicts with kinds,
  exit class, `Expected` map, `Owner` verdicts. Compare reports per case.
  Streams are deliberately excluded: they are not part of dstow's contract.
- **gostow is the second oracle.** For the surfaces real stow cannot answer
  (`Expected`, `Owner`, `IgnoreFunc` composition), gostow — itself
  conformance-tested against 2.4.1 — is the referent. Differential
  Rust-vs-gostow over the corpus transfers the accumulated quirk knowledge
  without transcribing the byte layer.
- **Real stow stays in the loop where it can speak**: tree + exit-class
  comparison against the 2.4.1 oracle over the same fixtures (a relaxed
  variant of `AssertSameAsOracle` that ignores streams) guards against
  gostow-and-Rust agreeing on a shared misreading.

### 2.3 The options, re-ranked

1. **Transliterate the §2.1 subset into a Rust engine crate that dstow owns**
   — **recommended.** Not a fresh implementation: a transliteration, the way
   gostow itself transliterated Stow.pm, carrying the PL-ledger reasoning
   forward as comments where it still applies. Preserves quirk knowledge;
   drops the byte-parity layer; verified per §2.2. gostow's annotated source
   remains available as the reference text even though it isn't the artifact
   being ported. Under Rocne's ruling ("whatever the engine becomes, dstow
   owns it") the natural home is a workspace member crate in the dstow repo
   (decision 3, §6).
2. **Port gostow wholesale (pack's option 1)** — only necessary if
   *gostow-the-product* should also become Rust. That is a legitimate
   separate project (Q7), but coupling dstow's port to it imports the
   byte-parity burden dstow doesn't need. Decouple: gostow-the-Go-product
   keeps existing, released and conformance-tested, no longer a dstow
   dependency once the Rust engine lands.
3. **Fresh Rust implementation (pack's option 2)** — rejected. The planner's
   virtual-filesystem semantics (`isANode`/`isALink` consulting plan before
   disk, `parentLinkScheduledForRemoval`, task cancellation-in-place) are
   subtle and exactly the kind of thing a fresh implementation gets
   plausibly wrong; transliteration is cheaper *and* safer here.
4. **Bundle/embed the Go engine (pack's option 3)** — agree with the pack's
   "weak as a destination," and the sequencing case is now weaker than the
   pack allows: the IPC surface cannot carry `Expected`/`Owner` without
   designing a machine-readable protocol onto gostow (a permanent surface on
   a separately-released product), and §2.1's costing removes the reason to
   want a two-step at all — a ~2.9k-line transliteration does not need an
   escape hatch. The porting *order* (engine before its consumers, §7)
   delivers the incremental verifiability the two-step was for, with no
   shipped intermediate and no IPC contract.
5. **Drop stow compatibility (pack's option 4)** — a product decision that
   §2.1 makes unnecessary; the compat layer's engine cost is modest once the
   byte layer is out. Not recommended.
6. **Don't port (pack's option 5)** — see §1's caveat; legitimate iff the
   post-v1 roadmap is empty.

---

## 3. Where this evaluation departs from the pack

Stated per the pack's own request, most consequential first.

1. **Engine costing and kind** (§2.1): the "roughly doubles the port" figure
   conflates gostow-the-product with dstow's engine. Subset ≈ 2.9k lines,
   and the dropped half is the highest-risk-per-line half.
2. **Harness re-pointability** (§2.2): the pack's unverified inference fails
   on two specifics (CLI coupling; library introspections invisible to the
   oracle). The verification design has to be built, though the corpus and
   method transfer. This was the pack's single most load-bearing unverified
   claim, which is why it got the closest check.
3. **`anstyle` is the wrong Style representation — keep the hand-rolled
   ordered-SGR type.** Evidence: dstow's `ui.Style` is an *ordered list of
   raw SGR parameters* with a hard round-trip contract —
   `ParseColorValue(EmitColorValue(st)) == st` (`ui/emit.go`) — over a
   grammar that includes `reset` (prefixed first), `default` (SGR 39/49),
   `normal` as a slot-occupying no-op, negations that parse-but-emit-nothing,
   duplicate attributes, and significant attribute order.
   `anstyle::Style` is a canonical *set* (effect bitflags + optional fg/bg);
   it cannot represent that grammar losslessly, so the emitter property test
   dies in translation. Meanwhile fatih/color is already vestigial in dstow —
   it supplies integer constants and an SGR-wrapping `Sprint`, both of which
   are ~15 lines to hand-roll. Recommendation: port `Style` as-is
   (`Vec` of SGR params, own render), **zero styling dependencies**; adopt
   `anstyle` only at the clap boundary if clap's native `Styles` help
   coloring is used. This also moots the pack's anstream-vs-§7.3 ownership
   worry: dstow keeps owning the enable chain because there is no second
   engine in the stack at all.
4. **`env_clear()` is not what the hooks code does — a pack erratum that
   would have shipped a bug.** The pack (03, twice) equates the
   absent-not-empty contract with "`env_clear()` then explicit re-add."
   `hooks.environ` strips exactly the twelve managed `DSTOW_HOOK_*` names
   and **keeps the rest of the inherited environment** — hooks run with the
   user's PATH, HOME, everything. `env_clear()` would empty it. The Rust
   translation is twelve `Command::env_remove` calls, then the per-level set.
5. **Q5 (help text) resolves to `build.rs`, and further than the pack
   pushed it: the `manual` tree is compile-time too.** The docs tree is
   embedded at compile time, so its shape is known at compile time; a
   `build.rs` that walks `docs/`, extracts the tagged regions, and generates
   the command-tree source makes a missing page, malformed tag, index-less
   directory, or name collision a **build failure** instead of a unit-suite
   failure — for the help strings *and* the manual tree. The pack treated
   the manual tree as "genuinely dynamic"; it is dynamic in cobra's
   lifecycle, not in the artifact. A proc macro adds machinery for the same
   guarantee; runtime extraction is what the motivation exists to rank last.
   (The generated code still parses `dstow:` tags with the same explicit
   rules — the grammar of §2.3/§2.4 ports intact, only the failure time
   moves.) One consequence for
   [the docs-driven help spin-out (#138)](https://github.com/rocne/dstow/issues/138):
   see decision 6, §6.
6. **Two pack errata in the "reliable" mechanical tier**, minor but the tier
   claimed verification: (a) 02 says `ops/candidates.go` consumes
   `stow.Owner` — it consumes `stow.CompilePattern` + `stow.IgnoreAnchor`
   (`Owner` is consumed via `engine.Owner` in `statusclass.go` and
   `rebuild.go`); (b) 01's dependency graph omits `hooks → config`
   (`hooks/paths.go`) and `hooks → name` (`hooks/env.go`). Also worth
   naming: gostow's import sites are **three files** (`engine`,
   `config/stowrc.go`, `ops/candidates.go`), so "one seam" is one
   *conceptual* seam with two satellite imports — the Rust engine crate's
   surface must include `CompilePattern`/`IgnoreAnchor` and the stowrc
   `Result`, which the pack's 26-symbol count does correctly include.
7. **Compatibility surfaces beyond the ledger.** The pack flags
   `ledger.json` for a byte-level fixture test; the same standard should
   cover the **registry** (`repos.toml` — Rust must read Go-written
   registries, both C9 forms, and write the canonical shorthand form
   byte-compatibly) and **theme TOML / packed `DSTOW_COLORS`** emissions
   (scripts may have captured `theme emit --format env` output). Cheap
   fixture tests, worth naming in the plan.

Where the pack is confirmed after checking, briefly: `serde_ignored` for the
unknown-key warning posture (the misplaced-key half is the C7 matrix as
ordinary code, as stated); the `ignore` crate as a strict upgrade; `IsTerminal`
with no crate; `rustix` for the flock (1:1 with fail-fast `LOCK_NB`);
`tempfile` + `sync_all` + `persist` for both atomic writers (plus the
directory fsync both Go writers do — verify `persist` alone doesn't cover it);
keep shelling out to system git (A17's rationale survives verbatim, and the
`Port`→trait, `Fake`→test-impl translation is mechanical); `etcetera`'s
mechanics and the Q4 framing (decision 2, §6); thiserror enums + one
exhaustive `match` for the A3 map — the single clearest instance of the
motivation, given that `classifyExit`'s fall-through-to-1 is unauditable by
construction today.

---

## 4. What the port buys, concretely

The motivation asks for bug classes made structurally impossible. From the
close read, the specific conversions with real defect history or live risk
(cross-referenced to [`09`](09-incidental-findings.md)):

- **Exit-code map**: `errors.As` chain with silent fall-through → exhaustive
  `match` over a closed error enum. A new error variant nobody mapped becomes
  a compile error (the [#139](https://github.com/rocne/dstow/issues/139)/[#151](https://github.com/rocne/dstow/issues/151) shape).
- **The gostow seam's diagnostics**: today a refusal class rides a string
  prefix match on another repo's error bytes (09/F2 — the sharpest ★ finding).
  A Rust engine crate returns typed diagnostics; the C21-vs-C19 routing
  becomes a match over variants.
- **Result shapes**: `PackageResult{Status, Actions, Err, …}` where Status
  gates field meaning → enums with payload; `expectedSet{exp, exists,
  answerable}` → a three-variant enum; `Field.Value any` → a typed value
  enum (09/F9 documents the drift the `any` already allowed).
- **Zero-value fallbacks**: `mapConflictKind` ([#125](https://github.com/rocne/dstow/issues/125))
  and `mapVerb` (09/F5) → exhaustive matches over the engine crate's enums;
  the "unknown gostow version" case ceases to exist in-process.
- **The H7 write-guard**: a cobra annotation a new command can forget
  (09/S7) → a required constructor field, so a command without a declared
  read/write kind does not compile.
- **FQN canonicality**: "every FQN's `String()` re-parses" is currently
  enforced nowhere and violated on unnormalized session dirs (09/F3) → a
  validated-constructor newtype makes the invariant structural.

And the honest other side: the discipline that *is* holding in Go — single
owners like `Contradicted`, `engineOp`, `orderWork`, `options()`; the A4
streams law; warnings-as-data — holds by the same conventions in Rust. The
port strengthens the weak points; it does not transform the strong ones.

---

## 5. musl (the live half of 05)

Confirmed direction with the pack's framing — static musl *recovers* the
`CGO_ENABLED=0` property, it is not new ambition. Code-side consequences:

1. **`~user` expansion**: real (`config/expand.go` resolves `~user` via
   `os/user`, pure-Go `/etc/passwd` under CGO-free builds). Rust must parse
   `/etc/passwd` directly — small, dependency-free, and the failure shape is
   soft (unresolved token → the absoluteness check refuses with provenance).
   Do not pull `uzers`/`users`.
2. **Allocator**: skip. dstow is syscall-bound (lstat/readlink walks, small
   parses); note it on the divergence list as "measure if anyone notices,"
   not a preemptive dependency.
3. **New, minor**: engine error prose interpolates strerror text; musl's
   strings differ from glibc's for some errnos. Under §2.1 this is dstow
   output, not a parity contract — divergence-list it, nothing more.

---

## 6. Decisions that are Rocne's — numbered, with leans

Per the charge: flagged, not decided. Everything in §7 that does not depend
on these proceeds regardless.

1. **Go/no-go and timing (Q3).** Options: (a) after v1, (b) instead of v1,
   (c) alongside. **Lean: (a).** Ship the Go v1 —
   [the acceptance walk (#52)](https://github.com/rocne/dstow/issues/52)
   proceeds unchanged — then port against a frozen, released behavioral
   contract with the e2e suite as the acceptance instrument. (b) throws away
   a finished release for no compiler benefit the port doesn't deliver
   anyway; (c) is the two-canonical-implementations trap.
2. **XDG on macOS (Q4).** Options: (a) `etcetera` XDG-everywhere — distinct
   config/state lanes on every platform, dissolving
   [the macOS lane collision (#181)](https://github.com/rocne/dstow/issues/181)'s
   carve-out, at the cost of a macOS ledger relocation; (b) `directories` —
   byte-compatible, carve-out preserved. **Lean: (a)**, with a one-time
   migration (detect the old path, announce, move under the lock). The Go
   fix was recorded *decided-not-derived and retractable* with exactly this
   counter-argument preserved — this is the doorway being walked through.
3. **Engine crate home, and gostow-the-product's fate (Q1/Q7).** Options:
   (a) workspace member crate in the dstow repo; (b) sibling repo.
   **Lean: (a)** — dstow owns it per the ruling, no second release pipeline,
   extraction later is additive. Separately: gostow-the-Go-product continues
   unported and independently alive under this plan; if Rocne *also* wants a
   Rust gostow (byte parity, CLI, mangen), that is a distinct project this
   plan deliberately does not depend on.
4. **Acceptance policy (Q10).** Proposal: **behavioral equivalence with the
   Go v1.0.0 except an explicit divergence list**, the list seeded with:
   macOS state relocation (if 2a), musl strerror prose, completion mechanism
   (if 5b/5c), and nothing else without a written entry. Confirm the policy;
   it prevents re-litigating per decision.
5. **Dynamic completion (Q6).** Options: (a) `clap_complete` `CompleteEnv`
   under `unstable-dynamic`, pinned, accepting the churn; (b) static
   completion only, dropping A20's dynamic names; (c) hand-rolled dynamic
   protocol (a hidden `__complete` subcommand + small shell functions —
   cobra's own pattern, ~200 lines, fully owned). **Lean: (c)** — A20 is
   already best-effort-silent custom logic; owning the protocol removes the
   unstable dependency and preserves the feature. Low stakes; can be
   revisited at implementation.
6. **[The help-infrastructure spin-out (#138)](https://github.com/rocne/dstow/issues/138).**
   The port removes dstow as the anchor consumer of a *cobra* library.
   Options: (a) reframe it as a cobra-ecosystem library independent of
   dstow; (b) drop it; (c) fork the idea into a Rust `build.rs` helper
   crate. **Lean: (a) or (b)** — the Rust mechanism (§3.5) is a build
   script, not a library, so (c) has little substance.

---

## 7. The plan

Prose phases, no map, no tickets (per ruling). Each phase names its
verification instrument; the testing charter (assert intended behavior from
REQUIREMENTS/DESIGN, never contrived-green) carries over verbatim.

**Phase 0 — decisions.** §6 above; only 1–3 block anything structural.

**Phase 1 — scaffolding and calibration.**
Workspace (`dstow` binary crate + `engine` crate placeholder), CI toolchain
swap (`cargo test` / `clippy` / `fmt` in the existing matrix), and the
one-line e2e harness swap proven early (`test/run-e2e.sh`'s single
`go build` line — the pack verified the rest of the harness is
toolchain-blind). Port **`name` first**: pure, zero-dep, exhaustively
tested; its 548/685 lines calibrate translation cost and test-port cost for
everything after, and it lands the FQN newtype (§4) on day one.

**Phase 2 — leaf packages, parallelizable.**
`ignore` (via the `ignore` crate), `ledger` (rustix flock + tempfile;
**byte fixture: a Go-written `ledger.json` reads identically, and a Rust
write round-trips**), `git` (trait + exec + fake), `hooks` (std
`Command`, `env_remove` semantics per §3.4), `ui` (own Style per §3.3,
grammar + emitter with the round-trip property test), `repo` (registry
fixtures per §3.7). Each ports with its colocated tests.

**Phase 3 — the engine crate.** The critical path; start as early as Phase 2.
Order: `stowrc` (tokens → getopt dialect → expand — the probed-behavior
tests in gostow port with it), then the engine subset (paths/dotfiles →
tasks+planner → engine walk → ignore → introspect → move). Build the
**semantic differential driver** (§2.2): fixture corpus reused, tree +
conflicts + exit class vs the real 2.4.1 oracle where the CLI can express
the case; Rust-vs-gostow library differential for `Expected`, `Owner`, and
`IgnoreFunc` composition. Port `engine_test`, `introspect_test`,
`ignorefunc_test`, and the stowrc tests. Exit criterion: corpus green both
ways.

**Phase 4 — composition.**
`config` (serde + serde_ignored; stowrc integration now against the Rust
crate's *typed* diagnostics — retiring the F2 string match structurally;
supplement diffing; expansion incl. the `/etc/passwd` path), then `ops`
(the transliteration bulk; expect the §4 enum collapses), then `cli`
(clap derive + the `build.rs` of §3.5 generating help strings and the manual
tree; thiserror exit map as one exhaustive match; prompter; completion per
decision 5). In-process `Run(args, streams) -> code` testability is
preserved — it is why cli has 2.1k lines of tests, and they port.

**Phase 5 — acceptance.**
The full Go suite ported (the largest single line item — tests are ~half the
total effort and that is the plan's honest center of gravity), e2e green in
Docker against the installed Rust binary, the 26 `--help` outputs checked as
content (never bytes — the [#96](https://github.com/rocne/dstow/issues/96)/[#141](https://github.com/rocne/dstow/issues/141)
rulings; `insta`/`trycmd` byte-pinning is the named trap to refuse),
divergence list finalized against decision 4. Release wiring: out of scope
per the ruling; musl target flags and the allocator note are the only
code-adjacent residue (§5).

**Sizing frame** (calibrate at Phase 1, not before): ~13.1k dstow production
+ ~2.9k engine-subset production ≈ 16k lines to port, plus ~17.5k test lines
that are most of the effort and all of the safety. The e2e suite gives an
end-to-end signal from the first phase; the differential corpus gives the
engine an oracle from the first week of Phase 3. Nothing in the plan ships a
two-language intermediate.

**What survives unchanged**: REQUIREMENTS, CONTEXT, the ADRs, DESIGN §1–§7 /
§9–§11 (§8 is replaced by a Rust-architecture section when the port is
real), the whole `docs/` tree as content, ADR 0003 and its CI guard, the
testing charter, `install.sh` / `snippet.sh`, and the e2e exercisers.
