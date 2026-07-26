# 02 — Third-party dependency map

Every library dstow depends on, what it is actually used for, exactly where,
and the Rust options. Symbol counts are from a source trace at `783c9d8`.

## Summary table

| Go dependency | Version | Consumers | Rust answer | Confidence |
|---|---|---|---|---|
| `BurntSushi/toml` | v1.6.0 | 4 files | `toml` + `serde` + **`serde_ignored`** | High |
| `adrg/xdg` | v0.5.3 | 4 files | **`etcetera`** (not `directories`) | High — and it *fixes* a shipped bug |
| `fatih/color` | v1.19.0 | 2 files | **`anstyle`** + `anstream` | High |
| `go-git/go-git/v5` | v5.19.1 | 1 file, 1 subpackage | **`ignore`** (ripgrep's) | Very high — strict upgrade |
| `mattn/go-isatty` | v0.0.20 | 1 file | **`std::io::IsTerminal`** — no crate | Very high |
| `spf13/cobra` | v1.10.1 | 11 files | **`clap` v4** (builder API, see below) | High, with caveats |
| `golang.org/x/sys` | v0.43.0 | 1 file (flock) | **`rustix`** or `fs4` | High |
| `rocne/gostow` | v0.4.0 | 3 files via 1 seam | **Nothing exists.** See [`03`](03-concept-map.md). | — |

dstow has **8 direct dependencies**. Seven of the eight have clean, often
better, Rust equivalents. The eighth is the whole problem.

---

## `BurntSushi/toml` v1.6.0 → `toml` + `serde` + `serde_ignored`

**Used in:** `config/native.go`, `repo/registry.go`, `repo/repo.go`, `ui/theme.go`.

**Symbols used:** `toml.Decode` (3×), `toml.MetaData`, `md.Undecoded()` (5×),
`toml.Primitive` + `md.PrimitiveDecode` (3×), `toml.NewEncoder`.

**Why this library was pinned** (ruled at issue #39): `md.Undecoded()` is
*exactly* DESIGN §3.5's unknown-key surface. dstow does not reject a config
file with an unrecognized key — it accepts it and reports the key as a
warning, with a did-you-mean suggestion (C18). One TOML owner across the
project; config, registry, and themes all use the same library.

**The Rust translation is not the obvious one.** serde's
`#[serde(deny_unknown_fields)]` produces a hard *error*, which is the wrong
behavior — it would turn a warning into a refusal, changing shipped semantics.

The correct tool is **[`serde_ignored`](https://docs.rs/serde_ignored)**
(dtolnay): it wraps a `Deserializer` and calls a callback with the *path* of
every field the target type ignored, while deserialization still succeeds.
Confirmed to work with self-describing formats including TOML. This maps
one-to-one onto `md.Undecoded()`.

`toml::Value`-based lazy decode covers `toml.Primitive`/`PrimitiveDecode`
(deferred sub-document decoding), and `toml::to_string` covers the encoder
(used for theme emission and registry writes).

**Watch item:** dstow's warnings distinguish *unknown* keys from *misplaced*
ones (a legal key at an illegal level — DESIGN §3.5, the C7 legality matrix).
`serde_ignored` gives unknown-ness; the misplaced case is dstow's own matrix
logic either way and ports as ordinary code.

---

## `adrg/xdg` v0.5.3 → `etcetera`

**Used in:** `config/paths.go`, `ledger/ledger.go`, `repo/managed.go`,
`repo/repo.go`.

**Symbols used:** `xdg.ConfigHome` (2×), `xdg.StateHome` (2×), `xdg.DataHome`,
`xdg.Reload` (15×, overwhelmingly a test seam for re-reading changed env).

dstow uses all three XDG lanes deliberately and distinctly — config (the
registry and `config.toml`), state (the ledger), and data (managed clones,
because links point *into* them, so they are neither state nor cache; A19).

### This choice carries a live bug — and Rust can dissolve it

`adrg/xdg` follows Apple convention on macOS and returns
`~/Library/Application Support` for **both** `ConfigHome` and `StateHome`.
They collide. That collision is the entire content of
[issue #181](https://github.com/rocne/dstow/issues/181): the ledger's
`ledger.json` and `ledger.lock` land inside the config directory, and dstow's
M5 reserved-territory scan flagged dstow's own files on nearly every command
on stock macOS. Linux was unaffected.

The shipped fix (v0.6.0, PR #186) is a **collision-conditional allow-list**:
when `GlobalConfigDir() == ledger.Dir()`, the config scan also claims the
ledger's reserved names. It required a new `config → ledger` dependency edge,
a DESIGN §5 carve-out, and documentation in `reference/files.md` — and Rocne
recorded it as *decided, not derived*, retractable, with the heavier
"relocate state on macOS" option preserved as the counter-argument.

**[`etcetera`](https://docs.rs/etcetera) makes the collision optional.** It is
the one Rust directories crate that lets the *caller* choose the strategy:
`choose_base_strategy()` / `choose_app_strategy()` use **XDG on Linux *and*
macOS** (native Windows), which is what CLI tools conventionally want;
`choose_native_strategy()` opts back into Apple's layout. Its `AppStrategy`
exposes `config_dir()`, `data_dir()`, `cache_dir()`, `state_dir()`, and
`runtime_dir()` — including explicit state-dir support.

So a Rust port choosing the XDG strategy gets **distinct config and state
lanes on every platform**, and #181's carve-out has nothing to attach to.

**But this is a behavior change, not a free win.** It relocates where a macOS
user's ledger lives, which means a migration question for anyone who installed
the Go dstow. Flag it as a decision, not a default. (The `directories`/`dirs`
crates do *not* offer the choice and would reproduce the Go behavior exactly —
a legitimate option if byte-compatibility with the Go build is wanted.)

**Bonus simplification:** `xdg.Reload` exists because `adrg/xdg` caches paths
at package init. Etcetera's strategies are constructed at call time, so the
15 `Reload` call sites and the test seam they exist for simply disappear.

---

## `fatih/color` v1.19.0 → `anstyle` (+ `anstream`)

**Used in:** `ui/color.go`, `ui/emit.go` only. Nothing outside `ui` styles
anything — that is A4, enforced structurally.

**Symbols used:** `color.Attribute` (77×) is the workhorse — dstow's `Style`
type is built on it. Plus the named FG/BG constants and the attribute set
(`Bold`, `Faint`, `Italic`, `Underline`, `BlinkSlow`, `ReverseVideo`,
`CrossedOut`, `Reset`), and `color.New` (1×).

**Design constraint (A5):** dstow uses fatih/color **per-instance, never the
package-global `NoColor`**, because the enable decision is per-stream (§7.3)
and must be independently controllable for stdout and stderr. Any Rust
replacement must be a value type, not a global switch.

### Recommendation: `anstyle`

The [`anstyle`](https://docs.rs/anstyle) ecosystem (rust-cli, the crates clap
itself uses) is the right fit, and specifically better than the Go original:

- **`anstyle::Style` is a plain value type** — exactly A5's per-instance
  requirement, with no global to avoid. Go's `*color.Color` is a heap object
  with mutable state; Rust's is `Copy`.
- **It is an interop layer, not an engine.** `anstyle` defines the *types*;
  `anstyle-owo-colors`, `anstyle-termcolor` etc. adapt to other libraries.
  That directly addresses the "two styling engines" smell that got `fang`
  ruled out (A18) — one type vocabulary, no competing renderers.
- **`anstream`** handles the stream side: it strips ANSI automatically when
  writing to a non-terminal, when `TERM=dumb`, when `NO_COLOR` is set, or
  when `CLICOLOR=0`. Note this **overlaps with, and may conflict with,
  dstow's own §7.3 enable chain**, which is dstow-specific and has a defined
  precedence. Decide whether anstream owns the decision or dstow keeps owning
  it and uses anstream purely as a writer. Recommend the latter — §7.3 is
  specified behavior with tests.
- `strip-ansi-escapes` covers `StripANSI` (the O11 strip contract).

`owo-colors` is the common alternative and is fine, but the ecosystem
consensus is that it requires annotating every call with a support check;
`termcolor` is actively discouraged (deprecated Windows Console APIs,
heavier API). Since dstow ships linux + macOS only, the Windows arguments are
moot either way.

**Genuine gap to check:** dstow's `Style` supports 256-color and truecolor
through its git-compatible value grammar (§7.3). `anstyle` covers
`Ansi256Color` and `RgbColor` natively, so this should be clean — but the
port must verify the *emitter* (`EmitColorValue`) round-trips, since that is
dstow's own grammar and has a `parse(emit(st)) == st` property test.

---

## `go-git/go-git/v5` v5.19.1 → the `ignore` crate

**Used in:** `ignore/ignore.go` only, and only the subpackage
`plumbing/format/gitignore`. **dstow does not use go-git as a git client** —
that was explicitly rejected (A17). It vendors go-git purely for a
correct gitignore matcher.

This is the heaviest dependency by transitive weight (`go-billy`, `gcfg`,
`go-context`, `golang.org/x/net`, `warnings.v0` are all indirect deps pulled
in by go-git) for the benefit of one 77-line package.

**Rust is strictly better here.** The [`ignore`](https://docs.rs/ignore) crate
is ripgrep's gitignore engine — the de-facto reference implementation —
and exposes precisely dstow's shape through `gitignore::GitignoreBuilder`
(build a matcher from a set of patterns rooted at a directory; match a path
with an is-dir flag). C16 semantics are its native semantics.

**Net effect:** one direct dependency with a huge transitive tail is replaced
by one focused, better-maintained crate. This is the clearest improvement in
the whole dependency set.

(For the record, DESIGN A15 notes `doublestar` was evaluated Go-side and
disqualified as *not* gitignore-semantic. The Rust analogue trap is `globset` —
it is glob matching, not gitignore matching. Use `ignore`, not `globset`.)

---

## `mattn/go-isatty` v0.0.20 → `std::io::IsTerminal`

**Used in:** `ui/printer.go`, one call site — `isatty.IsTerminal(f.Fd()) ||
isatty.IsCygwinTerminal(f.Fd())`, behind an injectable override (A6:
`Interactive()` = stdin TTY **&&** stderr TTY).

**Rust needs no crate.** `std::io::IsTerminal` has been stable since Rust
1.70 and is implemented for `Stdin`/`Stdout`/`Stderr`/`File`. The Cygwin
branch is Windows-only and irrelevant to dstow's targets.

**Dependency count: −1.**

---

## `spf13/cobra` v1.10.1 → `clap` v4

**Used in:** all 11 command-wiring files under `internal/cli`.

**Symbols used:** `AddCommand`, `Commands`, `CommandPath`, `Flags` (20×),
`Help`, `HasSubCommands`, `Name`, `Run`/`RunE`, `UsageString`,
`ValidArgsFunction`, plus `SilenceUsage`/`SilenceErrors` and command groups.

Treated in depth in [`04-rust-cli-practices.md`](04-rust-cli-practices.md) —
it is the package with real translation risk, not just a swap. The short
version:

- **clap v4 is the only serious candidate** for a CLI of this shape
  (subcommand groups, ~26 command paths, dynamic completion, custom help).
  `argh`, `bpaf`, `lexopt`, `xflags` are all lighter than dstow needs.
- **Prefer clap's builder API over derive** for at least the `manual` subtree,
  because that tree is *constructed at runtime* from an embedded filesystem
  walk. Derive macros want a compile-time-known command set.
- **Help text is dstow's hard case.** Every command's Short/Long/Examples are
  extracted from `docs/commands/**.md` at build/startup, not written in Go.
  clap's `about`/`long_about` accept runtime strings in the builder API, so
  this works — but the derive API's attribute form does not, and this is the
  main reason to lean builder.
- **Dynamic completion is possible but less mature.** cobra's
  `ValidArgsFunction` has no exact clap equivalent; `clap_complete`'s
  `CompleteEnv` (behind the `unstable-dynamic` feature) is the mechanism, and
  it carries a documented constraint that **nothing may write to stdout
  before `CompleteEnv::complete()` runs**. dstow's A20 rule ("best-effort
  silent; any error → no completions, never diagnostics") is compatible in
  spirit but the wiring differs.

---

## `golang.org/x/sys` v0.43.0 → `rustix` or `fs4`

**Used in:** `ledger/update.go` (production) and `ledger_test.go`. Symbols:
`unix.Flock`, `unix.LOCK_EX`, `unix.LOCK_NB`, `unix.LOCK_UN`, `unix.EWOULDBLOCK`.

dstow takes a **non-blocking exclusive `flock`** on `ledger.lock` for the
whole update operation and **fails fast** on contention (exit 3) rather than
waiting. The lock file is deliberately persistent and 0-byte — decided, not
derived, at [issue #182](https://github.com/rocne/dstow/issues/182): a
persistent flock target avoids the create/unlink race that unlinking on
release would reintroduce, and the flock dies with the process anyway,
crash included.

**Rust options, all viable:**

- **`rustix::fs::flock`** — thinnest, direct syscall wrapper, no libc linkage
  concerns under musl. Closest to what the Go code does.
- **`fs4`** — the fs2 successor; now built on rustix rather than libc, adds
  cross-platform and async support. Higher-level (`try_lock_exclusive`),
  and the non-blocking/contention distinction maps cleanly.
- **`fd-lock`** — also fine; RAII guard shape.

Recommend `rustix` if the port wants a 1:1 translation of the existing
semantics, `fs4` if it wants the ergonomics. Either supports the fail-fast
non-blocking requirement.

The other two ledger mechanisms are stdlib or near-stdlib in Rust:
**atomic replace** = `tempfile::NamedTempFile` + `persist()`;
**durability** = `File::sync_all()` before persist.

---

## `rocne/gostow` v0.4.0 → nothing

**Used in:** `config/stowrc.go` (`stowrc.ParseReader`), `engine/engine.go`
(the whole seam), `ops/candidates.go` (`stow.Owner` for adopt ranking).

26 symbols. One 374-line seam. No Rust equivalent exists.

This is [`03-concept-map.md`](03-concept-map.md) § *The engine question* —
the single largest decision in the evaluation.

---

## Indirect dependencies that simply vanish

`go-billy`, `gcfg`, `go-context`, `golang.org/x/net`, `warnings.v0` (all
pulled by go-git for git functionality dstow never uses); `mattn/go-colorable`
(fatih/color's Windows layer); `mousetrap` (cobra's Windows-Explorer guard);
`pflag` (folded into clap).

**Six transitive dependencies disappear** on the Rust side, five of them
because dstow's one go-git use is a subpackage that pulls a whole git
implementation behind it.

---

## Sources

- [serde_ignored](https://docs.rs/serde_ignored/latest/serde_ignored/) · [dtolnay/serde-ignored](https://github.com/dtolnay/serde-ignored)
- [etcetera](https://docs.rs/etcetera/latest/etcetera/) · [lunacookies/etcetera](https://github.com/lunacookies/etcetera) · [etcetera::base_strategy](https://docs.rs/etcetera/latest/etcetera/base_strategy/)
- [anstyle](https://docs.rs/anstyle/latest/anstyle/) · [rust-cli/anstyle](https://github.com/rust-cli/anstyle) · [anstream: simplifying terminal styling](https://epage.github.io/blog/2023/03/anstream-simplifying-terminal-styling/) · [Managing colors in Rust](https://rust-cli-recommendations.sunshowers.io/managing-colors-in-rust.html)
- [rustix::fs::flock](https://docs.rs/rustix/latest/rustix/fs/fn.flock.html) · [fs4](https://lib.rs/crates/fs4) · [fd-lock](https://crates.io/crates/fd-lock)
- [clap_complete::env](https://docs.rs/clap_complete/latest/clap_complete/env/index.html)
