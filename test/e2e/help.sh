#!/bin/sh
set -e

# Fresh HOME/XDG per exerciser, same convention as smoke.sh.
export HOME=/home/e2e
export XDG_CONFIG_HOME=/home/e2e/.config
export XDG_STATE_HOME=/home/e2e/.local/state
export XDG_DATA_HOME=/home/e2e/.local/share
mkdir -p /home/e2e

# Charter: assert intent from DESIGN.md, not the code's incidentals.
#
# Help text is owned by docs/commands/**.md (§2.3/§2.4); DESIGN owns the shape
# and the mechanism, and cobra owns layout. So this exerciser asserts two
# things and no prose of its own:
#
#   * STRUCTURE — §2.3's sections, the command inventory, the global flags, the
#     `dstow manual` footer §2.1 makes load-bearing, and stdout-only delivery.
#   * THE PIPELINE — that a command's help really is its page. The container has
#     no repo source, but the docs ride inside the binary, so `dstow manual
#     commands stow` prints docs/commands/stow.md raw, tags included: extract
#     its dstow:short and dstow:long here and assert they are what cobra
#     renders. That verifies embed → extract → cobra end to end, which is
#     exactly what structural assertions miss and what a unit test asserting
#     the docs against the docs could never establish.
#
# Colorization rides the §7.3 enable chain: piped help is plain, --color=always
# styles it, and stripping the styling restores the plain bytes exactly (O11).

# region <tag> reads a manual page on stdin and prints one tagged region's
# content — the same extraction the binary does, from the outside.
region() {
  sed -n "/<!-- dstow:$1 -->/,/<!-- \/dstow:$1 -->/p" | sed -e '1d' -e '$d'
}

got=$(dstow --help) || { printf 'FAIL: dstow --help exited nonzero\n'; exit 1; }

# §2.3's sections, in the root listing.
for title in 'Deploy:' 'Inspect:' 'Maintain:' 'Groups:' 'Also:'; do
  case $got in
    *"$title"*) ;;
    *) printf 'FAIL: dstow --help is missing the %s section\n' "$title"; exit 1 ;;
  esac
done

# §2.1's inventory: every command is listed, each with a description beside it.
for name in stow unstow restow adopt list info status check clean rebuild \
            repo snippet theme version completion; do
  if ! printf '%s\n' "$got" | grep -q "^  $name  *[A-Za-z]"; then
    printf 'FAIL: dstow --help does not list the %s command\n' "$name"
    exit 1
  fi
done

# The hidden name group stays out of the listing (§1.5).
if printf '%s\n' "$got" | grep -q '^  name  *[A-Za-z]'; then
  printf 'FAIL: the hidden name group leaked into dstow --help\n'
  exit 1
fi

# The global flags, and the footer that is the sole discovery affordance for
# the manual (§2.1: load-bearing, not decoration).
for atom in '--color' '--quiet' '--yes' '--help' \
            "Run 'dstow manual' for the full documentation."; do
  case $got in
    *"$atom"*) ;;
    *) printf 'FAIL: dstow --help is missing: %s\n' "$atom"; exit 1 ;;
  esac
done

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

# The derivation pipeline, at a leaf (stow) and at depth (repo add — which also
# proves the path↔command mapping below the root). A command's short renders in
# its parent's listing; its long is the body of its own help.
check_derived() {
  page=$1     # the manual node, e.g. "stow" or "repo add"
  parent=$2   # where the short renders, e.g. "" or "repo"
  raw=$(dstow manual commands $page) || {
    printf 'FAIL: dstow manual commands %s exited nonzero\n' "$page"; exit 1; }

  short=$(printf '%s\n' "$raw" | region short)
  if [ -z "$short" ]; then
    printf 'FAIL: docs page for %s carries no dstow:short region\n' "$page"
    exit 1
  fi
  listing=$(dstow $parent --help)
  case $(printf '%s\n' "$listing" | tr -s ' ') in
    *"$(printf '%s\n' "$short" | tr -s ' ')"*) ;;
    *) printf "FAIL: dstow %s --help does not carry %s's dstow:short\n" "$parent" "$page"; exit 1 ;;
  esac

  long=$(printf '%s\n' "$raw" | region long)
  if [ -z "$long" ]; then
    printf 'FAIL: docs page for %s carries no dstow:long region\n' "$page"
    exit 1
  fi
  help=$(dstow $page --help)
  printf '%s\n' "$long" | while IFS= read -r line; do
    if [ -n "$line" ]; then
      case $help in
        *"$line"*) ;;
        *) printf "FAIL: dstow %s --help is missing a dstow:long line: %s\n" "$page" "$line"; exit 1 ;;
      esac
    fi
  done || exit 1
}

check_derived stow ''
check_derived 'repo add' repo

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

printf 'PASS: help is structurally §2.3, derived from docs/commands/, colorized on the enable chain\n'
