# Work wrapper: captured behavior

> **Status: evidence and inspiration — not specification.** Nothing in this
> document is a dstow requirement unless a map ticket explicitly decides it.
> The wrapper's output format, status-tag vocabulary, conflict-hint flow, and
> config/file shapes are all undecided.

Resolves [dstow#2](https://github.com/rocne/dstow/issues/2). Source: the actual `dstow`
POSIX-sh script from the work machine, retrieved verbatim 2026-07-12 —
[`work-dstow-wrapper.sh`](work-dstow-wrapper.sh). The script supersedes
"reconstruct from memory": it is the concrete source of truth for the wrapper's behavior.

## What the wrapper is

A POSIX sh wrapper around GNU stow for a flat dotfiles tree at a **hardcoded
`$HOME/dotfiles`** (single root — no multi-root, no repos, no taps). Its downside,
per the user: not self-contained (requires stow installed).

## Behavior inventory

- **Invocation**: `dstow [stow-opts] pkg…`, `--all`, `--adopt pkg…`, `--list`,
  `--check`, `--clean`, `--help`. Unrecognized `-*` flags pass through to stow
  (`-D` unstow, `-R` restow ride this).
- **Every stow call** is `stow --dotfiles --no-folding --dir=$DOTFILES --target=<t>`:
  dot-translation always on; **folding always off**.
- **Per-package `.dstowrc`** (inside the package): `target=` (tilde-expanded;
  default `~`) and `ignore=true`. The target dir is `mkdir -p`'d before stowing.
- **Per-package `deps` file**: `command: install command` lines. On stow only
  (never `-D`/`-R`): if `command -v` misses, run the install command (optional
  proxy via `dstow_https_proxy`/`dstow_http_proxy`); package aborts if the
  command still missing after install.
- **Dry-run first** (`-n -v`), stderr parsed:
  - `over existing target` → **CONFLICT** report, per-file listing, and the hint
    `fix: dstow --adopt <pkg>`;
  - no `LINK:`/`UNLINK:` lines → **NO-OP**;
  - otherwise run for real.
- **Status-line output** on stderr: `[TAG] package -> target`, targets
  `~`-shortened; tags `LINKED` `UNLINKED` `RESTOWED` `ADOPTED` `CONFLICT` `NO-OP`
  `SKIP` `ERROR`; color only when stderr is a TTY.
- **`--list`**: package name → configured target (or `[ignored]`). Shows
  *configured* state, not live stowed state.
- **`--check` / `--clean`**: walk the union of package target dirs, find symlinks
  (`-maxdepth 3`) pointing into `*dotfiles*` whose destination no longer exists;
  report or `rm`.
- **Multi-package runs** continue past per-package failures, one status line each.
  (Whole-run exit code is the last package's — accidental, not a contract.)
- **Absent entirely**: hooks of any kind, multiple package roots, repos/taps,
  a true status view.

## Concept-doc validation (read back to the user this session)

- **Location independence** — confirmed; achieved today via the hardcoded root.
  Wanted future shape (config-declared paths and/or `DSTOW_PATH`) stays with
  [dstow#6](https://github.com/rocne/dstow/issues/6).
- **Package-local config** — confirmed live (`.dstowrc`: target, ignore).
- **Hooks** — greenfield; nothing in the wrapper. `deps` is the only
  lifecycle-adjacent mechanism.
- **System requirements** — *not* hypothetical: `deps` is implemented and
  load-bearing at work (the proxy support proves real use).
- **Fold policy** — **collision**: the concept doc ruled "default utterly
  faithful (folding on)", but the wrapper always runs `--no-folding`. Re-opened
  as its own ticket.
- **Dot-translation** — wrapper runs `--dotfiles` unconditionally.

## Decisions made in this session (2026-07-12)

- **v1 is a fully featured, totally complete, finished product** — no deferred
  stubs (standing preference, recorded in the map Notes).
- **Operations required in v1**: stow/unstow/restow, all-packages, adopt, list,
  a **true status view** (stowed vs not — beyond what the wrapper has), and
  check + clean stale-link maintenance.
- **`deps` capability is v1-required**; whether it ships as its own feature or a
  hook special-case is [dstow#5](https://github.com/rocne/dstow/issues/5)'s call.
- **Fold default** deferred to its own ticket rather than decided here.
