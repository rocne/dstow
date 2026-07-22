# dstow clean

<!-- dstow:short -->
Execute exactly what check reported (broken freely, orphans ask)
<!-- /dstow:short -->

<!-- dstow:long -->
Execute exactly what check reported.

Broken links are removed freely. Orphans are shown and confirmed (--yes
removes them without asking). Nothing else is ever touched.
<!-- /dstow:long -->

## What clean does per class

clean acts on exactly what `dstow manual commands check` classified, one
response per finding class (the class table is in `dstow manual concepts
states`):

- **broken** — removed freely, no confirmation.
- **orphaned** — removed **behind a confirmation**, because removing a link
  that still points at something real is a judgment about your intent.
- **contradicted** — the stale ledger entry is pruned; **disk is left
  untouched**, because the record was wrong, not the filesystem.
- **unobservable** — a report row only. clean never acts on a link it could not
  look at.

## What `--yes` and `--force` answer

clean is one of the two commands whose prompt is a confirmation of stated
intent, so **`-y` answers the orphan confirmation**: you said clean, and clean
is asking whether to carry out the removal it just described. `--force` skips
the confirmation in any context, interactive or not. Non-interactively, without
either, an orphan cannot be confirmed: clean leaves the link in place, reports
it as an error, and the run exits nonzero — a script that means to remove
orphans passes `--force` (broken links, which need no confirmation, are still
removed in the same run). The broader rule for `--yes` is in
`dstow manual reference`.

clean is not a tree cleaner: it only ever touches links the ledger records.
Anything outside the ledger is invisible to it.
