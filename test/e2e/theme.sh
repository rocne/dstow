#!/bin/sh
set -e

# theme group over the two-stage vocabulary (#115): the generic fourteen-slot
# roster (§3.3), tier derivation (§7.3), and the emission round-trip, driven
# through the real binary. Also the slot-vocabulary reference (theme slots,
# #116) and the emit rename (was theme show, #117).
export HOME=/home/e2e
export XDG_CONFIG_HOME=/home/e2e/.config
export XDG_STATE_HOME=/home/e2e/.local/state
export XDG_DATA_HOME=/home/e2e/.local/share
mkdir -p /home/e2e

# The fourteen slots in canonical §3.3 order, reused across checks.
slots="section1 section2 name1 name2 value1 value2 \
       error1 error2 warning1 warning2 success1 success2 info1 info2"

# Bare theme emit renders the effective stack: all fourteen generic slots —
# the seven declared tier-1s plus every derived tier-2 (§7.3 derivation).
out=$(dstow theme emit) || { printf 'FAIL: theme emit exited nonzero\n'; exit 1; }
count=$(printf '%s\n' "$out" | wc -l)
if [ "$count" -ne 14 ]; then
  printf 'FAIL: theme emit rendered %s rows, want 14\n%s\n' "$count" "$out"
  exit 1
fi
for slot in $slots; do
  printf '%s\n' "$out" | grep -q "^${slot} " \
    || { printf 'FAIL: theme emit missing slot %s\n%s\n' "$slot" "$out"; exit 1; }
done

# The old noun is gone with no alias (#117): theme show is now an unknown
# command (cobra's usage error, exit 2).
if dstow theme show 2>/dev/null; then
  printf 'FAIL: theme show still resolves; the rename kept no alias\n'
  exit 1
fi

# Derivation is remove-bold-else-add-dim off the family tier-1: the default
# error1 is "bold brightred", so the underived error2 must be "brightred".
printf '%s\n' "$out" | grep -q '^error1 *bold brightred$' \
  || { printf 'FAIL: error1 default is not bold brightred\n%s\n' "$out"; exit 1; }
printf '%s\n' "$out" | grep -q '^error2 *brightred$' \
  || { printf 'FAIL: error2 did not derive to brightred\n%s\n' "$out"; exit 1; }

# The old internal vocabulary is not a slot: a stowed= operand is a usage
# error (exit 2), and the remedy names the generic roster.
if dstow theme emit stowed=red 2>/tmp/err; then
  printf 'FAIL: stowed= operand accepted; internals are not slots\n'
  exit 1
fi
grep -q 'success2' /tmp/err \
  || { printf 'FAIL: unknown-slot remedy does not name the roster\n'; cat /tmp/err; exit 1; }

# theme slots is the slot-vocabulary reference (#116): all fourteen slots on
# stdout, each with a description. The header is commentary (stderr), so the
# stdout rows are pure.
slotsout=$(dstow theme slots 2>/dev/null) \
  || { printf 'FAIL: theme slots exited nonzero\n'; exit 1; }
slotscount=$(printf '%s\n' "$slotsout" | wc -l)
if [ "$slotscount" -ne 14 ]; then
  printf 'FAIL: theme slots printed %s rows, want 14\n%s\n' "$slotscount" "$slotsout"
  exit 1
fi
for slot in $slots; do
  printf '%s\n' "$slotsout" | grep -q "^${slot} " \
    || { printf 'FAIL: theme slots missing slot %s\n%s\n' "$slot" "$slotsout"; exit 1; }
done
# The description enumerates the state consumers from the code-owned mapping.
printf '%s\n' "$slotsout" | grep -q '^error1 .*damaged.*contradicted' \
  || { printf 'FAIL: error1 row omits its state consumers\n%s\n' "$slotsout"; exit 1; }

# Emission round-trip through the environment: a preset packs to DSTOW_COLORS
# form, and re-emitting under it reproduces the same slot values.
packed=$(dstow theme emit catppuccin-mocha --format env) \
  || { printf 'FAIL: --format env exited nonzero\n'; exit 1; }
case "$packed" in
  *"name1=bold #89b4fa"*) ;;
  *) printf 'FAIL: packed mocha missing name1: %s\n' "$packed"; exit 1 ;;
esac
reshow=$(DSTOW_COLORS="$packed" dstow theme emit) \
  || { printf 'FAIL: theme emit under DSTOW_COLORS exited nonzero\n'; exit 1; }
printf '%s\n' "$reshow" | grep -q 'name1 *bold #89b4fa' \
  || { printf 'FAIL: DSTOW_COLORS round-trip lost name1\n%s\n' "$reshow"; exit 1; }

# The 'normal red' idiom — a background without touching the foreground —
# round-trips: overriding error1 with it and re-emitting reproduces it verbatim.
nr=$(dstow theme emit error1='normal red' --format env) \
  || { printf 'FAIL: normal red override exited nonzero\n'; exit 1; }
case "$nr" in
  *"error1=normal red"*) ;;
  *) printf 'FAIL: normal red idiom lost in emission: %s\n' "$nr"; exit 1 ;;
esac

# slot=value operands override on top and survive toml emission.
dstow theme emit cargo section1='bold yellow' --format toml | grep -q '^section1 = "bold yellow"$' \
  || { printf 'FAIL: section1 override lost in toml emission\n'; exit 1; }

printf 'PASS: theme group speaks the fourteen-slot generic vocabulary\n'
