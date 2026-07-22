# Files: what dstow reads and writes

| Path | What | Written by |
|---|---|---|
| `$XDG_CONFIG_HOME/dstow/config.toml` | your global config | you |
| `$XDG_CONFIG_HOME/dstow/repos.toml` | the repo registry | **dstow** |
| `$XDG_CONFIG_HOME/dstow/themes/` | your theme presets, one `<name>.toml` each | you |
| `$XDG_CONFIG_HOME/dstow/hooks/` | global hooks | you |
| `$XDG_STATE_HOME/dstow/ledger.json` | the ledger | **dstow** |
| `$XDG_DATA_HOME/dstow/repos/` | managed clones | **dstow** |
| `<repo>/.dstow/config.toml` | repo config | you |
| `<repo>/.dstow/hooks/` | repo hooks | you |
| `<repo>/<pkg>/.dstow/config.toml` | package config | you |
| `<repo>/<pkg>/.dstow/hooks/` | package hooks | you |
| `<repo>/.stowrc`, `~/.stowrc` | GNU Stow compat, read only | you (or stow) |
| `<repo>/<pkg>/.stow-local-ignore` | GNU Stow compat, read only | you (or stow) |

## The metadata directory

`.dstow/` at a repo or package root — the never-stowed, dstow-claimed
directory carrying that scope's metadata. The global level's equivalent is
`$XDG_CONFIG_HOME/dstow/`.

The name is dot-prefixed by the ordinary tool-metadata convention (`.git`'s
lineage), which makes it inert under dot-translation by construction, and
visually distinct from stow's `.stow` marker file.

Its contents are **never deployed**: the engine auto-ignores it at package
roots, unconditionally, and that is not a config knob. The rule is anchored at
the package root only — a `.dstow` deeper inside a package is ordinary
content.

### Reserved territory

`.dstow/`'s top level is dstow's namespace. The claimed entries are:

    config.toml
    hooks/

An unknown top-level entry draws a warning with a did-you-mean, never a
refusal — the same posture as an unknown config key. The same applies in
`$XDG_CONFIG_HOME/dstow/`, where the claimed names are `config.toml`,
`repos.toml`, `themes/`, and `hooks/`.

## The managed directory

    $XDG_DATA_HOME/dstow/repos/<scheme>/<owner>/<name>

Where dstow clones remote-sourced repos. Directory names use the canonical
percent-encoded segments, so the layout is filesystem-safe by construction and
a link's destination is a canonical, predictable location.

**It is data, not a cache.** Links point into it and its contents are
load-bearing. Deleting it breaks every deployment sourced from a managed repo;
re-cloning is `dstow repo add` again, not an automatic repair.

Local-path repos are never copied here. They are registered in place, and
`repo remove` only forgets them.

## The ledger

One JSON document per machine: `$XDG_STATE_HOME/dstow/ledger.json`.
`dstow info -f ledger-path` prints its resolved location.

- It is a **current-state index, not a journal**: entries are links dstow
  currently believes exist. Unstow deletes entries; there is no history.
- Writes are **atomic** — a temporary file, fsynced, then renamed — so an
  interrupted write cannot leave a half-ledger.
- The whole operation runs under a **lock**. A second dstow that cannot take
  the lock fails fast with exit `3` rather than waiting indefinitely.
- A **corrupt** ledger, or one written by a newer dstow, is a refusal (exit
  `3`) naming the remedy. `dstow rebuild` reconstructs it by walking the
  configured targets.

You never hand-edit it, and it is the one dstow file that is genuinely
disposable: losing it costs you a `rebuild`, not your configuration.

## The registry

    $XDG_CONFIG_HOME/dstow/repos.toml

dstow-written, and the one file dstow writes that lives among your config
rather than your state. It sits there because it is **configuration, not
state**: which repos you use is intent, and intent is not reconstructible by
looking at disk.

    repos = ["github:rocne/dotfiles", "local:/home/you/dots"]

A bare string is shorthand for the table form with an explicit `source` key.
`dstow repo add` and `dstow repo remove` are how it changes — never a text
editor, and never `config.toml`, which cannot declare repos at all.
