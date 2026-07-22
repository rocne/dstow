# dstow restow

<!-- dstow:short -->
Unstow, then stow again (refresh links)
<!-- /dstow:short -->

<!-- dstow:long -->
Unstow, then stow again (refresh links).

Names are packages or repos, by any unambiguous suffix of the qualified
name; naming a repo restows all of its packages. With no names, dstow asks
before restowing everything — in scripts that is an error: pass --all.
restow is the idempotent refresh verb: on a package that is not stowed it
simply stows it (the unstow phase no-ops).

Each package succeeds or fails on its own; the run continues past failures
and exits nonzero if any package failed. Explicitly naming a package or
repo overrides its exclude-from-bulk setting.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow restow zsh git tmux
  dstow restow --all --adopt
<!-- /dstow:examples -->
