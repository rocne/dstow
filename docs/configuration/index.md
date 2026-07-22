# Configuration: the four levels

dstow reads TOML configuration from four levels. Nearer levels win, and
`ignore` chains are **additive** — a level adds to, and can never silence,
inherited ignores.

You write every config file yourself except the repo registry, which dstow
owns and you never hand-edit.

## Topics

- `keys` — every configuration key, its type and default, and which levels it
  is legal at.
- `ignores` — the `ignore` key's pattern language.
- `stow-compat` — migrating from GNU Stow, and what dstow honors of an
  existing stow setup.

## The files

| File | Location | Written by |
|---|---|---|
| Global config | `$XDG_CONFIG_HOME/dstow/config.toml` | you |
| Repo config | `<repo>/.dstow/config.toml` | you |
| Package config | `<repo>/<pkg>/.dstow/config.toml` | you |
| Repo registry | `$XDG_CONFIG_HOME/dstow/repos.toml` | **dstow** — never hand-edit |
| User theme presets | `$XDG_CONFIG_HOME/dstow/themes/<name>.toml` | you |

**One file name at every level.** The level is determined by placement alone,
and the legality matrix says which keys mean anything where. The file name
carries whatever context its location does not — and here, the location
carries all of it.

`config.toml` never declares repos. The registry is dstow's, because
registration is a command (`dstow repo add`) and dstow does not have
config-mutation commands that edit files you own. The registry is
**configuration, not state**: it is not reconstructible from disk, so it lives
with your intent rather than with the ledger.

## Precedence

The four levels, nearest first:

1. **Package** — `<repo>/<pkg>/.dstow/config.toml`
2. **Repo** — `<repo>/.dstow/config.toml`
3. **Global** — `$XDG_CONFIG_HOME/dstow/config.toml`
4. **Built-in defaults** — `target` is `$HOME`, `translate_dot_prefixes` is
   on, everything else off or unset.

For every key except `ignore`, the nearest level that sets it wins outright.
For `ignore`, all levels contribute: the chains concatenate.

`dstow info <name>` prints the effective values for a scope, which is the
reliable way to find out what actually applies.

## Spelling

Keys are `snake_case` — across config files, `--json` output, and the
color-slot vocabulary. One spelling of every term.

CLI flags keep GNU `kebab-case`. That is the one deliberate spelling mismatch
in dstow, and it exists because both conventions are load-bearing in their own
worlds.

Bool keys are imperative statements (`translate_dot_prefixes`,
`exclude_from_bulk`, `fold_trees`); value keys are nouns (`target`,
`packages_dir`, `ignore`, `theme`).

## Path values

Path-valued keys expand `~` and `$VAR`/`${VAR}` — GNU Stow's grammar —
evaluated **at use time, per invocation**, so a value that depends on the
environment tracks the environment.

- The expanded result must be **absolute**.
- An unset variable is a loud error naming the variable, the file, and the
  key. It is never silently empty.
- Failures scope per package: one package's unresolvable `target` fails that
  package, and the run continues.

`packages_dir` is deliberately outside this grammar — it names a location
*inside* the repo, relative to the repo root, so expanding it to an absolute
path would be meaningless.

## Unknown and misplaced keys

dstow **warns, and never refuses**:

- An **unknown key** gets a warning naming the file and the key, with a
  did-you-mean suggestion.
- A **misplaced key** — one that is legal at a different level — gets a
  warning naming the level where it *is* legal and that level's file. The key
  is ignored.

These are surprise-class announcements: they survive `--quiet`. A typo that
silently did nothing would be the worst outcome available, so it is the one
outcome dstow rules out.

There is no schema-version key. Forward evolution rides this unknown-key
policy; a change that could not ride it would be a breaking change by
definition.

## Shorthand

Wherever a collection entry has a scalar essence, a bare string is legal
shorthand for its table form. In v1 the registry is the single instance:

```toml
repos = ["github:rocne/dotfiles"]
```

is shorthand for the table form with an explicit `source` key. The rule is
schema-wide so that later collections inherit it without a new decision.
