# dstow status

<!-- dstow:short -->
What is deployed: live state of packages against their targets
<!-- /dstow:short -->

<!-- dstow:long -->
What is deployed — expected links against what targets actually hold.

Names scope to packages or whole repos. Package states: stowed, partially
stowed, not stowed, occupied, damaged — plus a drifted marker when the
deployed shape differs from what current config would produce. damaged is
only ever claimed with ledger evidence. Remote repos also show
behind/ahead as of the last update.

For a path: what occupies it, who owns it (per the ledger), and — if
occupied — the packages that could adopt it, ranked.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow status
  dstow status zsh
  dstow status ~/.zshrc      # the per-path view, adoption candidates incl.
<!-- /dstow:examples -->
