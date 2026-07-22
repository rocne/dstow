# dstow adopt

<!-- dstow:short -->
Import an existing file into a package, leaving a link behind
<!-- /dstow:short -->

<!-- dstow:long -->
Import an existing real file into a package; a link takes its place.
Live content always wins — adopt never destroys running configuration.

With a package: shows its plan and asks before overwriting differing
package content. Without a package: lists the packages that could adopt
the file, ranked, and asks you to pick (in scripts: an error that lists
the candidates as remedies). --occupied adopts every occupied path of the
named package.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow adopt ~/.zshrc zsh
  dstow adopt ~/.config/foo/foo.toml     # no package: pick from candidates
<!-- /dstow:examples -->

## How candidates are ranked when you name no package

With no package, adopt lists the packages that *could* adopt the file. A
package is a candidate when its effective target covers the path **and** the
package-relative source the file would map to is not ignored (the package's own
`translate_dot_prefixes` setting decides how that source is spelled — so
`~/.zshrc` maps to `dot-zshrc` under the default, or `.zshrc` with translation
off).

The list is ranked by **how many of that package's ledgered links already sit
in the file's own directory**, most first — the package that already owns the
neighbourhood is the likely home. Ties break in canonical qualified-name order.

This is a pure configuration-and-ledger computation: no tree is walked, so it
is instant and gives the same ranking whether or not the file is currently
deployed. `dstow manual commands status` shows the same ranked candidates for a
path.

## `--force`, `--yes`, and live content

Live content always wins — adopt overwrites the **package's** copy with what is
really deployed, and never the other way round. When the package already holds
differing content at that source, adopt guards the overwrite with a
confirmation.

That confirmation is a **guard**, not a confirmation of stated intent, so `-y`
does not answer it — only `--force` does. Non-interactively and without
`--force`, differing package content is a refusal (exit `3`), never a silent
overwrite. The general rule for what `--yes` answers and what it never touches
is in `dstow manual reference`.

`--occupied` adopts every occupied path of the named package in one run,
importing each live occupant and leaving a link in its place.
