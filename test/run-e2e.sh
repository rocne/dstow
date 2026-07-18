#!/bin/sh
# run-e2e.sh — canonical pre-merge e2e gate. Builds dstow from source HEAD.
#
# This script compiles dstow for linux/amd64 from the current working tree
# and runs the full exerciser suite inside Docker. It is the default e2e
# gate and should be run before merging to main.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

trap 'rm -f "${SCRIPT_DIR}/e2e/dstow"' EXIT

printf 'e2e: building dstow from source (linux/amd64)\n'
GOOS=linux GOARCH=amd64 go build -o "${SCRIPT_DIR}/e2e/dstow" "${REPO_ROOT}/cmd/dstow"

printf 'e2e: building docker image\n'
docker build -t dstow-e2e -f "${SCRIPT_DIR}/e2e/Dockerfile.local" "${SCRIPT_DIR}/e2e"

run_test() {
  EXERCISER="$1"
  printf '\n=== %s ===\n' "${EXERCISER}"
  docker run --rm \
    dstow-e2e \
    sh -c ". /procure/local.sh && sh /tests/${EXERCISER}"
}

run_test smoke.sh
run_test help.sh
run_test version.sh
run_test deploy.sh
run_test exitcodes.sh

printf '\nAll e2e tests passed.\n'
