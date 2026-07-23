# Configuration keys and where they are legal

Every configuration key dstow reads, its type and default, and the levels at
which it means anything. A key set at a level where it is not legal is
**warned and ignored**, never an error.

## The legality matrix

| Key | Type / default | package | repo | global |
|---|---|:---:|:---:|:---:|
| `target` | path — default `$HOME` | ✓ | ✓ | ✓ |
| `translate_dot_prefixes` | bool — default `true` | ✓ | ✓ | ✓ |
| `ignore` | list of patterns, **additive** | ✓ | ✓ | ✓ |
| `exclude_from_bulk` | bool — default `false` | ✓ | ✓ | — |
| `packages_dir` | repo-relative path — unset | — | ✓ | — |
| `fold_trees` | bool — default `false` | — | — | ✓ |
| `theme` | theme name or path — unset | — | — | ✓ |
| `[color]` table | fourteen slots, unset | — | — | ✓ |
| `repos` (registry) | written by dstow, in `repos.toml` | — | — | ✓ |

The built-in `$HOME` floor is the global cell's default for `target`: there is
always an effective target, even with no config file anywhere.

## The keys

### `target`

Where this scope's links go.

```toml
target = "~/.config"
```

A path value: `~` and `$VAR`/`${VAR}` expand at use time, and the result must
be absolute. Legal at all three levels, so a single package can be aimed
somewhere other than the rest of its repo.

### `translate_dot_prefixes`

Translate a leading `dot-` in a file name to `.` when linking — GNU Stow's
`--dotfiles` convention. So `dot-zshrc` in the package deploys as `.zshrc` in
the target.

```toml
translate_dot_prefixes = true
```

**On by default.** Turn it off for a repo whose files are already named with
literal leading dots.

### `ignore`

Patterns for files the package should never deploy.

```toml
ignore = ["*.log", "/build/", "**/__pycache__"]
```

**Additive at every level**: a package's `ignore` adds to its repo's, which
adds to global's. A level can never silence an inherited pattern — which is
why negation (`!pattern`) is refused. The pattern language is
gitignore-glob; see `dstow manual configuration ignores`.

### `exclude_from_bulk`

Keep this package out of `--all` and repo-wide runs.

```toml
exclude_from_bulk = true
```

Naming the package explicitly still acts on it — the setting governs bulk
selection, not permission. Legal at package and repo level; there is no global
form, because "exclude everything from bulk" is not a coherent request.

### `packages_dir`

A repo-root-relative directory where this repo's packages live.

```toml
packages_dir = "packages"
```

**Repo level only, and opt-in.** Unset means packages sit at the repo root,
which is what GNU Stow compatibility requires as the default. `"packages"` is
the documented convention and the recommendation for a fresh repo.

Setting it changes how hidden directories are treated during enumeration. With
`packages_dir` unset (root mode), hidden directories are skipped silently —
a repo root has plenty of legitimate ones, starting with `.git`. With
`packages_dir` set (scoped mode), every visible directory inside it is
definitionally a package, and a hidden directory there is skipped **loudly**:
you named a dedicated packages directory, so something hidden inside it is
worth telling you about.

`packages_dir` has no `dstow info` field, by design. `info`'s configured fields
report *composed* effective values (target, translate, fold, ignores,
exclude-from-bulk all resolve across the config levels); `packages_dir` is
repo-level only, so there is nothing to compose — its value is the one line in
the repo's `.dstow/config.toml`. Read it there, or here in the manual.

### `fold_trees`

GNU Stow's "tree folding": link a whole directory when dstow owns all of it,
rather than linking each file inside it.

```toml
fold_trees = true
```

**Global only, and off by default.** Global-only because folding decides the
shape of the deployed tree, and a per-package answer would produce a target
whose shape depends on the order packages were stowed in.

Changing it after deployment does not rewrite anything — it marks the affected
packages `drifted` in `status`, and `restow` re-lays them.

### `theme` and `[color]`

Global only. See `dstow manual theming` for the whole stack, and
`dstow manual theming values` for the value grammar.

```toml
theme = "catppuccin-mocha"

[color]
error1 = "bold red"
success2 = "#a6e3a1"
```

`theme` follows the name-or-path operand rule: a bare string is a theme
*name*, resolved against your themes directory first and then the bundled
presets; a path form is a theme *file* anywhere — including inside a repo,
which is what makes a repo-shipped theme possible.

The `[color]` table is a closed set of exactly fourteen keys. An unknown slot
name gets the usual unknown-key warning with a did-you-mean.

## Unknown and misplaced keys

Both warn; neither refuses:

    warning: unknown key "targets" — did you mean "target"? (the key is ignored,
             the rest still applies)

    warning: key "packages_dir" means nothing at the global level; it belongs at
             the repo level (<repo>/.dstow/config.toml) (the key is ignored here)

These warnings survive `--quiet`, because a typo that silently does nothing is
the failure mode worth being loud about.

## Seeing what actually applies

The matrix tells you what is legal; `info` tells you what is in force:

    dstow info zsh                    every field for one package
    dstow info zsh -f target          one field
    dstow info dotfiles -r            a repo and everything under it
    dstow info zsh --json             machine-readable

`info` reads configuration and metadata only — it never touches the target.

**`info` speaks fields, not config keys**, and the two vocabularies are not
the same words: the field for `translate_dot_prefixes` is `translate`, for
`fold_trees` it is `fold`, and for `ignore` it is `ignores` (plural, because
what it reports is the composed chain rather than one file's entry). See
`dstow manual reference` for the full field list.
