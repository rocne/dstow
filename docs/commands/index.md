# The dstow command surface

<!-- dstow:short -->
deploy dotfiles and configuration as symlinks, from packages in repos
<!-- /dstow:short -->

<!-- dstow:long -->
dstow — deploy dotfiles and configuration as symlinks, from packages in repos

Name packages and repos by any unambiguous suffix of their qualified name
(github:rocne/dotfiles::zsh). The working directory never changes what a
command does. See 'dstow <command> --help' for details and examples.
Run 'dstow manual' for the full documentation.
<!-- /dstow:long -->

## Topics

Every dstow command has a page here, mirroring the command tree: a group's page
is its directory's index.md, a leaf's page is its own file. Each page is both
the command's manual page and the source of its help text.

**Deploy** — change what is on disk:

- `stow` — link packages into their targets
- `unstow` — remove packages' links from their targets
- `restow` — unstow, then stow again; the idempotent refresh
- `adopt` — import an existing file into a package, leaving a link behind

**Inspect** — three questions, three sources (see `dstow manual concepts`):

- `list` — what is configured: repos, packages, targets. Never reads disk
- `info` — everything dstow knows about one repo or package
- `status` — what is deployed: live state against the targets

**Maintain** — the ledger and the links it records:

- `check` — verify every ledgered link; classify broken and orphaned
- `clean` — execute exactly what check reported
- `rebuild` — reconstruct a lost ledger by walking configured targets

**Groups**:

- `repo` — add, remove, update, upgrade the repos packages come from
- `snippet` — print canned shell snippets, starting with the rc bootstrap
- `theme` — list themes, describe slots, emit colors
- `name` — encode and decode name segments (a hidden scripting utility)

**Also**:

- `version` — print the version
- `completion` — generate shell completion (cobra's own; not documented here)
