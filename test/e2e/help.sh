#!/bin/sh
set -e

# Fresh HOME/XDG per exerciser, same convention as smoke.sh.
export HOME=/home/e2e
export XDG_CONFIG_HOME=/home/e2e/.config
export XDG_STATE_HOME=/home/e2e/.local/state
export XDG_DATA_HOME=/home/e2e/.local/share
mkdir -p /home/e2e

# Charter: assert intent from DESIGN.md, not the code's incidentals. Help is
# cobra-generated from the commands' own definitions and carries the §2.3
# canonical content (A2 as amended — issue #96); layout is cobra's and is not
# asserted. Colorization rides the §7.3 enable chain: piped help is plain,
# --color=always styles it, and stripping the styling restores the plain
# bytes exactly (O11).

got=$(dstow --help) || { printf 'FAIL: dstow --help exited nonzero\n'; exit 1; }

# The §2.3 content: title line, every section title, every command with its
# one-line description, the global flags, and the closing prose. Both sides
# are whitespace-squeezed so column padding (cobra's layout) is never
# asserted, only content.
norm_got=$(printf '%s\n' "$got" | tr -s ' ')
while IFS= read -r atom; do
  case $norm_got in
    *"$atom"*) ;;
    *) printf 'FAIL: dstow --help is missing §2.3 content: %s\n' "$atom"; exit 1 ;;
  esac
done <<'EOF'
dstow — deploy dotfiles and configuration as symlinks, from packages in repos
Deploy:
stow Link packages into their targets
unstow Remove packages' links from their targets
restow Unstow, then stow again (refresh links)
adopt Import an existing file into a package, leaving a link behind
Inspect:
list What is configured: repos, packages, targets (never reads disk)
info Everything dstow knows about one repo or package
status What is deployed: live state of packages against their targets
Maintain:
check Verify every link in the ledger; classify broken and orphaned
clean Execute exactly what check reported (broken freely, orphans ask)
rebuild Reconstruct a lost ledger by walking configured targets (rare)
Groups:
repo Manage repos: add, remove, update, upgrade
snippet Print canned shell snippets: rc bootstrap
colors Theming utilities: emit a theme for your session or a file
Also:
completion Generate shell completion (bash, zsh, fish, powershell)
version Print version
--color
--quiet
--yes
--help
Name packages and repos by any unambiguous suffix of their qualified name
(github:rocne/dotfiles::zsh). The working directory never changes what a
command does. See 'dstow <command> --help' for details and examples.
EOF

# Bare dstow prints the same help on stdout, exit 0 (§2.1).
bare=$(dstow) || { printf 'FAIL: bare dstow exited nonzero\n'; exit 1; }
if [ "$bare" != "$got" ]; then
  printf 'FAIL: bare dstow does not print the top-level help\n'
  exit 1
fi

# Help goes to stdout (A2), never stderr.
if [ -n "$(dstow --help 2>&1 1>/dev/null)" ]; then
  printf 'FAIL: dstow --help wrote to stderr\n'
  exit 1
fi

# Per-command help carries its §2.4 content (spot check: stow).
stow_help=$(dstow stow --help)
case $stow_help in
  *'naming a repo stows all of its packages'*) ;;
  *) printf 'FAIL: dstow stow --help is missing its §2.4 prose\n'; exit 1 ;;
esac
case $stow_help in
  *'--dry-run'*) ;;
  *) printf 'FAIL: dstow stow --help is missing its flags\n'; exit 1 ;;
esac

# Piped help (no TTY) is plain: no ANSI escapes (§7.3 enable chain).
esc=$(printf '\033')
case $got in
  *"$esc"*) printf 'FAIL: piped dstow --help contains ANSI escapes\n'; exit 1 ;;
esac

# --color=always styles the help even when piped...
colored=$(dstow --color=always --help)
case $colored in
  *"${esc}["*) ;;
  *) printf 'FAIL: dstow --color=always --help carries no ANSI styling\n'; exit 1 ;;
esac

# ...and stripping the styling restores the plain bytes exactly (O11).
stripped=$(printf '%s\n' "$colored" | sed "s/${esc}\[[0-9;]*m//g")
if [ "$stripped" != "$got" ]; then
  printf 'FAIL: strip(colored help) differs from plain help (O11)\n'
  exit 1
fi

# NO_COLOR disables styling; --color=always overrides it (§7.3 precedence).
no_color=$(NO_COLOR=1 dstow --help)
case $no_color in
  *"$esc"*) printf 'FAIL: NO_COLOR=1 dstow --help contains ANSI escapes\n'; exit 1 ;;
esac
forced=$(NO_COLOR=1 dstow --color=always --help)
case $forced in
  *"${esc}["*) ;;
  *) printf 'FAIL: --color=always must beat NO_COLOR (§7.3)\n'; exit 1 ;;
esac

printf 'PASS: help is cobra-generated with canonical content, colorized on the enable chain\n'
