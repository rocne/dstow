# dstow theme

<!-- dstow:short -->
Theming: list themes, describe slots, emit colors
<!-- /dstow:short -->

<!-- dstow:long -->
Theming: list themes, describe slots, emit colors.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow theme list
  dstow theme slots
  dstow theme emit
  export DSTOW_COLORS=$(dstow theme emit catppuccin-mocha --format env)
  dstow theme emit cargo section1='bold yellow' --format toml > ~/.config/dstow/themes/mine.toml
<!-- /dstow:examples -->

## Topics

- `list` — the available themes
- `slots` — the color slots and the value grammar
- `emit` — a theme's colors
