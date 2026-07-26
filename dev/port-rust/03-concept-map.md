# 03 — Capability map

Every *capability* dstow needs, matched to a Rust approach — including the
ones that are not a Go package and therefore do not appear in
[`02`](02-dependency-map.md). Ordered roughly by how much the port hinges on
them.

---

## The engine question

**This is the decision the evaluation exists to make.** Everything else in
this pack is tractable; this is not obviously so.

### The situation

dstow's deploy path is a **374-line seam** (`internal/engine`) onto
[gostow](https://github.com/rocne/gostow) — a separate 5,399-line Go
reimplementation of GNU Stow 2.4.1. Behind that seam sit tree folding,
conflict detection, dot-prefix translation, ignore resolution, the task model,
`Expected`/`Owner` introspection, and `.stowrc` parsing with Perl-faithful
`Getopt::Long` and `shellwords` semantics.

dstow consumes **26 symbols** from it. It never uses gostow's CLI, its
man generator, its colors, or the `Stow`/`Unstow`/`Restow` sugar.

**There is no Rust crate that fills this role.** Surveyed and rejected:

| Candidate | Why not |
|---|---|
| [`rustow`](https://github.com/ebihara99999/rustow) | Binary-only CLI, not a library. Claims "full GNU Stow compatibility" but has **no differential conformance testing** — the claim is asserted, not checked. 1 star, 0 forks, early-stage. |
| [`new-stow`/`nstow`](https://lib.rs/crates/new-stow) | Deliberately a *superset* with its own `stowfile` format. Different product. |
| [`stow-rs`](https://github.com/0xErgod/stow-rs) | Uses **hardlinks, not symlinks**. Different semantics; incompatible with dstow's ledger and status model. |
| [`rstow`](https://github.com/mbroeders/rstow) | Explicitly WIP. |
| `stowsave`, `farmbot` | Adjacent tools, not engines. |

### Why fidelity is not negotiable here

gostow's value proposition is not "a stow-like tool." Its README states the
guarantee precisely: *same flags, same output, same exit codes, same symlinks
— and that is checked, not asserted.* `internal/conformance/` runs **the real
GNU Stow 2.4.1 binary alongside gostow** over generated fixture trees and
compares stdout, stderr, exit status, *and* the resulting directory tree.

dstow inherits that guarantee and exposes it as a product promise: DESIGN §3.6
is an entire stow-compatibility layer, `.stowrc` files are honored with stow's
exact quirks, and `dstow` is meant to be adoptable by an existing stow user
without their tree changing meaning. A merely-plausible reimplementation
breaks the promise silently.

### The options, as this pack sees them

Stated as options, **not** a recommendation — this is the evaluation's call.

1. **Port gostow to Rust as a library crate.** Faithful, keeps the guarantee,
   and the conformance harness gives an executable acceptance test. Cost:
   ~5.4k lines of production Go plus a differential test harness, *including*
   a `Getopt::Long` port and a `shellwords` port. Roughly doubles the port.
2. **Keep gostow in Go; call the binary as a subprocess.** dstow already
   shells out to `git` on exactly this reasoning (A17). But the seam it needs
   is a *library* seam — `Expected` and `Owner` return structured maps that
   drive the ledger and every maintenance verb — and gostow's CLI does not
   expose them. Would require adding an IPC/JSON surface to gostow, and
   ships two binaries.
3. **Write a native Rust engine against the conformance harness.** Not a
   translation of gostow but a fresh implementation, verified by the same
   differential oracle re-pointed at it. Possibly cleaner than transliterating
   Go, but discards gostow's accumulated quirk-ledger knowledge unless the
   harness catches every case.
4. **Reduce scope: drop stow compatibility.** Makes the engine dstow's own
   and much smaller — but deletes DESIGN §3.6, the `.stowrc` layer, and the
   adoption story. A product decision, not a technical one.
5. **Don't port.** A legitimate conclusion for the evaluation to reach.

### What makes this more tractable than it sounds

Three assets already exist and transfer:

- **The seam is one small file.** A Rust engine must satisfy `internal/engine`'s
  374 lines and 26 symbols, not a diffuse API.
- **The spec is written down.** gostow's `docs/SPEC.md`, `docs/DIVERGENCES.md`,
  and `docs/TEST-PLAN.md`.
- **The spec is *executable*.** The differential oracle compares two binaries'
  observable behavior. Its *method* and its *fixtures* are language-agnostic;
  only the harness code is Go. Re-pointing it at a Rust engine is the single
  highest-leverage de-risking move available.

---

## Filesystem and systems capabilities

| Capability | Go today | Rust | Notes |
|---|---|---|---|
| Symlink create / read | `os.Symlink` (7×), `os.Readlink` (7×) | `std::os::unix::fs::symlink`, `std::fs::read_link` | Direct. |
| Link-vs-target inspection | `os.Lstat` (25×), `os.Stat` (26×) | `std::fs::symlink_metadata`, `std::fs::metadata` | Direct. The lstat/stat distinction is load-bearing throughout (the ledger's contradiction test, the four-class link classifier). |
| Atomic file replace | `os.CreateTemp` + `Sync` + `os.Rename` | `tempfile::NamedTempFile` + `File::sync_all()` + `.persist()` | Used by both ledger and registry writes with identical discipline. |
| Advisory locking | `unix.Flock` non-blocking exclusive | `rustix::fs::flock` or `fs4` | Fail-fast on contention → exit 3. See [`02`](02-dependency-map.md). |
| Directory walking | `os.ReadDir` + manual recursion | `std::fs::read_dir`, or `walkdir` / `ignore::Walk` | Package enumeration is **symlink-transparent by ruling** (issue #41): a symlinked directory *is* a package, silently; a broken symlink is loud in scoped mode only. Whichever walker is used must not follow-or-skip symlinks by default in a way that changes this. |
| Executable-bit check | `os.FileInfo.Mode()&0111` | `std::os::unix::fs::PermissionsExt` | Hook discovery (M6/M7 chmod hints). |
| Subprocess with controlled env | `exec.Cmd` with `Env`, `Dir`, stream wiring | `std::process::Command` — `env_clear`, `env`, `current_dir`, `stdin`/`stdout`/`stderr` | Two consumers: `git` (A17) and `hooks`. The hooks contract requires **absent-not-empty** env vars, i.e. genuine removal — `env_clear()` then explicit re-add, which is what the Go side does by stripping. |
| Home / tilde expansion | manual | manual, or `etcetera::home_dir` | §3 use-time expansion is dstow's own scoped logic and ports as code. **musl caveat:** expanding `~otheruser` needs `getpwnam`, which is the one thing a statically-linked musl binary handles poorly. Check whether dstow supports the `~user` form at all — if it only expands bare `~`, `$HOME` suffices and the issue is moot. |

---

## Language and grammar capabilities

| Capability | Go today | Rust |
|---|---|---|
| FQN grammar (`scheme:coordinate::package`) | `internal/name`, hand-rolled, zero deps | Hand-rolled module, zero deps. Direct port. |
| Percent-encoding (§1.2) | hand-rolled — **custom reserved set** | Hand-roll. The `percent-encoding` crate assumes URL reserved sets; dstow's is its own. |
| Edit distance / did-you-mean | **hand-rolled `editDistance`, duplicated in `config/native.go` and `ops/info.go`** | [`strsim`](https://docs.rs/strsim) — `levenshtein`. **Port-time cleanup opportunity:** consolidate the two copies into one owner while translating. Gate stays at distance ≤ 2 (ruled at issue #157). |
| gitignore matching (C16) | go-git's `plumbing/format/gitignore` | `ignore::gitignore::GitignoreBuilder` — strictly better. See [`02`](02-dependency-map.md). |
| Stow's *regex* ignore language | gostow's `stow/ignore.go` | `regex` crate. **Watch:** stow's patterns are Perl regexes; Rust's `regex` is RE2-style and rejects backreferences and lookaround. Go's `regexp` is *also* RE2 — so this constraint is **already present and already handled** (there is a non-RE2 refusal path, listed for behavioral verification in [issue #180](https://github.com/rocne/dstow/issues/180)). The port inherits the same trade-off, not a new one. |
| `Getopt::Long` emulation | gostow `internal/getopt`, 311 lines hand-rolled | Hand-roll. No crate reproduces Perl's bundling + permute + auto-abbrev + `:+` dialect. Part of the engine question. |
| `Text::ParseWords::shellwords` | gostow `stowrc/tokens.go` | [`shell-words`](https://docs.rs/shell-words) is close but **not** Perl-identical; gostow's quirk ledger (PL-01, PL-02) documents where stow's behavior is surprising. Verify against the oracle, don't assume. |
| TOML with unknown-key *warnings* | `BurntSushi/toml` + `md.Undecoded()` | `toml` + `serde` + `serde_ignored`. See [`02`](02-dependency-map.md). |
| JSON (ledger + `--json` views) | `encoding/json` | `serde_json`. The ledger schema is a **cross-implementation compatibility surface** — a Rust dstow must read a Go dstow's `ledger.json`. Warrants an explicit fixture test. |

---

## Application-architecture capabilities

| Capability | Go today | Rust |
|---|---|---|
| **Typed domain errors → exit codes** | ~10 typed error structs across `ops`/`ledger`/`git`/`repo`, matched with `errors.As` in one place (`cli.classifyExit`), mapping to the A3 map: 0 success · 1 negative answer · 2 usage · 3 refusal/environment | [`thiserror`](https://docs.rs/thiserror) enums. **Rust is a better fit than Go here**: the exit-code map becomes a `match` over an enum that the compiler checks for exhaustiveness, replacing a chain of `errors.As` that cannot be checked. A new error variant that nobody mapped becomes a compile error. |
| **Warnings-as-data** (A4) | every package returns `[]Warning`; only `ui` renders | Same shape — `Vec<Warning>` in return position. Idiomatic in both. |
| **Structured results** | request/result struct pairs per verb, ~60 types in `ops` | Rust enums-with-payload will collapse several Go struct-plus-status-code pairs (e.g. `PackageResult{Status, Actions, Notes, Warnings}` where `Status` gates which fields are meaningful) into types that make invalid states unrepresentable. Expect `ops` to *shrink*. |
| **Test seams / injectable ports** | `git.Port` interface with `Exec` + `Fake`; `ui.Interactive()` override; `hooks.Runner` injected streams; `ops.Prompter` | Traits with a production impl and a test impl. Direct, idiomatic. Consider whether generics or `dyn Trait` — `dyn` matches the Go shape and the call frequency is trivial. |
| **Streams owned by one module** (A4) | `ui` alone touches `os.Stdout`/`os.Stderr`; everything else returns data | Same, enforced the same way — by discipline plus tests. Rust can additionally make it structural by not importing `std::io::stdout` outside the printer module (a lint or a `#![deny]` at module scope is possible but probably overkill). |
| **Embedded assets** | 3 `go:embed` sites: `snippet.sh` (string), `all:docs` (whole `embed.FS` tree), `ui/themes/*.toml` | [`include_dir`](https://docs.rs/include_dir) for the tree (supports directory walking at runtime, which the `manual` command needs) or [`rust-embed`](https://docs.rs/rust-embed); `include_str!` for the two single-file cases. **Watch:** Go's `all:` prefix exists because plain `go:embed` silently drops dotfiles; verify the Rust equivalent's dotfile behavior explicitly rather than assuming. |
| **Runtime-constructed command tree** | the hidden `manual` group, built at startup by walking the embedded FS | clap **builder** API. Derive macros want a compile-time command set. See [`04`](04-rust-cli-practices.md). |
| **Shell completion** | cobra's generator + `ValidArgsFunction` for dynamic values | `clap_complete` for static; `clap_complete`'s `CompleteEnv` (feature `unstable-dynamic`) for dynamic. Materially less mature than cobra's. See [`04`](04-rust-cli-practices.md). |
| **Man page generation** | gostow has `internal/mangen`; dstow's is gated on a TODO in `.goreleaser/dstow.yaml` | `clap_mangen`. Note dstow does **not** ship a man page today — the goreleaser archive, nfpm `contents:`, and Homebrew cask blocks are all commented out waiting for one. A port could close that gap for free. |

---

## Capabilities with no Rust concern

Listed so the evaluation does not spend time on them: severity-prefixed
output, the quiet matrix, the four-level config chain, the legality matrix,
the fold guard, nested/LIFO hook ordering, canonical-FQN execution ordering,
the four-class link classifier, source classification, the still-stowed and
unsaved-work guards, ledger pruning discipline, and every `--json` view shape.

All of these are **dstow's own logic over its own data**. They are specified
in REQUIREMENTS/DESIGN, tested in the Go suite, and translate as ordinary
code. They are the bulk of the line count and the least of the risk.
