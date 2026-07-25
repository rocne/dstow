# dstow rebuild

<!-- dstow:short -->
Reconstruct a lost ledger by walking configured targets (rare)
<!-- /dstow:short -->

<!-- dstow:long -->
Reconstruct a lost ledger by scanning configured targets for links into
known repos. The only full tree walk dstow has; explicit and rare.
<!-- /dstow:long -->

## It rebuilds the record, never the links

rebuild reconstructs **the ledger**. It never creates, removes, or repairs a
deployment — no link on disk changes. It scans the union of the effective
targets of every package of every registered repo, looking for symlinks that
resolve into a known repo, and records those as the ledger. The scan is
`lstat`-based and never descends *through* a symlink, so a folded-tree link is
recorded as one entry rather than being followed into.

Each scanned target root's ledger group is **replaced wholesale** by what the
walk found there. A root whose walk fails part-way — a permission error mid-tree
— is left untouched rather than emptied, since replacing a group from a partial
sighting would claim absence it never observed. A missing root is a clean
sighting of nothing.

## When to reach for it

rebuild is for when the ledger is **lost** — deleted or gone — not for a
disagreement between the ledger and disk, which is `dstow manual commands
check` and `dstow manual commands clean`'s job. Run rebuild and the ledger is
reconstructed from what is deployed.

Two ledger conditions are worth distinguishing, because rebuild is not the
answer to both:

- A **corrupt** ledger — one dstow cannot parse — is refused by every other
  command, which is exactly what rebuild is for. It is the one command that
  does not need the old contents, since it reconstructs them from disk, so it
  reads past the corruption and replaces the file. You do not have to move
  anything aside first.

  It **warns** when it does this, because the unreadable file is discarded
  along with whatever it recorded: rebuild re-records the target roots it
  scans, and any links the old ledger tracked under a root this rebuild does
  not scan are no longer tracked. Restoring a backup, if you have one, keeps
  more than rebuilding does.
- A ledger written by a **newer** dstow is not rebuild's case at all: the remedy
  is to upgrade dstow, which the refusal names. dstow will not rewrite a
  newer-schema ledger down to an older one.

`dstow manual reference exit-codes` covers these refusals from the exit-code
side.
