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
