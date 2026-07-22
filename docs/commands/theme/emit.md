# dstow theme emit

<!-- dstow:short -->
Emit a theme's colors, each value in its own style: the effective palette (bare), a named theme, slot=value tweaks on top; --format env|toml for machines
<!-- /dstow:short -->

<!-- dstow:long -->
Emit a theme's colors — the effective palette (no name), a named theme, or
either with slot=value tweaks layered on top. The default view renders each
slot's value in its own style; --format env|toml emits for machines.

Values use git's color.* grammar — see 'dstow theme slots --help'.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow theme emit
  dstow theme emit catppuccin-mocha
  export DSTOW_COLORS=$(dstow theme emit catppuccin-mocha --format env)
  dstow theme emit cargo section1='bold yellow' --format toml > ~/.config/dstow/themes/mine.toml
<!-- /dstow:examples -->
