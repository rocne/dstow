# dstow stow

<!-- dstow:short -->
Link packages into their targets
<!-- /dstow:short -->

<!-- dstow:long -->
Link packages into their targets.

Names are packages or repos, by any unambiguous suffix of the qualified
name; naming a repo stows all of its packages. With no names, dstow asks
before stowing everything — in scripts that is an error: pass --all.

Each package succeeds or fails on its own; the run continues past failures
and exits nonzero if any package failed. Explicitly naming a package or
repo overrides its exclude-from-bulk setting.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow stow zsh git tmux
  dstow stow dotfiles              # a repo: all of its packages
  dstow stow dots::zsh             # qualified just enough to be unique
  dstow stow --all --adopt         # first run on a machine with live files
<!-- /dstow:examples -->

## What a run does, in order

A run acts in **canonical qualified-name order** — repos by their FQN, packages
within a repo by theirs — no matter what order or spelling you gave the
operands. `dstow stow zsh git`, `dstow stow git zsh`, and `dstow stow dotfiles`
all deploy the same packages in the same sequence. The order is fixed so a run
is reproducible; it matters when your hooks have side effects, since the hook
firing order follows it (`dstow manual hooks`).

Each package finishes with one outcome:

- **succeeded** — its plan applied.
- **failed** — something in its own plan went wrong (an unresolvable target, a
  malformed package config), and only that package is affected.
- **blocked** — a repo-level or global-level `pre-` hook refused, so every
  package beneath that scope is blocked without being attempted. A package's
  own `pre-` hook failing is that package's failure, not a block on others. See
  `dstow manual hooks` for how a `pre-` hook gates what nests under it.
- **not found** — an operand resolved to nothing. This is a per-package outcome
  reported inline, not a refusal of the whole run: the other operands still run,
  and a genuinely malformed invocation is a different thing (`dstow manual
  reference exit-codes`).

The run exits `0` only when every package succeeded; any other outcome —
failed, blocked, or not found — exits `1`.

## The quiet cases

- **An empty plan is a silent no-op.** A package with nothing to deploy (after
  ignores and dot-translation) succeeds without output and **fires no hooks** —
  the nothing-to-do path stays quiet all the way down, so a `pre-`/`post-` pair
  never runs for a package that had no work.
- **`--dry-run` fires no hooks and asks nothing.** It prints the plan each
  package would apply — additions, and `--adopt`'s imports marked — and then
  changes nothing on disk and nothing in the ledger. No `pre-`/`post-` hook
  runs, and no confirmation is asked, including the differing-content
  confirmation `--adopt` would otherwise raise.

## The target directory and the first run

A real run **creates the target directory when it is missing** and announces
it. On the very first deploy on a machine — before any ledger exists — dstow
also announces that tree-folding is off by default, naming the global config
file where `fold_trees` would turn it on. Both are one-time notices about a
surprise, not routine chatter, so `--quiet` keeps them.

The bulk gate, `--all`, and `exclude_from_bulk` are covered in the summary
above; the setting itself lives in `dstow manual configuration keys`.
