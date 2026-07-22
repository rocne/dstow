# Reference

The lookup tables: what dstow's exit codes mean, which environment variables
it reads, and where its files live.

## Topics

- `exit-codes` — the four exit codes and exactly what each one claims.
- `environment` — every environment variable dstow reads or sets.
- `files` — every path dstow reads or writes, and who owns it.

## Machine output

`--json` is available on `list`, `info`, `status`, and `check` — the four read
surfaces. It is the supported scripting interface for anything more structured
than an exit code.

- JSON keys are `snake_case`.
- JSON **always carries fully-qualified names**, never the shortest-unique
  display form.
- JSON goes to stdout. Diagnostics go to stderr, always, so a pipeline never
  has to parse around them.

### Per-command shapes

Each surface emits one shape, so a script never has to run the command once to
discover its fields. State and class strings are spelled exactly as `dstow
manual concepts states` gives them — the space in `partially stowed` included.

**`list`** — one of three shapes, chosen by the operand:

```json
{ "repos": [
    { "fqn": "...", "source": "...", "scheme": "...", "root": "...",
      "excluded_from_bulk": false, "managed": true, "session": false } ] }
```

is the bare / `--repos` listing. Naming a repo (or `--packages`) lists packages,
with `scope` naming the repo when one was given:

```json
{ "scope": "...", "packages": [ { "fqn": "...", "repo": "..." } ] }
```

Naming a package lists its paths — the raw walk, relative to the package
directory:

```json
{ "package": "...", "paths": [ "rel/path" ] }
```

**`info`** — a flat object keyed by the snake_case field name (the *Fields*
table below), each value the field's native type: a string, a bool, a list, or
`null` for a known-but-unset field:

```json
{ "target": "/home/you", "translate": true, "ignores": [] }
```

Under `--recurse` it is an array of these objects, each carrying an extra
`qualified_name` that attributes its scope:

```json
[ { "qualified_name": "...", "target": "/home/you" } ]
```

**`status`** — names give package states, plus sync counts for remote repos:

```json
{ "packages": [
    { "fqn": "...", "state": "partially stowed", "drifted": false,
      "links": [ { "link": "...", "source": "...", "state": "stowed",
                   "detail": "..." } ] } ],
  "repos": [
    { "fqn": "...", "ahead": 0, "behind": 0, "known": true,
      "error": "..." } ] }
```

A single path instead gives the location view:

```json
{ "path": {
    "path": "...", "exists": true, "is_symlink": true, "link_dest": "...",
    "kind": "...", "owner": "...", "owner_known": true,
    "candidates": [ { "fqn": "...", "source": "...", "neighbors": 0 } ] } }
```

**`check`** — the findings, one object per broken or orphaned link:

```json
{ "findings": [
    { "class": "orphaned", "target_root": "...", "link": "...",
      "package": "...", "source": "...", "destination": "...",
      "evidence": "..." } ] }
```

Seven keys are **conditional** — present only when they carry a value:
`scope`, `detail`, `error`, `link_dest`, `owner`, and a finding's `source` and
`destination`.

## Fields

`dstow info` reports **fields**, and the field vocabulary is its own — shorter
than the config-key vocabulary, and not a synonym for it. Fields are
`kebab-case` in human output and `snake_case` in `--json`.

| Field | Scope | Kind | What it reports |
|---|---|---|---|
| `repo` | package | inherent | the repo's FQN |
| `source` | package, repo | inherent | the qualified source the repo was registered from |
| `scheme` | package, repo | inherent | the scheme — `local`, `github`, … |
| `managed-path` | package, repo | inherent | absolute path to the package or repo directory |
| `qualified-name` | package, repo | inherent | the scope's own FQN |
| `version` | global | inherent | the running dstow's version |
| `managed-repos-dir` | global | inherent | where managed clones live |
| `global-config-dir` | global | inherent | the global config directory |
| `metadata-dir` | global | inherent | the global metadata directory |
| `ledger-path` | global | inherent | the ledger file |
| `target` | all | configured | the effective target root |
| `translate` | all | configured | the effective `translate_dot_prefixes` |
| `fold` | all | configured | the effective `fold_trees` |
| `ignores` | all | configured | the composed ignore chain |
| `exclude-from-bulk` | package, repo | configured | the effective `exclude_from_bulk` |

**Inherent** fields are facts of the thing as it exists — what it *is*, or how
it came to be. **Configured** fields are effective values from the config
chain — what you *want* for it. Three configured fields are deliberately named
differently from their config keys (`translate`, `fold`, `ignores`), because
what `info` reports is the composed effective value rather than one file's
declaration.

    dstow info zsh -f target -f ignores     repeatable
    dstow info dotfiles -r                  a repo and everything under it

An **unknown** field name is a usage error (exit `2`). A **known** field that
is applicable but has no value is a negative answer (exit `1`).

## Global flags

Available on every command:

| Flag | Meaning |
|---|---|
| `--color <when>` | `auto` (default), `always`, or `never`. The value is required. |
| `-q`, `--quiet` | Suppress routine chatter. Announcements, warnings, and errors always survive. |
| `-y`, `--yes` | Assume "yes" at confirmations of stated intent. Never resolves an ambiguity, answers the bulk prompt, or bypasses a guard. |
| `-h`, `--help` | Help for dstow or any command. |
| `-v`, `--version` | Print the version (root only). |

Per-command: `-n`/`--dry-run` on `stow`, `unstow`, `restow`, and `adopt`;
`--json` on `list`, `info`, `status`, and `check`.

## What `--yes` does and does not do

Worth stating precisely, because it is the flag most likely to be reached for
in a script:

- **It answers confirmations of stated intent** — "you asked to remove this
  orphan; confirm?" — because you already said what you wanted.
- **It does not resolve ambiguity.** A name matching two packages is still an
  error: dstow will not pick one for you, since you never said which.
- **It does not answer the bulk prompt.** Acting on everything wants `--all`,
  which is an explicit statement, not an assumed yes.
- **It does not bypass a guard.** `repo remove` on a still-stowed repo still
  refuses; `--unstow` or `--force` is the answer, and both are statements of
  intent in their own right.

## Output conventions

- **stdout carries the answer; stderr carries everything else.** Commentary,
  warnings, errors, prompts, and hook output all go to stderr.
- **Every commentary line carries a greppable word prefix** — `note:`,
  `warning:`, `error:`, `fix:` — so output stays meaningful without color.
- **`fix:` lines are runnable or precisely pointed.** Where dstow can name the
  remedy, it names it as something you can execute.
- **`--quiet` drops notes**, and nothing else. Announcements, warnings, and
  errors survive it, because they report surprises rather than progress.
