#!/bin/sh
set -e

# Fresh HOME/XDG per exerciser, same convention as smoke.sh.
export HOME=/home/e2e
export XDG_CONFIG_HOME=/home/e2e/.config
export XDG_STATE_HOME=/home/e2e/.local/state
export XDG_DATA_HOME=/home/e2e/.local/share
mkdir -p /home/e2e

# Charter: assert intent from DESIGN.md §2.1's manual carve-out, never the
# manual's prose — the tree's content is written elsewhere and this exerciser
# must not couple to a word of it. What binds: dstow is learnable from the
# command line alone, so the tree is reachable from root help; a node prints
# its markdown raw rather than its help; and the entry point is hidden from
# the top-level surface while remaining completable once named.

out=$(dstow manual) || { printf 'FAIL: dstow manual exited nonzero\n'; exit 1; }
if [ -z "$out" ]; then
  printf 'FAIL: dstow manual printed nothing\n'
  exit 1
fi

# The manual is data → stdout, nothing on stderr (A2/A4).
if [ -n "$(dstow manual 2>&1 1>/dev/null)" ]; then
  printf 'FAIL: dstow manual wrote to stderr\n'
  exit 1
fi

# A bare node prints its index.md, not its help — that is the §2.1 carve-out,
# and it is what makes the tree navigable without --help.
case $out in
  *'Usage:'*) printf 'FAIL: dstow manual printed help; a bare node prints its markdown\n'; exit 1 ;;
esac
case $out in
  '#'*) ;;
  *) printf 'FAIL: dstow manual did not print markdown (no leading heading)\n'; exit 1 ;;
esac

# Raw and verbatim: no styling pass, even when color is forced on. "The
# command prints the file" is the property worth keeping.
esc=$(printf '\033')
forced=$(dstow --color=always manual)
case $forced in
  *"$esc"*) printf 'FAIL: dstow manual styled its output; nodes print the file raw\n'; exit 1 ;;
esac
if [ "$forced" != "$out" ]; then
  printf 'FAIL: dstow manual output changed under --color=always\n'
  exit 1
fi

# Root help is the sole discovery affordance: one footer line names the tree.
help=$(dstow --help)
case $(printf '%s\n' "$help" | tr -s ' ') in
  *"Run 'dstow manual' for the full documentation."*) ;;
  *) printf 'FAIL: root help carries no pointer at the manual\n'; exit 1 ;;
esac

# ...while the group itself stays out of the command listing (hidden).
if printf '%s\n' "$help" | grep -qE '^[[:space:]]+manual[[:space:]]'; then
  printf 'FAIL: root help lists manual as a command; the group is hidden\n'
  exit 1
fi

# Completion mirrors that: `dstow <TAB>` omits the hidden group...
if dstow __complete '' 2>/dev/null | grep -q '^manual'; then
  printf 'FAIL: dstow <TAB> offers the manual; the group is hidden\n'
  exit 1
fi

# ...and naming it still completes, so the tree is walkable by TAB once found.
if ! dstow __complete manual '' >/dev/null 2>&1; then
  printf 'FAIL: dstow manual <TAB> does not complete\n'
  exit 1
fi

# Nodes take no operands (§2.1): a stray argument is a usage error, exit 2.
dstow manual stray >/dev/null 2>&1 && code=0 || code=$?
if [ "$code" != "2" ]; then
  printf 'FAIL: dstow manual stray exit = %s, want 2 (usage error)\n' "$code"
  exit 1
fi

# The five topics an agent cannot learn from the binary any other way (#131).
# Asserted as reachable command paths, never by their prose: the requirement is
# that dstow be fully learnable from the command line alone, and reachability is
# exactly what that requires of the tree. What each page SAYS is the authors',
# and this exerciser stays out of it.
for topic in \
  'reference exit-codes' \
  'configuration keys' \
  'reference environment' \
  'hooks context' \
  'concepts states'
do
  # shellcheck disable=SC2086
  page=$(dstow manual $topic) || {
    printf 'FAIL: dstow manual %s exited nonzero; the topic is unreachable\n' "$topic"
    exit 1
  }
  case $page in
    '#'*) ;;
    *) printf 'FAIL: dstow manual %s did not print its markdown page\n' "$topic"; exit 1 ;;
  esac
done

# Each of those is reachable by walking from the root, not only by knowing the
# path: a parent completes the child that follows it. This is the navigational
# claim docs/index.md makes ("run dstow manual <topic>, then follow what it
# lists"), checked at the surface an agent actually uses.
for parent in reference configuration hooks concepts commands; do
  if ! dstow __complete manual "$parent" 2>/dev/null | grep -q "^$parent"; then
    printf 'FAIL: dstow manual <TAB> does not offer the topic %s\n' "$parent"
    exit 1
  fi
  if ! dstow __complete manual "$parent" '' >/dev/null 2>&1; then
    printf 'FAIL: dstow manual %s <TAB> does not complete its children\n' "$parent"
    exit 1
  fi
done

printf 'PASS: the manual tree is reachable, raw, and hidden from the top level\n'
