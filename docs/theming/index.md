# Theming: how dstow colorizes

dstow colorizes through a **two-stage vocabulary**. Themes speak fourteen
generic slots (`error1`, `success2`, `section1`, …). dstow's internals —
package states, check classes, severity prefixes, prose roles — reach those
slots through a fixed mapping owned by dstow's code.

The consequence is the point: **a theme never needs to know what "orphaned"
means.** You theme a vocabulary of prominence and severity, and dstow decides
which of its concepts wear which slot.

## Topics

- `slots` — the fourteen slots, what consumes each, and how undeclared slots
  derive.
- `values` — the value grammar shared by every theming surface.

## Color survives being turned off

Every commentary line carries a greppable word prefix — `note:`, `warning:`,
`error:`, `fix:` — so output stays fully meaningful with color disabled.
Color is an enhancement of dstow's output, never its only carrier of meaning.

Defaults use only the **16 base ANSI colors**. Your terminal theme therefore
re-themes dstow automatically, and colorblind or low-vision users retheme
through terminal preferences they have already set once.

## Enablement comes first

Whether color is on at all is decided by a fixed precedence, before any theme
is consulted:

    --color  >  NO_COLOR  >  CLICOLOR_FORCE  >  CLICOLOR  >  TTY detection (and TERM=dumb)

`--color` takes `auto` (the default), `always`, or `never`, and the value is
required.

**Theme choices are strictly downstream.** A theme can never re-enable color
that this chain turned off. With color disabled, dstow emits plain text and
the theming stack has no observable effect.

## The stack

Themes layer. **Top wins**, per slot:

1. **`DSTOW_COLORS`** — the one theming environment variable. Packed per-slot
   overrides in the `LS_COLORS` family syntax:

       export DSTOW_COLORS='error1=bold red:success2=#a6e3a1'
       export DSTOW_COLORS=$(dstow theme emit catppuccin-mocha --format env)

   There is deliberately no `DSTOW_THEME`: a name would be a second way to say
   what the config key already says, and the packed form can express anything
   a name can.

2. **The `[color]` table** in global config — one key per slot, same grammar:

       [color]
       error1 = "bold red"
       success2 = "#a6e3a1"

3. **The `theme` config key** — a bare string is a theme *name* (your themes
   directory first, then the bundled presets); a path form is a theme *file*
   anywhere, including inside a repo, which makes repo-shipped themes free:

       theme = "catppuccin-mocha"
       # theme = "~/themes/mine.toml"

4. **The default palette** — seven tier-1 declarations, ANSI-16 only.

Composition is per slot, not per layer: a `[color]` table setting one slot does
not displace a `theme` that set the other thirteen.

## Derivation

After the stack composes, any slot still undeclared **derives from its
family's effective tier-1**, by attribute step-down: remove `bold` if present,
otherwise add `dim`.

Derivation is attribute-only, so it works identically for named ANSI colors,
256-color values, and hex. A value declared at any layer always beats
derivation, and the default palette's tier-1 floor guarantees every slot
resolves to something.

The practical effect: **a sparse theme is coherent.** Declare the seven tier-1
slots and the other seven follow, in the same hues, one prominence step
quieter.

## The bundled presets

    dstow theme list

Six ship in the binary:

- **The four [catppuccin](https://github.com/catppuccin/catppuccin) flavors** —
  `catppuccin-latte`, `catppuccin-frappe`, `catppuccin-macchiato`,
  `catppuccin-mocha` — generated from one
  [Whiskers](https://github.com/catppuccin/whiskers) template, so the role
  mapping is identical across all four.
- **Two ANSI-16 ports of established CLI schemes**: `cargo`
  ([Cargo's help styling](https://github.com/crate-ci/clap-cargo)) and
  `fang-ansi` ([charmbracelet/fang](https://github.com/charmbracelet/fang)'s
  `AnsiColorScheme`). These declare only the slots their source actually
  specifies; the rest falls through the stack. They are sparse on purpose —
  a port that invented values its source never chose would not be a port.

Your own presets go in `$XDG_CONFIG_HOME/dstow/themes/<name>.toml`, and the
file's basename is the theme's name. A user preset **shadows** a bundled one
of the same name.

### What a theme file looks like

A theme file is exactly the `[color]` schema, **bare** — the slot keys at the
top level, no wrapper table and no other keys:

```toml
# ~/.config/dstow/themes/mine.toml
section1 = "bold yellow"
name1    = "bold brightcyan"
error1   = "bold brightred"
```

Note the difference from `config.toml`, where the very same keys live *under*
a `[color]` header. In a theme file there is nothing else in the document, so
there is nothing to distinguish them from — a header would be pure ceremony.
`dstow theme emit --format toml` writes exactly this form, which is the
reliable way to produce one.

Declaring only some slots is normal and supported: the rest fall through the
stack and derive.

## One vocabulary, three representations

The packed `DSTOW_COLORS` string, the `[color]` config table, and a theme file
share one slot vocabulary and one value grammar. Any of them converts to any
other without loss, and `theme emit` performs the conversion:

    dstow theme list                                   the roster: name, origin, active
    dstow theme slots                                  every slot, what it colors, its consumers
    dstow theme emit                                   the effective palette, rendered
    dstow theme emit catppuccin-mocha                  a named theme, rendered
    dstow theme emit catppuccin-mocha --format env     packed DSTOW_COLORS string
    dstow theme emit cargo section1='bold yellow' --format toml \
      > ~/.config/dstow/themes/mine.toml               a tweaked theme file

`theme emit` takes `slot=value` operands rather than a flag per slot: one
grammar, applied on top of whatever theme you named.
