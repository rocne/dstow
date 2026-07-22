# dstow check

<!-- dstow:short -->
Verify every link in the ledger; classify broken and orphaned
<!-- /dstow:short -->

<!-- dstow:long -->
Verify every ledgered link — instant, no tree walk.

Classifies stale links: broken (destination gone) and orphaned (resolves
into a known repo, but no current package config would produce it). clean
executes exactly this report — the two can never disagree.
<!-- /dstow:long -->

## The full classification

The one-liner above names *broken* and *orphaned*, the two you act on most; the
complete classification has **four** classes. The other two are `contradicted`
(dstow's record and disk disagree) and `unobservable` (an observation the check
needed could not be made — a permission error, an unreadable destination). The
class is what decides what `clean` will do about each finding, so the full
table — every class, what it claims, and clean's response — lives in one place:
`dstow manual concepts states`.

check itself is **read-only**: it walks the ledger, touches nothing, and exits
`1` when it has any findings and `0` when every ledgered link is healthy.
