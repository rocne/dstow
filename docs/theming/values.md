# Color values: the value grammar

Slot values use **git's `color.*` grammar** — whitespace-separated words, in
any order. One grammar across all three theming surfaces: the packed
`DSTOW_COLORS` string, the `[color]` config table, and theme files.

    error1   = "bold red"
    success2 = "#a6e3a1"
    value2   = "normal 240"
    section1 = "reset bold ul brightgreen"

## The words

| Form | Words | Notes |
|---|---|---|
| Basic colors | `black` `red` `green` `yellow` `blue` `magenta` `cyan` `white` | plus their `bright*` variants — `brightred`, `brightblue`, … |
| 256-color | an integer `0`–`255` | |
| True color | `#RRGGBB` hex | |
| `normal` | — | leaves a channel to the terminal; emits nothing |
| `default` | — | resets a channel to the terminal default; emits `SGR 39/49` |
| Attributes | `bold` `dim` `italic` `ul` `blink` `reverse` `strike` | any number, each negatable with `no`/`no-` — `nobold`, `no-dim` |
| `reset` | — | clears first |

## Foreground and background

Position among the *color* words decides: **the first color word is the
foreground, the second is the background.** A third is an error.

    error1 = "red"              red foreground
    error1 = "white red"        white on red
    error1 = "bold white red"   the same, bold

To set a background without touching the foreground, put `normal` in the
foreground position:

    warning1 = "normal yellow"  yellow background, foreground left alone

Attributes may appear anywhere in the string — order only matters among the
color words.

## `normal` versus `default` versus undeclared

These are three different things, and none of them means "dstow's default".

- **`normal`** leaves the channel to the terminal and emits no code. A slot
  you declare `normal` is a declaration like any other: because the stack is
  top-wins, it **replaces** dstow's default for that slot wholesale, and the
  slot renders plain.
- **`default`** actively emits the terminal-default code (`SGR 39/49`) rather
  than nothing. Use it when you need to *reset* a channel that something else
  set, not merely to abstain from setting it.
- **Undeclared** is the only way to keep dstow's default for a slot. Leave the
  key out entirely and it falls through the stack to the default palette, or
  derives from its family's tier-1.

The same logic governs negated attributes: `nobold` renders as nothing, and it
does not restore whatever dstow would have chosen. A themed slot replaces its
default wholesale.

## `reset`

`reset` clears first, then applies the rest of the string. It is how you write
a value that must not inherit anything from surrounding output.

## Round-tripping

The grammar is losslessly convertible between all three representations, in
both directions. `dstow theme emit` is the converter:

    dstow theme emit --format toml       the effective palette as a theme file
    dstow theme emit --format env        the same, as a packed DSTOW_COLORS string
    dstow theme emit catppuccin-mocha --format env

Parsing and emitting are inverse operations, so a value dstow prints is a
value dstow accepts, and the parameter order it prints is canonical.

## The packed form

`DSTOW_COLORS` packs the same `slot=value` pairs into one string,
colon-separated, in the `LS_COLORS` family syntax:

    export DSTOW_COLORS='error1=bold red:success2=#a6e3a1:value2=dim'

Values containing spaces need no quoting *within* the packed string — the
colon is the only separator — but the whole assignment wants shell quoting, as
above.

## Trying a value out

`theme emit` takes `slot=value` operands on top of a named theme, so you can
see a change before committing it to a file:

    dstow theme emit cargo section1='bold yellow'
    dstow theme emit cargo section1='bold yellow' --format toml \
      > ~/.config/dstow/themes/mine.toml

And `dstow theme slots` renders every slot name in its own effective style,
which is the fastest way to see what a theme actually looks like.
