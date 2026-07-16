# dstow v1 — Design

**What this document is.** The complete, binding design of dstow v1 — the
synthesis of every resolution on the [design map](https://github.com/rocne/dstow/issues/16).
[REQUIREMENTS.md](REQUIREMENTS.md) pins *behavior*; this document pins
everything REQUIREMENTS delegated: CLI surface, config format and schema, the
metadata directory, the naming grammar, the ledger, output design, the hook
context contract, bootstrap and installer mechanics, the Go architecture, and
release wiring. Where REQUIREMENTS speaks, this document obeys; where this
document speaks, implementation obeys. Implementation is intended to be a
decision-free execution effort.

**Pinned vs. internal.** Persistent formats are always pinned fully — the
ledger schema, the config schema, the hook contract, anything user files or
user scripts depend on (migration cost makes them non-internal). Ephemeral
internals (in-memory mechanics, file organization inside a package) belong to
implementation. Presentation specifics called *provisional* below may be
revisited at implementation without a design amendment.

**Vocabulary.** This document speaks [CONTEXT.md](../CONTEXT.md)'s canonical
language. Rationale lives in each section's ticket — see
[Provenance](#12-provenance). If this document is found to disagree with a
ticket resolution as amended, that is a bug in this document.

---

## 1. Naming grammar

*Resolution: [Naming grammar: sources, repos, packages (#20)](https://github.com/rocne/dstow/issues/20).*

### 1.1 The grammar

**`scheme:coordinate::package`** — one grammar for sources, repos, and
packages. A repo's FQN drops the `::package` tail.

- **`:` separates only the scheme; `::` separates only the package.** The
  coordinate is path-shaped (one or more `/`-segments); schemes may interpret
  theirs (`github:` reads `owner/name`; `local:` reads an absolute filesystem
  path, canonicalized at add time) but the grammar itself does not. `X:Y` is
  always scheme-then-coordinate — an unknown scheme is a clean §1.4 error
  listing known schemes.
- **Suffix resolution**: any suffix cut at a segment boundary that resolves
  uniquely is a valid name (`zsh` → `dots::zsh` → `rocne/dotfiles::zsh` →
  FQN). A leading `::` forces package-kind (`::zsh`).
- **Ambiguity is exclusively a property of user input** — the model never
  contains it; same-named packages from different sources coexist. A name
  matching multiple entities gets the §1.2 treatment: interactive explicit
  choice; non-interactive hard error naming the qualified spellings.
  Cross-kind ties (a bare name matching both a repo and a package) follow the
  same rule.
- **Naming a repo where packages are expected means all its packages.**
- **The repo set is unordered** (REQUIREMENTS §2.3–2.5 as amended): bulk spans
  all packages of all registered repos; `DSTOW_PATH` contributes session repos
  with no priority.

### 1.2 Encoding

Reserved characters percent-encode — every path is spellable, zero
exceptions. dstow always emits the canonical encoded form itself; users paste
rather than construct.

| Reserved | Encoding | Why |
|---|---|---|
| `:` | `%3A` | scheme separator |
| `%` | `%25` | the escape itself |
| `@` | `%40` | reserved scheme-interpreted suffix (below) |
| control characters | `%0A` etc. | `DSTOW_HOOK_PACKAGES` is newline-separated; a pathological name must not corrupt the list |

**The `@` reservation (decided at synthesis, 2026-07-16).** `@` is reserved
in coordinates as an **optional, scheme-interpreted suffix**:
`scheme:coordinate@suffix::package`. v1 assigns it no semantics — a literal
`@` in a name expression must be written `%40`. What an `@`-suffix *means*
belongs entirely to any scheme that later chooses to interpret it; the
anticipated lineage is revision pinning (`pkg@version`, npm/brew/go-modules),
but the reservation binds only the syntax, never that meaning. Reserving now
makes future adoption non-breaking; schemes that ignore it lose nothing.

At `repo add`, a path that would need encoding triggers an interactive
continue-or-rename confirmation (continue-affirmative polarity, §7.2);
non-interactively it proceeds with a loud announcement.

### 1.3 Names vs. paths

A **path operand** starts with `/`, `~/`, `./`, or `../` and always refers to
the *target* world. A **name expression** is anything else and resolves
against the registry. The first character decides; no command ever guesses.

### 1.4 Identifier resolution and aliasing

Identifier resolution is **one generic system**: canonical identifiers plus
optional input-side aliases, resolving through a single path with §1.2
ambiguity semantics; output is always canonical. Field names are one
application of it (naming resolution above is another). **v1 ships zero
aliases** — the system degenerates to an interface over an identity resolve,
and nothing speculative is built — but the rule binds so that v2 aliases
(field shorthands, prefix resolution) arrive in an already-specified story.
*(F12 on [#27](https://github.com/rocne/dstow/issues/27); instance-set emptied
by the dependency removal, doorway kept at synthesis.)*

### 1.5 Display and utilities

- **Display rule (O9)**: shortest-unique suffix everywhere by default; full
  FQN whenever showing a tie, an ambiguity error, or `--json` (which always
  carries FQNs). Local coordinates display with `~` abbreviation.
- **Hidden `name` utility group**: `dstow name encode` / `dstow name decode` —
  absent from top-level help, documented in the manual. Natural future home
  for `name resolve`.
- Shell brace expansion gives multi-select for free:
  `dstow stow dotfiles::{bash,zsh}`.

## 2. CLI surface

*Resolution: [CLI surface: commands, flags, help text (#22)](https://github.com/rocne/dstow/issues/22);
full decision ledger D1–D16 there. Help text below is the approved canonical
text (prototype v2 as amended through 2026-07-16).*

### 2.1 The shape rule

Every command is a **group** (subcommands only, no operands; bare group
prints its help) or a **leaf** (operands only, never subcommands) —
operand/subcommand collision is impossible by construction. Groups are nouns
(`repo`, `snippet`, `colors`); leaves are verbs plus the established query
names (`list`, `info`, `status`).

- Groups: `repo` {add, remove, update, upgrade} · `snippet` {rc} ·
  `colors` {theme} · hidden `name` {encode, decode}.
- Leaves: `stow` `unstow` `restow` `adopt` · `list` `info` `status` ·
  `check` `clean` `rebuild` · `completion` `version`.

### 2.2 Standing principles

- **A flag never changes a command's concept** (D5).
- **Remedy pre-acceptance** (D15): a flag may pre-accept the exact remedy its
  refusal would name (`stow --adopt`, `repo remove --unstow`).
- **Long-canonical-plus-alias naming; glossary terms canonical** (D7).
- **No CLI flag twins for config knobs** (D12): no `--target`,
  `--no-folding`, `--ignore` flags; refusals name the knob and its file.
- **No config-mutation commands**: declare by editing the metadata file (like
  every knob); write commands are deferred to the v2 property store, where
  they can arrive as a designed family.
- **Three read surfaces**: `list` enumerates a scope's content
  (global ⊃ repos ⊃ packages ⊃ paths — the registry is the global scope's
  content); `info` reads one scope's fields (config+metadata, never disk);
  `status` inspects reality.
- `--yes` pre-answers confirmations of *stated intent* only; it never
  resolves ambiguity, never answers the bulk prompt, never bypasses guards
  (D2/D9). `--force` exists per-command only where REQUIREMENTS names an
  override.
- `-n/--dry-run` on stow/unstow/restow/adopt (D8). `--json` per-command:
  list, info, status, check (D13).
- `clean` takes no scope flags; it executes exactly check's plan (D10).
- `restow` on a not-stowed package just stows — the unstow phase no-ops;
  restow is the idempotent refresh verb (D16).

### 2.3 Top-level help (canonical)

```
dstow — deploy dotfiles and configuration as symlinks, from packages in repos

Usage:
  dstow <command> [args] [flags]

Deploy:
  stow        Link packages into their targets
  unstow      Remove packages' links from their targets
  restow      Unstow, then stow again (refresh links)
  adopt       Import an existing file into a package, leaving a link behind

Inspect:
  list        What is configured: repos, packages, targets (never reads disk)
  info        Everything dstow knows about one repo or package
  status      What is deployed: live state of packages against their targets

Maintain:
  check       Verify every link in the ledger; classify broken and orphaned
  clean       Execute exactly what check reported (broken freely, orphans ask)
  rebuild     Reconstruct a lost ledger by walking configured targets (rare)

Groups:
  repo        Manage repos: add, remove, update, upgrade
  snippet     Print canned shell snippets: rc bootstrap
  colors      Theming utilities: emit a theme for your session or a file

Also:
  completion  Generate shell completion (bash, zsh, fish, powershell)
  version     Print version

Global flags:
      --color <when>   Colorize output: auto (default), always, never
  -q, --quiet          Suppress informational output (announcements survive)
  -y, --yes            Assume "yes" at confirmation prompts
  -h, --help           Help for dstow or any command

Name packages and repos by any unambiguous suffix of their qualified name
(github:rocne/dotfiles::zsh). The working directory never changes what a
command does. See 'dstow <command> --help' for details and examples.
```

*(The `snippet` line reads "rc bootstrap" only and the `colors` group
appears, per the dependency removal ([#28](https://github.com/rocne/dstow/issues/28) B8)
and the Output design surface addition ([#26](https://github.com/rocne/dstow/issues/26)).)*

### 2.4 Per-command help (canonical)

#### stow / unstow / restow (leaves)

```
Link packages into their targets.

Usage:
  dstow stow <name>... [flags]
  dstow stow --all

Names are packages or repos, by any unambiguous suffix of the qualified
name; naming a repo stows all of its packages. With no names, dstow asks
before stowing everything — in scripts that is an error: pass --all.

Each package succeeds or fails on its own; the run continues past failures
and exits nonzero if any package failed. Explicitly naming a package or
repo overrides its exclude-from-bulk setting.

Flags:
      --all       Every package of every registered repo, without asking
      --adopt     Where an expected path holds a real file, adopt it into
                  the package instead of refusing (adopt rules apply:
                  plan shown, confirmation on differing content)
  -n, --dry-run   Show the plan; change nothing

Examples:
  dstow stow zsh git tmux
  dstow stow dotfiles              # a repo: all of its packages
  dstow stow dots::zsh             # qualified just enough to be unique
  dstow stow --all --adopt         # first run on a machine with live files
```

`unstow` and `restow` are identical in shape; bare `unstow` confirms with the
same care — it is the destructive one. `restow` on a package that is not
stowed simply stows it (the unstow phase no-ops); `--adopt` composes with
stow and restow.

#### adopt (leaf)

```
Import an existing real file into a package; a link takes its place.
Live content always wins — adopt never destroys running configuration.

Usage:
  dstow adopt <file> [<package>] [flags]
  dstow adopt --occupied <package>

With a package: shows its plan and asks before overwriting differing
package content. Without a package: lists the packages that could adopt
the file, ranked, and asks you to pick (in scripts: an error that lists
the candidates as remedies). --occupied adopts every occupied path of the
named package.

Flags:
      --occupied   All occupied paths of the named package
  -n, --dry-run    Show the plan; change nothing
      --force      Overwrite differing package content without asking

Examples:
  dstow adopt ~/.zshrc zsh
  dstow adopt ~/.config/foo/foo.toml     # no package: pick from candidates
```

#### repo (group)

```
Manage repos — where packages come from.

Usage:
  dstow repo <command>

Commands:
  add       Register a repo from a source (path, URL, github:owner/name)
  remove    Unregister a repo (deletes managed clones only)
  update    Download remote repo changes; touch nothing on disk
  upgrade   Fast-forward clean clones to what update downloaded

Environment:
  DSTOW_PATH   Colon-separated directories registered as session repos —
               additions to the repo set for this shell only; no priority
```

```
Register a repo — where packages come from.

Usage:
  dstow repo add <source> [flags]

Sources: a local directory path, a full URL/ssh form, or a qualified source
like github:owner/name (bare owner/name asks first; in scripts, qualify).
Remote sources clone into the managed directory; local paths are registered
in place and never modified.

Adding stows nothing: dstow announces the repo's packages and any bare
names that now need qualification. If the path would need percent-encoding
in name expressions, dstow shows the encoded form and asks whether to
continue or rename first. Re-adding a present repo is a safe, announced
no-op.

Flags:
      --stow   After adding, stow this repo's packages (exclusions apply)

Examples:
  dstow repo add ~/dotfiles
  dstow repo add github:rocne/dotfiles
  dstow repo add rocne/dotfiles --stow
```

```
Unregister a repo. Managed clones are deleted; local-path repos are only
forgotten — your directory is never touched.

Usage:
  dstow repo remove <repo> [flags]

Refuses while the repo still has stowed links (offers unstow-then-remove),
and refuses to delete a managed clone holding work not present at its
source. Every refusal names its remedy.

Flags:
      --unstow   Unstow the repo's packages first, without prompting
      --force    Override both guards (unsaved work will be lost)
```

```
Sync remote repos in two explicit phases. Neither ever runs on its own.

Usage:
  dstow repo update [<repo>...]     Download changes; alter nothing on disk
  dstow repo upgrade [<repo>...]    Fast-forward clean clones; report old -> new

With no repos named, both act on every remote repo. update touches the
network and nothing else — afterwards, status reports behind/ahead.
upgrade is fast-forward only: divergence or local work refuses loudly,
with no stash, merge, or rebase negotiation. upgrade never re-stows;
structural drift shows up in status. Linked files change the moment
upgrade moves them — update first, review, then upgrade.

Examples:
  dstow repo update
  dstow status
  dstow repo upgrade rocne/dotfiles
```

#### list (leaf)

```
What is configured — repos, packages, targets, exclusions, sources.
Reads only configuration: instant, side-effect free, never inspects disk.
Deployment truth lives in 'dstow status'.

Usage:
  dstow list [<name>] [flags]

Bare list shows repos (the global scope's content); naming a repo lists its
packages; naming a package lists its paths, relative to the package
directory.

Flags:
      --repos      Repos only (source, scheme, bulk-exclusion)
      --packages   Packages only (repo-attributed; same-named entries shown
                   with qualified names)
      --json       Machine-readable listing
```

*(The scope operand is the info resolution's restatement — list enumerates a
scope's content; package content is raw paths, no translation flags: the
translated view belongs to `-n/--dry-run`, reality to `status`.)*

#### info (leaf)

```
One scope's fields — the facts and effective config dstow holds about it,
from configuration and metadata, never by inspecting targets (that is
status's job). The scope is the global installation (named by no operand),
a repo, or a package.

Usage:
  dstow info [<name>] [flags]

With no name, details the global scope. Fields come in two groups: inherent
facts of the thing as it exists (version and paths; a package's repo, source,
scheme, managed path, qualified name) — permanently read-only — and
configured values from the config chain (effective target, dot-translation,
fold, ignores). The full human view prints the inherent group first, then
the configured group.

Flags:
  -f, --field <field>   Only the named field(s); repeatable. One field prints
                        its bare value; several print labeled lines
  -r, --recurse         Visit every applicable scope in turn, per-scope
                        attributed; scopes a named field does not apply to are
                        silently skipped
      --json            Machine-readable: a flat object keyed by field name
                        (an array of scope objects under --recurse)

Exit status:
  0   the requested field(s) carry a value
  1   applicable but unset or empty (shown as (unset) or [])
  2   unknown field, or a field illegal for the scope (names a suggestion)
  3   global refusal

Examples:
  dstow info                       # the global scope
  dstow info zsh
  dstow info dotfiles -f source -f scheme
  dstow info -r -f target          # target across every scope
```

#### status (leaf)

```
What is deployed — expected links against what targets actually hold.

Usage:
  dstow status [<name>...] [flags]
  dstow status <path>

Names scope to packages or whole repos. Package states: stowed, partially
stowed, not stowed, occupied, damaged — plus a drifted marker when the
deployed shape differs from what current config would produce. damaged is
only ever claimed with ledger evidence. Remote repos also show
behind/ahead as of the last update.

For a path: what occupies it, who owns it (per the ledger), and — if
occupied — the packages that could adopt it, ranked.

Flags:
      --json   Machine-readable status

Examples:
  dstow status
  dstow status zsh
  dstow status ~/.zshrc      # the per-path view, adoption candidates incl.
```

#### check / clean / rebuild (leaves)

```
Verify every ledgered link — instant, no tree walk.

Usage:
  dstow check [flags]

Classifies stale links: broken (destination gone) and orphaned (resolves
into a known repo, but no current package config would produce it). clean
executes exactly this report — the two can never disagree.

Flags:
      --json   Machine-readable report
```

```
Execute exactly what check reported.

Usage:
  dstow clean [flags]

Broken links are removed freely. Orphans are shown and confirmed (--yes
removes them without asking). Nothing else is ever touched.

Flags:
      --force   Remove orphans without confirmation in any context
```

```
Reconstruct a lost ledger by scanning configured targets for links into
known repos. The only full tree walk dstow has; explicit and rare.

Usage:
  dstow rebuild
```

*(check additionally reports **contradicted** entries — ledger evidence disk
disagrees with; clean prunes those entries only, never touching disk. The
help wording above names the two disk-acting classes; the full contract is
[§6.4](#64-command-contracts).)*

#### snippet (group)

```
Print canned POSIX-sh snippets to stdout. dstow never edits rc files.

Usage:
  dstow snippet <command>

Commands:
  rc            The shell-rc bootstrap: installs dstow iff absent — silent
                and network-free whenever dstow is already present

Examples:
  dstow snippet rc >> ~/.bashrc
```

#### colors (group)

```
Theming utilities.

Usage:
  dstow colors <command>

Commands:
  theme         Emit a named theme — as a packed DSTOW_COLORS string
                (default) or a theme file (--format toml)

Examples:
  export DSTOW_COLORS=$(dstow colors theme catppuccin-mocha)
  dstow colors theme catppuccin-mocha --format toml > ~/.config/dstow/themes/mine.toml
```

*(`--format env|toml` — exact flag spelling is implementation detail; a
format flag doesn't change the concept, per the `--json` precedent.)*

## 3. Configuration

*Resolutions: [Config format survey (#17)](https://github.com/rocne/dstow/issues/17),
[Config schema (#23)](https://github.com/rocne/dstow/issues/23) — full ledger
C1–C23 as amended.*

**Format: TOML (spec v1.1.0), parsed with BurntSushi/toml; XDG paths via
adrg/xdg; four-level merge hand-wired over the plain parser — no config
framework** (viper and koanf ruled out: the four-level merge *is* domain
logic).

### 3.1 Files

| File | Location | Written by |
|---|---|---|
| Global config | `$XDG_CONFIG_HOME/dstow/config.toml` | user |
| Repo config | `<repo>/.dstow/config.toml` | user |
| Package config | `<repo>/<pkg>/.dstow/config.toml` | user |
| Repo registry | `$XDG_CONFIG_HOME/dstow/repos.toml` | **dstow** |
| User theme presets | `$XDG_CONFIG_HOME/dstow/themes/<name>.toml` | user |

- **One name at every level** (`config.toml`); the level is determined by
  placement alone; the legality matrix says which keys mean anything where.
  Naming principle: *the file name carries whatever context its location
  doesn't* (C1).
- **`config.toml` never declares repos** — the registry is dstow-written
  (single ownership; reconciles "no config-mutation commands" with §5's
  persistent registration; gh `config`/`hosts` split precedent). Entries are
  canonical qualified source strings (`repos = ["github:rocne/dotfiles"]`); a
  table with `source` is the documented growth form. The registry is *config,
  not state*: not reconstructible from disk, so it lives with intent (C3).
- Theme preset name = file basename; user presets shadow bundled embedded
  ones on name collision (C4).

### 3.2 Key grammar and the schema

- **snake_case keys** across config, JSON output, and color-slot vocabulary —
  one spelling of every term. CLI flags keep GNU kebab: a deliberate,
  documented mismatch of exactly one rule (C5).
- **Bool keys are imperative statements; value keys are nouns** (C6):
  `target` (string), `translate_dot_prefixes` (default true),
  `exclude_from_bulk` (default false), `fold_trees` (default false — stow's
  own "tree folding" term).
- **Legality matrix** (C7, as amended):

  | Key | package | repo | global |
  |---|---|---|---|
  | `target` | ✓ | ✓ | ✓ *(the built-in `$HOME` floor is the global cell's default)* |
  | `translate_dot_prefixes` | ✓ | ✓ | ✓ |
  | `ignore` (additive) | ✓ | ✓ | ✓ |
  | `exclude_from_bulk` | ✓ | ✓ | — |
  | `packages_dir` | — | ✓ | — |
  | `fold_trees` | — | — | ✓ |
  | registry (`repos.toml`) | — | — | ✓ |
  | `[color]` + `theme` | — | — | ✓ |

- **Path-valued keys expand `~` and `$VAR`/`${VAR}`** (stow's grammar),
  evaluated at use time per invocation. Unset variable = loud error naming
  variable + file + key. The expanded result must be absolute. Failures scope
  per-package (§3.2 of REQUIREMENTS) (C8).
- **Schema-wide shorthand idiom** (C9): wherever a collection entry has a
  scalar essence, a bare string is legal shorthand for its table form (the
  registry is the v1 instance).
- **`packages_dir`** (M3): opt-in, repo level only, a repo-root-relative path
  naming where the repo's packages live. Unset = packages at the repo root
  (stow compat binds the default). Deliberately outside C8's
  expand-to-absolute grammar — it names a location *inside* the repo.
  `"packages"` is the documented convention and fresh-start recommendation.
- **No schema-version key** (C11): forward evolution rides the unknown-key
  policy; breaking changes are v2 territory by definition.

### 3.3 Theming keys

- **`[color]` table** (C12): global only; sixteen keys, a closed set — states
  `stowed` `partially_stowed` `not_stowed` `occupied` `damaged` `drifted`;
  check classes `broken` `orphaned` `contradicted`; severities `note`
  `warning` `error` `fix` (bare words — the colon is rendering); prose `name`
  `heading` `muted`. Values in git's `color.*` grammar. One vocabulary across
  `[color]` / `DSTOW_COLORS` / theme files.
- **`theme` key** (C13): name-or-path per the operand rule; the path form
  follows C8.

### 3.4 Ignores

- **Carrier: the `ignore` key** at every level, additive per §4.1 — a level
  adds to, never silences, inherited ignores. **No native ignore file**:
  "native mirror" means the concept is expressible natively, not
  file-for-file twinning; stow-named files stay honored as compat (C15).
- **Language: gitignore-glob semantics**, matched per package against
  package-root-relative paths (no slash = basename at any depth; slash =
  anchored; trailing slash = dir-only; `**` supported) (C16).
- **Two refused-and-reserved forms**: leading `!` (negation would silence
  inherited ignores — forbidden by the additive law) and leading `//`
  (reserved for a possible future regex marker) (C16).
- **Pattern language is a property of the carrier** (C17): native carriers
  speak glob; compat carriers (`.stow-local-ignore`, `--ignore`) speak stow
  regex; never mixed in one file.

### 3.5 Unknown and misplaced keys

**Warn, never refuse** (C18): unknown key → warning naming file + key with
did-you-mean; misplaced key (legal elsewhere per the matrix) → warning naming
the legal level and its file, key ignored. These are surprise-class
announcements — they survive `--quiet`.

### 3.6 Stow compatibility

- **Routing (renamed rc)**: a native config whose content is flag-lines
  (first significant token starts with `-` — impossible in top-level TOML)
  routes to the compat parser with a loud announcement naming the native
  equivalent. The native format never bends to compat (#17).
- **rc parsing is consumed from gostow's public `stowrc` package**
  (quirk-faithful, conformance-tested). dstow keeps discovery, slotting
  (`~/.stowrc` → global; `<repo>/.stowrc` → repo, discovered via the repo,
  never cwd), knob mapping, and supplement diffing (C20).
- **Option mapping** (C19): `--target`→`target`; `--no-folding`→
  `fold_trees=false` (flag *absence* maps to nothing — dstow's default
  applies); `--dotfiles`→`translate_dot_prefixes=true`; `--ignore`→additive
  chain entry in carrier language; `--dir` in `~/.stowrc`→session-repo
  contribution, announced, `fix:` suggests `repo add`; `--dir` at repo
  level→warn-and-ignore. Unmappables (`--adopt`, `--override`, `--defer`,
  verbosity, simulate, verbs): warn-and-ignore per option, naming why + the
  native remedy. The file always runs; degradation is loud, never rejection.
- **Non-RE2 compat patterns: refuse, scoped** to the level the pattern
  governs (a package-level failure fails that package; the run continues),
  naming file, pattern, RE2 error, and both remedies (C21).
- **Supplement mode** (C22): per-knob diff where *both* carriers set the knob.
  Non-overlap silent; equal values silent; differing values → native wins,
  loud warning naming level/files/knob/values/winner + `fix:` suggesting
  removal from the rc. Ignores never conflict (additive, per-carrier
  language). **Supplement mode survives design contact — REQUIREMENTS §4.3's
  fallback is not triggered.**

### 3.7 Environment

**`DSTOW_PATH`** (C23): colon-separated **absolute local directory paths**,
PATH convention. No qualified sources (`:` collides with the separator;
remote sources need clones — `repo add`'s job); no dstow-side expansion;
relative entries refused loudly; empty entries warn-and-skip.
**`DSTOW_COLORS`** is the only other environment variable dstow reads
([§7.3](#73-theming--the-dircolors-architecture)); `DSTOW_HOOK_*` is dstow's
output lane ([§5](#5-hook-context-contract)).

## 4. Metadata directory

*Resolution: [Metadata directory name and layout (#24)](https://github.com/rocne/dstow/issues/24) —
ledger M1–M8 as amended.*

- **The name: `.dstow/`** at both levels (`<repo>/.dstow/`,
  `<repo>/<pkg>/.dstow/`); the global level's equivalent is
  `$XDG_CONFIG_HOME/dstow/`. Dot-prefixed tool-metadata convention (`.git`
  lineage); inert under dot-translation by construction; visually distinct
  from stow's `.stow` marker file (M1).
- **Package identity is locational; hidden directories are never packages**
  (M2). No marker files (migrated repos must work out-of-the-box;
  mkdir-and-go stays intact). Root mode: hidden dirs skipped silently.
  Scoped mode (`packages_dir` set): every visible directory is
  definitionally a package; hidden dirs skipped **loudly**.
- **Reserved territory** (M5): `.dstow/`'s top level is dstow's namespace —
  claimed entries are `config.toml` and `hooks/`; unknown top-level entries
  draw a C18-style warning, never a refusal. Same posture in
  `$XDG_CONFIG_HOME/dstow/` (claimed: `config.toml`, `repos.toml`, `themes/`,
  `hooks/`).
- **Hooks: `.dstow/hooks/`, eight per-event executables, git-style** (M6):
  `{pre,post}-{stow,unstow,restow,adopt}` at all three levels (global:
  `$XDG_CONFIG_HOME/dstow/hooks/`). **Restow fires only the restow pair.**
  Execution is direct `exec`; shebang decides the interpreter.
  Subdirectories of `hooks/` are inert helper territory — never fired, never
  warned — with `lib/` the documented convention. Non-hook *files* directly
  in `hooks/` warn with did-you-mean (`pre_stow`, `prestow`); a valid-named
  non-executable warns naming the `chmod +x` fix.
- **`<name>.d/` reserved** (M7): the eight `.d` spellings are
  inert-and-warned ("reserved, not yet meaningful") in v1; drop-in execution
  is committed v2 (see [§10](#10-reserved-doorways)).
- **Auto-ignore rides the engine's `IgnoreFunc` seam** (M8): the closure
  returns true for `.dstow` **anchored at the package root only** — deeper
  `.dstow` names are content. Always on, in no config, unsilencable
  (REQUIREMENTS §2.7 is law, not an ignore-chain entry). check/status/deploy
  consult the same closure, so observation and deployment can never disagree.

## 5. Hook context contract

*Resolutions: [Hook context contract (#25)](https://github.com/rocne/dstow/issues/25) —
ledger H1–H8 as amended; lifecycle is REQUIREMENTS §9.1.*

- **Mechanism: environment variables, `DSTOW_HOOK_*`, no arguments, no
  stdin-feeding** (H1). Additive evolution: new context = new variable. The
  prefix partitions the env namespace into lanes — `DSTOW_PATH` /
  `DSTOW_COLORS` are user→dstow inputs; `DSTOW_HOOK_*` is the dstow→hook
  output lane.
- **The variable set** (H2) — absent-not-empty everywhere; explicit beats
  derived:

  | Variable | Value | package | repo | global |
  |---|---|---|---|---|
  | `DSTOW_HOOK_LEVEL` | `package` \| `repo` \| `global` | ✓ | ✓ | ✓ |
  | `DSTOW_HOOK_ACTION` | `stow` \| `unstow` \| `restow` \| `adopt` | ✓ | ✓ | ✓ |
  | `DSTOW_HOOK_PHASE` | `pre` \| `post` | ✓ | ✓ | ✓ |
  | `DSTOW_HOOK_FQN` | the scope's own FQN, canonical-encoded | ✓ | ✓ | — |
  | `DSTOW_HOOK_SCHEME` | decoded (`github`) | ✓ | ✓ | — |
  | `DSTOW_HOOK_COORDINATE` | decoded (`rocne/dotfiles`; a `local:` repo's is the actual absolute path) | ✓ | ✓ | — |
  | `DSTOW_HOOK_PACKAGE` | bare package name, decoded — not the FQN | ✓ | — | — |
  | `DSTOW_HOOK_PACKAGE_DIR` | absolute path | ✓ | — | — |
  | `DSTOW_HOOK_TARGET` | effective target root, absolute | ✓ | — | — |
  | `DSTOW_HOOK_REPO_FQN` | the repo's FQN, canonical-encoded | ✓ | ✓ | — |
  | `DSTOW_HOOK_REPO_DIR` | absolute path | ✓ | ✓ | — |
  | `DSTOW_HOOK_PACKAGES` | FQNs of all packages acting under this scope | — | ✓ | ✓ |

  **Encoding stance**: `*FQN` values carry the canonical percent-encoded
  spelling (paste-able straight back into dstow commands); decomposed
  segments carry decoded real values — encoding protects the grammar, and a
  standalone segment has no grammar to protect.
- **"Coordinate" carries the always-explain rule** (H3): every doc surface
  that says the word states its lineage (Maven coordinates).
- **`DSTOW_HOOK_PACKAGES` is newline-separated**, one canonical FQN per line;
  `while IFS= read -r pkg` is the documented idiom. Canonical encoding of
  control characters makes one-per-line airtight (H4).
- **cwd = the scope's own directory** (H5): package dir / repo dir /
  `$XDG_CONFIG_HOME/dstow/`. Hooks never inherit the user's incidental cwd.
- **Both hook streams land on dstow's stderr; stdin passes through** (H6).
  Nothing a hook prints is dstow's answer to the question asked — hook
  output is commentary definitionally. A hook that needs to emit data writes
  a file or is invoked directly.
- **Write commands refuse from inside a hook; reads stay fully allowed**
  (H7). Detection: `DSTOW_HOOK_ACTION` present in the environment. Deploy
  verbs, adopt, clean, rebuild, and repo mutations refuse with a §1.4 error
  naming cause and remedy; `list` / `info` / `status` / `check` all work — an
  install hook may read dstow (`dstow status --json`, `dstow info -f target`)
  while installing tools with its own commands. No bypass in v1:
  refuse-now-allow-later is additive; the reverse breaks users.

## 6. Ledger

*Resolution: [Ledger format (#21)](https://github.com/rocne/dstow/issues/21);
trade-off record [ADR 0001 — the ledger is a current-state index, not a journal](adr/0001-ledger-is-a-current-state-index.md).*

### 6.1 Format

**A current-state index** — entries are links dstow currently believes
exist; unstow deletes entries; no history. **One JSON document per machine**
at `$XDG_STATE_HOME/dstow/ledger.json` (machine state in JSON, human config
in TOML — each format in its conventional lane). Pretty-printed as a
write-side courtesy only.

```json
{
  "version": 1,
  "targets": {
    "/home/rocne": [
      {
        "link": ".config/nvim",
        "package": "github:rocne/dotfiles::nvim",
        "source": "dot-config/nvim",
        "destination": "../repos/github/rocne/dotfiles/nvim/dot-config/nvim",
        "recorded_at": "2026-07-15T09:12:03Z"
      }
    ]
  }
}
```

- `targets`: absolute target root → entry list. The grouping is
  load-bearing: it is rebuild's replace boundary.
- `link` is target-relative and `source` package-relative — exactly how
  `stow.Expected` keys and values its results; `stow.Owner` recovers the
  same, so ledger, Expected, and Owner compose directly.
- `package`: canonical percent-encoded FQN (one field — the grammar already
  unified repo+package).
- `destination`: the literal symlink text as written — the **damaged**
  evidence, comparable against disk without recomputation.
- `recorded_at`: RFC 3339 UTC, scoped strictly to "when this entry entered
  the ledger" — it claims nothing about any file or action, so disk-is-truth
  can never contradict it.
- Deliberately absent: a fold flag (derivable), the repo directory (config's
  business; storing it invites stale copies).

### 6.2 Atomicity and locking

- Every write: marshal full document → temp file **in the same directory** →
  fsync → rename. Readers and crashes see old or new, never torn.
- Writers take an exclusive advisory `flock` on sibling `ledger.lock`, held
  for the **whole operation** (read ledger → filesystem work → write
  ledger). Pure readers take no lock. Contention **fails fast** with a clear
  error (dpkg/pacman precedent).

### 6.3 Pruning discipline

- **Reads never write.** `list`/`info`/`status`/`check` are lock-free,
  mutation-free, idempotent. §8.1's "pruned on sight" is realized by
  writers.
- **Scoped pruning.** A writer prunes contradicted entries in its scope:
  entries of the packages it operates on, plus entries at paths it touches
  anyway. Unrelated damage evidence survives.
- **`clean` is the ledger-wide broom** (its scope is by definition the whole
  ledger).

### 6.4 Command contracts

- **check** (read): no lock, writes nothing; reads ledger + config + repo
  package trees ("no tree walk" means no *target* walk; each entry costs one
  lstat/readlink). Classifies **broken** (destination gone), **orphaned**
  (resolves into a known repo, current config wouldn't produce it),
  **contradicted** (disk disagrees with the entry).
- **clean** (write): takes the lock, **recomputes check's plan fresh under
  the lock** (never a stale saved report), then: broken → remove link +
  entry, freely; orphaned → remove link + entry behind confirm /
  non-interactive error / force; contradicted → **prune entry only, never
  touches disk** — freely but loudly, each prune reported with its evidence.
  One atomic ledger write at the end.
- **rebuild** (write): walks every currently configured target root,
  recursively, lstat-based, never descending through symlinks. Each symlink
  found is tested with `stow.Owner` against known repos; owned links become
  entries (`destination` from readlink, `recorded_at` = now). Hand-made
  links into repos get ledgered — that is how §8.2's hand-made-link example
  later surfaces as an orphan. **Replace per scanned target group**: rebuild
  wholesale-replaces the entry groups of targets it scanned and leaves
  unscanned groups untouched.
- **Deploy verbs**: stow/adopt add entries for links created, unstow deletes
  entries for links removed, restow does both — each under the standard lock
  + atomic write, pruning contradicted entries in scope as they go.

### 6.5 Versioning and failure modes

- `version` marks the schema; bumped only on incompatible change.
- Missing file = empty ledger, never an error.
- Version newer than this dstow → refuse loudly ("written by a newer dstow;
  upgrade"); never guess, never rewrite down.
- Version older → migrate forward in memory; the file is rewritten in the
  new schema only when a write command commits.
- Corrupt/unparseable → refuse loudly, name the path, point at the remedy
  (`rebuild` or restore). **Corruption must never degrade into amnesia.**

## 7. Output

*Resolutions: [Output design (#26)](https://github.com/rocne/dstow/issues/26) —
ledger O1–O12; [Theming survey (#31)](https://github.com/rocne/dstow/issues/31).
Flow mockups: the output-design prototype (branch `prototype/output-design`);
presentation specifics there are provisional, the rules here bind.*

### 7.1 The three governing rules

1. **stdout is data, stderr is commentary — absolute** (O1). The thing you
   asked for (listings, status, JSON, snippets) goes to stdout, pure;
   everything dstow *says about* the work (notes, warnings, errors, prompts,
   progress) goes to stderr. `dstow snippet rc >> ~/.bashrc` and
   `dstow status --json | jq` are clean by construction.
2. **Severity is a word, color is a reinforcement** (O2). Every commentary
   line starts with a greppable prefix — `note:` / `warning:` / `error:` /
   `fix:` — colored but meaningful uncolored, padded to `warning:`'s width so
   stacked commentary aligns *(padding provisional — revisit at
   implementation if it looks weird)*. The **`fix:` line makes §1.4
   structural**: every refusal is followed by a runnable command or config
   pointer; blue, not green (green means success, and a fix appears precisely
   when nothing succeeded).
3. **Semantic slots, not colors, in the code** (O3). One owned printer maps
   CONTEXT.md vocabulary to styles; nothing outside it ever names a color.
   Test contract: `strip_ansi(styled) == plain` (O11).

### 7.2 Palette, prompts, quiet

- **The semantic palette (ANSI-16)**: stowed green · partially stowed yellow
  · not stowed dim · occupied magenta · damaged bold red · drifted cyan ·
  broken red · orphaned yellow · contradicted bold red (same evidence as
  damaged, two views) · note: cyan · warning: yellow · error: bold red ·
  fix: blue · names bold · heading bold · muted dim.
- **The O4 promise**: dstow's *defaults* emit only the 16 base ANSI slots, so
  the terminal theme rethemes dstow automatically and colorblind/low-vision
  users retheme through terminal preferences (gh's Accessible pattern made
  the only default mode). User choices may exceed ANSI-16 (the grammar
  allows 0–255/hex); the promise binds defaults only.
- **`--color <when>`** requires a value: auto (default) / always / never; no
  bare `--color`, no `--no-color` sugar — NO_COLOR covers the env side (O6).
- **`--quiet` drops routine chatter only** (O7): no-op lines, all-good
  summaries, progress. Announcements (§1.3), warnings, errors, and `fix:`
  lines always survive.
- **Per-package run lines** (O8): `verb name result` one-liners, failure
  detail indented under its line, summary last, exit per §3.2.
- **Confirm polarity** (O12): destructive/bulk defaults No (`[y/N]`);
  benign-continue defaults Yes (`[Y/n]`, e.g. the encoding confirm).
- **JSON conventions** (O10): per-command shapes; lower_snake keys;
  CONTEXT.md state strings verbatim (including the space in
  `"partially stowed"`); FQNs always present. `info --json` is a flat object
  keyed by canonical field name (an array of scope objects under `-r`).

### 7.3 Theming — the dircolors architecture

Layered, top wins, all strictly downstream of the enable/disable chain
(`--color` > `NO_COLOR` > `CLICOLOR_FORCE` > `CLICOLOR` > TTY+`TERM=dumb`) —
theme config can never re-enable color the chain turned off:

1. **`DSTOW_COLORS`** — the only theming env var: packed per-slot overrides,
   LS_COLORS-family syntax (`damaged=bold red:stowed=#a6e3a1`), values in
   git's `color.*` grammar. Populated by hand or by generator:
   `export DSTOW_COLORS=$(dstow colors theme catppuccin-mocha)`. There is no
   `DSTOW_THEME`.
2. **`[color]` TOML table** — global config, one key per slot, same grammar.
3. **`theme` config key** — name or path per the operand rule: a bare string
   is a theme *name* (user themes dir first, then bundled presets — TOML
   files embedded via go:embed, one loader); a path form is a theme *file*
   anywhere — including inside a repo, which makes repo-shipped themes free.
4. **The default ANSI-16 semantic palette.**

**North-star invariants (bound in v1)**: a theme file is exactly the bare
`[color]` schema, no wrapper keys; the packed string, the config table, and
theme files share one slot vocabulary and one value grammar — losslessly
convertible between representations, forever. v1 ships
`dstow colors theme <name> [--format env|toml]`.

## 8. Go architecture

*Resolution: [Go architecture (#29)](https://github.com/rocne/dstow/issues/29) —
ledger A1–A20 as amended. Altitude: module boundaries, public interfaces,
gostow-consumption seams; ephemeral internals are implementation's.*

### 8.1 Layout and wiring

- **Module `github.com/rocne/dstow`**; `cmd/dstow/main.go` contains only
  wiring (streams, version via goreleaser ldflags, `os.Exit`); everything
  else under `internal/` — no public Go API in v1. **Eleven packages**:
  `cli, ui, name, config, repo, engine, ignore, ledger, hooks, git, ops`
  (A1 as amended — `deps` removed with the dependency concept).
- **`cli.Run(args, version, stdin, stdout, stderr) int`** —
  constructor-injected commands over an app struct, no package-level command
  globals. Environment read via `os.Getenv` at point of use. cobra
  (v1.10.x, no viper) with `SilenceUsage`+`SilenceErrors`; all `RunE` errors
  are typed domain errors that `cli` alone maps to exit codes and renders
  through the printer — one owner for §1.4 wording. Help stays on stdout
  (help *is* the requested data) (A2).
- **Exit-code map** (A3), mapped in exactly one place in `cli`:
  `0` success · `1` negative answer (any package failed; a requested field
  applicable but unset/empty; check found findings) · `2` usage error ·
  `3` refusal/environment (non-interactive ambiguity, corrupt ledger, lock
  contention, ledger-from-newer-dstow).

### 8.2 Output plumbing

- **`ui` is the only module that touches the streams** (A4). Every other
  module returns data — diagnostics included (config warnings come back as
  values; `ui` renders them). This law makes `--json`, `--quiet`, and
  strip-roundtrip structural.
- **Printer internals** (A5): fatih/color per-instance (never the
  package-global `NoColor`); the per-stream enable precedence of §7.3;
  theming stack resolved strictly downstream of enable; go:embed presets +
  user themes dir behind one theme loader, inside `ui`; the git-grammar
  color-value parser lives in `ui` — one owner for the one value grammar.
- **Interactivity seam** (A6): `ui.Interactive()` = stdin TTY **&&** stderr
  TTY (prompts live on stderr); injectable for tests.
- **fang is ruled out** (A18): experimental, lipgloss v2 + colorprofile is a
  second styling engine (the two-engines smell), auto commands collide with
  the pinned leaves, and it restyles help approved verbatim. Engine
  unification: **fatih/color alone**.

### 8.3 Domain modules

- **`name` is pure** (A7): FQN parse/format, percent encode/decode,
  segment-boundary suffix matching, `::` kind-forcing, path-vs-name
  classification. Zero I/O, zero deps.
- **`config`** (A8): the four-level chain, legality matrix, use-time path
  expansion, warnings-as-data, stowrc discovery/slotting/mapping/supplement
  diff consuming `gostow/stowrc`, content-sniff routing. The metadata
  location is hidden behind one accessor.
- **`repo`** (A9): the repo set (registry read/write with the same
  temp+fsync+rename discipline as the ledger; `DSTOW_PATH`), managed-dir
  layout, scheme dispatch (github/local, internal — no exported plugin seam
  until a third scheme exists), package enumeration, name resolution over
  the set (via `name`). Cross-domain guards compose in `ops`, not here.
- **`ledger`** (A10): [§6](#6-ledger) verbatim. `Load` = lock-free read
  snapshot; `Update(scope, fn)` = flock for the whole operation → scoped
  pruning → atomic write.
- **`hooks`** (A11): discovery via `config`'s metadata accessor, nested/LIFO
  ordering, once-per-invocation firing, no-op semantics, §9.1.4 failure
  rules; the context contract of [§5](#5-hook-context-contract).
- **`ops` is the app core** (A13): the verbs as deep modules returning
  structured results — deploy (per-package independence loop, nested/LIFO
  hook ordering, ledger transactions), maintenance (clean recomputes check's
  plan under the lock), views, repo ops, snippet, colors. One package;
  internal file organization is implementation's.

### 8.4 Seams to third parties

- **gostow** (A14): pin `>= v0.3.0`, moving to `>= v0.4.0` when cut (the
  `IgnoreFunc` seam ships in it). `NoGlobalIgnoreFile: true` always;
  `FixQuirks: true` for engine operations; stowrc *compat parsing* stays
  quirk-faithful; `Options.Log = io.Discard` (dstow reports from `Result`,
  never engine prose); per-package `Apply` calls so §3.2 independence holds;
  `Expected`/`Owner`/`Conflict{Path,Kind}` mapped into dstow's typed results
  in `engine`, nowhere else.
- **Ignore languages stay in their lanes** (A15): compat stow-regex chains
  ride gostow's own `Options.Ignore`; native gitignore-glob chains are
  matched dstow-side by `ignore`, wrapping go-git's
  `plumbing/format/gitignore` (verified full C16 semantics; doublestar
  verified not gitignore-semantic — disqualified). Negation and `//` never
  reach the matcher (refused at config parse).
- **Engine-ignore seam** (A16): `stow.Options.IgnoreFunc` — landed upstream
  (gostow#41); lstat-based `isDir`; additive-only (exclude more, never
  resurrect what stow ignores). The pre-match fallback is moot — do not
  implement it.
- **git = system git behind a port** (A17): `git.Port` in repo's terms
  (Clone, Fetch, FFApply old→new, AheadBehind, HasLocalWork), exec adapter
  for production, fake for tests. go-git rejected — credential-helper and
  ssh-config fidelity is exactly what dotfiles users depend on. Missing git
  binary = clean §1.4 error at remote-scheme operations only.

### 8.5 Pins and completion

- **Managed directory** (A19):
  `$XDG_DATA_HOME/dstow/repos/<scheme>/<owner>/<name>` — data, not state,
  not cache: links point into it. Directory names use the canonical
  percent-encoded segments, filesystem-safe by construction.
- **Dynamic completion** (A20): `ValidArgsFunction` completes package/repo
  names and schemes through the same `repo` resolver, best-effort-silent
  (any error → no completions, never diagnostics), config loaded in quiet
  mode, never fires hooks or network.

## 9. Bootstrap and distribution

*Resolutions: [Bootstrap snippet and installer design (#28)](https://github.com/rocne/dstow/issues/28) —
ledger B1–B9; [Release-ci and installer wiring survey (#19)](https://github.com/rocne/dstow/issues/19).*

### 9.1 The rc snippet

Emitted by `dstow snippet rc`, to stdout, composable (`>> ~/.bashrc`); dstow
never edits rc files. **Present ⇒ silent, invisible, offline** — the
installer is never even fetched. Canonical text (B1):

```sh
# dstow bootstrap — https://github.com/rocne/dstow
# Ensure the install dir is on PATH, then install dstow only if missing.
# POSIX "is dir in PATH" idiom (builtin, fork-free):
#   https://unix.stackexchange.com/q/32210
case ":$PATH:" in
  *":$HOME/.local/bin:"*) ;;                 # already on PATH
  *) PATH="$HOME/.local/bin:$PATH" ;;
esac

if ! command -v dstow >/dev/null 2>&1; then
  curl -fsSL https://raw.githubusercontent.com/rocne/dstow/main/install.sh | sh
fi
```

**Every snippet emission is a go:embed document** — real files, diffable,
shellcheck-able in CI; never string literals (B2).

### 9.2 The installer

- **Vendored canonical** (B3): `install.sh` at dstow's repo root, served
  raw-on-main from dstow's own URL; *authored* in release-ci, propagated
  downstream by push-based PRs. **dstow's first release blocks on
  [release-ci#1](https://github.com/rocne/release-ci/issues/1)** — no
  fallback design.
- **Invocation** (B4): bare `curl -fsSL …/rocne/dstow/main/install.sh | sh`;
  the repo slug is baked as the vendored default; no arguments.
- **The silence matrix** (B5): rc + present = nothing; rc + absent = installs
  and announces; direct invocation always speaks (one status line + exit 0
  when present — "exits cleanly" means exit 0, not muteness).
- **The installer contract is a floor** (B6): `command -v` presence check;
  `--force`; `--version vX.Y.Z` implies force; `~/.local/bin` default bound
  as *contract* (the snippet hardcodes it); checksum mandatory, cosign
  opportunistic. Anything above the floor is release-ci's to grow freely;
  the snippet depends on nothing above it.

### 9.3 Release wiring

Inherited wholesale from release-ci (proven by gostow and dot-dagger;
copy-paste-and-rename): the single reusable `workflow_call` workflow
(GoReleaser build → SLSA attestation → cosign/Fulcio verify → Cloudsmith
apt/dnf smoke), a `.goreleaser/dstow.yaml`, a release-please workflow plus
break-glass tag-push workflow, and three repo secrets set manually before
first release: `GPG_PRIVATE_KEY`, `HOMEBREW_TAP_GITHUB_TOKEN`,
`CLOUDSMITH_API_KEY`.

## 10. Reserved doorways

v1 behavior deliberately keeps these doors open. Each binds a *refusal or
reservation* in v1; none ships semantics.

| Doorway | v1 binds | Future |
|---|---|---|
| `@` coordinate suffix | literal `@` encodes as `%40` ([§1.2](#12-encoding)) | scheme-interpreted suffix; semantics belong to the interpreting scheme ([#20](https://github.com/rocne/dstow/issues/20), decided 2026-07-16) |
| Identifier aliasing | resolution rule stated; zero instances ([§1.4](#14-identifier-resolution-and-aliasing)) | v2 field aliases, prefix resolution (F12) |
| `//` ignore prefix | refused as reserved (C16) | possible regex marker (maybe-v2) |
| `!` ignore prefix | refused (additive law) | — (negation stays forbidden) |
| `<hook>.d/` dirs | eight spellings inert-and-warned (M7) | **committed v2**: run-parts drop-ins with a main-file delegation point |
| Bare field namespace | all of it dstow's; built-ins + aliases reserved (F14) | v2 user properties arrive dot-qualified, never bare |
| `.dstow/` top level & global config dir | unknown entries warn, never refuse (M5) | future kinds addable without a schema version |
| `fix:` line | structural, machine-stable runnable remedy (O2) | maybe-v2 fix-runner |
| Theming convertibility | one slot vocabulary + one value grammar across env/table/file (§7.3) | converter/builder family; first-class repo-shipped themes (maybe-v2) |
| Hook write-refusal | H7 refuses writes inside hooks | maybe-v2 hook-orchestrator (refuse-now-allow-later is additive) |
| `DSTOW_HOOK_*` namespace | absent-not-empty contract (H2) | scheme-specific variables (maybe-v2) |
| `packages_dir` kind-first convention | `"packages"` documented convention (M3) | repo aggregator sibling room (maybe-v2) |
| `snippet` group | `rc` only (B8) | `snippet hook <name>` skeletons — candidate v1.1 |

ADRs: [0001 — the ledger is a current-state index](adr/0001-ledger-is-a-current-state-index.md);
[0002 — no dependency concept](adr/0002-no-dependency-concept.md) (superseded
design preserved in [docs/attic/dependencies.md](attic/dependencies.md)).
Every other resolution ran the ADR three-part test and minted none.

## 11. Provenance

Each section's binding detail and full rationale live in its ticket's
resolution (as amended — read resolution comments *and* addenda):

| Section | Ticket(s) |
|---|---|
| §1 Naming grammar | [Naming grammar (#20)](https://github.com/rocne/dstow/issues/20); `@` reservation decided on [Draft DESIGN.md (#30)](https://github.com/rocne/dstow/issues/30) |
| §2 CLI surface | [CLI surface (#22)](https://github.com/rocne/dstow/issues/22); [info: the property read (#27)](https://github.com/rocne/dstow/issues/27); [CLI framework survey (#18)](https://github.com/rocne/dstow/issues/18) |
| §3 Configuration | [Config format survey (#17)](https://github.com/rocne/dstow/issues/17); [Config schema (#23)](https://github.com/rocne/dstow/issues/23) |
| §4 Metadata directory | [Metadata directory (#24)](https://github.com/rocne/dstow/issues/24) |
| §5 Hook context contract | [Hook context contract (#25)](https://github.com/rocne/dstow/issues/25) |
| §6 Ledger | [Ledger format (#21)](https://github.com/rocne/dstow/issues/21); [ADR 0001](adr/0001-ledger-is-a-current-state-index.md) |
| §7 Output | [Output design (#26)](https://github.com/rocne/dstow/issues/26); [Theming survey (#31)](https://github.com/rocne/dstow/issues/31) |
| §8 Go architecture | [Go architecture (#29)](https://github.com/rocne/dstow/issues/29); [CLI framework survey (#18)](https://github.com/rocne/dstow/issues/18) |
| §9 Bootstrap and distribution | [Bootstrap design (#28)](https://github.com/rocne/dstow/issues/28); [Release/installer survey (#19)](https://github.com/rocne/dstow/issues/19) |
| §10 Reserved doorways | the map's Out of scope section + the tickets cited per row |
| Dependency removal (throughout) | [Bootstrap design (#28)](https://github.com/rocne/dstow/issues/28); [ADR 0002](adr/0002-no-dependency-concept.md); [consistency sweep (#32)](https://github.com/rocne/dstow/issues/32); [judgment calls (#33)](https://github.com/rocne/dstow/issues/33) |

Synthesized 2026-07-16 on [Draft DESIGN.md (#30)](https://github.com/rocne/dstow/issues/30)
from the [dstow v1 design map (#16)](https://github.com/rocne/dstow/issues/16).
