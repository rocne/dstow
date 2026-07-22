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

## upgrade is the apply phase

upgrade moves a clone to what `update` already fetched — no network of its own.
It is **fast-forward only**: it reports the old and new revisions and whether
anything moved, and it advances a clone only when the fetched upstream is a
clean continuation of it. A clone that has **diverged**, or carries local work
the fast-forward would clobber, is **refused per repo** — the rest of the run
continues — and dstow **never attempts a stash, merge, or rebase** to force it;
the message tells you to resolve the divergence yourself or re-add the repo.

## It moves deployed files, and never re-stows

Deployed links point into the repo, so **the content behind them changes the
instant upgrade moves the clone** — an upgrade is a live change to what your
target resolves to, not a staged one. That is exactly why the two phases are
separate: `update` fetches, you review with `dstow manual commands status`, and
then `upgrade` applies. Nothing changes until you run this half.

upgrade **never re-stows**. If an upgrade changes a package's shape — files
added, removed, or renamed upstream — that shows up as drift in `dstow manual
commands status`, and `dstow manual commands restow` is the fix. Keeping
upgrade to the git fast-forward alone is what keeps its effect predictable.
