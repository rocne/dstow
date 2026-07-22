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
