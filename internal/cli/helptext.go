package cli

// This file carries the approved canonical help text (DESIGN.md §2.3 top-level
// and §2.4 per-command), verbatim. cli prints these strings through a custom
// help func rather than cobra's template, so the output matches the pinned
// text exactly (A2: help is the requested data, printed to stdout). Editing a
// string here is a design change, not an implementation one.

// topLevelHelp is DESIGN.md §2.3, verbatim.
const topLevelHelp = `dstow — deploy dotfiles and configuration as symlinks, from packages in repos

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
`

// stowHelp is DESIGN.md §2.4 stow/unstow/restow, verbatim.
const stowHelp = `Link packages into their targets.

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
`

// unstowHelp/restowHelp mirror stow in shape (§2.4: "identical in shape"), with
// the verb-specific opening line and the destructive/idempotent notes.
const unstowHelp = `Remove packages' links from their targets.

Usage:
  dstow unstow <name>... [flags]
  dstow unstow --all

Names are packages or repos, by any unambiguous suffix of the qualified
name; naming a repo unstows all of its packages. With no names, dstow asks
before unstowing everything — in scripts that is an error: pass --all.
unstow is the destructive verb; the bare confirm guards it with the same
care.

Each package succeeds or fails on its own; the run continues past failures
and exits nonzero if any package failed. Explicitly naming a package or
repo overrides its exclude-from-bulk setting.

Flags:
      --all       Every package of every registered repo, without asking
  -n, --dry-run   Show the plan; change nothing

Examples:
  dstow unstow zsh git tmux
  dstow unstow dotfiles            # a repo: all of its packages
`

const restowHelp = `Unstow, then stow again (refresh links).

Usage:
  dstow restow <name>... [flags]
  dstow restow --all

Names are packages or repos, by any unambiguous suffix of the qualified
name; naming a repo restows all of its packages. With no names, dstow asks
before restowing everything — in scripts that is an error: pass --all.
restow is the idempotent refresh verb: on a package that is not stowed it
simply stows it (the unstow phase no-ops).

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
  dstow restow zsh git tmux
  dstow restow --all --adopt
`

// adoptHelp is DESIGN.md §2.4 adopt, verbatim.
const adoptHelp = `Import an existing real file into a package; a link takes its place.
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
`

// repoHelp is DESIGN.md §2.4 repo (group), verbatim.
const repoHelp = `Manage repos — where packages come from.

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
`

// repoAddHelp is DESIGN.md §2.4 repo add, verbatim.
const repoAddHelp = `Register a repo — where packages come from.

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
`

// repoRemoveHelp is DESIGN.md §2.4 repo remove, verbatim.
const repoRemoveHelp = `Unregister a repo. Managed clones are deleted; local-path repos are only
forgotten — your directory is never touched.

Usage:
  dstow repo remove <repo> [flags]

Refuses while the repo still has stowed links (offers unstow-then-remove),
and refuses to delete a managed clone holding work not present at its
source. Every refusal names its remedy.

Flags:
      --unstow   Unstow the repo's packages first, without prompting
      --force    Override both guards (unsaved work will be lost)
`

// repoUpdateHelp / repoUpgradeHelp share the DESIGN.md §2.4 two-phase block,
// with each verb's own opening line and usage.
const repoUpdateHelp = `Download remote repo changes; touch nothing on disk.

Usage:
  dstow repo update [<repo>...]

With no repos named, acts on every remote repo. update touches the network
and nothing else — afterwards, status reports behind/ahead. It never alters
a working tree; upgrade is the separate, explicit apply phase.

Examples:
  dstow repo update
  dstow status
`

const repoUpgradeHelp = `Fast-forward clean clones to what update downloaded; report old -> new.

Usage:
  dstow repo upgrade [<repo>...]

With no repos named, acts on every remote repo. upgrade is fast-forward
only: divergence or local work refuses loudly, with no stash, merge, or
rebase negotiation. It never re-stows; structural drift shows up in status.
Linked files change the moment upgrade moves them — update first, review,
then upgrade.

Examples:
  dstow repo upgrade rocne/dotfiles
`

// listHelp is DESIGN.md §2.4 list, verbatim.
const listHelp = `What is configured — repos, packages, targets, exclusions, sources.
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
`

// infoHelp is DESIGN.md §2.4 info, verbatim.
const infoHelp = `One scope's fields — the facts and effective config dstow holds about it,
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
`

// statusHelp is DESIGN.md §2.4 status, verbatim.
const statusHelp = `What is deployed — expected links against what targets actually hold.

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
`

// checkHelp is DESIGN.md §2.4 check, verbatim.
const checkHelp = `Verify every ledgered link — instant, no tree walk.

Usage:
  dstow check [flags]

Classifies stale links: broken (destination gone) and orphaned (resolves
into a known repo, but no current package config would produce it). clean
executes exactly this report — the two can never disagree.

Flags:
      --json   Machine-readable report
`

// cleanHelp is DESIGN.md §2.4 clean, verbatim.
const cleanHelp = `Execute exactly what check reported.

Usage:
  dstow clean [flags]

Broken links are removed freely. Orphans are shown and confirmed (--yes
removes them without asking). Nothing else is ever touched.

Flags:
      --force   Remove orphans without confirmation in any context
`

// rebuildHelp is DESIGN.md §2.4 rebuild, verbatim.
const rebuildHelp = `Reconstruct a lost ledger by scanning configured targets for links into
known repos. The only full tree walk dstow has; explicit and rare.

Usage:
  dstow rebuild
`

// snippetHelp is DESIGN.md §2.4 snippet (group), verbatim.
const snippetHelp = `Print canned POSIX-sh snippets to stdout. dstow never edits rc files.

Usage:
  dstow snippet <command>

Commands:
  rc            The shell-rc bootstrap: installs dstow iff absent — silent
                and network-free whenever dstow is already present

Examples:
  dstow snippet rc >> ~/.bashrc
`

// colorsHelp is DESIGN.md §2.4 colors (group), verbatim.
const colorsHelp = `Theming utilities.

Usage:
  dstow colors <command>

Commands:
  theme         Emit a named theme — as a packed DSTOW_COLORS string
                (default) or a theme file (--format toml)

Examples:
  export DSTOW_COLORS=$(dstow colors theme catppuccin-mocha)
  dstow colors theme catppuccin-mocha --format toml > ~/.config/dstow/themes/mine.toml
`
