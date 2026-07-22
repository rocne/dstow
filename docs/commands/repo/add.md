# dstow repo add

<!-- dstow:short -->
Register a repo from a source (path, URL, github:owner/name)
<!-- /dstow:short -->

<!-- dstow:long -->
Register a repo — where packages come from.

Sources: a local directory path, a full URL/ssh form, or a qualified source
like github:owner/name (bare owner/name asks first; in scripts, qualify).
Remote sources clone into the managed directory; local paths are registered
in place and never modified.

Adding stows nothing: dstow announces the repo's packages and any bare
names that now need qualification. If the path would need percent-encoding
in name expressions, dstow shows the encoded form and asks whether to
continue or rename first. Re-adding a present repo is a safe, announced
no-op.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow repo add ~/dotfiles
  dstow repo add github:rocne/dotfiles
  dstow repo add rocne/dotfiles --stow
<!-- /dstow:examples -->
