# 04 — Rust CLI practices, and the cobra → clap translation

Ecosystem research, focused on the things dstow actually does. The general
reference is [Command Line Applications in Rust](https://rust-cli.github.io/book/)
and [Rain's Rust CLI recommendations](https://rust-cli-recommendations.sunshowers.io/).

---

## Framework choice

**`clap` v4 is the only serious candidate.** dstow has ~26 command paths,
nested groups, command-group sections in root help, per-command `--json`,
shell completion, and custom help rendering. `argh`, `bpaf`, `lexopt`, and
`xflags` are all deliberately lighter than that.

This is also the low-friction choice for the rest of the stack: clap is built
on `anstyle`, which [`02`](02-dependency-map.md) already recommends replacing
`fatih/color` with. One styling vocabulary across the CLI framework and the
printer — which is exactly the unification A18 wanted when it ruled out `fang`
for being a second styling engine.

---

## The three real translation problems

### 1. Help text comes from `docs/`, not from source

This is dstow's most distinctive mechanism and the one that constrains the
framework choice most.

**Today:** `docs/commands/**.md` is the **single owner** of help text
(ruled at [issue #132](https://github.com/rocne/dstow/issues/132)). Each page
carries namespaced comment tags — `<!-- dstow:short -->`, `<!-- dstow:long -->`,
`<!-- dstow:examples -->` with explicit closes — and `internal/cli/helpdoc.go`
extracts the regions and assigns them to each cobra command's `Short`, `Long`,
and `Example`. Flag usage strings stay with the flag definitions, so a flag
appears in help by construction. The docs tree is embedded in the binary, so
this works from the shipped artifact with no filesystem access.

The rule this enforces (ADR 0003): **`docs/**` is binary content, not project
documentation** — a change there is user-facing product and must ship in a
versioned release. CI enforces it (`docs-release-guard.yml`).

**In clap:** `Command::about()` / `long_about()` / `after_help()` accept
runtime `impl Into<StyledStr>` values in the **builder** API. So the
mechanism ports directly — extract the tagged regions from the embedded tree,
feed them to the builder.

The **derive** API's `#[command(about = "...")]` attribute wants a literal,
which is the wrong shape. Derive can reference a `const`, and `include_str!`
can pull a file at compile time — but the *tag extraction* step is runtime
string processing over a directory tree, so a const-only path would require
moving extraction into a build script or a proc macro. Possible, and arguably
cleaner (it would make a missing tag a compile error rather than a test
failure), but it is a design decision, not a mechanical translation.

**Flag: this is worth an explicit evaluation.** Three plausible designs —
runtime extraction into the builder (closest to today), a `build.rs` that
generates the strings at compile time, or a proc macro. They differ in where
drift is caught.

### 2. The `manual` command tree is built at runtime

The hidden `manual` group mirrors the embedded `docs/` tree: directories
become command groups, markdown files become leaves, filenames are command
spellings exactly, and every node prints its own file. It is generated at
startup from an `embed.FS` walk, and it is **independent of content** — it
ships correct at one `docs/index.md` and stays correct as pages are added.

That is a genuinely dynamic command tree, and derive macros cannot express it.

**The good news — and this is the non-obvious finding:** clap lets you mix.
`Subcommand::augment_subcommands()` adds derive-defined subcommands to a
builder-constructed `Command`. So the natural design is:

- **Derive** the 26 fixed command paths — type-safe, low boilerplate, the
  bulk of the surface.
- **Builder** the `manual` subtree by walking the embedded tree at startup.
- Merge them on one root `Command`.

**Under the stated motivation this split is not merely convenient — it is the
right boundary**, and it should be pushed as far toward derive as it will go.
Derive gives compile-time checking of the command surface; builder is runtime
construction, which spends exactly the currency the port exists to acquire.
Confining runtime dynamism to the one subtree that genuinely needs it — and
considering whether even that could be generated at build time from the
embedded tree, since the tree is known at compile time — is the design the
motivation argues for.

Also verified for the Go side and worth re-verifying in clap: cobra v1.10.1
lets a *hidden parent* still complete its visible children, so `dstow <TAB>`
omits `manual` while `dstow manual <TAB>` works. Confirm clap's
`hide(true)` has the same completion behavior — **this is an assumption, not
a verified fact.**

### 3. Dynamic completion is less mature than cobra's

**Today (A20):** cobra's `ValidArgsFunction` completes package names, repo
names, and schemes through the same `repo` resolver the commands use.
Best-effort-silent: any error yields no completions and never a diagnostic;
config loads in quiet mode; it never fires hooks and never touches the network.

**In clap:** `clap_complete` handles static completion scripts. Dynamic
completion is `clap_complete::env::CompleteEnv`, behind the
**`unstable-dynamic`** feature. Usage is
`CompleteEnv::with_factory(cli).complete()` called at the very start of
`main`, before anything else runs.

Three documented constraints that matter for dstow:

- **Nothing may write to stdout before `CompleteEnv::complete()` runs.**
  dstow's `cli.Run` takes injected streams and its printer is constructed
  early — the ordering needs care.
- Because completion runs *before* application initialization, custom
  completers cannot reuse that initialization and must do their own minimal
  setup. dstow's A20 already has this shape (config in quiet mode, resolver
  only), so the constraint is compatible — but the wiring is different.
- The interface is explicitly unstable; the recommended practice is
  regenerating the shell code on shell startup rather than writing completion
  files to disk, so it is self-correcting across upgrades. **That changes the
  installation story** — `install.sh` currently installs completion files.

**Assessment:** achievable, but this is the one area where the Rust ecosystem
is behind cobra, and it deserves explicit treatment in the plan rather than
being assumed away.

---

## Error handling and exit codes

dstow's A3 map lives in exactly one place (`cli.classifyExit`) and matches
~10 typed domain errors via `errors.As`:

```
0  success
1  negative answer   (a package failed; a field applicable but unset; check found findings)
2  usage error
3  refusal / environment  (non-interactive ambiguity, corrupt ledger, lock contention, newer ledger)
```

**Rust is a better fit than Go here.** Model domain errors as
[`thiserror`](https://docs.rs/thiserror) enums and the exit map as a `match`.
The compiler then enforces exhaustiveness: a new error variant that nobody
mapped to an exit code is a **compile error**, where Go's `errors.As` chain
silently falls through to a default. Given that this codebase has already been
bitten twice by "a detector was built and the caller contract was deferred"
(issues #139, #151), compiler-enforced exhaustiveness is a real gain.

Use `anyhow` only at the outermost boundary, if at all — dstow's errors are
*typed on purpose* and the types are load-bearing all the way to the exit code.
Resist the reflex to `anyhow` the internals.

`std::process::ExitCode` covers the return; `main` returning `ExitCode` is the
idiom. Keep `Run(args, …) -> i32`'s testability property — the current design
lets tests drive the whole CLI in-process with injected streams, which is why
`cli` has 2,133 lines of tests. Preserve that: make the equivalent function
take streams as `impl Write` and return the code.

---

## Streams, color, and the enable chain

`ui` is the sole stream owner (A4) with a per-stream enable precedence (§7.3)
and the O11 strip contract. Two Rust cautions:

- **`anstream` will make its own color decision** (NO_COLOR, CLICOLOR=0,
  TERM=dumb, non-tty). dstow's §7.3 chain is *specified behavior with tests*
  and has its own precedence. Decide explicitly whether anstream owns the
  decision or dstow does. **Recommend dstow keeps ownership** and uses
  anstream purely as a writer with color forced on/off per its own chain —
  otherwise §7.3 becomes two owners disagreeing, which is the exact smell
  A5 and A18 were written to avoid.
- **Help output must be themed through the same printer.** clap supports this
  natively: `Command::styles(clap::builder::Styles)` takes `anstyle` values.
  Because dstow's palette is already `anstyle`-shaped under this plan, the
  help styling and the output styling share one source — better than today,
  where help styling is a separate path in `helpstyle.go`.
- `strip-ansi-escapes` for `StripANSI`; `std::io::IsTerminal` for TTY
  detection (no crate).

---

## Testing

dstow has 11,788 lines of Go tests and 992 lines of shell e2e. The testing
charter (map [#35](https://github.com/rocne/dstow/issues/35)) requires that
tests assert **intended** behavior from DESIGN/REQUIREMENTS, never
contrived-green against the code. That charter is language-agnostic and
should carry over verbatim.

| Layer | Go today | Rust |
|---|---|---|
| Unit | colocated `_test.go`, one per production file | `#[cfg(test)] mod tests` in-file — same locality, same idiom. |
| In-process CLI | `cli.Run` with injected streams, 2,133 lines | Same design; keep it. This is where most CLI-behavior assertions live. |
| Out-of-process CLI | — | [`assert_cmd`](https://docs.rs/assert_cmd) + [`assert_fs`](https://docs.rs/assert_fs) + [`predicates`](https://docs.rs/predicates) for precise assertions; `assert_fs` is a good fit for the temp-tree fixtures dstow builds constantly. |
| Snapshot / golden | hand-rolled comparisons | [`insta`](https://insta.rs) for structured snapshots (JSON views, result shapes) with `cargo insta review`; [`trycmd`](https://docs.rs/trycmd)/[`snapbox`](https://github.com/assert-rs/snapbox) for many blunt CLI cases from TOML files. **Caution:** the map has ruled twice that byte-pinning help output is the wrong test (issues #96, #141 — content, not bytes). Snapshot tooling makes byte-pinning *easy*, which is precisely the trap. Use snapshots for JSON and structured data; keep help assertions content-based. |
| e2e | 11 POSIX-sh exercisers in Docker | **Unchanged.** The harness drives an installed binary through a shell. Drop a Rust binary in and it works. This is the port's best verification instrument. |
| Differential | gostow's conformance oracle vs real GNU Stow | The method transfers; the harness is Go code. See [`03`](03-concept-map.md). |

**The strongest de-risking sequence available**, stated as an observation:
port `name` first (pure, zero-dep, exhaustively tested — calibrates
translation cost with no risk), then run the *existing* e2e suite against
whatever exists. The e2e layer gives an end-to-end signal from the first day
of the port to the last.

---

## Sources

- [clap builder `Command`](https://docs.rs/clap/latest/clap/builder/struct.Command.html) · [clap derive API](https://docs.rs/clap/latest/clap/_derive/index.html) · [clap subcommands (DeepWiki)](https://deepwiki.com/clap-rs/clap/7.3-subcommands)
- [clap_complete::env / CompleteEnv](https://docs.rs/clap_complete/latest/clap_complete/env/index.html) · [CompleteEnv struct](https://docs.rs/clap_complete/latest/clap_complete/env/struct.CompleteEnv.html) · [Kevin K — CLI Shell Completions in Rust](https://kbknapp.dev/shell-completions/)
- [anstream: simplifying terminal styling](https://epage.github.io/blog/2023/03/anstream-simplifying-terminal-styling/) · [Managing colors in Rust](https://rust-cli-recommendations.sunshowers.io/managing-colors-in-rust.html)
- [Command Line Applications in Rust — Testing](https://rust-cli.github.io/book/tutorial/testing.html) · [snapbox](https://github.com/assert-rs/snapbox) · [trycmd](https://docs.rs/trycmd) · [Snapshot Testing — Rust Project Primer](https://www.rustprojectprimer.com/testing/snapshot.html)
