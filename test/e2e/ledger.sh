#!/bin/sh
set -e

# Regression for #181. On the macOS default layout dstow's global config dir
# and the ledger's state dir are the SAME directory: adrg/xdg maps both
# $XDG_CONFIG_HOME and $XDG_STATE_HOME to ~/Library/Application Support. The
# ledger files then land inside the config dir, and dstow's M5
# reserved-territory scan must NOT flag ledger.json / ledger.lock — the state
# files dstow wrote itself — as unexpected intruders. We reproduce the
# collision on Linux by pointing both XDG bases at one directory.
export HOME=/home/e2e-colo
COLO=/home/e2e-colo/appsupport
export XDG_CONFIG_HOME="$COLO"
export XDG_STATE_HOME="$COLO"
export XDG_DATA_HOME=/home/e2e-colo/.local/share
mkdir -p /home/e2e-colo

# A local repo with one package (dot-translation deploys dot-zshrc to ~/.zshrc).
mkdir -p /home/e2e-colo/dots/zsh
printf 'export E2E=1\n' > /home/e2e-colo/dots/zsh/dot-zshrc

dstow repo add /home/e2e-colo/dots >/dev/null 2>&1 \
  || { printf 'FAIL: repo add exited nonzero\n'; exit 1; }

# A mutation creates the ledger (ledger.json + ledger.lock) inside the shared dir.
dstow stow zsh >/dev/null 2>&1 || { printf 'FAIL: stow exited nonzero\n'; exit 1; }

# Precondition: the ledger really did land in the config dir (the collision).
[ -f "$COLO/dstow/ledger.json" ] \
  || { printf 'FAIL: precondition — ledger.json is not in the shared config dir\n'; exit 1; }

# A read command scans the reserved territory; capture its stderr only.
ERR="$(dstow list 2>&1 >/dev/null)"
if printf '%s' "$ERR" | grep -Eq 'unexpected entry "ledger\.(json|lock)"'; then
  printf 'FAIL: M5 flagged dstow own ledger files on the colocated layout:\n%s\n' "$ERR"
  exit 1
fi

# The allow-list is precise, not blanket: a genuinely stray entry in the shared
# dir MUST still warn.
printf 'boo\n' > "$COLO/dstow/stray.txt"
STRAY_ERR="$(dstow list 2>&1 >/dev/null)"
if ! printf '%s' "$STRAY_ERR" | grep -q 'unexpected entry "stray.txt"'; then
  printf 'FAIL: M5 no longer warns about a genuinely stray entry:\n%s\n' "$STRAY_ERR"
  exit 1
fi

printf 'PASS: colocated config/state dir — own ledger files unflagged, stray still warns\n'
