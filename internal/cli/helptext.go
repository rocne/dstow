package cli

// This file carries the approved canonical help content (DESIGN.md §2.3
// top-level and §2.4 per-command) as the material fed into cobra's help
// generation: Long prose and Example blocks here, Short lines and flag usage
// strings at each command's definition. The blocks bind content, not bytes
// (A2 as amended — issue #96): editing wording here is a design change, while
// layout belongs to cobra. Tests assert this content appears in the generated
// help, extracted from DESIGN.md itself.

// rootShort is §2.3's title line, after the "dstow — " prefix.
const rootShort = "deploy dotfiles and configuration as symlinks, from packages in repos"

// rootLong opens with the §2.3 title line and carries its closing prose.
const rootLong = `dstow — deploy dotfiles and configuration as symlinks, from packages in repos

Name packages and repos by any unambiguous suffix of their qualified name
(github:rocne/dotfiles::zsh). The working directory never changes what a
command does. See 'dstow <command> --help' for details and examples.
Run 'dstow manual' for the full documentation.`

// Deploy leaves (§2.4 stow / unstow / restow, adopt).

const stowLong = `Link packages into their targets.

Names are packages or repos, by any unambiguous suffix of the qualified
name; naming a repo stows all of its packages. With no names, dstow asks
before stowing everything — in scripts that is an error: pass --all.

Each package succeeds or fails on its own; the run continues past failures
and exits nonzero if any package failed. Explicitly naming a package or
repo overrides its exclude-from-bulk setting.`

const stowExample = `  dstow stow zsh git tmux
  dstow stow dotfiles              # a repo: all of its packages
  dstow stow dots::zsh             # qualified just enough to be unique
  dstow stow --all --adopt         # first run on a machine with live files`

const unstowLong = `Remove packages' links from their targets.

Names are packages or repos, by any unambiguous suffix of the qualified
name; naming a repo unstows all of its packages. With no names, dstow asks
before unstowing everything — in scripts that is an error: pass --all.
unstow is the destructive verb; the bare confirm guards it with the same
care.

Each package succeeds or fails on its own; the run continues past failures
and exits nonzero if any package failed. Explicitly naming a package or
repo overrides its exclude-from-bulk setting.`

const unstowExample = `  dstow unstow zsh git tmux
  dstow unstow dotfiles            # a repo: all of its packages`

const restowLong = `Unstow, then stow again (refresh links).

Names are packages or repos, by any unambiguous suffix of the qualified
name; naming a repo restows all of its packages. With no names, dstow asks
before restowing everything — in scripts that is an error: pass --all.
restow is the idempotent refresh verb: on a package that is not stowed it
simply stows it (the unstow phase no-ops).

Each package succeeds or fails on its own; the run continues past failures
and exits nonzero if any package failed. Explicitly naming a package or
repo overrides its exclude-from-bulk setting.`

const restowExample = `  dstow restow zsh git tmux
  dstow restow --all --adopt`

const adoptLong = `Import an existing real file into a package; a link takes its place.
Live content always wins — adopt never destroys running configuration.

With a package: shows its plan and asks before overwriting differing
package content. Without a package: lists the packages that could adopt
the file, ranked, and asks you to pick (in scripts: an error that lists
the candidates as remedies). --occupied adopts every occupied path of the
named package.`

const adoptExample = `  dstow adopt ~/.zshrc zsh
  dstow adopt ~/.config/foo/foo.toml     # no package: pick from candidates`

// repo group (§2.4).

const repoLong = `Manage repos — where packages come from.

Environment:
  DSTOW_PATH   Colon-separated directories registered as session repos —
               additions to the repo set for this shell only; no priority`

const repoAddLong = `Register a repo — where packages come from.

Sources: a local directory path, a full URL/ssh form, or a qualified source
like github:owner/name (bare owner/name asks first; in scripts, qualify).
Remote sources clone into the managed directory; local paths are registered
in place and never modified.

Adding stows nothing: dstow announces the repo's packages and any bare
names that now need qualification. If the path would need percent-encoding
in name expressions, dstow shows the encoded form and asks whether to
continue or rename first. Re-adding a present repo is a safe, announced
no-op.`

const repoAddExample = `  dstow repo add ~/dotfiles
  dstow repo add github:rocne/dotfiles
  dstow repo add rocne/dotfiles --stow`

const repoRemoveLong = `Unregister a repo. Managed clones are deleted; local-path repos are only
forgotten — your directory is never touched.

Refuses while the repo still has stowed links (offers unstow-then-remove),
and refuses to delete a managed clone holding work not present at its
source. Every refusal names its remedy.`

// repoSyncLong is §2.4's combined update/upgrade block: one artifact covering
// both verbs, shared by both commands.
const repoSyncLong = `Sync remote repos in two explicit phases. Neither ever runs on its own.

With no repos named, both act on every remote repo. update touches the
network and nothing else — afterwards, status reports behind/ahead.
upgrade is fast-forward only: divergence or local work refuses loudly,
with no stash, merge, or rebase negotiation. upgrade never re-stows;
structural drift shows up in status. Linked files change the moment
upgrade moves them — update first, review, then upgrade.`

const repoSyncExample = `  dstow repo update
  dstow status
  dstow repo upgrade rocne/dotfiles`

// Inspect leaves (§2.4 list / info / status).

const listLong = `What is configured — repos, packages, targets, exclusions, sources.
Reads only configuration: instant, side-effect free, never inspects disk.
Deployment truth lives in 'dstow status'.

Bare list shows repos (the global scope's content); naming a repo lists its
packages; naming a package lists its paths, relative to the package
directory.`

const infoLong = `One scope's fields — the facts and effective config dstow holds about it,
from configuration and metadata, never by inspecting targets (that is
status's job). The scope is the global installation (named by no operand),
a repo, or a package.

With no name, details the global scope. Fields come in two groups: inherent
facts of the thing as it exists (version and paths; a package's repo, source,
scheme, managed path, qualified name) — permanently read-only — and
configured values from the config chain (effective target, dot-translation,
fold, ignores). The full human view prints the inherent group first, then
the configured group.

Exit status:
  0   the requested field(s) carry a value
  1   applicable but unset or empty (shown as (unset) or [])
  2   unknown field, or a field illegal for the scope (names a suggestion)
  3   global refusal`

const infoExample = `  dstow info                       # the global scope
  dstow info zsh
  dstow info dotfiles -f source -f scheme
  dstow info -r -f target          # target across every scope`

const statusLong = `What is deployed — expected links against what targets actually hold.

Names scope to packages or whole repos. Package states: stowed, partially
stowed, not stowed, occupied, damaged — plus a drifted marker when the
deployed shape differs from what current config would produce. damaged is
only ever claimed with ledger evidence. Remote repos also show
behind/ahead as of the last update.

For a path: what occupies it, who owns it (per the ledger), and — if
occupied — the packages that could adopt it, ranked.`

const statusExample = `  dstow status
  dstow status zsh
  dstow status ~/.zshrc      # the per-path view, adoption candidates incl.`

// Maintain leaves (§2.4 check / clean / rebuild).

const checkLong = `Verify every ledgered link — instant, no tree walk.

Classifies stale links: broken (destination gone) and orphaned (resolves
into a known repo, but no current package config would produce it). clean
executes exactly this report — the two can never disagree.`

const cleanLong = `Execute exactly what check reported.

Broken links are removed freely. Orphans are shown and confirmed (--yes
removes them without asking). Nothing else is ever touched.`

const rebuildLong = `Reconstruct a lost ledger by scanning configured targets for links into
known repos. The only full tree walk dstow has; explicit and rare.`

// snippet and theme groups (§2.4).

const snippetLong = `Print canned POSIX-sh snippets to stdout. dstow never edits rc files.`

const snippetExample = `  dstow snippet rc >> ~/.bashrc`

// snippetRCShort is the rc entry's description in the §2.4 snippet block.
const snippetRCShort = "The shell-rc bootstrap: installs dstow iff absent — silent and network-free whenever dstow is already present"

const themeLong = `Theming: list themes, describe slots, emit colors.`

const themeExample = `  dstow theme list
  dstow theme slots
  dstow theme emit
  export DSTOW_COLORS=$(dstow theme emit catppuccin-mocha --format env)
  dstow theme emit cargo section1='bold yellow' --format toml > ~/.config/dstow/themes/mine.toml`

// slotsLong is the §2.4 theme slots block: the slot reference intro plus the
// value-grammar enumeration (git's color.* grammar), kept compact and in the
// help-text voice of the other commands.
const slotsLong = `Every generic slot and what it colors, each name shown in its own effective
style. dstow's internals — package states, check classes, severity prefixes,
prose roles — reach these slots through a fixed code-owned mapping (§7.2); each
description names the slot's consumers.

Slot values use git's color.* grammar: whitespace-separated words, in any
order. The first color word is the foreground, the second the background, a
third is an error. Color words are the 8 basics (black red green yellow blue
magenta cyan white), their bright* variants, an integer 0-255 (256-color), or
#RRGGBB hex. Write 'normal red' to set a background without touching the
foreground. Attributes, any number: bold dim italic ul blink reverse strike,
each negatable with no or no- (a negation renders as nothing — a themed slot
replaces its default wholesale); 'reset' comes first.

normal leaves a channel to the TERMINAL, not to dstow's default: a slot set to
normal replaces its default wholesale (§7.3 top-wins) and renders plain — the
only way to keep dstow's default for a slot is to leave it undeclared. default
differs: it emits the terminal-default code (SGR 39/49) rather than nothing.`

const emitLong = `Emit a theme's colors — the effective palette (no name), a named theme, or
either with slot=value tweaks layered on top. The default view renders each
slot's value in its own style; --format env|toml emits for machines.

Values use git's color.* grammar — see 'dstow theme slots --help'.`

const emitExample = `  dstow theme emit
  dstow theme emit catppuccin-mocha
  export DSTOW_COLORS=$(dstow theme emit catppuccin-mocha --format env)
  dstow theme emit cargo section1='bold yellow' --format toml > ~/.config/dstow/themes/mine.toml`

// themeListShort, themeSlotsShort, and themeEmitShort are the verb descriptions
// in the §2.4 theme block.
const themeListShort = "List the available themes: bundled presets and your themes dir, active theme marked"

const themeSlotsShort = "Describe every color slot: what it colors and its consumers, plus the value grammar"

const themeEmitShort = "Emit a theme's colors, each value in its own style: the effective palette (bare), a named theme, slot=value tweaks on top; --format env|toml for machines"
