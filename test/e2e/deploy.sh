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
