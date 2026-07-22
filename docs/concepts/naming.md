# Naming: the qualified-name grammar

Every repo and package has one fully-qualified name (FQN), and it is that
thing's stable identity. You rarely type the whole thing.

## The grammar

    scheme:coordinate::package

A repo's FQN drops the `::package` tail:

    github:rocne/dotfiles          a repo
    github:rocne/dotfiles::zsh     a package in it
    local:/home/you/dots::vim      a package in a local repo

    github:rocne/dotfiles::starship
    └─┬──┘ └──────┬──────┘  └──┬──┘
      │           │            package
      │           coordinate
      scheme

One grammar covers sources, repos, and packages.

- **`:` separates only the scheme.** `X:Y` is always scheme-then-coordinate.
  An unknown scheme is a clean error listing the known ones.
- **`::` separates only the package**, and forces the repo/package boundary.
  `dots::zsh` means "the package `zsh` in a repo whose name ends `dots`",
  never a repo called `dots::zsh`.
- **The coordinate is path-shaped** — one or more `/`-separated segments. A
  scheme may interpret its own: `github:` reads `owner/name`, `local:` reads
  an absolute filesystem path, canonicalized when the repo is added. The
  grammar itself does not interpret it.

## Suffix resolution

**Refer to anything by any unambiguous suffix of its FQN**, cut at a segment
boundary. All four of these can name the same package:

    zsh
    dots::zsh
    rocne/dotfiles::zsh
    github:rocne/dotfiles::zsh

Three rules fall out of that:

- **A leading `::` forces package-kind.** `::zsh` will only ever match a
  package, never a repo that happens to end in `zsh`.
- **A scheme prefix requires the full coordinate.** `github:rocne/dotfiles`
  resolves; `github:dotfiles` does not — a scheme names a whole coordinate,
  not a suffix of one.
- **Naming a repo where packages are expected means all of its packages.**
  `dstow stow dotfiles` stows every package in that repo.

Shell brace expansion gives you multi-select for free:

    dstow stow dotfiles::{bash,zsh}

## Ambiguity

Ambiguity is exclusively a property of *your input*, never of the model:
same-named packages from different repos coexist perfectly well, and the
qualified names always distinguish them.

When what you typed matches more than one thing, dstow does not guess:

- **Interactively**, it asks you to choose explicitly.
- **Non-interactively**, it is a hard error (exit 3) listing the qualified
  spellings so you can re-run with one of them.

Cross-kind ties follow the same rule — a bare name matching both a repo and a
package is ambiguous like any other.

## Names versus paths

Some commands take either. The **first character** decides, and no command
ever guesses:

- A **path operand** starts with `/`, `~/`, `./`, or `../`, and always refers
  to the *target* world — a real location on disk.
- A **name expression** is anything else, and resolves against the registry.

So `dstow status ~/.zshrc` asks what occupies that path, while
`dstow status zsh` asks about the package.

## Encoding

Reserved characters percent-encode, so every possible path is spellable —
there are no exceptions and no unrepresentable names.

| Reserved | Encoded | Why |
|---|---|---|
| `:` | `%3A` | the scheme separator |
| `%` | `%25` | the escape character itself |
| `@` | `%40` | a reserved scheme-interpreted suffix (see below) |
| control characters | `%0A`, … | `DSTOW_HOOK_PACKAGES` is newline-separated, so a pathological name must not corrupt the list |

**dstow always emits the canonical encoded form itself.** You paste what it
printed rather than constructing encodings by hand. If you do need to spell a
segment exactly as dstow would, the hidden `name` group does it:

    dstow name encode 'weird:name'      # -> weird%3Aname
    dstow name decode 'weird%3Aname'    # -> weird:name

It operates on one coordinate *segment*, encoding the characters the grammar
reserves and leaving ordinary ones alone. `/` is not reserved, so path
separators survive untouched.

When `repo add` is given a path that would need encoding, it asks you
interactively whether to continue or rename; non-interactively it proceeds
with a loud announcement rather than refusing.

### The `@` reservation

`@` is reserved in coordinates as an optional, scheme-interpreted suffix:

    scheme:coordinate@suffix::package

**v1 assigns it no meaning.** A literal `@` in a name expression must be
written `%40`. What an `@`-suffix means belongs to whichever scheme later
chooses to interpret it — the anticipated lineage is revision pinning, in the
manner of npm, Homebrew, and Go modules — but the reservation binds only the
syntax. Reserving it now is what makes that future non-breaking.

## How dstow displays names

- **Shortest unique suffix, everywhere, by default.** Output stays readable.
- **The full FQN whenever it matters**: showing a tie, reporting an ambiguity
  error, or emitting `--json`, which always carries FQNs.
- **Local coordinates abbreviate with `~`** where they sit under your home
  directory.
