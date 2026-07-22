# dstow unstow

<!-- dstow:short -->
Remove packages' links from their targets
<!-- /dstow:short -->

<!-- dstow:long -->
Remove packages' links from their targets.

Names are packages or repos, by any unambiguous suffix of the qualified
name; naming a repo unstows all of its packages. With no names, dstow asks
before unstowing everything — in scripts that is an error: pass --all.
unstow is the destructive verb; the bare confirm guards it with the same
care.

Each package succeeds or fails on its own; the run continues past failures
and exits nonzero if any package failed. Explicitly naming a package or
repo overrides its exclude-from-bulk setting.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow unstow zsh git tmux
  dstow unstow dotfiles            # a repo: all of its packages
<!-- /dstow:examples -->

## What it removes, and what it leaves

unstow removes **only the links the ledger attests dstow made for that
package** — nothing else at the target is ever touched. A real file you put
there, or a foreign symlink some other tool made, stays exactly where it is.

If the ledger records a link at a path but disk now holds something else — a
real file where the link used to be, say — unstow does not delete what is
there. It removes its stale ledger entry instead, with a note explaining the
mismatch, and leaves the file alone. dstow only ever deletes a link it can
still see is its own. (This is the `contradicted` finding class, seen from the
deploy side; `dstow manual concepts states` has the vocabulary.)

A target directory that does not exist makes unstow **trivially done**: there
are no links to remove, and dstow invents no directory to go looking in.

## Orphans are a different job

unstow acts on the package you name. Links that a package *used to* produce but
no longer does — because its files or its config changed — are **orphaned**,
and they are not unstow's to clean up: unstowing the package does not remove a
link the package would no longer make. That is `dstow manual commands clean`'s
work, driven by `dstow manual commands check`.
