#!/bin/sh
set -e

# Fresh HOME and XDG_DATA_HOME per exerciser, same convention as dot-dagger's
# exercisers.
export HOME=/home/e2e
export XDG_DATA_HOME=/home/e2e/.local/share
mkdir -p /home/e2e

# Charter: this exerciser asserts only INTENDED behavior (dev/DESIGN.md),
# never whatever the stub happens to do. Right now internal/cli.Run is a
# stub that prints nothing and returns 0; DESIGN.md §2.1 ("bare group prints
# its help") and §2.3 (the canonical help text) say bare `dstow` prints its
# top-level help on stdout and exits 0 — help is the requested data, not an
# error. Exit 0 is true under both the stub and the intended behavior, so
# it's the only assertion this seed test can honestly make today. Output
# assertions (the actual help text on stdout) arrive with the cli ticket
# (#47); asserting the stub's current silence would be a contrived-green
# test tied to scaffold, not design, and is forbidden.
dstow \
  || { printf 'FAIL: bare dstow exited nonzero\n'; exit 1; }

printf 'PASS: smoke test\n'
