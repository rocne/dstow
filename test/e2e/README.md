# dstow e2e tests

Container-based integration tests. Each exerciser is a POSIX `sh` script
that runs the real `dstow` binary (built from HEAD) inside a fresh
Ubuntu container and prints `PASS:`/`FAIL:` lines.

## The charter

Assert intent, never the code's incidental behavior: every assertion must
trace to something [dev/DESIGN.md](../../dev/DESIGN.md) actually promises,
never to whatever the current implementation (or a stub) happens to do.

## Adding an exerciser

1. Write `test/e2e/<behavior>.sh` asserting DESIGN.md-intended behavior for
   that command or flow. Follow the conventions in `smoke.sh`: `#!/bin/sh`,
   `set -e`, a fresh `HOME`/`XDG_*` per script, `PASS:`/`FAIL:` lines.
2. Add one `run_test <behavior>.sh` line in [`run-e2e.sh`](../run-e2e.sh).

Nothing else — the Dockerfile picks up new `test/e2e/*.sh` files
automatically (see the comment in `Dockerfile.local`), and the procurement
step (`procure/local.sh`) already stages the binary onto `PATH` before any
exerciser runs.

## Running locally

```sh
./test/run-e2e.sh
```

Requires Docker.
