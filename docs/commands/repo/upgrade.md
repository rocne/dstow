# dstow repo upgrade

<!-- dstow:short -->
Fast-forward clean clones to what update downloaded
<!-- /dstow:short -->

<!-- dstow:long -->
Sync remote repos in two explicit phases. Neither ever runs on its own.

With no repos named, both act on every remote repo. update touches the
network and nothing else — afterwards, status reports behind/ahead.
upgrade is fast-forward only: divergence or local work refuses loudly,
with no stash, merge, or rebase negotiation. upgrade never re-stows;
structural drift shows up in status. Linked files change the moment
upgrade moves them — update first, review, then upgrade.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow repo update
  dstow status
  dstow repo upgrade rocne/dotfiles
<!-- /dstow:examples -->
