#!/bin/sh
set -e

# A stow / list / status happy path over a local repo (the cli ticket wires the
# whole surface end to end). Assertions trace to DESIGN §2.4 behavior.
export HOME=/home/e2e
export XDG_CONFIG_HOME=/home/e2e/.config
export XDG_STATE_HOME=/home/e2e/.local/state
export XDG_DATA_HOME=/home/e2e/.local/share
mkdir -p /home/e2e

# A local repo with one package: zsh, holding dot-zshrc (dot-translation on by
# default → deploys to ~/.zshrc).
mkdir -p /home/e2e/dots/zsh
printf 'export E2E=1\n' > /home/e2e/dots/zsh/dot-zshrc

# repo add registers in place and never modifies a local path (§2.4 add).
dstow repo add /home/e2e/dots >/dev/null 2>&1 \
  || { printf 'FAIL: repo add exited nonzero\n'; exit 1; }

# list shows the repo (bare list = the global scope's content, §2.4).
dstow list | grep -q 'dots' \
  || { printf 'FAIL: list does not show the added repo\n'; exit 1; }

# list <repo> shows its packages.
dstow list dots | grep -q '^zsh$' \
  || { printf 'FAIL: list <repo> does not show the package\n'; exit 1; }

# stow the package: the run continues past failures, exits 0 on success (§2.4).
dstow stow zsh || { printf 'FAIL: stow exited nonzero\n'; exit 1; }

# The link now exists in the target and points into the package.
if [ ! -L /home/e2e/.zshrc ]; then
  printf 'FAIL: stow did not create the ~/.zshrc symlink\n'
  exit 1
fi

# status inspects reality (§2.4) and reports the package stowed, on stdout.
dstow status zsh | grep -q 'stowed' \
  || { printf 'FAIL: status does not report zsh stowed\n'; exit 1; }

# status --json spells the state verbatim (O10).
dstow status zsh --json | grep -q '"state": "stowed"' \
  || { printf 'FAIL: status --json state string wrong\n'; exit 1; }

# check finds a healthy ledger (exit 0, no findings).
dstow check || { printf 'FAIL: check on a healthy ledger exited nonzero\n'; exit 1; }

# unstow removes the link.
dstow unstow zsh || { printf 'FAIL: unstow exited nonzero\n'; exit 1; }
if [ -e /home/e2e/.zshrc ] || [ -L /home/e2e/.zshrc ]; then
  printf 'FAIL: unstow did not remove the link\n'
  exit 1
fi

printf 'PASS: stow / list / status / check / unstow happy path\n'

# --- #146: dry-run must never fail where the real run succeeds ---------------
# A first deploy on a new machine is dry-run's primary scenario, and the exact
# case where the target directory does not exist yet. It used to fail there
# ("canon_path: cannot chdir to ..."), while the real run created the directory
# and stowed — the preview more restrictive than the execution, backwards.
export HOME=/home/e2e-drynewtarget
export XDG_CONFIG_HOME=$HOME/.config
export XDG_STATE_HOME=$HOME/.local/state
export XDG_DATA_HOME=$HOME/.local/share
mkdir -p "$HOME/dots/tmux/.dstow" "$HOME/dots/.dstow"
printf 'set -g mouse on\n' > "$HOME/dots/tmux/dot-tmux.conf"
printf 'target = "%s/newtarget"\n' "$HOME" > "$HOME/dots/.dstow/config.toml"
dstow repo add "$HOME/dots" >/dev/null 2>&1

[ -d "$HOME/newtarget" ] && { printf 'FAIL: precondition — target already exists\n'; exit 1; }

DRY="$(dstow stow --dry-run tmux 2>&1)" && CODE=0 || CODE=$?
if [ "$CODE" != "0" ]; then
  printf 'FAIL: dry-run against a missing target exit = %s, want 0\n%s\n' "$CODE" "$DRY"
  exit 1
fi
printf '%s' "$DRY" | grep -q 'would create target directory' \
  || { printf 'FAIL: dry-run did not report the would-create:\n%s\n' "$DRY"; exit 1; }
printf '%s' "$DRY" | grep -q '.tmux.conf' \
  || { printf 'FAIL: dry-run did not plan the link:\n%s\n' "$DRY"; exit 1; }
# Dry-run stays mutation-free: it must not create the directory it previewed.
[ -d "$HOME/newtarget" ] && { printf 'FAIL: dry-run created the target directory\n'; exit 1; }

# The real run does what the plan said.
dstow stow tmux >/dev/null 2>&1 || { printf 'FAIL: real run failed after a clean plan\n'; exit 1; }
[ -L "$HOME/newtarget/.tmux.conf" ] \
  || { printf 'FAIL: the link the plan promised was not created\n'; exit 1; }

printf 'PASS: dry-run plans against a not-yet-existing target and creates nothing\n'
