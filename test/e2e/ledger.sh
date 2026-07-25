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

# --- #145: rebuild is the remedy every other command names, so it must work ---
# A corrupt ledger is refused by every command; the refusal's fix: line says
# "dstow rebuild". Before this, rebuild refused with the identical error, so
# the remedy pointed at itself. rebuild is the one command that does not need
# the old contents.
export HOME=/home/e2e-corrupt
export XDG_CONFIG_HOME=$HOME/.config
export XDG_STATE_HOME=$HOME/.local/state
export XDG_DATA_HOME=$HOME/.local/share
mkdir -p "$HOME/dots/zsh"
printf 'export E2E=1\n' > "$HOME/dots/zsh/dot-zshrc"
dstow repo add "$HOME/dots" >/dev/null 2>&1
dstow stow zsh >/dev/null 2>&1

LEDGER="$XDG_STATE_HOME/dstow/ledger.json"
[ -f "$LEDGER" ] || { printf 'FAIL: precondition — no ledger to corrupt\n'; exit 1; }
printf 'this is not json{{{\n' > "$LEDGER"

# Every other command still refuses (exit 3) and names rebuild.
dstow status >/dev/null 2>&1 && CODE=0 || CODE=$?
[ "$CODE" = "3" ] || { printf 'FAIL: status on a corrupt ledger exit = %s, want 3\n' "$CODE"; exit 1; }
dstow status 2>&1 >/dev/null | grep -q 'dstow rebuild' \
  || { printf 'FAIL: the corrupt refusal no longer names dstow rebuild\n'; exit 1; }

# The named remedy works, in one command, with no manual file surgery.
OUT="$(dstow rebuild 2>&1)" && CODE=0 || CODE=$?
if [ "$CODE" != "0" ]; then
  printf 'FAIL: rebuild on a corrupt ledger exit = %s, want 0 (it is the named remedy)\n%s\n' "$CODE" "$OUT"
  exit 1
fi
# §6.5: corruption must never degrade into amnesia — the loss is announced.
printf '%s' "$OUT" | grep -q 'unreadable' \
  || { printf 'FAIL: rebuild recovered silently, without announcing the discarded contents:\n%s\n' "$OUT"; exit 1; }

# The ledger is readable again and tracks the deployed link.
dstow status >/dev/null 2>&1 || { printf 'FAIL: status still fails after rebuild\n'; exit 1; }
grep -q '"version"' "$LEDGER" || { printf 'FAIL: ledger was not rebuilt\n'; exit 1; }

# A NEWER ledger is not corruption: rebuild must refuse it and leave it alone.
printf '{"version": 99, "targets": {}}\n' > "$LEDGER"
dstow rebuild >/dev/null 2>&1 && CODE=0 || CODE=$?
[ "$CODE" = "3" ] || { printf 'FAIL: rebuild on a newer ledger exit = %s, want 3\n' "$CODE"; exit 1; }
grep -q '"version": 99' "$LEDGER" \
  || { printf 'FAIL: rebuild rewrote a newer-schema ledger down\n'; exit 1; }

printf 'PASS: rebuild recovers a corrupt ledger loudly; a newer one still refuses\n'
