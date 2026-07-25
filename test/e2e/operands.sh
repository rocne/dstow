#!/bin/sh
set -e

# Regression for #183, on the real binary. Two claims the unit suite cannot make
# from inside the module: what a *user* sees, and that no surface anywhere leaks
# the internal "dstow/name:" package prefix.
#
# The §1.3 operand rule: a path operand (/ ~/ ./ ../) always refers to the
# target world. `repo remove` takes a repo NAME, so a path has no reading there
# and is refused as a malformed invocation (exit 2). `repo add` accepting a path
# is not a counter-example — add takes a *source*, a different grammar.
export HOME=/home/e2e-operands
export XDG_CONFIG_HOME=$HOME/.config
export XDG_STATE_HOME=$HOME/.local/state
export XDG_DATA_HOME=$HOME/.local/share
mkdir -p "$HOME"

# A registered local repo, added by path — the asymmetry this ticket is about.
mkdir -p "$HOME/dots/zsh"
printf 'export E2E=1\n' > "$HOME/dots/zsh/dot-zshrc"
dstow repo add "$HOME/dots" >/dev/null 2>&1 \
  || { printf 'FAIL: repo add by path exited nonzero\n'; exit 1; }

# --- The refusal: all four §1.3 prefixes behave alike -------------------------
# Before the fix they did not: a leading "/" produced a raw parser message while
# the other three parsed as ordinary names and fell through to "not found".
for OPERAND in "$HOME/dots" "./dots" "~/dots" "../dots"; do
  OUT="$(dstow repo remove "$OPERAND" 2>&1)" && CODE=0 || CODE=$?

  if [ "$CODE" != "2" ]; then
    printf 'FAIL: repo remove %s exit = %s, want 2 (malformed invocation)\n%s\n' \
      "$OPERAND" "$CODE" "$OUT"
    exit 1
  fi
  if ! printf '%s' "$OUT" | grep -q 'is a path'; then
    printf 'FAIL: repo remove %s did not name the operand-kind mismatch:\n%s\n' "$OPERAND" "$OUT"
    exit 1
  fi
  if ! printf '%s' "$OUT" | grep -q 'fix:'; then
    printf 'FAIL: repo remove %s printed no fix: line:\n%s\n' "$OPERAND" "$OUT"
    exit 1
  fi
  if printf '%s' "$OUT" | grep -q 'not found'; then
    printf 'FAIL: repo remove %s resolved a path as a name:\n%s\n' "$OPERAND" "$OUT"
    exit 1
  fi
done

# The absolute-path refusal names the runnable qualified spelling. The other
# three have none: "local:~/dots" parses, but its coordinate is a literal "~"
# segment rather than the home directory, so it must never be suggested.
ABS_OUT="$(dstow repo remove "$HOME/dots" 2>&1 || true)"
printf '%s' "$ABS_OUT" | grep -q "dstow repo remove local:$HOME/dots" \
  || { printf 'FAIL: absolute-path fix did not name the qualified command:\n%s\n' "$ABS_OUT"; exit 1; }

TILDE_OUT="$(dstow repo remove '~/dots' 2>&1 || true)"
if printf '%s' "$TILDE_OUT" | grep -q 'local:~'; then
  printf 'FAIL: fix suggested "local:~/…", a spelling that resolves to nothing:\n%s\n' "$TILDE_OUT"
  exit 1
fi

# --- The refusal did not overreach: names still work --------------------------
dstow repo remove "local:$HOME/dots" >/dev/null 2>&1 \
  || { printf 'FAIL: repo remove by canonical name stopped working\n'; exit 1; }
dstow repo add "$HOME/dots" >/dev/null 2>&1

# An absent name is still the not-found family (exit 1), never usage.
dstow repo remove no-such-repo >/dev/null 2>&1 && CODE=0 || CODE=$?
if [ "$CODE" != "1" ]; then
  printf 'FAIL: absent repo name exit = %s, want 1 (not-found family)\n' "$CODE"
  exit 1
fi

# --- The prefix is gone from every surface that renders a name error ----------
# A path-shaped name operand (the scheme dropped from an FQN dstow itself
# prints) reaches the parser on the view and deploy surfaces.
for CMD in "info /abs/path::pkg" "list /abs/path::pkg" "stow /abs/path::pkg" "repo remove bad%zz"; do
  # shellcheck disable=SC2086
  OUT="$(dstow $CMD 2>&1 || true)"
  if printf '%s' "$OUT" | grep -q 'dstow/name'; then
    printf 'FAIL: "dstow %s" leaked the internal package prefix:\n%s\n' "$CMD" "$OUT"
    exit 1
  fi
done

# And the error:/fix: pairing holds where the error is returned rather than
# reported as a per-package run line.
INFO_OUT="$(dstow info /abs/path::pkg 2>&1 || true)"
printf '%s' "$INFO_OUT" | grep -q 'error:' \
  || { printf 'FAIL: info printed no error: line:\n%s\n' "$INFO_OUT"; exit 1; }
printf '%s' "$INFO_OUT" | grep -q 'fix:.*local:/abs/path::pkg' \
  || { printf 'FAIL: info fix: did not name the qualified spelling:\n%s\n' "$INFO_OUT"; exit 1; }

printf 'PASS: §1.3 operand rule at repo remove; no internal prefix on any name error\n'
