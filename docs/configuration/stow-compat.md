# Migrating from GNU Stow

dstow reads your existing GNU Stow configuration. You do not have to rewrite
anything to start, and a migrated stow repo works out of the box: package
identity is locational, there are no marker files to add, and `mkdir`-and-go
stays intact.

## What is honored

Two stow files are read as **compatibility inputs**, parsed quirk-faithfully:

| File | Slotted as |
|---|---|
| `~/.stowrc` | global level |
| `<repo>/.stowrc` | repo level |
| `.stow-local-ignore` | that package's ignores |

Repo-level files are discovered **through the repo**, never through the
working directory — the same rule as everything else in dstow.

Parsing is delegated to a conformance-tested implementation of stow's own rc
grammar, so quirks are reproduced rather than approximated. dstow keeps the
parts that are its own: discovery, slotting, knob mapping, and supplement
diffing.

## Option mapping

| Stow option | dstow equivalent |
|---|---|
| `--target` | `target` |
| `--no-folding` | `fold_trees = false` |
| `--dotfiles` | `translate_dot_prefixes = true` |
| `--ignore` | an additive `ignore` chain entry, in stow-regex — its native language |
| `--dir` in `~/.stowrc` | a **session repo** for this shell, announced, with a `fix:` suggesting a permanent `dstow repo add` |
| `--dir` at repo level | warned and ignored |

**Flag absence maps to nothing.** A missing `--no-folding` does not assert
`fold_trees = true`; dstow's own default applies.

Options dstow does not map — `--adopt`, `--override`, `--defer`, the verbosity
flags, simulate, and the verbs themselves — are **warned and ignored, one
warning per option**, naming why and the native remedy.

**The file always runs.** Degradation is loud, never a rejection: dstow will
tell you exactly what it did not honor, and then do everything it can.

## The renamed-rc case

A native `config.toml` whose content is flag-lines — the first significant
token starts with `-`, which is impossible in top-level TOML — is routed to
the compat parser with a loud announcement naming the native equivalent.

This catches the specific mistake of renaming a `.stowrc` to `config.toml` and
expecting it to work. dstow reads it correctly and tells you what it should
have been. The native format never bends to accommodate compat: the routing
happens at the door, not in the parser.

## Supplement mode

When both a stow rc and a native config set the same knob, dstow diffs them
per knob:

- **Non-overlapping knobs** — silent. Each carrier contributes what it sets.
- **Equal values** — silent. There is nothing to report.
- **Different values** — **native wins**, with a loud warning naming the
  level, both files, the knob, both values, and the winner, plus a `fix:`
  suggesting removal from the rc.

`ignore` chains never conflict: they are additive and per-language, so both
carriers' patterns apply.

The intent is that you can migrate incrementally. Move one knob at a time into
`config.toml`; dstow tells you each time a native value has taken over from an
rc value, so the rc can be trimmed with confidence and eventually deleted.

## Compat patterns that will not compile

Stow's ignore patterns are regexes, and dstow compiles them with RE2. A
pattern RE2 cannot accept is **refused, scoped to the level the pattern
governs** — a package-level failure fails that package and the run continues.
The refusal names the file, the pattern, the RE2 error, and both remedies
(fix the regex, or express it natively as a glob).

## What does not carry over

- **Stow's `--dir`/`--target` as per-invocation flags.** dstow has no `--dir`:
  repos are registered, not passed. `DSTOW_PATH` covers the
  "just for this shell" case; see `dstow manual reference environment`.
- **Stow's tree-folding default.** Stow folds by default; dstow does not.
  Set `fold_trees = true` globally if you want stow's behavior.
- **Verbs.** `stow`, `unstow`, and `restow` are dstow commands, not rc
  options.
