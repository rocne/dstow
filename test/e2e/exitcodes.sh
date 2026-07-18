#!/bin/sh
set -e

# The A3 exit-code map, exercised on the real binary: usage (2), refusal (3),
# success (0). Assertions trace to DESIGN §8.1 A3.
export HOME=/home/e2e
export XDG_CONFIG_HOME=/home/e2e/.config
export XDG_STATE_HOME=/home/e2e/.local/state
export XDG_DATA_HOME=/home/e2e/.local/share
mkdir -p /home/e2e

# version succeeds (exit 0) and prints on stdout.
if ! dstow version >/dev/null; then
  printf 'FAIL: dstow version exited nonzero\n'
  exit 1
fi

# An unknown flag is a usage error → exit 2.
dstow stow --bogus >/dev/null 2>&1 && code=0 || code=$?
if [ "$code" != "2" ]; then
  printf 'FAIL: unknown flag exit = %s, want 2\n' "$code"
  exit 1
fi

# An unknown command is a usage error → exit 2.
dstow no-such-command >/dev/null 2>&1 && code=0 || code=$?
if [ "$code" != "2" ]; then
  printf 'FAIL: unknown command exit = %s, want 2\n' "$code"
  exit 1
fi

# A bare deploy verb with no names and no --all, non-interactive, refuses → 3.
dstow stow </dev/null >/dev/null 2>&1 && code=0 || code=$?
if [ "$code" != "3" ]; then
  printf 'FAIL: non-interactive bulk stow exit = %s, want 3\n' "$code"
  exit 1
fi

# The refusal names its remedy on stderr (a fix: line, O2), stdout clean (O1).
err=$(dstow stow </dev/null 2>&1 1>/dev/null || true)
printf '%s' "$err" | grep -q 'fix:' \
  || { printf 'FAIL: bulk refusal did not print a fix: line\n'; exit 1; }
out=$(dstow stow </dev/null 2>/dev/null || true)
if [ -n "$out" ]; then
  printf 'FAIL: bulk refusal wrote to stdout\n'
  exit 1
fi

printf 'PASS: exit-code map (usage 2, refusal 3, success 0)\n'
