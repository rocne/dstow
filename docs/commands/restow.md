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

## It reconciles the record, even when nothing on disk moves

A link that is already correct produces **no action** — restow does not remove
and re-create a symlink that already points where it should. But the package's
ledger entries are still **reconciled to what deploying it now expects**. So a
restow after a configuration change repairs the record even where nothing on
disk had to move: if the ledger had drifted from the deployment it describes,
restow brings the two back into agreement without disturbing the links
themselves.

That makes restow the natural follow-up to a change that alters what dstow
*expects* without changing what is *deployed* — after which `dstow manual
commands status` and `dstow manual commands check` will agree with disk again.

## Hooks

restow fires only the `pre-restow`/`post-restow` pair — never stow's or
unstow's, even though it does both internally. The event names the action you
asked for, not the steps it decomposes into; `dstow manual hooks` has the rule.
