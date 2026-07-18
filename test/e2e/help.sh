#!/bin/sh
set -e

# Fresh HOME/XDG per exerciser, same convention as smoke.sh.
export HOME=/home/e2e
export XDG_CONFIG_HOME=/home/e2e/.config
export XDG_STATE_HOME=/home/e2e/.local/state
export XDG_DATA_HOME=/home/e2e/.local/share
mkdir -p /home/e2e

# Charter: assert intent from DESIGN.md, not the code's incidentals. This is the
# approved canonical top-level help (docs/DESIGN.md §2.3), verbatim. Both
# `dstow --help` and bare `dstow` print exactly this on stdout and exit 0 (A2:
# help is the requested data; §2.1: a bare group prints its help).
expected=$(cat <<'EOF'
dstow — deploy dotfiles and configuration as symlinks, from packages in repos

Usage:
  dstow <command> [args] [flags]

Deploy:
  stow        Link packages into their targets
  unstow      Remove packages' links from their targets
  restow      Unstow, then stow again (refresh links)
  adopt       Import an existing file into a package, leaving a link behind

Inspect:
  list        What is configured: repos, packages, targets (never reads disk)
  info        Everything dstow knows about one repo or package
  status      What is deployed: live state of packages against their targets

Maintain:
  check       Verify every link in the ledger; classify broken and orphaned
  clean       Execute exactly what check reported (broken freely, orphans ask)
  rebuild     Reconstruct a lost ledger by walking configured targets (rare)

Groups:
  repo        Manage repos: add, remove, update, upgrade
  snippet     Print canned shell snippets: rc bootstrap
  colors      Theming utilities: emit a theme for your session or a file

Also:
  completion  Generate shell completion (bash, zsh, fish, powershell)
  version     Print version

Global flags:
      --color <when>   Colorize output: auto (default), always, never
  -q, --quiet          Suppress informational output (announcements survive)
  -y, --yes            Assume "yes" at confirmation prompts
  -h, --help           Help for dstow or any command

Name packages and repos by any unambiguous suffix of their qualified name
(github:rocne/dotfiles::zsh). The working directory never changes what a
command does. See 'dstow <command> --help' for details and examples.
EOF
)

got=$(dstow --help) || { printf 'FAIL: dstow --help exited nonzero\n'; exit 1; }
if [ "$got" != "$expected" ]; then
  printf 'FAIL: dstow --help does not match DESIGN.md §2.3 verbatim\n'
  printf '=== got ===\n%s\n=== want ===\n%s\n' "$got" "$expected"
  exit 1
fi

bare=$(dstow) || { printf 'FAIL: bare dstow exited nonzero\n'; exit 1; }
if [ "$bare" != "$expected" ]; then
  printf 'FAIL: bare dstow does not print the canonical help\n'
  exit 1
fi

# Help goes to stdout (A2), never stderr.
if [ -n "$(dstow --help 2>&1 1>/dev/null)" ]; then
  printf 'FAIL: dstow --help wrote to stderr\n'
  exit 1
fi

printf 'PASS: top-level help is verbatim on stdout\n'
