# CLI theming and color customization — survey (2026-07-14)

**Question.** Colorized output is high-priority UX for dstow (per the
[CLI-framework survey](cli-framework-survey-2026-07-14.md)). Given that
decision — cobra + fatih/color behind one owned printer, semantic style
names, `--color` > `NO_COLOR` > `CLICOLOR_FORCE` > `CLICOLOR` > TTY — this
survey asks: what do users expect to be able to customize about a CLI's
colors, is "theming" a real convention for line-oriented status tools (as
opposed to document/syntax renderers), and does anything here change the
printer decision? Default assumption per the brief: the printer stays;
theming layers into it.

**Vocabulary anchor** ([CONTEXT.md](../../CONTEXT.md)): any semantic style
table needs one slot per package state (stowed / partially stowed / not
stowed / occupied / damaged / drifted, broken / orphaned) plus severities
(error/warning/hint/announcement, REQUIREMENTS.md §1.3–1.4). REQUIREMENTS.md
is silent on user theming; §1.7 requires "one output style" and
machine-consumable output where specified; §4 is the four-level config chain
(package → repo → global → built-in) where any theme knob would slot in —
almost certainly global-only, like folding.

---

## A. Standards and de facto conventions

### A1. The ANSI-16 convention is the zero-config theming most CLIs rely on

Emitting only the 16 named ANSI SGR colors (plus bright variants) means the
**user's terminal emulator's palette settings silently retheme the app** —
no app-side config needed. This is exactly the mechanism GitHub CLI's
accessibility feature is built on: `gh`'s `accessible_colors` config /
`GH_ACCESSIBLE_COLORS` env var (added in
[cli/cli#10820](https://github.com/cli/cli/pull/10820), fixing
[cli/cli#857](https://github.com/cli/cli/issues/857)) **restricts output to
the 16 standard ANSI colors specifically so users can recolor gh's output
through their terminal's own color-scheme settings** — the PR frames this as
accessibility (contrast, colorblind accommodation) via the terminal's
existing customization surface rather than a gh-specific one. gh's
`pkg/iostreams/color.go` (`ColorScheme.Accessible` field, doc comment:
*"Accessible is whether colors must be base 16 colors that users can
customize in terminal preferences"*) is the load-bearing code — a boolean
that forces every style function down to the 16-slot palette.

256-color and truecolor output (`COLORTERM=truecolor`,
`#RRGGBB`/`color(N)` codes) **bypass this mechanism entirely** — a hex color
looks the same regardless of the user's terminal theme, because it isn't a
named slot the terminal remaps. That's the right choice for *content*
(syntax highlighting, diffs, markdown rendering, TUI chrome/borders) where
exact appearance matters more than theme integration, but it is a
**regression** for a status/announcement tool: it takes theming control away
from the user's terminal and gives dstow nothing in return, since dstow has
no document content to render precisely.

**Verdict: dstow is squarely an ANSI-16 tool.** Its output is short status
lines and severities, not documents or diffs — the case where 256/truecolor
earns its keep.

### A2. git's `color.*` config — the closest precedent for a TOML CLI

[git-config docs](https://git-scm.com/docs/git-config) (`color.<slot>`
family, e.g. `color.status.added`, `color.diff.new`, `color.branch.current`):

> "The value for a variable that takes a color is a list of colors (at most
> two, one for foreground and one for background) and attributes (as many as
> you want), separated by spaces."

Full syntax, verbatim from the docs:

- **Basic colors**: `normal black red green yellow blue magenta cyan white
  default`, each with a `bright` variant (`brightred`). `normal` = no change
  (usable as foreground when specifying background alone: `normal red`).
  `default` explicitly resets to the terminal default.
- **Extended**: `0`–`255` (ANSI 256-color), `#ff0ab3` (24-bit hex), `#f1b`
  (12-bit hex, expands to `#ff11bb`).
- **Attributes**: `bold dim ul blink reverse italic strike`; any attribute
  negated with `no`/`no-` prefix (`noreverse`); position relative to colors
  doesn't matter; pseudo-attribute `reset` clears prior colors/attributes
  before applying the rest (`reset green` = plain green foreground).
- **Empty string** = no color effect at all (a per-slot "off" switch distinct
  from disabling color globally).

This is **precisely** the shape a `[color]` TOML table override wants: one
key per semantic slot, value is a small space-separated grammar of
name/number/hex + attributes. It is old (git has shipped this since the
2000s), universally recognized by git's enormous user base, and requires no
new parsing concept dstow's users haven't already met.

### A3. Env-var `key=style` conventions (LS_COLORS family)

- **`LS_COLORS`/`dircolors`** ([GNU coreutils manual](https://www.gnu.org/software/coreutils/manual/html_node/dircolors-invocation.html)):
  `dircolors` compiles a config file (keywords like `NORMAL`, `DIR`, `LINK`,
  extension globs `*.tar`) into a colon-separated `KEY=SGR:KEY=SGR:...`
  environment variable that `ls` and everything downstream of it reads
  directly — no app-specific config file, just an env var as the wire
  format. `dircolors --print-database` dumps a commented example.
- **`GREP_COLORS`** ([GNU grep manual](https://www.gnu.org/software/grep/manual/grep.html)):
  same `key=SGR` shape, small fixed key set (`sl`, `cx`, `mt`, `ms`, `mc`,
  `fn`, `ln`, `bn`, `se`), default value spelled out in the manual:
  `ms=01;31:mc=01;31:sl=:cx=:fn=35:ln=32:bn=32:se=36` ("bold red matched
  text, magenta file names, green line numbers... cyan separators, and
  default terminal colors otherwise").
- **`EZA_COLORS`** ([eza man page](https://github.com/eza-community/eza/blob/main/man/eza_colors.5.md)):
  same colon/`=`/`;` grammar, extends rather than replaces `LS_COLORS`
  ("Values in EZA_COLORS override those given in LS_COLORS, so you don't
  need to re-write an existing LS_COLORS variable"), ~40 two-letter codes
  plus file-glob keys in the same namespace.
- **`JQ_COLORS`** (verified directly in
  [jq source](https://github.com/jqlang/jq/blob/master/src/jv_print.c)):
  colon-separated list of SGR codes, positional (by `jv_kind` order, last
  slot = object keys), default
  `"1;30:0;39:0;39:0;39:0;32:1;39:1;39:34;1"` roughly (`DEFAULT_COLORS`
  macro: null, false, true, numbers, strings, arrays, objects, object keys).
  No key names at all — pure positional list, the least discoverable of the
  four.

**Pattern across all four**: single env var, colon-separated entries, each
entry is a short key (name or two-letter code) mapped to a *raw SGR
attribute string* (`01;31`), not named colors. This is the "escape hatch"
end of the spectrum — maximally portable (works with zero app-side parsing
beyond splitting), minimally friendly (users must know SGR numbers, or copy
someone else's string). git's approach (A2) is friendlier — named colors
and attribute words — while landing in the same conceptual slot-map shape.

### A4. Theme-file ecosystems — document/syntax renderers, not status CLIs

Checked directly against source, to answer "is theming a real convention for
line-oriented CLIs":

- **base16/base24** ([tinted-theming/home builder.md](https://github.com/tinted-theming/home/blob/main/builder.md),
  [styling.md](https://github.com/tinted-theming/home/blob/main/styling.md)):
  a *scheme* is 16 (or 24) hex colors (`base00`–`base0F`) with defined
  semantic roles — `base00`–`base07` are a background→foreground ramp
  ("used for foreground and background, status bars, line highlighting"),
  `base08`–`base0F` are construct colors ("Variables... Strings...
  Functions... Keywords" — explicitly a *syntax-highlighting* vocabulary). A
  *builder* renders a scheme through a *template* (Mustache) into an
  app-specific config file — one template per consuming app (base16-vim,
  base16-emacs, terminal emulators). This is **whole-environment,
  truecolor-hex theming for editors and terminal emulators**, built and
  consumed through per-app templates, not something a line-oriented CLI
  reads directly.
- **bat**: confirmed via its own docs — `--theme`/`BAT_THEME` selects a
  `.tmTheme` (Sublime Text color scheme) file; `bat --list-themes` lists
  them. Explicitly for **syntax-highlighting file *contents***, not UI
  chrome or status messages.
- **delta**: [`themes.gitconfig`](https://github.com/dandavison/delta/blob/main/themes.gitconfig)
  ships community "themes" as git-config `[delta "name"]` sections (reusing
  A2's grammar plus delta-specific keys like `commit-decoration-style`,
  `hunk-header-style = syntax bold italic 237`) — themes here are bundles of
  git-color-config values for a **diff renderer**, selected via
  `[delta] features = <name>`, each tagged `dark = true`/`light = true`.
  Closer to dstow's territory than base16/bat (it's git-config syntax, not a
  document format) but still document-rendering (diff hunks), not status
  lines.
- **glamour** (verified directly,
  [`styles/dark.json`](https://github.com/charmbracelet/glamour/blob/main/styles/dark.json)):
  JSON keyed by **Markdown element** — `document`, `h1`...`h6`,
  `block_quote`, `emph`, `strong`, `hr` — each with `color`,
  `background_color`, `bold`/`italic`, prefix/suffix strings. This is a
  **Markdown-document style sheet**, unambiguously the wrong shape for a
  package-status line.

**Honest categorization: dstow is not in this category.** All four are
theming systems for *rendering documents or whole environments*
(syntax-highlighted files, diffs, terminal emulators, editors). dstow emits
short, semantically-named status lines — closer to `ls`/`grep`/git-status
territory (A3/A2) than to bat/delta/glamour/base16 territory. Adopting a
theme-*file* ecosystem here would be solving a problem dstow doesn't have at
the cost of a document-model dstow doesn't need.

### A5. Accessibility

- **NO_COLOR** and **CLICOLOR/CLICOLOR_FORCE**: already resolved in the
  framework survey (§4 there) — no new material here beyond confirming they
  are the accessibility floor every candidate must clear, which fatih/color
  + dstow's printer already do.
- **gh CLI's accessible-colors feature (A1)** is the one documented,
  shipped, accessibility-motivated theming feature found in this territory:
  restrict to the 16-slot palette *on user request* so colorblind/low-vision
  users can retheme through their terminal rather than fighting a
  hardcoded truecolor palette. No colorblind-safe-default-palette
  convention was found documented anywhere in this survey (base16/k9s/
  lazygit schemes are aesthetic choices, not accessibility-audited
  defaults) — if dstow wants a specific accessibility posture beyond
  "16-slot by default," that posture doesn't have an off-the-shelf
  precedent to copy; it would be original work.

---

## B. Go packages for theme/style management

### B1. fatih/color + owned printer — what it already gives, what's missing

Confirmed from the framework survey: fatih/color gives per-instance
`color.New(...)` construction, `SprintFunc()` (identity when disabled),
NO_COLOR-aware, isatty-based. What it does **not** give, that a "real"
theming layer would:

- **No style-string parser.** fatih/color's API is Go constructor calls
  (`color.New(color.FgRed, color.Bold)`), not a string grammar like git's
  `"bold red on black"`. Turning a *user-supplied config string* into a
  `*color.Color` requires dstow to write the parser — small (git's own
  grammar is ~6 color words + `bright` prefix + 256/hex + ~7 attribute
  words + `no`-negation + `reset`), but it is new code, not something
  fatih/color hands you.
- **No theme-file loading.** Not a gap for dstow per A4 — this is a
  non-goal.
- **No color-degradation** (truecolor→256→16 downsampling) — irrelevant if
  dstow stays ANSI-16 by policy (A1), which is the recommendation below.

### B2. Other Go libraries — maintenance snapshots (GitHub API, 2026-07-14)

| Library | Stars | Last push | Verdict |
|---|---|---|---|
| [gookit/color](https://github.com/gookit/color) | 1,602 | 2026-06-04 | Actively maintained, broader style-string-ish helpers than fatih/color, but a second full color library — redundant next to the already-chosen fatih/color, no unique capability dstow needs. |
| [jwalton/gchalk](https://github.com/jwalton/gchalk) | 343 | **2022-03-22** | Effectively unmaintained (4+ years stale). Rule out. |
| [alecthomas/chroma](https://github.com/alecthomas/chroma) | 4,977 | 2026-07-13 (very active) | Syntax-highlighting engine (Pygments-style lexers/themes) — same category problem as A4; not the right altitude for status lines. |
| [charmbracelet/glamour](https://github.com/charmbracelet/glamour) | 3,594 | 2026-06-14 | Confirmed document-renderer (B4/A4) — not applicable. |
| muesli/termenv, charmbracelet/lipgloss | — | — | Already assessed in the framework survey: termenv runner-up (v0.x, coasting since 2025-02), lipgloss ruled out (TUI layout engine, major-version churn, `charm.land` import-path migration). Nothing in this theming-specific research changes either verdict. |

**None of these displace fatih/color.** gookit/color is the only live
alternative and it solves a problem dstow doesn't have (dstow already picked
a color library); chroma/glamour are confirmed wrong-category; gchalk is
stale.

### B3. Comparable Go CLIs' user color config — three instructive precedents

1. **gh CLI** (`pkg/iostreams/color.go` — read directly from source): a
   `ColorScheme` struct with `Enabled`, `EightBitColor`, `TrueColor`,
   **`Accessible`** (doc comment: *"colors must be base 16 colors that users
   can customize in terminal preferences"*), `ColorLabels`, `Theme`
   (`dark`/`light`/`none`) fields, and named methods (`Bold`, `Muted`,
   `Red`, `GreenBold`...) that branch on those flags — e.g. `Muted()`
   switches between `lightThemeMuted`/`darkThemeMuted` SGR funcs by
   `c.Theme`. This is a **semantic-method** shape (methods named for
   *meaning*, not raw color), exactly dstow's "one owned printer" plan, plus
   an explicit accessibility toggle (A1/A5) dstow could adopt as a rung
   above bare ANSI-16-always. No user-facing per-slot override — `Theme` is
   dark/light/none, not a customizable palette.
2. **lazygit** (`docs/Config.md#color-attributes`, read directly): a
   `theme:` block in `config.yml` maps ~15 named UI slots
   (`activeBorderColor`, `selectedLineBgColor`, `cherryPickedCommitFgColor`,
   `unstagedChangesColor`, `defaultFgColor`...) to a **YAML list of
   attributes**, e.g. `activeBorderColor: [green, bold]`. Allowed color
   values: the 8 basic names or a `'#ff00ff'` hex string; allowed modifiers:
   `bold`, `default`, `reverse`, `underline`, `strikethrough`. This is
   git-config's grammar (A2) re-expressed as a YAML list instead of a
   space-separated string — same expressive power, different syntax sugar.
   Directly relevant: lazygit is a TUI, but its color-*config* shape (named
   semantic slot → color+attributes) is line-CLI-portable and nearly
   identical to what dstow would want for TOML.
3. **k9s** (`skins/dracula.yaml`, read directly): a full-file **skin**
   keyed by dozens of nested UI regions (`body.fgColor`, `frame.status.
   errorColor`, `views.charts.defaultDialColors`...) using YAML anchors for
   a hex palette reused across slots. This is the **theme-file** end of the
   spectrum (A4) — appropriate for a TUI with dozens of visual regions, and
   the clearest illustration that "theme files" show up for *screen-painting*
   apps, scaling with UI surface area. dstow's status-line surface (a
   handful of semantic states + severities) doesn't have that many slots —
   git-config/lazygit's per-slot approach fully covers it without needing a
   skin file.
4. **delta** (A4) — git-config-shaped `[delta "name"]` theme sections is
   the closest any comparable tool comes to "git-config grammar, packaged
   as shareable named themes." If dstow ever wants shareable presets (a v2
   idea, not v1 scope per the brief), delta's approach — themes are just
   named bundles of the same per-slot config syntax, not a separate file
   format — is the precedent to copy, not base16/glamour.

---

## C. The click/rich/rich-click anchor — mapped to Go

The human's standing reference (Python, work CLIs). Verified directly
against docs/source.

| Rich/rich-click niceties | What it is | Honest Go equivalent |
|---|---|---|
| **`rich.theme.Theme`** — dict of semantic name → style string (`Theme({"info": "dim cyan", "warning": "magenta", "danger": "bold red"})`), attached to a `Console(theme=...)`, referenced by name at call sites (`console.print(..., style="info")`) | A first-class, library-provided semantic-theme object with a lookup-by-name API | **No direct equivalent in any Go color library surveyed.** fatih/color has no named-theme-map concept — dstow's "one owned printer" *is* dstow's homemade version of this (a Go map/struct of semantic name → `*color.Color`, hand-built). gookit/color doesn't provide this either. This is a place dstow writes ~20 lines dstow already planned to write; there's no library gap being left unfilled, just no library doing it *for* you. |
| **Style-string grammar**: `"blink bold red underline on white"`, colors as name / `color(N)` / `#hex` / `rgb(...)`, background via `on`, loadable from an `.ini` file via `Theme.read()` | A parser turning a human-typed string into a style object, plus a text-file loading convenience | **git's `color.*` grammar (A2) is the honest Go-world equivalent convention** — same shape (colors + attributes, space-separated, `no`-negation, `reset`), just no shared Go library implements *parsing* it generically. Writing this parser is small (bounded vocabulary, no nesting) — a natural owned-printer addition if dstow adds per-slot config overrides (ladder rung 2, below). termenv doesn't parse this either — it does color-profile *degradation*, a different problem. |
| **rich-click**: styled/colorized `--help` output for a `click` app; ~100 bundled visual themes selectable via `RICH_CLICK_THEME` env var; panel-grouped options; HTML/SVG export | A framework-level help-renderer bolt-on | **[ivanpirog/coloredcobra](https://github.com/ivanpirog/coloredcobra)** is the real but thin Go equivalent: patches cobra's usage template, colorizes it via fatih/color, single `Config{}` struct. Confirmed via GitHub API: 67 stars, **last pushed 2025-05-01** — over a year stale as of this survey, no bundled themes, no panel grouping, no HTML/SVG export. **Honest verdict: nothing in Go's cobra ecosystem matches rich-click's polish or maintenance activity.** If dstow wants styled `--help`, that's dstow's owned printer routing cobra's `SetUsageFunc`/`SetHelpFunc` through its own semantic styles (already the plan per the framework survey's Go-architecture ticket item 6) — not adopting a third-party cobra add-on. |

**Bottom line on the anchor**: rich/rich-click's edge over the Go ecosystem
isn't a missing *library* so much as a missing *convention* — Python's
`click`/`rich` ecosystem converged on a shared theming vocabulary
(Console/Theme/style-strings) that many CLIs share; Go's ecosystem hasn't,
so every serious Go CLI (gh, lazygit, dot-dagger) hand-rolls its own
semantic wrapper. dstow doing the same (as already planned) is not a
compromise — it's the Go-ecosystem norm, evidenced by every comparable
tool in B3.

---

## D. Synthesis: the options ladder

| Rung | Shape | Cost | Precedent |
|---|---|---|---|
| **1. ANSI-16 semantic palette only, no user config** | Printer maps CONTEXT.md states + severities to the 8 basic ANSI colors (+bright), full stop. Terminal theme does 100% of the theming. | Near-zero: this is what the framework survey already scoped. | The A1 default-case convention nearly every CLI relies on implicitly. |
| **2. + a `[color]`-style TOML table, git-config value grammar, per-semantic-slot override** | Global-only config knob (like folding, REQUIREMENTS.md §4.2), one key per CONTEXT.md state/severity, value parsed with a small owned parser copying git's grammar (A2): color name / 0-255 / hex + attributes + `no`-negation + `reset`. | Small, bounded: ~20-40 lines to parse, ~10-15 keys to define. No new dependency — reuses fatih/color's `color.New(Attribute...)` as the parser's output type. | git `color.*` (A2, the closest CLI-config precedent that exists); lazygit's YAML-list variant of the same grammar (B3.2) if TOML arrays read better than a git-style string in this codebase's config style. |
| **3. + full theme files (shareable named presets, or hex-palette skins)** | A theme-*file* format (base16-style palette, or k9s/delta-style named bundles) users can swap wholesale. | Real: a file format to design, load, validate, document, and a "which 12 hex slots are canonical" design question with no dstow-specific answer yet. | base16 (A4), k9s skins (B3.3), delta themes.gitconfig (B3.4/A4) — all found in TUI or document-rendering tools scaling to dozens of visual regions; **none is a precedent for a status-line tool with ~10-15 slots.** |

### Recommendation

**Rung 2.** Rung 1 alone under-serves a CLI whose whole differentiated value
is legible status output (stowed/drifted/occupied/damaged... — CONTEXT.md's
entire vocabulary exists to be *seen*), and REQUIREMENTS.md §4's four-level
config chain already normalizes "one more global-only knob." But rung 3 is
solving a problem dstow's output shape doesn't have: dstow has on the order
of 10-15 semantic slots (package states + severities), not the dozens of
regions that make k9s/lazygit reach for full skin files, and it renders no
documents (no markdown, no diffs, no syntax) that would justify a
glamour/bat/delta-style theme file. **Rung 2, sized to CONTEXT.md's
vocabulary, is where the precedent (git) and the actual UI surface (a
handful of named slots) meet.**

### Does any rung change the fatih/color choice?

**No — fatih/color survives at every rung, including rung 2.** The thing
rung 2 needs that fatih/color doesn't hand you (a style-string parser,
C.2) is small, bounded, and owned-printer-shaped — exactly the kind of
"one owned printer absorbs it" seam the framework survey already
identified for the NO_COLOR/CLICOLOR precedence logic. termenv's
architectural cleanliness (per-writer `Output`) is irrelevant here — parsing
a config string into attributes has nothing to do with per-stream
degradation, which is the axis termenv actually wins on and dstow doesn't
need (dstow stays ANSI-16, A1). Reaching for termenv or gookit/color to get
a style parser would be importing a library for a feature neither provides
natively anyway (neither parses git's grammar either) — the honest choice is
a small hand-written parser behind the existing fatih/color seam, matching
the framework survey's "libraries over hand-rolling when good ones exist"
principle by *not* pretending one exists here that doesn't.

---

## Open questions handed to the Output design ticket

1. **Is user-customizable theming in v1 scope at all?** This survey maps
   the territory (recommends rung 2 *if* scope includes it) but scope is
   the ticket's call, not this survey's, per the brief.
2. If in scope: **exact TOML shape** — one `[color]` table with keys named
   after CONTEXT.md states/severities directly (`stowed`, `drifted`,
   `occupied`, `damaged`, `error`, `warning`, `hint`...), or a nested table
   separating "state colors" from "severity colors"?
3. **Value syntax**: adopt git's space-separated string grammar verbatim
   (`"bold red"`), or an array-of-strings (lazygit's `[red, bold]`) to fit
   TOML idiom better than a mini-DSL-in-a-string? Either is equally cheap to
   parse; this is a style-consistency call for the config-format survey's
   existing TOML conventions, not a capability question.
4. **Precedence with `--color=never`/`NO_COLOR`**: confirm user theme
   overrides never re-enable color when the precedence chain says off —
   theme config should be "which color IF color is on," strictly downstream
   of the already-resolved enable/disable decision, never a separate
   override path.
5. **Accessible-colors rung**: does dstow want gh's explicit
   `Accessible`-flag pattern (B3.1) as a documented promise ("dstow only
   ever emits the 16 base ANSI colors, full stop, so your terminal theme
   always wins") rather than an implementation default that could quietly
   grow 256-color usage later? This is a one-line policy decision with no
   cost either way, worth pinning explicitly given the CONTEXT.md naming
   principle's general spirit (be deliberate, don't let convenience decide).
6. **Named/shareable presets (rung 3)**: explicitly out of v1 per this
   survey's recommendation — flag as a place design may "keep the doorway
   open" (REQUIREMENTS.md §11 language) by choosing per-slot key names now
   that a future preset file could simply set in bulk, without designing
   the preset file itself.
