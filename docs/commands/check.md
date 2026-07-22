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
