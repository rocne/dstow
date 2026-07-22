# Hooks: running your own executables around deploy actions

Hooks let you run your own programs before and after dstow deploys. They are
git-style: one executable per event, discovered by file name, `exec`'d
directly.

## Topics

- `context` — the `DSTOW_HOOK_*` environment variables dstow passes, and what
  each one carries at each level.

## Where hooks live

A `hooks/` directory inside the metadata directory, at any of the three
levels:

    <repo>/<pkg>/.dstow/hooks/      package-level
    <repo>/.dstow/hooks/            repo-level
    $XDG_CONFIG_HOME/dstow/hooks/   global

## The eight events

One file per event, named exactly:

    pre-stow      post-stow
    pre-unstow    post-unstow
    pre-restow    post-restow
    pre-adopt     post-adopt

**`restow` fires only the restow pair** — not stow's and not unstow's, even
though a restow does both internally. The event names an action you asked for,
not the steps it decomposes into.

## Execution

- Each hook is **`exec`'d directly**; the shebang chooses the interpreter.
  There is no shell wrapper, so a hook is whatever kind of executable you make
  it.
- It must be **executable**. A correctly-named file without the bit set is
  warned, with the `chmod +x` fix named. dstow never silently skips a file
  that was clearly meant to be a hook.
- A **misspelled** name — `pre_stow`, `prestow` — is warned with a
  did-you-mean, for the same reason.
- The **working directory** is the scope's own directory: the package
  directory, the repo directory, or the global config directory. Hooks never
  inherit whatever directory you happened to be standing in.

## Firing order

Hooks nest, and each fires **at most once per invocation**. The order wraps
the action, outermost first on the way in and last on the way out:

    global-pre → repo-pre → package-pre → THE ACTION →
                            package-post → repo-post → global-post

Once fired is fired, whether it succeeded or failed: a later package in the
same run does not re-fire the repo's or the global `pre-` hook.

A hook that fails is not silently swallowed. **A failing `pre-` hook blocks
everything beneath it** — a failed repo-level `pre-stow` blocks every package
in that repo, and a failed global one blocks the run. That is what makes a
`pre-` hook usable as a precondition check rather than a notification.

## The `hooks/` directory is also helper space

- **Subdirectories are inert.** They are never fired and never warned about —
  helper territory, with `lib/` the documented convention for shared scripts.
- **`<event>.d/` directories are reserved.** All eight `.d` spellings are
  inert-and-warned in v1 ("reserved, not yet meaningful"). Drop-in execution
  in the run-parts manner is committed for v2, and reserving the spelling now
  is what makes that non-breaking.

## Two rules about what a hook may do

**Both hook output streams go to dstow's stderr; stdin passes through.**
Nothing a hook prints is dstow's answer to the question you asked — hook
output is commentary, definitionally. A hook that must emit data writes a
file, or is invoked directly rather than through dstow.

**Write commands refuse from inside a hook; reads are fully allowed.** A hook
may run `dstow status --json` or `dstow info -f target` to read state, but the
deploy verbs, `adopt`, `clean`, `rebuild`, and the repo mutations refuse, with
an error naming the cause and the remedy. Detection is the presence of
`DSTOW_HOOK_ACTION` in the environment.

The asymmetry is deliberate. An install hook legitimately wants to *read*
dstow while installing tools with its own commands. Recursive *writes* are how
you get a deploy that reenters itself mid-transaction. There is no bypass in
v1: allowing later is additive, and forbidding later would break whoever
relied on it.

## A worked example

`<repo>/zsh/.dstow/hooks/post-stow`:

```sh
#!/bin/sh
set -e

# cwd is the package directory; the target is in the environment.
[ "$DSTOW_HOOK_ACTION" = stow ] || exit 0

# Compile the zsh completion cache into the deployed location.
zsh -c "compinit -d '$DSTOW_HOOK_TARGET/.zcompdump'"
```

`<repo>/.dstow/hooks/post-stow`, at the repo level, sees the whole acting set:

```sh
#!/bin/sh
set -e

while IFS= read -r pkg; do
  printf 'deployed %s\n' "$pkg"
done <<EOF
$DSTOW_HOOK_PACKAGES
EOF
```

See `dstow manual hooks context` for every variable and where it applies.
