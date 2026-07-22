# dstow repo remove

<!-- dstow:short -->
Unregister a repo (deletes managed clones only)
<!-- /dstow:short -->

<!-- dstow:long -->
Unregister a repo. Managed clones are deleted; local-path repos are only
forgotten — your directory is never touched.

Refuses while the repo still has stowed links (offers unstow-then-remove),
and refuses to delete a managed clone holding work not present at its
source. Every refusal names its remedy.
<!-- /dstow:long -->

## The two guards

remove refuses in two situations:

- **Still-stowed links** — applies to every repo. Removing it would leave its
  deployed links dangling.
- **A managed clone holding unsynced work** — applies to managed clones only. If
  the clone has commits not present at its source, deleting it would lose them.
  A local-path repo has no such guard: removal only forgets it, so there is
  nothing to lose.

## The flag asymmetry

The two guards and the two flags line up precisely:

- **`--unstow`** unstows the repo's packages first, without prompting, then
  removes it — the answer to the still-stowed guard.
- **`--force`** overrides **both** guards. On a managed clone with unpushed
  commits it will delete them, so it is the flag that can lose work.
- **`-y` overrides neither.** These are guards, not confirmations of stated
  intent, and `-y` never answers a guard. `dstow manual reference` has the
  `--yes` rule in full.

## What removal does, per kind

- A **managed clone** is deleted from the managed directory.
- A **local-path repo** is only forgotten from the registry; its directory is
  left exactly as it was. `dstow manual reference files` covers which repos live
  where.
