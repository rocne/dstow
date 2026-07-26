# 01 — Codebase index

Everything that would have to be ported, sized and described. Read at
`783c9d8` (v0.6.2).

## Totals

| | Production Go | Test Go | Other |
|---|---:|---:|---|
| **dstow** (`github.com/rocne/dstow`) | 13,124 | 11,788 | 992 lines of POSIX-sh e2e; 2,524 lines of embedded markdown |
| **gostow** (`github.com/rocne/gostow`) | 5,399 | 5,738 | conformance fixtures + golden files |
| **Combined** | **18,523** | **17,526** | |

Test-to-production is roughly 1:1 in both repos. That ratio is the story of
this port: **the suites are a large fraction of the work, and they are also
the thing that makes the port verifiable.**

---

## Part 1 — dstow

### Dependency graph

Strictly acyclic, and shallow. Arrows point at what a package imports.

```
cmd/dstow ──> cli ──> ops ──> engine ──> ignore ──> config ──> ledger
                │      │        │                      │
                │      ├──> repo ──> name              └──> name
                │      ├──> hooks
                │      ├──> ledger
                │      ├──> git
                │      └──> ui ──> name
                └──> (ui, config, repo, ledger, name, hooks, engine, git)
```

Notable edges and non-edges, all deliberate and all documented in DESIGN §8:

- **`name` imports nothing.** Zero I/O, stdlib only (A7).
- **`git` imports nothing.** It shells out to system `git`; the port interface
  is defined in terms of repo's vocabulary, not git's.
- **`repo` must not import `config`** — config owns *where* the registry file
  lives and passes the path in. Keeps the arrow one-way.
- **`config → ledger`** is a newer edge (added by #181) so config can borrow
  the ledger's reserved filenames when the config and state dirs collide.
- **`ops` is the only package importing `engine`, `hooks`, and `git` together.**
  It is the composition core.
- **`cli` is the composition root** — the only package importing cobra, and
  the only package that touches exit codes.

### Package by package

Line counts are `prod / test`.

#### `internal/name` — 548 / 685

The naming grammar: `scheme:coordinate::package` FQN parse and format,
percent-encoding of reserved bytes (§1.2), segment-boundary suffix matching,
`::` package-kind forcing, path-vs-name operand classification (§1.3), and
`ShortestUnique` display abbreviation.

**Exports:** `Decode`, `Encode`, `IsPathOperand`, `ShortestUnique`, `Expr`
(+`ParseExpr`), `FQN` (+`ParseFQN`), `ParseError`.

**Port note:** the cleanest possible port target — pure string/byte functions,
no I/O, no dependencies, exhaustive unit tests. Translates to a dependency-free
Rust module almost mechanically. The percent-codec has a *custom* reserved set,
so the `percent-encoding` crate is a poor fit; hand-roll, as Go does. Good
candidate to port first as a calibration exercise.

#### `internal/ui` — 1,232 / 1,094

Sole owner of the process streams (A4). Holds: the `Printer` (severity
prefixes, quiet matrix, per-stream color-enable chain), the fourteen-slot
style vocabulary (§3.3), git's `color.*` value grammar as a parser *and* an
emitter, the ANSI-16 default palette, embedded theme presets plus a user
themes directory behind one loader, tier derivation, and `StripANSI`.

**Exports:** `Printer`/`New`/`Options`, `Theme`/`Slot`/`Style`/`Face`/`Role`,
`ParseColorValue`/`EmitColorValue`, `ParseColorTable`, `ParseDSTOWColors`/
`PackDSTOWColors`, `LoadTheme`/`ListThemes`/`BundledThemes`/`EmitThemeTOML`,
`ComposeTheme`/`DeriveTiers`/`DefaultPalette`, `SlotReference`, `StripANSI`,
`ColorMode`, `Warning`, `ThemeNotFoundError`, `UnemittableError`.

**Port note:** medium difficulty, mostly mechanical. The value grammar is
dstow's own (git-compatible) and ports as parsing code. The styling backend
swaps out — see [`02`](02-dependency-map.md) on `fatih/color` → `anstyle`.
The one real design translation is that Go's `*color.Color` per-instance
objects become `anstyle::Style` value types, which is *simpler*, not harder.

#### `internal/config` — 1,165 / 1,027

The four-level configuration chain (builtin → global → repo → package) with
the legality matrix over one key vocabulary, `Effective` composition (nearest
level wins per knob; the ignore chain is additive), use-time path expansion
scoped per package, unknown/misplaced-key warnings-as-data, `DSTOW_PATH`
parsing, the metadata-directory accessor, M5 reserved-territory scans, and the
whole GNU Stow compatibility layer (`.stowrc` discovery, slotting, option
mapping, supplement diffing, content-sniff routing for a renamed rc).

**Exports:** `LoadGlobal`/`LoadRepoLevel`/`LoadPackageLevel`, `Effective`,
`GlobalLevel`/`RepoLevel`/`PackageLevel`, `Level`, `IgnorePattern`/`Language`,
`Warning`, `ParseDSTOWPath`, `GlobalConfigDir`/`GlobalConfigFile`/
`RegistryFile`/`UserThemesDir`/`MetadataDir`, `ExpandError`, `PatternError`.

**Port note:** the warnings-as-data posture is the interesting constraint.
dstow does **not** hard-fail on unknown keys — it collects them and reports.
In Go that is `toml.MetaData.Undecoded()`. Rust's `#[serde(deny_unknown_fields)]`
is the *wrong* shape (it errors); the right tool is `serde_ignored`. See
[`02`](02-dependency-map.md). The stowrc half depends on gostow and inherits
the engine question.

#### `internal/ledger` — 433 / 642

The current-state index (ADR 0001): one JSON document at
`$XDG_STATE_HOME/dstow/ledger.json`, never a journal. Splits along a read/write
line — `Load` is a lock-free snapshot that never creates a file; `Update(scope, fn)`
takes an exclusive `flock` for the whole operation, prunes contradicted entries
in scope, runs the caller's mutation, and commits with temp-file + fsync +
rename. `Recover` is the corruption-tolerant variant (added by #145 so
`rebuild` can read past a corrupt file). `Contradicted` is the single owner of
the disk-disagrees test.

**Exports:** `Load`, `Update`, `Recover`, `Ledger`/`Entry`/`Scope`/`Pruned`,
`Path`/`Dir`/`LockPath`/`ReservedNames`, `KindOf`, `Version`, `CorruptError`/
`NewerVersionError`/`LockedError`.

**Port note:** small, dense, and the most *systems*-flavored package. Three
mechanisms to re-source: advisory locking (`golang.org/x/sys/unix.Flock` →
`fs4`/`rustix`), atomic replace (`tempfile` + `persist`), and durability
(`File::sync_all`). All have direct Rust equivalents. The JSON schema is a
compatibility surface — a Rust dstow must read a Go dstow's ledger byte-wise
identically, which serde handles but which deserves an explicit round-trip test.

#### `internal/repo` — 580 / 742

The repo set: registry read/write with the ledger's atomic discipline, the
source grammar and its `github:`/`local:` schemes, source-input classification,
the A19 managed-clone directory layout
(`$XDG_DATA_HOME/dstow/repos/<scheme>/<owner>/<name>`, percent-encoded and
filesystem-safe by construction), package enumeration (M2/M3, symlink-transparent),
and name resolution over an unordered set via `name`.

**Exports:** `LoadRegistry`, `Registry`, `Repo`/`BuildSet`, `Source`/`ParseSource`,
`Classification`/`ClassifySourceInput`, `Entity`/`Entities`/`Resolve`,
`ManagedReposRoot`, `Warning`, `SourceError`.

**Port note:** straightforward. Directory walking and TOML; no exotic mechanisms.

#### `internal/git` — 308 / 350

The version-control seam (A17). **dstow shells out to the user's own `git`
binary rather than embedding a git implementation** — go-git was explicitly
rejected because credential-helper and ssh-config fidelity is precisely what
dotfiles users depend on. `Port` is an interface stated in repo's terms
(Clone, Fetch, FFApply old→new, AheadBehind, HasLocalWork) with two
implementations: `Exec` (production) and `Fake` (an in-memory double other
packages' tests use).

**Exports:** `Port`, `Exec`, `Fake`, `ClonePair`, `NotInstalledError`/
`DivergedError`/`CommandError`.

**Port note:** **the A17 rationale survives the port intact** — `gix` and
`git2` are the Rust analogues of go-git and lose the same fidelity. Keep
shelling out. `std::process::Command` needs no crate. The `Port` interface
becomes a Rust `trait`, and the `Fake` becomes a test-only impl — an idiom
Rust expresses at least as naturally as Go. This package also has a musl
consequence: because git is a subprocess, the static binary never needs
in-process DNS or credential handling. See [`05`](05-distribution-and-musl.md).

#### `internal/ignore` — 77 / 174

The smallest package. Compiles the additive ignore chain's gitignore-glob
entries once and matches them per package against package-root-relative paths.
C16 semantics (no slash ⇒ basename at any depth; a slash anchors to the
package root; trailing slash ⇒ directory-only; `**` supported) come from
go-git's `plumbing/format/gitignore`. Refuses the two reserved-and-refused
forms (`!` negation, `//`) again as an invariant guard.

**Exports:** `Chain`, `Compile`.

**Port note:** **this is the single best-served capability in the whole port.**
Rust's `ignore` crate (from ripgrep) is the reference-grade gitignore
implementation and exposes exactly this shape via `gitignore::GitignoreBuilder`.
Strictly better-supported than the Go side.

#### `internal/engine` — 374 / 474

**The one seam onto gostow.** The deployment verbs as per-package operations,
plus the two introspections (`Expected`, `Owner`) that the ledger and
maintenance verbs compose against. gostow's types stop here — `Conflict`,
`Task`, and the fatal class are mapped to dstow's typed results in this
package and nowhere else.

Carries the standing option law (A14): `NoGlobalIgnoreFile` always,
`FixQuirks` on, engine log discarded, `Apply` per-package so §3.2 independence
holds. And the three-way ignore composition (A15/A16/M8): compat stow-regex
entries ride gostow's own `Options.Ignore`; native gitignore-glob entries are
matched dstow-side behind `Options.IgnoreFunc`; the always-on `.dstow`
auto-ignore composes into the same closure. `Apply` and `Expected` build
options identically so observation and deployment cannot disagree.

**Exports:** `Apply`, `Expected`, `Owner`, `Op`, `Verb`, `Result`, `Action`/
`ActionKind`, `Conflict`/`ConflictKind`, `ConflictError`, `OpError`.

**Port note:** **374 lines that decide the shape of the whole port.** Because
gostow's types are confined here, the surface a Rust engine has to satisfy is
this file — not the whole codebase. That is a genuine architectural gift: the
evaluation can scope "what must a Rust stow engine provide?" by reading one
package. See [`03`](03-concept-map.md) § *The engine question*.

#### `internal/hooks` — 666 / 969

The hook engine (§5, A11): discovery of eight per-event executables in one
hooks directory with M6/M7 warnings (`.d` reserved, chmod hints, did-you-mean),
the twelve `DSTOW_HOOK_*` environment variables under an absent-not-empty
contract (enforced by stripping inherited vars), a runner with injected streams
(both hook streams → stderr, stdin passthrough, cwd = scope dir), the
nested/LIFO once-per-invocation `Invocation` sequencer, typed `HookError`
carrying a blocking level, and `InHook()` for the H7 write-refusal.

**Exports:** `Discover`/`Set`/`Hook`, `Invocation`/`NewInvocation`, `Runner`,
`Action`/`Phase`/`Level`, `GlobalScope`/`RepoScope`/`PackageScope`,
`ScopeHooksDir`, `InHook`, `HookError`, `Warning`, the `Env*` constants.

**Port note:** process spawning with a controlled environment and injected
streams. `std::process::Command` covers all of it (`env_clear`, `env`,
`current_dir`, `stdin`/`stdout`/`stderr` redirection). No crate needed. The
executable-bit check is `std::os::unix::fs::PermissionsExt`.

#### `internal/ops` — 4,573 / 3,455

**The application core (A13)** and by far the largest package: the verbs as
deep modules returning structured results. 21 production files —
`deploy`, `adopt`, `check`, `clean`, `rebuild`, `list`, `info`, `status`,
`repoadd`, `reporemove`, `reposync`, `snippet`, `theme`, plus the shared
`app`, `classify`, `statusclass`, `resolve`, `scope`, `order`, `candidates`.

Composes config + repo + engine + ledger + hooks + git into results, and owns
every cross-domain rule: per-package independence loops, the §3.3 fold guard
over *effective* values, nested/LIFO hook ordering with §9.1.4 blocking
classification, the §6.4 ledger transactions, the shared four-class link
classifier that keeps `check` and `clean` from ever disagreeing, the
still-stowed and unsaved-work guards, source classification with
confirm-unless-unambiguous, canonical-FQN execution ordering, and the
`Prompter` seam (ops never decides interactively; cli does).

**Exports:** ~60 types — a request/result pair per verb plus the shared
vocabularies (`PackageStatus`, `PackageState`, `LinkState`, `Class`,
`FieldStatus`, …) and ~10 typed errors that cli maps to exit codes.

**Port note:** the bulk of the work, but the *least* exotic. It is composition
logic over the other packages' data — no I/O primitives of its own, no
third-party dependencies beyond gostow-via-engine. It ports as a direct
transliteration; the Rust version likely gets *shorter* thanks to enums with
payloads replacing Go's struct-plus-status-code result shapes and `?`
replacing explicit error plumbing.

#### `internal/cli` — 3,091 / 2,133

The command-line front end and composition root (A1/A2). 16 production files.
Owns: the full §2.1 command surface on cobra v1.10.1 (no viper), help text
**derived at build time from `docs/commands/**.md`** via namespaced
`<!-- dstow:short|long|examples -->` comment tags, the A3 exit-code map in
exactly one place (`classifyExit`, every ops typed error via `errors.As`),
per-command `--json` views, the O12 prompter, A20 best-effort-silent dynamic
completion, the hidden `manual` command group generated at startup from an
`embed.FS` walk of `docs/`, help styling through the ui printer, and the
`hookguard` H7 write-refusal.

**Exports:** `Run(args, version, stdin, stdout, stderr) int` — one function.

**Port note:** the highest-translation-risk package, because it is the most
framework-coupled. Three specific concerns, all treated in
[`04`](04-rust-cli-practices.md): (1) the docs-derived help mechanism vs clap's
`#[command(about = "...")]` compile-time attributes; (2) the hidden `manual`
tree, which is a *dynamically constructed* command tree — natural in cobra's
builder API, and a reason to prefer clap's builder over derive for at least
that subtree; (3) dynamic completion, which clap now supports but through a
different (and less mature) mechanism than cobra's `ValidArgsFunction`.

#### `cmd/dstow/main.go` — 52

Wiring only: streams, version via ldflags, `os.Exit(cli.Run(...))`.

#### Root — `embed.go`

Two `go:embed` directives: `snippet.sh` (the vendored rc bootstrap, emitted
verbatim by `dstow snippet rc`) and `all:docs` (the whole manual tree). The
`all:` prefix is load-bearing — without it, dotfiles in the tree are silently
dropped.

### Test and verification assets

| Asset | Size | Notes |
|---|---|---|
| Go unit/integration tests | 11,788 lines | Colocated `_test.go`, one per production file. Assert *intended* behavior from DESIGN/REQUIREMENTS, not observed behavior. |
| e2e exercisers | 992 lines POSIX sh | `test/run-e2e.sh` builds a linux/amd64 binary, builds a Docker image, and runs 11 exercisers (`smoke`, `help`, `manual`, `version`, `snippet`, `deploy`, `ledger`, `hooks`, `exitcodes`, `operands`, `theme`) against the installed binary. |
| Help/docs bijection tests | in `cli/helpdoc_test.go`, `manual_test.go` | Assert every command has a docs page and vice versa, and that no shipped text cites an internal document. |

**The e2e layer is language-agnostic.** It drives a binary through a shell,
via an install script. A Rust dstow can be dropped under the identical
harness with zero changes to the exercisers — which makes it the natural
port-verification instrument, and arguably the single most valuable existing
asset for de-risking the port.

---

## Part 2 — gostow

`github.com/rocne/gostow` @ `v0.4.0`, local checkout at `~/git/rocne/gostow`.

**Zero third-party dependencies.** Its `go.mod` has no `require` block at all —
pure standard library, including its own `getopt` implementation of Perl's
`Getopt::Long` and its own `Text::ParseWords::shellwords` port.

| Component | Lines | Role |
|---|---:|---|
| `stow/engine.go` | 1,008 | The planner: tree folding, conflict detection, dot-prefix translation, ignore resolution. |
| `stow/tasks.go` | 382 | Task model and application (create/remove/move × link/dir/file). |
| `stow/ignore.go` | 301 | Stow's ignore-list semantics — regex, not glob. `.stow-local-ignore`, `.stow-global-ignore`, built-in defaults. |
| `stow/introspect.go` | 140 | `Expected` and `Owner` — the introspections dstow's ledger and maintenance verbs are built on. |
| `stowrc/` | ~450 | `.stowrc` parsing with stow 2.4.1's exact semantics: shellwords tokenizing, `Getopt::Long` with bundling/permute/auto-abbrev/`:+`, post-parse env and tilde expansion. |
| `internal/getopt/` | 311 | A faithful `Getopt::Long` port. |
| `internal/cli/`, `internal/ui/`, `internal/mangen/` | ~900 | gostow's own CLI, colors, and man-page generator. **dstow does not use any of this** — dstow consumes `stow` and `stowrc` only. |
| `internal/conformance/` | ~1,400 (test) | The differential oracle harness. |

### Public API dstow actually consumes

Traced by symbol across dstow's source. dstow touches **26 distinct symbols**:

- **`stow`** — `Apply`, `Expected`, `Owner`, `Options`, `Request`, `Result`,
  `Task`/`TaskCreate`/`TaskRemove`/`TaskMove`, `TypeLink`/`TypeDir`/`TypeFile`,
  `Action`/`ActionStow`/`ActionUnstow`/`ActionRestow`, `Conflict`/`ConflictKind`
  and its five kind constants, `ConflictError`, `CompilePattern`, `IgnoreAnchor`,
  `Manual` (a docs constant, referenced 20×).
- **`stowrc`** — `ParseReader`.

That is a **small, well-bounded surface** — a Rust engine has to satisfy 26
symbols behind one 374-line seam, not reproduce gostow wholesale. Note in
particular that dstow never uses gostow's CLI, its man generator, its colors,
or `Stow`/`Unstow`/`Restow` sugar (it calls `Apply` directly).

### The conformance harness — the asset that matters most

`internal/conformance/` runs **the real GNU Stow 2.4.1 binary** alongside
gostow over generated fixture trees and compares stdout, stderr, exit status,
*and* the resulting directory tree. Supporting files: `oracle.go`,
`differential.go`, `compare.go`, `normalize.go`, `golden.go`, `tree.go`,
`snapshot.go`, plus case tables and golden data.

Two consequences for the evaluation:

1. **The spec is executable.** `docs/SPEC.md` and `docs/DIVERGENCES.md` in the
   gostow repo document the semantics, and the harness enforces them. A Rust
   engine is not being written against prose — it is being written against a
   differential oracle that already exists.
2. **The harness itself is language-agnostic in principle.** It compares two
   binaries' observable behavior. Re-pointing it at a Rust engine is a
   translation of the harness (Go test code) but not of the *method* or the
   fixtures. This is the strongest available answer to "how would we know a
   Rust stow engine is correct?"

**No comparable Rust crate exists.** Surveyed: `rustow` (binary-only CLI,
1 star, no conformance testing, claims compatibility without a differential
suite), `new-stow`/`nstow` (deliberately a *superset* with its own stowfile
format), `stow-rs` (hardlinks, not symlinks — different semantics),
`rstow` (WIP). None is a library, none is conformance-tested, none could be
depended on for dstow's fidelity guarantee.
