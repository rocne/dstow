# dstow stow

<!-- dstow:short -->
Link packages into their targets
<!-- /dstow:short -->

<!-- dstow:long -->
Link packages into their targets.

Names are packages or repos, by any unambiguous suffix of the qualified
name; naming a repo stows all of its packages. With no names, dstow asks
before stowing everything — in scripts that is an error: pass --all.

Each package succeeds or fails on its own; the run continues past failures
and exits nonzero if any package failed. Explicitly naming a package or
repo overrides its exclude-from-bulk setting.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow stow zsh git tmux
  dstow stow dotfiles              # a repo: all of its packages
  dstow stow dots::zsh             # qualified just enough to be unique
  dstow stow --all --adopt         # first run on a machine with live files
<!-- /dstow:examples -->
