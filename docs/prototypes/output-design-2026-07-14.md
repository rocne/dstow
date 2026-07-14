# PROTOTYPE — dstow output design (mockups to react to)

**Throwaway artifact** for [Output design](https://github.com/rocne/dstow/issues/26).
React to the **Decision ledger** (O1–O12) at the bottom. Colors can't render
in markdown — run the sibling script to see every mockup with real ANSI in
your terminal:

```
bash docs/prototypes/output-demo-2026-07-14.sh      # on this branch
```

Color annotations here use `⟨color⟩text⟨/⟩` notation. All colors are the
16 ANSI slots only (your terminal theme owns the actual appearance).

---

## The philosophy (three rules everything follows)

1. **stdout is data, stderr is commentary.** The thing you asked for —
   listings, status, JSON, snippets — goes to stdout, pure. Everything
   dstow *says about* the work — notes, warnings, errors, prompts,
   progress — goes to stderr. `dstow snippet rc >> ~/.bashrc` and
   `dstow status --json | jq` are always clean by construction.
2. **Severity is a word, color is a reinforcement.** Every commentary line
   starts with a greppable, screen-reader-friendly prefix — `note:`,
   `warning:`, `error:`, `fix:` — colored, but meaningful uncolored, and
   **padded to a fixed width** (that of `warning:`) so stacked commentary
   aligns like check's `broken`/`orphaned` columns.
   The printer's test contract: `strip_ansi(styled) == plain` (gostow's
   invariant, adopted).
3. **Semantic slots, not colors, in the code.** One owned printer maps
   CONTEXT.md vocabulary to styles; nothing outside it ever names a color.

## The semantic palette (ANSI-16)

| Slot | Style | Rationale |
|---|---|---|
| stowed | green | good/done (universal) |
| partially stowed | yellow | attention, not failure |
| not stowed | dim | neutral absence |
| occupied | magenta | foreign-but-neutral (no blame, per glossary) |
| damaged | bold red | ledger-attested breakage — the alarm color |
| drifted (marker) | cyan | informational deviation |
| broken | red | dead link |
| orphaned | yellow | unowned, like partial: attention |
| note: | cyan | calm information |
| warning: | yellow | — |
| error: | bold red | — |
| fix: | blue | actionable hint — deliberately *not* green (green means success, and a fix line appears precisely when nothing succeeded) |
| names/FQNs in prose | bold | scannability |
| repo header | bold; source dimmed | — |

---

## Flow mockups

### `dstow status` (the flagship view)

```
⟨bold⟩rocne/dotfiles⟨/⟩ ⟨dim⟩github · up to date⟨/⟩
  zsh      ⟨green⟩stowed⟨/⟩
  git      ⟨green⟩stowed⟨/⟩ ⟨cyan⟩~ drifted⟨/⟩
  tmux     ⟨yellow⟩partially stowed⟨/⟩ ⟨dim⟩(3/5 links)⟨/⟩
  vim      ⟨dim⟩not stowed⟨/⟩

⟨bold⟩work/dots⟨/⟩ ⟨dim⟩local:~/work/dots · —⟨/⟩
  zsh      ⟨magenta⟩occupied⟨/⟩ ⟨dim⟩(1 path)⟨/⟩
  ssh      ⟨bold-red⟩damaged⟨/⟩ ⟨dim⟩(~/.ssh/config: ledgered link replaced)⟨/⟩
```

Repo header: bold name, dimmed source + sync state (`up to date`,
`behind 3`, `ahead 1`, `—` for local). Package lines: name column padded,
colored state word, dimmed parenthetical detail. Names shown as
shortest-unique suffix; FQN on ties (O9).

### `dstow status ~/.zshrc` (per-path view)

```
⟨bold⟩~/.zshrc⟨/⟩
  occupant:  real file ⟨dim⟩(1.2 KiB, modified 2026-07-12)⟨/⟩
  ledger:    not dstow's ⟨dim⟩(no entry)⟨/⟩
  adoptable by:
    1) ⟨bold⟩rocne/dotfiles::zsh⟨/⟩   ⟨dim⟩owns 11 neighboring paths⟨/⟩
    2) ⟨bold⟩work/dots::zsh⟨/⟩        ⟨dim⟩owns 2⟨/⟩
⟨blue⟩fix:⟨/⟩ dstow adopt ~/.zshrc dotfiles::zsh
```

### `dstow stow tmux vim work/dots::zsh` (mixed run)

```
stow ⟨bold⟩tmux⟨/⟩ ⟨green⟩linked 5⟨/⟩
stow ⟨bold⟩vim⟨/⟩ ⟨dim⟩no-op (already stowed)⟨/⟩
stow ⟨bold⟩work/dots::zsh⟨/⟩ ⟨bold-red⟩failed⟨/⟩ ⟨magenta⟩occupied⟨/⟩
  ~/.zshrc is a real file, not dstow's
  ⟨blue⟩fix:⟨/⟩ dstow adopt ~/.zshrc work/dots::zsh — or re-run with --adopt

⟨bold⟩1 stowed⟨/⟩, 1 no-op, ⟨bold-red⟩1 failed⟨/⟩
```

One line per package (§3.2), verb echoed, result colored; failures get
indented detail + `fix:` immediately below their line; summary last;
exit 1. With `--quiet`: no-op lines and the all-good summary vanish;
failures and their fixes never do.

### Bulk confirm (bare `dstow stow`, interactive)

```
Stow all ⟨bold⟩12⟨/⟩ packages? zsh git tmux vim ⟨dim⟩… 8 more⟨/⟩  [y/N]
```

First 4 names + dimmed count. Default No (capital N) — walking away
does nothing. Non-interactive: `error:` naming `--all`.

### Ambiguity choice (interactive) and error (script)

```
⟨bold⟩zsh⟨/⟩ matches 2 packages:
  1) rocne/dotfiles::zsh
  2) work/dots::zsh
Stow which? [1/2/q]
```

```
⟨bold-red⟩error:⟨/⟩ ⟨bold⟩zsh⟨/⟩ is ambiguous — matches rocne/dotfiles::zsh, work/dots::zsh
⟨blue⟩fix:⟨/⟩ qualify it: dstow stow dots::zsh
```

### Announcements (§1.3)

```
⟨cyan⟩note:⟨/⟩ created target directory ~/.config/foo
⟨cyan⟩note:⟨/⟩ folding is off (dstow's default: predictable one-link-per-file).
      To fold like classic stow: [folding] in ~/.config/dstow/config.toml
```

### D12 knob refusal

```
⟨bold-red⟩error:⟨/⟩ --no-folding is not a dstow flag
⟨blue⟩fix:⟨/⟩ folding is a global setting: [folding] in ~/.config/dstow/config.toml
      (renamed .stowrc fold flags are honored — see docs on stow compat)
```

### `repo add` with encoding (continue-affirmative)

```
⟨cyan⟩note:⟨/⟩ this path contains ':' and will be percent-encoded in names:
      local:/data/weird%3A%3Adir/dots
Register it with this name? [Y/n]  ⟨dim⟩(n cancels — rename the directory, then re-add)⟨/⟩
```

### `dstow check`

```
⟨red⟩broken⟨/⟩    ~/.config/old/rc → ⟨dim⟩(gone) rocne/dotfiles/old/rc⟨/⟩
⟨yellow⟩orphaned⟨/⟩  ~/.vimrc → rocne/dotfiles/vim/vimrc ⟨dim⟩(ignored since 2026-07-01 config)⟨/⟩

⟨bold⟩1 broken⟨/⟩ (clean removes freely), ⟨bold⟩1 orphaned⟨/⟩ (clean will ask)
```

## JSON shapes (per-command `--json`, all stdout-pure)

Stable lower-snake keys; states exactly as CONTEXT.md spells them;
booleans over string-enums where binary; FQNs always included.

```json
// dstow status --json
{"repos":[{"fqn":"github:rocne/dotfiles","scheme":"github","sync":{"behind":0,"ahead":0},
  "packages":[{"name":"zsh","fqn":"github:rocne/dotfiles::zsh","state":"stowed","drifted":false},
              {"name":"tmux","fqn":"github:rocne/dotfiles::tmux","state":"partially stowed",
               "links_present":3,"links_expected":5}]}]}
```

```json
// dstow check --json
{"broken":[{"path":"~/.config/old/rc","destination":"..."}],
 "orphaned":[{"path":"~/.vimrc","destination":"...","reason":"ignored-by-config"}]}
```

```json
// dstow info zsh --json   (fields per the info ticket; shape reserved here)
{"fqn":"github:rocne/dotfiles::zsh","identity":{"repo":"github:rocne/dotfiles","scheme":"github"},
 "configuration":{"target":"~","dot_translation":true,"dependencies":[{"names":["zsh"],"hint":null}]}}
```

---

## Decision ledger — react by number

**O1 — stdout=data / stderr=commentary, absolute.** Prompts included on
stderr. This is what makes `--json` and `snippet` composition-safe.

**O2 — severity prefixes `note:` / `warning:` / `error:` + the `fix:` line,
fixed-width aligned.** `fix:` is the §1.4 remedy made structural: every
refusal is followed by a blue `fix:` line containing a *runnable command or
config pointer* (blue, not green — green means success). All prefixes pad
to `warning:`'s width so stacked commentary aligns. *(Padding is
provisional by the human's own note — accepted to try, revisit at
implementation if it looks weird in practice.)* The structural,
machine-stable `fix:` line is also what makes a future fix-runner
(`dstow fix` re-running the last suggestion — recorded maybe-v2 on the
map) cheap to add later.

**O3 — the semantic palette as tabled above** (ANSI-16 slots only).

**O4 — ANSI-16 is a documented promise about dstow's own palette**: dstow's
*defaults* only ever emit the 16 base ANSI slots, so your terminal theme
(Catppuccin included — it ships palettes for every major emulator)
rethemes dstow automatically, and colorblind/low-vision users retheme
through terminal preferences instead of fighting a hardcoded palette.
(That is the "gh Accessible pattern": gh added a mode restricting output
to the 16 slots for exactly this reason; dstow makes it the only mode its
defaults use.)

**O5 — theming ships in v1: theme files + per-slot overrides, one
mechanism (the dircolors architecture).** Layered, top wins, all strictly
downstream of the enable/disable chain:

1. `DSTOW_COLORS` — the ONLY theming env var: packed per-slot overrides,
   LS_COLORS-family convention (`damaged=bold red:stowed=#a6e3a1`), values
   in git's `color.*` grammar. Populated by hand or by generator:
   `export DSTOW_COLORS=$(dstow colors theme catppuccin-mocha)` — the
   `eval $(dircolors)` pattern. There is no `DSTOW_THEME`.
2. `[color]` TOML table — global config, one key per palette slot, same
   grammar (`damaged = "bold red"`).
3. `theme` config key — **name or path, per the operand rule**: a bare
   string is a theme *name* (user themes dir (XDG) first, then the
   bundled presets — themselves TOML files embedded via `go:embed`,
   served by the same loader); a path form (`/`, `~/`, `./`) is a theme
   *file*, Unix-resolvable, living anywhere — including inside a repo,
   which makes repo-shipped themes free. Dropping a file in the themes
   dir creates a named theme; presets and extendability are one
   mechanism.
4. The default ANSI-16 semantic palette (O4's promise).

**North-star invariants (bound now so the rest stays reachable):** a
theme file is exactly the bare `[color]` schema, no wrapper keys; the
packed string, the config table, and theme files share one slot
vocabulary (CONTEXT.md states + severities) and one value grammar —
losslessly convertible between representations, forever. v1 command:
`dstow colors theme <name>` (emit the packed string; required, since it
is the only session-theming path). Recorded v2: the converter/builder
family (`--from-file`, `file --from-env`, per-slot builder flags);
first-class repo-shipped themes (observation: a plain package targeting
the themes dir already ships themes with zero new mechanism — dstow
theming itself via dstow). Grammar allows 0–255/`#RRGGBB`, so user
choices may exceed ANSI-16 — O4 binds dstow's defaults only. Theme
config can never re-enable color the `--color`/`NO_COLOR` chain turned
off.

**O6 — `--color <when>` requires a value** (auto/always/never; auto
default). No bare `--color`, no `--no-color` sugar — NO_COLOR covers the
env side.

**O7 — `--quiet` drops routine chatter only**: no-op lines, all-good
summaries, progress. §1.3 announcements, warnings, errors, and `fix:`
lines always survive. (Quiet mutes small talk, never surprises.)

**O8 — per-package run lines**: `verb name result` one-liners, failure
detail indented under its line, summary line last, exit per §3.2.

**O9 — name display rule**: shortest-unique suffix everywhere by default;
full FQN whenever showing a tie, an error about ambiguity, or `--json`
(which always carries FQNs). Local coordinates display with `~`
abbreviation.

**O10 — JSON conventions**: per-command shapes as sketched; lower_snake
keys; CONTEXT.md state strings verbatim (including the space in
`"partially stowed"` — glossary spelling beats key-friendliness);
FQN always present.

**O11 — strip-roundtrip is the printer's test contract**
(`strip_ansi(styled) == plain`), inherited from gostow's approach.

**O12 — confirm polarity**: destructive/bulk defaults No (`[y/N]`);
benign-continue defaults Yes (`[Y/n]`, e.g. the encoding confirm).

## Handoffs

- **info ticket**: the `info --json` field shape (sketched here, owned
  there); whether `"partially stowed"` state strings appear in dep-query
  contexts.
- **Config schema**: the `[color]` table lands in the global-config schema
  (keys = palette slots).
- **Go architecture**: printer package seam (already queued there),
  exit-code map.
