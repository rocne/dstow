# Ignores: the `ignore` key and its pattern language

The `ignore` key is the one carrier for "do not deploy this". It is legal at
all three levels and it is **additive**: a level adds patterns to the chain it
inherits, and can never remove one.

```toml
ignore = ["*.log", "/build/", "**/__pycache__"]
```

There is no native ignore *file*. The concept is expressible natively, in the
config you already have, and one carrier is better than two — file-for-file
mirroring of GNU Stow was never the goal. Stow's own ignore files stay honored
as compatibility inputs; see `dstow manual configuration stow-compat`.

## The language

Patterns are **gitignore-glob**, matched **per package** against paths
relative to that package's root.

| Form | Meaning |
|---|---|
| `*.log` | No slash: matches the basename at any depth. |
| `/build` | Leading slash: anchored at the package root. |
| `build/` | Trailing slash: matches directories only. |
| `**/__pycache__` | `**` matches any number of path segments. |
| `doc/*.txt` | An interior slash anchors the pattern at the package root. |

If you have written a `.gitignore`, you already know this language; the
semantics are the same, including the `**` handling and the directory-only
suffix.

## Two refused forms

Both are refused loudly, and both are reserved:

- **Leading `!` (negation).** Negation would let a nearer level silence an
  inherited pattern, which the additive law forbids. It is not merely
  unimplemented — it contradicts the design of the chain.
- **Leading `//`.** Reserved for a possible future regex marker, so that
  introducing one later cannot change the meaning of a pattern anyone has
  already written.

## What dstow always ignores

A package's own root `.dstow/` directory is never deployed. That is not a
config knob and cannot be switched off — it is dstow's metadata directory, and
deploying it would link dstow's own configuration into your target.

The rule is anchored at the **package root only**. A `.dstow` directory deeper
inside a package is ordinary content and deploys normally: if you keep a
dotfiles package for dstow itself, its nested `.dstow` is your data.

Deploy, `status`, and `check` all consult the same ignore logic, so what dstow
deploys and what dstow expects to find can never disagree.

## Pattern language is a property of the carrier

Native carriers speak glob. Compatibility carriers — `.stow-local-ignore` and
stow's `--ignore` option — speak stow's regex, which is their native language.
The two are never mixed inside one file, and dstow never rewrites one into the
other: a pattern is read in the language of the file it was written in.

This is why a stow ignore file keeps working unchanged after migration, and
why patterns copied out of one into a `config.toml` need translating by hand.

## Seeing the effective chain

    dstow info zsh -f ignores

prints the composed chain for that package — every pattern, from every level,
in one list. Because the chain is additive, that is usually more informative
than reading any one config file.

Note the field is `ignores`, plural: the config *key* is `ignore`, and what
`info` reports is the composed result of all of them.
