# The hook context contract

dstow passes context to hooks as environment variables, all prefixed
`DSTOW_HOOK_`. **No arguments, no stdin feeding** — the mechanism is the
environment and nothing else, which is what makes it additive: new context is
a new variable, and a hook that does not read it is unaffected.

The prefix partitions the environment into lanes. `DSTOW_PATH` and
`DSTOW_COLORS` are user-to-dstow inputs; `DSTOW_HOOK_*` is dstow's output lane
to your hooks.

## Absent, never empty

**A variable that does not apply is absent from the environment**, not set to
the empty string. So a hook can test presence:

```sh
if [ -n "${DSTOW_HOOK_PACKAGE+set}" ]; then
  : # package level
fi
```

and never has to distinguish "unset" from "set to nothing". dstow strips the
whole `DSTOW_HOOK_*` namespace before layering on the level's set, so a stale
variable from an outer invocation can never leak into an inner one.

## The twelve variables

| Variable | Value | package | repo | global |
|---|---|:---:|:---:|:---:|
| `DSTOW_HOOK_LEVEL` | `package` / `repo` / `global` | ✓ | ✓ | ✓ |
| `DSTOW_HOOK_ACTION` | `stow` / `unstow` / `restow` / `adopt` | ✓ | ✓ | ✓ |
| `DSTOW_HOOK_PHASE` | `pre` / `post` | ✓ | ✓ | ✓ |
| `DSTOW_HOOK_FQN` | this scope's own FQN, canonical-encoded | ✓ | ✓ | — |
| `DSTOW_HOOK_SCHEME` | the scheme, decoded — e.g. `github` | ✓ | ✓ | — |
| `DSTOW_HOOK_COORDINATE` | the coordinate, decoded — e.g. `rocne/dotfiles`; for a `local:` repo, the actual absolute path | ✓ | ✓ | — |
| `DSTOW_HOOK_PACKAGE` | the bare package name, decoded — not the FQN | ✓ | — | — |
| `DSTOW_HOOK_PACKAGE_DIR` | absolute path to the package directory | ✓ | — | — |
| `DSTOW_HOOK_TARGET` | the effective target root, absolute | ✓ | — | — |
| `DSTOW_HOOK_REPO_FQN` | the repo's FQN, canonical-encoded | ✓ | ✓ | — |
| `DSTOW_HOOK_REPO_DIR` | absolute path to the repo directory | ✓ | ✓ | — |
| `DSTOW_HOOK_PACKAGES` | the FQNs acting under this scope, newline-separated | — | ✓ | ✓ |

At the **repo** level, `DSTOW_HOOK_FQN` and `DSTOW_HOOK_REPO_FQN` are the same
value — the scope *is* the repo.

("Coordinate" is used in the Maven sense: the parts that locate a repo.)

## Encoding

The rule is that **encoding protects the grammar, and a standalone segment has
no grammar to protect**:

- **`*FQN` values carry the canonical percent-encoded spelling.** They paste
  straight back into a dstow command:

      dstow status "$DSTOW_HOOK_FQN"

- **Decomposed segments carry decoded, real values.** `DSTOW_HOOK_PACKAGE` is
  the package's actual name, `DSTOW_HOOK_COORDINATE` the actual coordinate.
  These are for reading and for building paths, not for feeding back to dstow.

See `dstow manual concepts naming` for the encoding itself.

## Iterating `DSTOW_HOOK_PACKAGES`

One canonical FQN per line. Canonical encoding of control characters is what
makes one-per-line airtight — a package name cannot contain a raw newline by
the time it reaches the variable.

The documented idiom:

```sh
while IFS= read -r pkg; do
  dstow info "$pkg" -f target
done <<EOF
$DSTOW_HOOK_PACKAGES
EOF
```

It is present at the repo and global levels — the scopes that act on a *set*
of packages. At the package level the scope is one package, and
`DSTOW_HOOK_PACKAGE` names it.

## Working directory

The hook's working directory is **the scope's own directory**:

| Level | cwd |
|---|---|
| package | the package directory (`$DSTOW_HOOK_PACKAGE_DIR`) |
| repo | the repo directory (`$DSTOW_HOOK_REPO_DIR`) |
| global | `$XDG_CONFIG_HOME/dstow/` |

Hooks never inherit the user's incidental working directory, so a relative
path inside a hook means the same thing every time it runs.

## Streams

**Both of a hook's output streams land on dstow's stderr; its stdin passes
through** from dstow's own.

Hook output is commentary by definition — it is never dstow's answer to the
question the user asked, which is why it cannot reach stdout. A hook that
needs to produce data writes a file, or gets invoked directly instead of
through dstow.

Stdin passing through is what lets a hook prompt, when dstow is running
interactively.

## What a hook may run

`DSTOW_HOOK_ACTION` being present in the environment is how dstow detects that
it is running inside a hook.

- **Reads are fully allowed**: `list`, `info`, `status`, `check`.
- **Writes refuse**: the deploy verbs, `adopt`, `clean`, `rebuild`, and the
  repo mutations, each with an error naming cause and remedy.

## A complete example

`<repo>/.dstow/hooks/pre-stow`, refusing the deploy when a required tool is
missing:

```sh
#!/bin/sh
set -e

command -v zsh >/dev/null 2>&1 || {
  printf 'zsh is not installed; install it before stowing %s\n' \
    "$DSTOW_HOOK_REPO_FQN" >&2
  exit 1
}

# Which packages are about to be stowed?
while IFS= read -r pkg; do
  printf 'will stow: %s\n' "$pkg" >&2
done <<EOF
$DSTOW_HOOK_PACKAGES
EOF
```

A non-zero exit from a `pre-` hook blocks the action it guards, which is what
makes this a precondition check rather than a notification.
