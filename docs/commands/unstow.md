# dstow unstow

<!-- dstow:short -->
Remove packages' links from their targets
<!-- /dstow:short -->

<!-- dstow:long -->
Remove packages' links from their targets.

Names are packages or repos, by any unambiguous suffix of the qualified
name; naming a repo unstows all of its packages. With no names, dstow asks
before unstowing everything — in scripts that is an error: pass --all.
unstow is the destructive verb; the bare confirm guards it with the same
care.

Each package succeeds or fails on its own; the run continues past failures
and exits nonzero if any package failed. Explicitly naming a package or
repo overrides its exclude-from-bulk setting.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow unstow zsh git tmux
  dstow unstow dotfiles            # a repo: all of its packages
<!-- /dstow:examples -->
