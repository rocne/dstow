#!/bin/sh
set -e

# version prints the build version to stdout and exits 0 (§2.1). The e2e binary
# is built from source with no ldflag, so the exact string is build-dependent;
# the promise this asserts is "prints something, on stdout, exit 0".
export HOME=/home/e2e
export XDG_CONFIG_HOME=/home/e2e/.config
export XDG_STATE_HOME=/home/e2e/.local/state
export XDG_DATA_HOME=/home/e2e/.local/share
mkdir -p /home/e2e

out=$(dstow version) || { printf 'FAIL: dstow version exited nonzero\n'; exit 1; }
if [ -z "$out" ]; then
  printf 'FAIL: dstow version printed nothing\n'
  exit 1
fi

# version is data → stdout, nothing on stderr.
if [ -n "$(dstow version 2>&1 1>/dev/null)" ]; then
  printf 'FAIL: dstow version wrote to stderr\n'
  exit 1
fi

printf 'PASS: version prints %s on stdout\n' "$out"
