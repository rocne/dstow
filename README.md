# dstow

**Deploy dotfiles and configuration as symlinks, from packages in repos.**

dstow links the files in a *package* into a *target* directory (your `$HOME` by
default), the same way GNU Stow does — but it remembers what it deployed, works
across many repos at once, names things by a stable qualified identity, and
never depends on the directory you happen to be standing in.

- **Repos** are where packages come from — a local directory, or a remote
  clone (`github:owner/name`).
- **Packages** are the top-level directories inside a repo. Each package is a
  tree of files to link into the target.
- **Targets** are where the links go. The default is `$HOME`; any level of
  config can point elsewhere.

```
github:rocne/dotfiles::zsh
└─┬──┘ └─────┬──────┘  └┬┘
  │          │          package
  │          coordinate
  scheme
```

That fully-qualified name (FQN) is a package's stable identity. You rarely type
the whole thing — **any unambiguous suffix works** (`zsh`, `dotfiles::zsh`,
`rocne/dotfiles::zsh`), and **the working directory never changes what a
command does.**

---

## Table of contents

- [Install](#install)
- [Quickstart](#quickstart)
- [The model](#the-model)
- [Naming](#naming)
- [Commands](#commands)
- [Configuration](#configuration)
- [Migrating from GNU Stow](#migrating-from-gnu-stow)
- [Theming](#theming)
- [Hooks](#hooks)
- [Reference](#reference)

---

## Install

### Bootstrap snippet (recommended)

Add dstow's bootstrap to your shell rc. It puts `~/.local/bin` on `PATH` and
installs dstow **only if it is missing** — when dstow is already present the
snippet is silent, invisible, and offline (the installer is never even
fetched):

```sh
dstow snippet rc >> ~/.bashrc      # or ~/.zshrc, ~/.profile, …
```

The emitted snippet is plain POSIX sh — dstow never edits your rc files, it only
prints the text to stdout for you to redirect. On a fresh machine where dstow is
not yet installed, run the snippet's install line directly:

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dstow/main/install.sh | sh -s --
```

The installer drops `dstow` into `~/.local/bin`. It is idempotent: run against a
machine that already has dstow, it prints one status line and exits 0.

**Installer flags:** `--force` reinstalls even when present; `--version vX.Y.Z`
ensures exactly that release — already installed at that version it exits 0,
otherwise it installs it (no implied force). The install dir is tunable
(`--install-dir`, `DSTOW_INSTALL_DIR`, `XDG_BIN_HOME`); `~/.local/bin` is the
default the snippet relies on. Downloads are checksum-verified always, and
cosign-verified when cosign is available. See `install.sh --help` for the
full surface.

Add installer args right after the `--`, such as `--force` or
`--version <version>`:

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dstow/main/install.sh | sh -s -- --force
```

### Linux packages (dnf / apt)

Signed `.rpm` and `.deb` packages are published to a hosted repo, so dstow
installs by name and stays current through your normal system upgrades. Add the
repo once, then install:

```sh
# Fedora, RHEL, CentOS Stream, openSUSE, … (dnf/yum)
curl -1sLf 'https://dl.cloudsmith.io/public/rocne/releases/setup.rpm.sh' | sudo -E bash
sudo dnf install dstow
```

```sh
# Debian, Ubuntu, … (apt)
curl -1sLf 'https://dl.cloudsmith.io/public/rocne/releases/setup.deb.sh' | sudo -E bash
sudo apt install dstow
```

The setup script drops a repo file into `/etc/yum.repos.d` (or
`/etc/apt/sources.list.d`) and imports the repo's index-signing key, so
`dnf upgrade` / `apt upgrade` pick up new dstow releases automatically. The
packages themselves are GPG-signed by a separate key (fingerprint `64894FE3…`),
whose public half is attached to every GitHub release as
`dstow-signing-key.asc` if you want to verify a download by hand.

### With Go

If you have a Go toolchain, install straight from source — the binary reports
the module version it was built from:

```sh
go install github.com/rocne/dstow/cmd/dstow@latest
```

### Shell completion

```sh
dstow completion bash        # also: zsh, fish, powershell
```

See `dstow completion --help` for where to place the output for your shell.

---

## Quickstart

```sh
# 1. Register a repo of packages (clones a remote; registers a local path in place).
dstow repo add github:rocne/dotfiles
dstow repo add ~/my-dotfiles

# 2. See what you have (reads config only — never touches disk).
dstow list                       # your repos
dstow list dotfiles              # a repo's packages

# 3. Deploy. Name packages, or a whole repo, by any unambiguous suffix.
dstow stow zsh git tmux
dstow stow dotfiles              # a repo: all of its packages

# 4. Check reality.
dstow status                     # what is actually deployed, live

# 5. Later: refresh, or remove.
dstow restow zsh                 # unstow then stow (pick up changes)
dstow unstow tmux                # remove tmux's links
```

First run on a machine that already has live config files? Adopt them into the
package as you stow, so nothing is destroyed:

```sh
dstow stow --all --adopt
```

---

## The model

dstow keeps three things straight, and gives you a command for each question:

| Question | Command | Reads |
|---|---|---|
| *What do I have configured?* | `dstow list` | config only, never disk |
| *What does dstow know about this?* | `dstow info` | config + metadata, never disk |
| *What is actually deployed?* | `dstow status` | the live filesystem |

- A **repo** is registered once (`repo add`) and remembered in dstow's own
  registry — you never hand-edit that file. A repo is either **managed** (a
  remote dstow cloned for you) or a **local path** (registered in place and
  never modified by dstow).
- A **package** is a top-level directory in a repo. By default packages live at
  the repo root; a repo may instead collect them under a subdirectory (see
  [`packages_dir`](#configuration)).
- The **target** is where links go — `$HOME` unless configured otherwise.
- dstow records every link it makes in a **ledger**, so `status`, `check`, and
  `clean` can reason about what *should* be there versus what *is*.

Each package succeeds or fails on its own: a bulk run continues past failures
and exits non-zero if any package failed.

---

## Naming

Every repo and package has a fully-qualified name:

```
github:rocne/dotfiles          # a repo
github:rocne/dotfiles::zsh     # a package in it
local:/home/you/dots::vim      # a package in a local repo
```

- **Refer to anything by any unambiguous suffix** of its FQN. `zsh`,
  `dotfiles::zsh`, and the full form all name the same package — as long as the
  suffix is unique. When it is not, dstow refuses and lists the qualified
  spellings to pick from.
- **`::` forces the package/repo boundary.** `dots::zsh` means "the package
  `zsh` in a repo whose name ends `dots`", never a repo called `dots::zsh`.
- **A scheme prefix requires the full coordinate** (`github:rocne/dotfiles`,
  not `github:dotfiles`).
- **The working directory is irrelevant.** dstow never guesses from where you
  are standing.

Paths and names never collide: an operand that looks like a path (`~/.zshrc`,
`./x`, `/etc/...`) is treated as a path; everything else is a name expression.

---

## Commands

Run `dstow <command> --help` for the full help and examples of any command.

### Deploy

| Command | What it does |
|---|---|
| `dstow stow <name>… \| --all` | Link packages into their targets |
| `dstow unstow <name>… \| --all` | Remove packages' links |
| `dstow restow <name>… \| --all` | Unstow then stow — the idempotent refresh |
| `dstow adopt <file> [<package>]` | Import an existing file into a package, leaving a link behind |

- Names are packages **or** repos; naming a repo acts on all of its packages.
- With no names, dstow asks before acting on everything. In a script that is an
  error — pass `--all`.
- Explicitly naming a package overrides its `exclude_from_bulk` setting.
- `restow` on a not-stowed package simply stows it (the unstow phase no-ops).

**Flags:** `--all` (every package of every repo, no prompt) · `--adopt`
(on stow/restow: adopt a real file found at an expected path instead of
refusing) · `-n`/`--dry-run` (show the plan, change nothing).

`adopt` — live content always wins; adopt never destroys running configuration:

```sh
dstow adopt ~/.zshrc zsh                 # import one file into the zsh package
dstow adopt ~/.config/foo/foo.toml       # no package named: pick from ranked candidates
dstow adopt --occupied zsh               # adopt every occupied path of a package
```

`adopt` flags: `--occupied` · `-n`/`--dry-run` · `--force` (overwrite differing
package content without asking).

### Inspect

```sh
dstow list [<name>]        # configured content: repos ⊃ packages ⊃ paths
dstow info [<name>]        # every field dstow holds about one scope
dstow status [<name>…]     # live deployment state
dstow status <path>        # what occupies a path, who owns it, adoption candidates
```

- **`list`** enumerates a scope's content and never inspects disk. Flags:
  `--repos`, `--packages`, `--json`.
- **`info`** reads one scope's fields — inherent facts (version, paths, source,
  scheme, qualified name) and effective config (target, dot-translation, fold,
  ignores) — from configuration and metadata, never by touching targets. Flags:
  `-f`/`--field <field>` (repeatable), `-r`/`--recurse`, `--json`.
- **`status`** inspects reality. Package states: `stowed`, `partially stowed`,
  `not stowed`, `occupied`, `damaged`, plus a `drifted` marker when the deployed
  shape differs from what current config would produce. Remote repos also show
  behind/ahead as of the last `repo update`. Flag: `--json`.

### Maintain

```sh
dstow check      # verify every ledgered link; classify broken and orphaned
dstow clean      # execute exactly what check reported
dstow rebuild    # reconstruct a lost ledger by walking configured targets (rare)
```

- **`check`** is instant (no tree walk) and read-only. It classifies stale
  links as **broken** (destination gone) or **orphaned** (resolves into a known
  repo, but no current config would produce it), and reports **contradicted**
  ledger entries (disk disagrees with the record). Flag: `--json`.
- **`clean`** executes exactly check's report — the two can never disagree.
  Broken links are removed freely; orphans are confirmed (or `--yes` / `--force`
  to remove without asking). Contradicted entries are pruned, disk untouched.
- **`rebuild`** is the only full tree walk dstow has — explicit and rare, for
  when the ledger is lost.

### Groups

```sh
dstow repo add <source> [--stow]     # register a repo (path, URL, github:owner/name)
dstow repo remove <repo> [--unstow] [--force]
dstow repo update [<repo>…]          # download remote changes; touch nothing on disk
dstow repo upgrade [<repo>…]         # fast-forward clean clones to what update fetched

dstow snippet rc                     # print the shell-rc bootstrap snippet
dstow theme list                     # list available themes
dstow theme slots                    # describe every color slot and the value grammar
dstow theme emit [<name>] [slot=value ...] [--format env|toml]   # render / emit colors
```

- **`repo update` and `repo upgrade` are two explicit phases; neither runs on
  its own.** `update` touches the network and nothing else — afterwards
  `status` shows behind/ahead. `upgrade` is fast-forward only: divergence or
  local work refuses loudly, with no stash/merge/rebase, and never re-stows
  (structural drift shows up in `status`). Update, review, then upgrade.
- **`repo remove`** deletes managed clones but only *forgets* local-path repos
  (your directory is never touched). It refuses while the repo still has stowed
  links (offering `--unstow`) and refuses to delete a managed clone holding
  work not present at its source (`--force` overrides both).

### Also

```sh
dstow completion <shell>    # bash | zsh | fish | powershell
dstow version               # print version
```

### Global flags

Available on every command:

| Flag | Meaning |
|---|---|
| `--color <when>` | `auto` (default), `always`, or `never` — the value is required |
| `-q`, `--quiet` | Suppress routine chatter; announcements, warnings, and errors always survive |
| `-y`, `--yes` | Assume "yes" at confirmations of stated intent (never resolves ambiguity, answers a bulk prompt, or bypasses a guard) |
| `-h`, `--help` | Help for dstow or any command |

`-n`/`--dry-run` is available on `stow`/`unstow`/`restow`/`adopt`. `--json` is
available on `list`/`info`/`status`/`check`.

---

## Configuration

dstow reads TOML config from four levels; nearer levels win, and `ignore` chains
are **additive** (a level adds to, never silences, inherited ignores). You write
every config file yourself except the repo registry, which dstow owns.

| File | Location | Written by |
|---|---|---|
| Global config | `$XDG_CONFIG_HOME/dstow/config.toml` | you |
| Repo config | `<repo>/.dstow/config.toml` | you |
| Package config | `<repo>/<pkg>/.dstow/config.toml` | you |
| Repo registry | `$XDG_CONFIG_HOME/dstow/repos.toml` | **dstow** — never hand-edit |
| User theme presets | `$XDG_CONFIG_HOME/dstow/themes/<name>.toml` | you |

Keys are `snake_case`. CLI flags use `kebab-case` — the one deliberate spelling
mismatch. There is no config-mutation command: you declare configuration by
editing the file, like every other knob.

### Keys and where they are legal

| Key | Type / default | package | repo | global |
|---|---|:---:|:---:|:---:|
| `target` | path — default `$HOME` | ✓ | ✓ | ✓ |
| `translate_dot_prefixes` | bool — default `true` | ✓ | ✓ | ✓ |
| `ignore` | list of patterns (additive) | ✓ | ✓ | ✓ |
| `exclude_from_bulk` | bool — default `false` | ✓ | ✓ | — |
| `packages_dir` | repo-relative path | — | ✓ | — |
| `fold_trees` | bool — default `false` | — | — | ✓ |
| `[color]` table + `theme` | see [Theming](#theming) | — | — | ✓ |

- **`target`** — where this scope's links go. Path values expand `~` and
  `$VAR`/`${VAR}` (evaluated per invocation); the result must be absolute. An
  unset variable is a loud error naming the variable, file, and key.
- **`translate_dot_prefixes`** — translate a leading `dot-` in filenames to `.`
  (the stow `--dotfiles` convention). On by default.
- **`exclude_from_bulk`** — keep a package out of `--all` and repo-wide runs;
  naming it explicitly still acts on it.
- **`packages_dir`** — opt-in, repo level only: a repo-root-relative directory
  where the repo's packages live. Unset means packages sit at the repo root.
  `"packages"` is the recommended convention for fresh repos.
- **`fold_trees`** — GNU Stow's "tree folding". Off by default.

### Ignores

The `ignore` key carries **gitignore-glob** patterns, matched per package
against package-root-relative paths (no slash = basename at any depth; a leading
slash anchors; a trailing slash is directory-only; `**` is supported):

```toml
ignore = ["*.log", "/build/", "**/__pycache__"]
```

Leading `!` (negation) and leading `//` are refused and reserved. dstow always
ignores a package's own root `.dstow/` directory — that is not a config knob.

### Unknown or misplaced keys

dstow warns, never refuses: an unknown key gets a did-you-mean; a key that is
legal at a different level names that level. These warnings survive `--quiet`.

---

## Migrating from GNU Stow

dstow reads your existing stow configuration — you do not have to rewrite it to
start.

- **`.stow-local-ignore` and `.stowrc` are honored** as compat files, parsed
  quirk-faithfully. A native `config.toml` whose content is flag-lines (the
  first token starts with `-`) is routed to the compat parser with a loud
  announcement naming the native equivalent.
- **`~/.stowrc`'s `--dir`** contributes a **session repo** for the current
  shell, announced, with a `fix:` suggesting a permanent `repo add`. A
  repo-level `--dir` is warned and ignored.
- **Option mapping:** `--target` → `target`; `--no-folding` → `fold_trees =
  false`; `--dotfiles` → `translate_dot_prefixes = true`; `--ignore` → an
  additive `ignore` entry (in stow-regex, its native language). Options dstow
  does not map (`--adopt`, `--override`, `--defer`, verbosity, simulate) are
  warned-and-ignored per option, naming why and the native remedy. The file
  always runs; degradation is loud, never a rejection.
- **Supplement mode:** when both a stow rc and native config set the same knob,
  non-overlapping and equal values are silent; a genuine conflict is resolved
  **native-wins** with a loud warning naming the level, files, knob, values, and
  winner, plus a `fix:` suggesting removal from the rc. `ignore` chains never
  conflict — they are additive and per-language.

A migrated stow repo works out of the box: package identity is locational, there
are no marker files to add, and `mkdir`-and-go stays intact.

---

## Theming

dstow colorizes its output through a **two-stage vocabulary**: themes speak
fourteen generic slots (`error1`, `success2`, `section1`, …), and dstow's
internals — package states, severity prefixes, help roles — map onto those
slots in code, so a theme never needs to know what "orphaned" means. Every
commentary line carries a greppable word prefix (`note:`, `warning:`,
`error:`, `fix:`) so output stays meaningful with color off. Defaults use only the 16 base ANSI colors, so **your terminal theme
re-themes dstow automatically**, and colorblind/low-vision users retheme through
terminal preferences.

Color is enabled by a fixed precedence — `--color` > `NO_COLOR` >
`CLICOLOR_FORCE` > `CLICOLOR` > TTY detection — and theme choices are strictly
downstream: a theme can never re-enable color the chain turned off.

Themes layer, top wins:

1. **`DSTOW_COLORS`** — the one theming environment variable; packed per-slot
   overrides in an `LS_COLORS`-family syntax, values in git's `color.*` grammar:

   ```sh
   export DSTOW_COLORS='error1=bold red:success2=#a6e3a1'
   # or generate a whole theme:
   export DSTOW_COLORS=$(dstow theme emit catppuccin-mocha --format env)
   ```

2. **The `[color]` table** in global config — one key per slot, same grammar:

   ```toml
   [color]
   success2 = "#a6e3a1"
   error1 = "bold red"
   ```

3. **The `theme` config key** — a bare string is a theme *name* (your themes dir
   first, then the bundled presets); a path form is a theme *file* anywhere,
   including inside a repo (so a repo can ship its own theme):

   ```toml
   theme = "catppuccin-mocha"
   # theme = "~/themes/mine.toml"
   ```

   The bundled presets are the four [catppuccin](https://github.com/catppuccin/catppuccin)
   flavors — `catppuccin-latte`, `catppuccin-frappe`, `catppuccin-macchiato`,
   `catppuccin-mocha` — generated from one
   [Whiskers](https://github.com/catppuccin/whiskers) template, plus two
   ANSI-16 ports of established CLI schemes: `cargo`
   ([Cargo's help styling](https://github.com/crate-ci/clap-cargo)) and
   `fang-ansi` ([charmbracelet/fang](https://github.com/charmbracelet/fang)'s
   `AnsiColorScheme`). The ports declare only the slots their source
   specifies; the rest falls through this stack.

4. **The default ANSI-16 palette** — cargo-grounded: it declares the seven
   tier-1 slots (heading, name, value, and the four message families), and
   every tier-2 derives automatically. Because defaults stay within the 16
   named ANSI colors, your terminal theme supplies the actual colors.

Discover, inspect, and emit themes with the `theme` group — the packed string,
the config table, and theme files share one slot vocabulary and one value
grammar, so they are losslessly convertible:

```sh
dstow theme list                                          # the roster: name, origin, active
dstow theme slots                                         # every slot, what it colors, its consumers
dstow theme emit                                          # the effective palette, rendered
dstow theme emit catppuccin-mocha                         # a named theme, rendered
dstow theme emit catppuccin-mocha --format env            # packed DSTOW_COLORS string
dstow theme emit cargo section1='bold yellow' --format toml \
  > ~/.config/dstow/themes/mine.toml                      # a tweaked theme file
```

The color slots come in two groups. Content: `section1` `section2` (headings),
`name1` `name2` (names), `value1` `value2` (values and placeholders). Messages:
`error1` `error2` `warning1` `warning2` `success1` `success2` `info1` `info2` —
four families, two prominence tiers each (1 is louder). A slot you leave
undeclared derives from its family's tier-1 (bold is removed, or dim added), so
a sparse theme stays coherent. A theme file is exactly the bare `[color]`
schema — no wrapper keys. `dstow theme slots` prints the whole vocabulary with
each slot's consumers, and its `--help` carries the full value grammar.

### Color values

Slot values are git's `color.*` grammar — whitespace-separated words, in any
order:

| Form | Words | Notes |
|---|---|---|
| Foreground / background | a color word | first color is the foreground, the second the background; a third is an error |
| Basic colors | `black` `red` `green` `yellow` `blue` `magenta` `cyan` `white` | plus their `bright*` variants (`brightred`, …) |
| 256-color | an integer `0`–`255` | |
| True color | `#RRGGBB` hex | |
| `normal` | leaves a channel to the **terminal** | see below |
| `default` | resets a channel to the terminal default | emits `SGR 39/49` |
| Attributes | `bold` `dim` `italic` `ul` `blink` `reverse` `strike` | any number; each negatable with `no`/`no-` (`nobold`), which renders as nothing |
| `reset` | clears first | |

Background without touching the foreground: put `normal` in the foreground slot
— `normal red` is a red background, foreground left alone.

`normal` and `default` are not the same, and neither means "dstow's default":

- **`normal`** leaves the channel to the terminal and emits no code. A slot you
  declare `normal` is a declaration like any other — it **replaces** dstow's
  default for that slot wholesale (themes are top-wins), so the slot renders
  plain.
- **`default`** actively emits the terminal-default code (`SGR 39/49`) rather
  than nothing.
- The only way to keep **dstow's** default for a slot is to leave it
  undeclared, so it falls through the stack to the default palette.

---

## Hooks

Run your own executables around deploy actions. Hooks live in a `hooks/`
directory inside `.dstow/`, at any of the three levels:

```
<repo>/<pkg>/.dstow/hooks/      # package-level
<repo>/.dstow/hooks/            # repo-level
$XDG_CONFIG_HOME/dstow/hooks/   # global
```

Eight git-style per-event executables, one file per event:

```
pre-stow    post-stow
pre-unstow  post-unstow
pre-restow  post-restow
pre-adopt   post-adopt
```

- Each hook is `exec`'d directly — the shebang chooses the interpreter — and
  must be executable (a correctly-named but non-executable file is warned with a
  `chmod +x` hint; a misspelled name gets a did-you-mean).
- **`restow` fires only the restow pair** (not stow/unstow).
- The working directory is the scope's own directory (package dir, repo dir, or
  the global config dir).
- Subdirectories of `hooks/` are inert helper space — never fired, never warned;
  `lib/` is the documented convention for shared scripts.
- `<event>.d/` directories are reserved (inert-and-warned in v1; run-parts
  drop-ins are committed for v2).

### Hook context

dstow passes context as `DSTOW_HOOK_*` environment variables (no arguments, no
stdin). A variable is **absent**, never empty, when it does not apply:

| Variable | Value | pkg | repo | global |
|---|---|:---:|:---:|:---:|
| `DSTOW_HOOK_LEVEL` | `package` / `repo` / `global` | ✓ | ✓ | ✓ |
| `DSTOW_HOOK_ACTION` | `stow` / `unstow` / `restow` / `adopt` | ✓ | ✓ | ✓ |
| `DSTOW_HOOK_PHASE` | `pre` / `post` | ✓ | ✓ | ✓ |
| `DSTOW_HOOK_FQN` | this scope's FQN (canonical-encoded) | ✓ | ✓ | — |
| `DSTOW_HOOK_SCHEME` | e.g. `github` (decoded) | ✓ | ✓ | — |
| `DSTOW_HOOK_COORDINATE` | e.g. `rocne/dotfiles` (decoded) | ✓ | ✓ | — |
| `DSTOW_HOOK_PACKAGE` | bare package name (decoded) | ✓ | — | — |
| `DSTOW_HOOK_PACKAGE_DIR` | absolute path | ✓ | — | — |
| `DSTOW_HOOK_TARGET` | effective target root (absolute) | ✓ | — | — |
| `DSTOW_HOOK_REPO_FQN` | the repo's FQN (canonical-encoded) | ✓ | ✓ | — |
| `DSTOW_HOOK_REPO_DIR` | absolute path | ✓ | ✓ | — |
| `DSTOW_HOOK_PACKAGES` | FQNs acting under this scope, newline-separated | — | ✓ | ✓ |

("Coordinate" is used in the Maven sense — the parts that locate a repo.) FQN
values are percent-encoded so they paste straight back into dstow commands;
decomposed segments carry decoded real values. Iterate `DSTOW_HOOK_PACKAGES`
with `while IFS= read -r pkg; do …; done`.

- **Both hook output streams go to dstow's stderr; stdin passes through.** A
  hook's output is commentary, never dstow's answer — a hook that must emit data
  writes a file.
- **Write commands refuse from inside a hook; reads are fully allowed.** A hook
  may run `dstow status --json` or `dstow info -f target` to read state, but
  deploy verbs, `adopt`, `clean`, `rebuild`, and repo mutations refuse (detected
  via `DSTOW_HOOK_ACTION` in the environment).

---

## Reference

### Environment variables

| Variable | Effect |
|---|---|
| `DSTOW_PATH` | Colon-separated **absolute local directory paths**, registered as session repos for this shell only (PATH convention; no priority, no qualified sources, no `~`/`$VAR` expansion — relative entries are refused loudly) |
| `DSTOW_COLORS` | Packed per-slot theme overrides (see [Theming](#theming)) |
| `NO_COLOR` | Disables color (standard) |
| `CLICOLOR` / `CLICOLOR_FORCE` | Standard color enable/force signals, below `--color` in precedence |
| `XDG_CONFIG_HOME` | Config + registry + themes root (`…/dstow/`); defaults to `~/.config` |
| `XDG_STATE_HOME` | Ledger location (`…/dstow/`); defaults to `~/.local/state` |
| `XDG_DATA_HOME` | Managed clones (`…/dstow/repos/…`); defaults to `~/.local/share` |
| `DSTOW_HOOK_*` | dstow's output lane to hooks — set by dstow, read by your hooks (see [Hooks](#hooks)) |

Managed clones live at
`$XDG_DATA_HOME/dstow/repos/<scheme>/<owner>/<name>` (percent-encoded, so links
point into a filesystem-safe, canonical location).

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Negative answer — a package failed, a requested field is unset/empty, or `check` found findings |
| `2` | Usage error — a bad flag, wrong argument count, or unknown command |
| `3` | Refusal / environment — non-interactive ambiguity, a corrupt or newer-than-known ledger, or lock contention |

### The `name` group

A hidden utility group for scripting the naming grammar directly. It operates on
one coordinate *segment*, percent-encoding the characters the grammar reserves
(`:`, `@`, …) and leaving ordinary ones alone:

```sh
dstow name encode 'weird:name'    # -> weird%3Aname
dstow name decode 'weird%3Aname'  # -> weird:name
```

### Files at a glance

| Path | What |
|---|---|
| `$XDG_CONFIG_HOME/dstow/config.toml` | your global config |
| `$XDG_CONFIG_HOME/dstow/repos.toml` | the dstow-owned repo registry |
| `$XDG_CONFIG_HOME/dstow/themes/` | your theme presets |
| `$XDG_CONFIG_HOME/dstow/hooks/` | global hooks |
| `$XDG_STATE_HOME/dstow/` | the ledger |
| `$XDG_DATA_HOME/dstow/repos/` | managed clones |
| `<repo>/.dstow/` | repo config + hooks |
| `<repo>/<pkg>/.dstow/` | package config + hooks |

---

Everything above is the v1 surface. `dstow <command> --help` is the
authoritative, always-current reference for any command's flags and examples.

## License

dstow is licensed under the [GNU General Public License v3.0](LICENSE).
Copyright (c) 2026 Rocne Scribner.
