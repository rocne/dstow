# The dstow model

dstow links the files in a *package* into a *target* directory, the same way
GNU Stow does — but it remembers what it deployed, works across many repos at
once, names things by a stable qualified identity, and never depends on the
directory you happen to be standing in.

## Topics

- `naming` — the qualified-name grammar, suffix matching, and how dstow tells
  a name from a path.
- `states` — the package states (`stowed`, `occupied`, `damaged`, …), what
  each one claims, and the precedence between them.

## The four things

- **Repo** — where packages come from. A repo is registered once
  (`dstow repo add`) and remembered in dstow's own registry. It is either
  **managed** (a remote dstow cloned for you, living under dstow's managed
  directory) or a **local path** (registered in place; dstow never modifies
  the directory, and removing the repo only forgets it).
- **Package** — a top-level directory inside a repo, holding a tree of files
  to link into the target. Package identity is **locational**: there are no
  marker files to add, so a migrated GNU Stow repo works out of the box and
  `mkdir`-and-go stays intact. Hidden directories are never packages. By
  default packages sit at the repo root; a repo may instead collect them under
  a subdirectory (`packages_dir`).
- **Target** — where the links go. `$HOME` unless configured otherwise; any
  level of config can point elsewhere.
- **Ledger** — dstow's record of every link it currently believes it made, so
  `status`, `check`, and `clean` can reason about what *should* be there
  versus what *is*.

## The three views

dstow keeps three questions apart, and gives you a command for each. Which
sources a command reads is part of its contract, not an implementation detail:

| Question | Command | Reads |
|---|---|---|
| What do I have configured? | `dstow list` | config only, never disk |
| What does dstow know about this? | `dstow info` | config + metadata, never disk |
| What is actually deployed? | `dstow status` | the live filesystem |

`list` and `info` are therefore instant and safe on a machine whose targets
are unavailable; `status` is the one that inspects reality.

## The ledger

The ledger is a **current-state index, not a journal**: its entries are links
dstow currently believes exist. Unstowing deletes entries. There is no
history, and the ledger is not a cache — it records intent that cannot be
re-derived from disk with certainty.

It is one JSON document per machine, under `$XDG_STATE_HOME/dstow/`, written
atomically and updated under a whole-operation lock. You never hand-edit it.

Three commands work on it directly:

- **`check`** verifies every ledgered link without walking any tree. It is
  instant and read-only, and classifies what it finds: a **broken** link
  (destination gone), an **orphaned** link (it resolves into a known repo, but
  no current configuration would produce it), and a **contradicted** entry
  (dstow linked this path and disk now disagrees).
- **`clean`** executes exactly what `check` reported — the two share one
  classifier, so they can never disagree. Broken links are removed freely;
  orphans are confirmed before removal; contradicted entries are pruned from
  the ledger with disk left untouched.
- **`rebuild`** reconstructs a lost ledger by walking the configured targets.
  It is the only full tree walk dstow has: explicit, rare, and for when the
  ledger is gone.

## Per-package independence

Every package succeeds or fails on its own. A bulk run continues past a
failed package and exits non-zero if any package failed — one broken package
never strands the rest.

## What dstow never does

- **Never guesses from the working directory.** Where you stand changes
  nothing about what a command does.
- **Never edits your config files.** There is no config-mutation command: you
  declare configuration by editing a file. The one file dstow writes is the
  repo registry, which is dstow's, and which you never hand-edit.
- **Never destroys live content on adopt.** Adoption imports the file that is
  really there into the package, leaving a link behind — live content wins.
- **Never merges, rebases, or stashes.** `repo upgrade` is fast-forward only;
  divergence or local work refuses loudly.
