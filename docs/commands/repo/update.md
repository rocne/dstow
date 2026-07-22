# dstow repo update

<!-- dstow:short -->
Download remote repo changes; touch nothing on disk
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

## update is the fetch phase

update touches the **network and nothing else**. It fetches each remote repo's
upstream and updates dstow's knowledge of it; no working tree changes, and no
deployed link moves. After it, `dstow manual commands status` reports each
repo's behind/ahead **from what was fetched, with no further network** — the
fetch is what makes an offline status meaningful.

Per-repo outcomes, and the run **continues past a failure**: one repo that
cannot be reached does not stop the others.

## What has no upstream is skipped

A repo with no upstream to fetch — a local-path repo, or a `DSTOW_PATH` session
repo (`dstow manual reference environment`) — has nothing to sync. Name such a
repo explicitly and update says so, telling you why nothing happened; in a
run over everything it is simply passed over.

If **git is not installed**, that surfaces as the affected repo's outcome —
naming git as the missing tool — never as a crash. `dstow manual reference
exit-codes` covers the git-side refusals from the exit-code side.
