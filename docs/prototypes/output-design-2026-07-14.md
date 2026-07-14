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
   `warning:`, `error:`, `fix:` — colored, but meaningful uncolored.
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
| fix: | green | the remedy is the good news |
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
⟨green⟩fix:⟨/⟩ dstow adopt ~/.zshrc dotfiles::zsh
```

### `dstow stow tmux vim work/dots::zsh` (mixed run)

```
stow ⟨bold⟩tmux⟨/⟩ ⟨green⟩linked 5⟨/⟩
stow ⟨bold⟩vim⟨/⟩ ⟨dim⟩no-op (already stowed)⟨/⟩
stow ⟨bold⟩work/dots::zsh⟨/⟩ ⟨bold-red⟩failed⟨/⟩ ⟨magenta⟩occupied⟨/⟩
  ~/.zshrc is a real file, not dstow's
  ⟨green⟩fix:⟨/⟩ dstow adopt ~/.zshrc work/dots::zsh — or re-run with --adopt

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
⟨green⟩fix:⟨/⟩ qualify it: dstow stow dots::zsh
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
⟨green⟩fix:⟨/⟩ folding is a global setting: [folding] in ~/.config/dstow/config.toml
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

**O2 — severity prefixes `note:` / `warning:` / `error:` + the `fix:` line.**
`fix:` is the §1.4 remedy made structural: every refusal is followed by a
green `fix:` line containing a *runnable command or config pointer*.
Greppable, screen-reader-honest, colored second.

**O3 — the semantic palette as tabled above** (ANSI-16 slots only).

**O4 — ANSI-16 is a documented promise, not just a default** (gh
`Accessible` pattern per the theming survey): "dstow only ever emits the
16 base colors; your terminal theme always wins."

**O5 — theming ships in v1 at rung 2**: a global-only `[color]` TOML table,
one key per palette slot, value in git's `color.*` string grammar
(`damaged = "bold red"`). Strictly downstream of the enable/disable chain —
theme config can never re-enable color that `--color=never`/`NO_COLOR`
turned off.

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
