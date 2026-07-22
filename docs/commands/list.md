# dstow list

<!-- dstow:short -->
What is configured: repos, packages, targets (never reads disk)
<!-- /dstow:short -->

<!-- dstow:long -->
What is configured — repos, packages, targets, exclusions, sources.
Reads only configuration: instant, side-effect free, never inspects disk.
Deployment truth lives in 'dstow status'.

Bare list shows repos (the global scope's content); naming a repo lists its
packages; naming a package lists its paths, relative to the package
directory.
<!-- /dstow:long -->

## The package-paths view is a raw walk

When you name a package, the paths list is a **raw walk of the package
directory** — the files as they sit in the repo, with **no dot-translation and
no ignore filtering applied**. So `dot-zshrc` lists as `dot-zshrc`, not
`.zshrc`, and a file an `ignore` pattern would exclude still appears. This is
`list` doing exactly what it promises: reporting what is configured on disk in
the package, not what would deploy from it.

Three views answer three different questions, and it is worth keeping them
apart:

- **`dstow list <package>`** — the raw package contents, untranslated.
- **`dstow stow --dry-run <package>`** — the translated plan: names as they
  would land in the target, ignored files dropped.
- **`dstow status <package>`** — what is actually deployed right now.

The package's own root `.dstow/` directory is excluded from the paths view: it
is dstow's bookkeeping, not package content, and it never deploys (`dstow
manual configuration ignores`).

## `--repos` and `--packages`

- `--repos` narrows the listing to repos alone, with each source and scheme.
- `--packages` widens it to every package across every repo, **attributing each
  to its repo** and qualifying any same-named packages so two packages called
  `zsh` from different repos are told apart; unique names stay bare.
