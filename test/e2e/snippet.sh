#!/bin/sh
set -e

# snippet rc emits the vendored snippet.sh verbatim on stdout (§9.1 B1/B2):
# valid POSIX sh, the contractual PATH line, the presence guard, nothing on
# stderr. Byte-identity with the repo's snippet.sh is the unit suite's job
# (the file isn't shipped into this container); this exerciser asserts the
# emitted text is real, runnable shell holding the §9.1 contract.
export HOME=/home/e2e
export XDG_CONFIG_HOME=/home/e2e/.config
export XDG_STATE_HOME=/home/e2e/.local/state
export XDG_DATA_HOME=/home/e2e/.local/share
mkdir -p /home/e2e

out=$(dstow snippet rc) || { printf 'FAIL: dstow snippet rc exited nonzero\n'; exit 1; }
if [ -z "$out" ]; then
  printf 'FAIL: dstow snippet rc printed nothing\n'
  exit 1
fi

# The snippet is data → stdout, nothing on stderr.
if [ -n "$(dstow snippet rc 2>&1 1>/dev/null)" ]; then
  printf 'FAIL: dstow snippet rc wrote to stderr\n'
  exit 1
fi

# The emitted text must be syntactically valid sh — it lands in rc files.
if ! printf '%s\n' "$out" | sh -n; then
  printf 'FAIL: emitted snippet is not valid sh\n'
  exit 1
fi

# §9.1 contract: the PATH line bakes the contractual default install dir.
case "$out" in
  *'PATH="$HOME/.local/bin:$PATH"'*) ;;
  *) printf 'FAIL: snippet does not bake ~/.local/bin onto PATH\n'; exit 1 ;;
esac

# §9.1 contract: an install guard exists — the snippet never fetches blindly.
case "$out" in
  *'! command -v'*) ;;
  *) printf 'FAIL: snippet has no presence guard\n'; exit 1 ;;
esac

# Sourcing the snippet on a box where dstow is present must be a silent no-op
# (present ⇒ silent, invisible, offline). dstow is on PATH in this container.
noise=$(printf '%s\n' "$out" | sh 2>&1) || {
  printf 'FAIL: sourcing the snippet with dstow present exited nonzero\n'; exit 1
}
if [ -n "$noise" ]; then
  printf 'FAIL: snippet was not silent with dstow present: %s\n' "$noise"
  exit 1
fi

printf 'PASS: snippet rc emits the canonical rc bootstrap\n'
