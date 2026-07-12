# Landscape verification — dstow differentiator claims vs. live primary docs

**Date:** 2026-07-12. All sources fetched live on this date, primary only (official
docs sites, official repos, man pages, source where docs were ambiguous).

**What was verified:** the "Comparison to the field" section of the concept doc
(`gostow-dstow-concept-2026-07-09.md`, copies in `~/git/gostow/.claude/docs/` and
`~/git/rocne/dot-dagger/.claude/docs/`), which claims five dstow differentiators
against GNU stow, dotbot, chezmoi, yadm, and rcm. The concept doc's `.dsto` /
`DSTO_PATH` spellings are mistranscriptions; this doc uses `dstow` / `DSTOW_PATH`.

---

## Verdict summary

| # | Claimed differentiator | Verdict |
|---|---|---|
| 1 | Convention over manifest (no per-package manifest) | **Partially true.** Genuine vs. dotbot only. GNU stow and chezmoi are both already convention-driven (layout / filename attributes define everything, no manifest). The differentiator is *convention + named packages + ergonomics*, not convention itself. |
| 2 | Package-local encapsulation (config lives in the package) | **Partially true.** No tool bundles config + hooks per package. But GNU stow already has a package-local config file (`.stow-local-ignore` lives inside each package), and chezmoi's `.chezmoiignore` works in any source subdirectory. The *general* form is genuine; "nobody has package-local config" would be false. |
| 3 | Multi-level hooks (package + repo + global) | **Genuine as a combination; contradicted as "others lack hooks."** rcm has per-dotfiles-directory pre/post hooks; chezmoi has in-tree `run_before_`/`run_after_` scripts *plus* global per-command config hooks (two levels); yadm hooks every command globally. Nobody offers package-level + repo-level + global simultaneously. |
| 4 | Composable source paths (`DSTOW_PATH`) | **Contradicted as stated.** The concept doc says "nobody does this cleanly" — rcm does exactly this: `DOTFILES_DIRS` in `rcrc(5)` is a first-class, documented multi-directory source list, and `rcup -d` can be given multiple times. dstow can still differentiate on the `$PATH`-style env var + git-tap composition, but not on "nobody composes source dirs." |
| 5 | Single standalone binary | **Not unique.** chezmoi is "a single stand-alone statically-linked binary with no dependencies" (its own words). Genuine only within the symlink-farm niche (vs. stow/Perl, dotbot/Python, yadm/Bash+Git, rcm/POSIX sh). |

---

## Per-tool findings

### GNU stow

Source: GNU Stow manual for **version 2.4.1 (8 September 2024)**,
<https://www.gnu.org/software/stow/manual/stow.html>.

1. **Convention over manifest — yes, fully.** No manifest exists; "the structure
   of each private tree should reflect the desired structure in the common tree"
   (manual, *Installation* section). Directory layout is the entire configuration.
2. **Package-local config — partially exists.** `.stow-local-ignore` lives "within
   any top-level package directory" (manual, *Ignore Lists*). So stow *does* have
   per-package config — but only for ignore patterns. General per-package options
   (`.stowrc`) live in the current directory or `~/.stowrc`, not in packages
   (manual, *Resource Files*).
3. **Hooks — none.** No pre/post or script mechanism anywhere in the manual.
4. **Composable sources — no search path, but not cwd-bound either.** `-d`/`--dir`
   and the `STOW_DIR` environment variable select the stow directory without
   `cd`-ing into it (manual, *Invoking Stow*). Multiple stow directories are
   supported via `.stow` marker files (manual, *Multiple Stow Directories*), but
   each invocation targets one stow dir; there is no path-style package lookup.
5. **Binary — no.** "Stow is implemented as a combination of a Perl script
   providing a CLI interface, and a backend Perl module which does most of the
   work" (manual, *Introduction*). Requires a Perl runtime.

### dotbot

Source: official README, <https://github.com/anishathalye/dotbot> (fetched
2026-07-12).

1. **Convention over manifest — no; manifest-driven.** "Dotbot uses YAML or
   JSON-formatted configuration files to let you specify how to set up your
   dotfiles" (README, *Configuration*). Conventional file: `install.conf.yaml`.
   There is no manifest-free mode.
2. **Package-local config — no.** One central manifest at the repo root; tasks
   "are run in the order in which they are specified."
3. **Hooks — manifest-level only.** The `shell` directive runs commands ("Shell
   commands are run in the base directory", README, *Shell*) at the point it
   appears in the task list; custom directives come from plugins (`--plugin`
   option or config, README, *Plugins*). No package or directory hierarchy of
   hooks — there are no packages.
4. **Composable sources — no.** One config file / base directory per invocation;
   the README describes no multi-source composition.
5. **Binary — no.** Python program (PyPI); distributed as a git submodule
   (`git submodule add …/dotbot`, README, *Integrate with Existing Dotfiles*) **or**
   standalone via `pip`/`brew install dotbot` (README, *Standalone*). Either way a
   Python runtime is required.

### chezmoi

Sources (all fetched 2026-07-12): <https://www.chezmoi.io/why-use-chezmoi/>,
<https://www.chezmoi.io/reference/concepts/>,
<https://www.chezmoi.io/reference/source-state-attributes/>,
<https://www.chezmoi.io/reference/special-files/> (and
`…/special-files/chezmoiignore/`),
<https://www.chezmoi.io/reference/configuration-file/hooks/>.

1. **Convention over manifest — yes.** No manifest: "chezmoi stores the source
   state of files, symbolic links, and directories in regular files and
   directories in the source directory"; behavior is set entirely by filename
   attribute prefixes (`dot_`, `private_`, `exact_`, `run_`, `run_once_`,
   `run_before_`, `run_after_`, …) (*Source state attributes*). A config file is
   optional. The concept doc's framing of chezmoi as the "heavyweight" pole is
   fair (templating, secrets, encryption, machine state), but it is **not**
   manifest-driven — it sits on the convention side of the split.
2. **Package-local config — partially, within one tree.** No package concept
   (single source dir), but special files nest: ".chezmoiignore files in source
   state subdirectories apply only to that subdirectory"
   (*Special files → .chezmoiignore*); scripts can live in the tree or in
   `.chezmoiscripts/` (*Special files*).
3. **Hooks — two mechanisms, arguably two levels.** (a) In-tree `run_`
   scripts with `before_`/`after_`/`once_`/`onchange_` modifiers (*Source state
   attributes*). (b) Config-file hooks: pre/post for **every command** plus
   `git-auto-commit`, `git-auto-push`, and `read-source-state` events, with
   `CHEZMOI_COMMAND` etc. in the environment (*Configuration file → hooks*).
4. **Composable sources — no.** "The source directory is where chezmoi stores the
   source state. By default it is `~/.local/share/chezmoi`" (*Concepts*); a single
   directory, overridable but not composable as a search path.
5. **Binary — yes, exactly the claim.** "chezmoi is distributed as a single
   stand-alone statically-linked binary with no dependencies that you can simply
   copy onto your machine and run" (*Why use chezmoi?*). This is why claim 5 is
   not a unique differentiator.

### yadm

Sources: <https://yadm.io/docs/overview>, <https://yadm.io/docs/hooks>,
<https://yadm.io/docs/install>, and the script itself
<https://github.com/yadm-dev/yadm/blob/master/yadm> (fetched 2026-07-12).

1. **Convention over manifest — different model entirely.** yadm is "like having a
   version of Git, that only operates on your dotfiles" (*Overview*): a Git
   wrapper over files in `$HOME`, no symlinking, no packages, no manifest.
   Alternates (`##hostname` suffixes) and templates are filename conventions.
2. **Package-local config — n/a.** No package concept; config is the yadm repo
   plus `$HOME/.config/yadm/`.
3. **Hooks — global per-command, plus bootstrap.** "For every command yadm
   supports, a program can be provided to run before or after that command":
   `pre_`/`post_` + command name, stored in `$HOME/.config/yadm/hooks`, executable;
   a failing `pre_` hook aborts the command; env vars `YADM_HOOK_COMMAND`,
   `YADM_HOOK_EXIT`, etc. (*Hooks*). One global level only.
4. **Composable sources — no.** One repository ("use a single repository" is a
   stated design goal, *Overview*).
5. **Binary — no.** A single **Bash** script: the shebang is `/bin/sh` for
   portability but it immediately re-executes itself with bash
   (`[ -z "$BASH_VERSION" ] && exec bash "$0" "$@"`, script lines ~19–22), and it
   requires Git (`require_git`). The install docs' OpenWRT recipe lists
   `bash git git-http gnupg coreutils-chmod coreutils-stat` (*Install*). The
   built-in template processor "only depends upon awk" (yadm.md man source).

### rcm

Sources: <https://github.com/thoughtbot/rcm> (README),
<https://thoughtbot.github.io/rcm/rcup.1.html>,
<https://thoughtbot.github.io/rcm/rcrc.5.html>, and repo source
`bin/rcup.in` (fetched 2026-07-12).

1. **Convention over manifest — largely yes.** Directory layout in `~/.dotfiles`
   is the config; `tag-*` and `host-*` directory prefixes select subsets
   ("Tagged files go in a directory named for the tag, prefixed with tag-…";
   "Host-specific files go in a directory named for the host, prefixed with
   host-", rcup(1)). Central optional config `~/.rcrc` (rcrc(5)) tunes behavior
   but no per-file manifest exists.
2. **Package-local config — no.** Config is central (`~/.rcrc`, overridable via
   the `RCRC` env var; variables: `DOTFILES_DIRS`, `TAGS`, `EXCLUDES`,
   `COPY_ALWAYS`, `SYMLINK_DIRS`, `UNDOTTED`, `HOSTNAME` — rcrc(5)).
3. **Hooks — yes, per dotfiles directory.** "Two hooks are supported by rcup:
   pre-up and post-up. These go in files or directories with predictable
   filenames: .dotfiles/hooks/pre-up and .dotfiles/hooks/post-up, or
   …/pre-up/* and …/post-up/*" (rcup(1)); run alphabetically, expected idempotent.
   Because hooks live under each dotfiles dir and multiple dirs are supported,
   this is effectively repo-level hooks — closer to dstow's idea than any other
   tool surveyed.
4. **Composable sources — YES, first-class.** `DOTFILES_DIRS`: "the source
   directories for dotfiles. The first in the list is the source to which
   dotfiles created using mkrc(1) are installed. The default value is ~/.dotfiles"
   (rcrc(5)); `rcup -d DIR` "can be specified multiple times" (rcup(1)). This is
   precisely a composable source path, and it directly falsifies the concept
   doc's "nobody does this cleanly."
5. **Binary — no.** POSIX shell scripts: `bin/rcup.in` begins `#!@SHELL@` and
   sources `share/rcm/rcm.sh`. (GitHub's language stats showing "Perl 63%" are an
   artifact of the `.t` cram-style test files, not the tools.) Needs a POSIX sh.

---

## Corrections to the concept doc

Against `gostow-dstow-concept-2026-07-09.md`, "Comparison to the field"
(lines 140–163) and related claims:

1. **"`DSTO_PATH` composable sources — nobody does this cleanly" (line 157) —
   wrong.** rcm's `DOTFILES_DIRS` (rcrc(5)) is exactly a documented, first-class
   multi-source list, with repeatable `-d` on the CLI and defined precedence
   (first dir wins for `mkrc`). dstow's honest claim is the `$PATH`-style env
   var + XDG git-tap dir composing with it — an ergonomics refinement, not a
   category first.
2. **"GNU stow … cwd-bound" (line 144) — overstated.** Stow has `-d`/`--dir`,
   `STOW_DIR`, and `.stowrc` (manual, *Invoking Stow*, *Resource Files*); the cwd
   is only the *default* stow dir. What stow actually lacks is package lookup by
   name across roots — that, not "must cd," is dstow's location-independence win.
3. **"GNU stow … no per-package config" (line 144) — overstated.**
   `.stow-local-ignore` lives inside each package (manual, *Ignore Lists*). Stow
   has narrow (ignore-only) package-local config; dstow generalizes it, so say
   "no general per-package config," not "none."
4. **Claim 1 framing ("convention over manifest") — true only vs. dotbot.**
   chezmoi is convention-driven (filename attributes, no manifest; config file
   optional) and stow is pure convention. The manifest-driven camp in the
   surveyed field is dotbot alone. Positioning should be "convention-driven
   *with* packages, package-local config, and hooks," not convention per se.
5. **Claim 5 ("single standalone binary — no Perl, no Python") — not unique.**
   chezmoi ships as "a single stand-alone statically-linked binary with no
   dependencies" (*Why use chezmoi?*). The doc elsewhere concedes chezmoi is a
   single Go binary (line 148), so the differentiator list item should be scoped:
   "the only *stow-model* (symlink-farm) tool that is a single static binary."
6. **"dotbot … distributed as a committed git submodule" (line 146) — stale as
   an exclusive.** Submodule integration remains the flagship pattern, but dotbot
   also installs standalone from PyPI (`pip`) and Homebrew (README, *Standalone*).
   Python runtime still required either way, so the substance of the point stands.
7. **Claim 3 (multi-level hooks) — needs sharper wording.** rcm has repo-level
   pre/post hooks; chezmoi has both in-tree scripts and global per-command config
   hooks; yadm hooks every command globally. The genuine dstow novelty is hooks
   at *package + repo + global* levels within a package model — "others have no
   hooks" would be false; "no one has package-scoped hooks composed across
   levels" is true.
8. **yadm/rcm one-liner (line 149) — fine but thin.** "git-$HOME-centric /
   tag-and-host-based" is accurate as far as it goes; note both also have hook
   systems (yadm per-command global; rcm pre-up/post-up per dotfiles dir), which
   matters when claiming hooks as a differentiator.

**Net positioning that survives verification:** dstow's defensible, honest
differentiator is the *combination* — a stow-model, convention-driven package
tool with package-local config and hooks, multi-level hook composition, a
`DSTOW_PATH` search path, and single-static-binary distribution. Every individual
dimension exists somewhere in the field; no surveyed tool combines more than two.
