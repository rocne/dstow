# dstow manual

The complete dstow documentation, reachable from the command line alone. No
external document is needed to use dstow, script it, or write a hook for it.

Every topic prints itself and lists the topics beneath it, so the tree is
navigable without `--help`: run `dstow manual <topic>`, then follow what it
lists. Tab completion walks it too — `dstow manual <TAB>`.

## Topics

- `commands` — one page per dstow command, mirroring the command tree. Each
  page is also where that command's help text comes from, so
  `dstow manual commands repo add` and `dstow repo add --help` are the same
  content.
- `concepts` — the model dstow works in: repos, packages, targets, the ledger;
  the naming grammar; the package states.
- `configuration` — the four config levels, the key schema and where each key
  is legal, ignore patterns, and GNU Stow compatibility.
- `theming` — how dstow colorizes, the slot vocabulary, and the value grammar.
- `hooks` — running your own executables around deploy actions, and the
  context contract dstow passes them.
- `reference` — exit codes, environment variables, and file locations.

## Where to start

    dstow manual concepts            what dstow is actually doing
    dstow manual commands            the command surface
    dstow manual reference           exit codes, environment, file locations

## Scripting dstow

Four facts cover most of it, and each has its own page:

- **Exit codes** — `dstow manual reference exit-codes`. Four codes, with a
  stable meaning each; `2` is always malformed invocation, never a negative
  answer.
- **Machine output** — `--json` on `list`, `info`, `status`, and `check`. The
  JSON key vocabulary is `snake_case`, the same spelling as config keys.
- **Non-interactive behavior** — dstow refuses rather than guesses when it
  cannot ask. `-y`/`--yes` answers confirmations of *stated* intent only; it
  never resolves an ambiguity, never answers the bulk prompt, and never
  bypasses a guard. Bulk operations want `--all` in a script.
- **Environment** — `dstow manual reference environment`.

## How this manual relates to help

`docs/commands/` is the single owner of dstow's help text: a command's page
here *is* the source of its `--help`. The tagged regions of a command page
(`<!-- dstow:short -->`, `long`, `examples`) are what cobra renders; anything
untagged is manual-only. Help and manual therefore cannot disagree — there is
only one copy.

The topics outside `commands` are manual-only: they carry the cross-command
material that has no single command to hang off.
