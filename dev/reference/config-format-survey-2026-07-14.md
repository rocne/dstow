# Config format & parsing library — survey (2026-07-14)

**Question.** Which configuration file format and Go parsing library should
dstow use for its config chain (REQUIREMENTS §4)? Recommendation with a clear
runner-up. All maintenance data below checked against the live repos on
2026-07-14 via the GitHub API.

**Binding constraints (REQUIREMENTS §4, CONTEXT.md vocabulary):**

- Four-level chain **package → repo → global → built-in**; nearer level wins
  *per knob*, ignore patterns compose **additively** across levels (§4.1). The
  format must make per-knob presence unambiguous — the merge needs to know
  "unset at this level" vs "set to the zero value".
- Knob shapes the format must express: scalars (target, dot-translation,
  exclude-from-bulk, folding), string lists (ignore patterns, persistent
  repos), and **structured records** — global dependency declarations carry
  *names* (a list) plus an optional *hint* (§4.2, §9.2).
- Global config lives in an XDG location; the ledger lives in XDG *state*
  (§8.1, CONTEXT "Ledger").
- **Stow compat is a strict superset** (§4.3): out-of-the-box on a stow setup;
  *renaming a `.stowrc` to the native config must just work*; supplement mode
  means `.stowrc` and native config coexist at a level, dstow winning per-knob
  conflicts with a loud warning.
- Philosophy: tried-and-true actively-maintained libraries over hand-rolling;
  single ownership; lean; v1 is complete — no stubs.

---

## 1. The `.stowrc` compat crux

### What `.stowrc` actually is (primary sources)

GNU stow manual, *Resource Files* node
([gnu.org](https://www.gnu.org/software/stow/manual/html_node/Resource-Files.html);
texi source: [doc/stow.texi](https://github.com/aspiers/stow/blob/master/doc/stow.texi)):

- "Default command line options may be set in `.stowrc` (current directory)
  or `~/.stowrc` (home directory)… appended together if they both exist. The
  effect… is similar to simply prepending the options to the command line."
- Contents are **CLI option lines**, e.g. `--dir=/usr/local/stow2`,
  `--target=/usr/local`, `--ignore='~'`, `--ignore='^CVS'` (manual's own
  examples).
- "For options that take a file path, environment variables and the tilde
  character (`~`) are expanded" — `$VAR` or `${VAR}` form; escape `$`/`~` with
  backslash; values are "first subject to standard shell quoting rules".
- `-D`, `-S`, `-R` and any package names in a resource file are **ignored**.

The implementation
([bin/stow.in](https://github.com/aspiers/stow/blob/master/bin/stow.in),
`get_config_file_options`) pins the details:

- Each line is tokenized with **`Text::ParseWords::shellwords`** — shell-style
  quoting, multiple options per line are legal.
- The token stream feeds the **same Getopt::Long parser as the CLI**.
- Env/tilde expansion (`expand_filepath`) is applied **only to `--target` and
  `--dir`**, after parsing.
- Merge with the CLI: scalar rc options are overwritten by CLI; list options
  (`--ignore`, `--defer`, `--override`) are appended, rc values first.

### What the ignore files actually are

Manual, *Types And Syntax Of Ignore Lists* node
([gnu.org](https://www.gnu.org/software/stow/manual/html_node/Types-And-Syntax-Of-Ignore-Lists.html);
confirmed in the texi source):

- **Perl regular expressions, one per line**, `#` comments (escapable), blank
  lines stripped, in `.stow-local-ignore` (top of a package) or
  `~/.stow-global-ignore`.
- **Exclusive, not additive**: local *replaces* global *replaces* built-in —
  "In the absence of this package-specific ignore list, Stow will instead use
  the contents of `~/.stow-global-ignore`… If neither… exist, Stow will use
  its own built-in default ignore list."
- Two-set matching: patterns containing `/` are exact-anchored against
  `/`-prefixed subpaths of the package-relative path; patterns without `/`
  are exact-anchored against the basename.

Two consequences beyond format choice: (1) dstow's *additive* chain (§4.1)
deliberately diverges from stow's replacement semantics — the gostow audit
already flags this seam
([gostow-audit-2026-07-12.md](gostow-audit-2026-07-12.md) §4, gostow#36);
(2) these are **Perl** regexes, and Go's `regexp` is RE2 (no lookarounds or
backreferences) — most real-world patterns are RE2-safe, but the compat layer
needs a defined failure mode for those that aren't.

### What "renaming a `.stowrc` must just work" implies

A bare `--no-folding` line is not valid TOML, YAML, INI, or JSON. The three
readings:

| Reading | Mechanics | Trade-offs |
|---|---|---|
| (a) Dual-mode native parser | Native loader tries native format, falls back to flag-line parsing | Mechanically identical to (c); framing implies flag-lines are a *native* dialect forever, blurring "compat layer only" (§4.3 "a dstow user never *needs* to know stow-named artifacts") |
| (b) Native format IS extended getopt-lines | One syntax for everything | Compat is free, but flag lines are flat: no per-level structured knobs, no sane spelling for dependency declarations (names-list + hint per entry, §9.2), no established Go library, no spec to lean on. Fails §4.1's "per-knob values clean enough that merging is unambiguous" the moment a knob isn't a scalar or string-list |
| (c) Content-sniffing → compat path, loudly | Loader inspects the file; flag-line content is routed to the stowrc-compat parser with an announcement, regardless of file name | Keeps native format clean; costs one sniff. The announcement satisfies "surprises are announced" (§1.3) |

**The decisive fact: supplement mode already mandates a full stowrc parser.**
`.stowrc` may coexist with native config at a level (§4.3), so dstow ships a
shellwords-tokenizing, Getopt-style, `$VAR`/`~`-expanding parser for stow's
syntax *no matter what* the native format is. Given that parser exists,
reading (c) is nearly free: sniffing is deterministic — a valid stowrc's
first significant token starts with `-`, which can never begin a top-level
TOML/YAML/INI statement — and "just works" is satisfied by routing, not by
contorting the native grammar. Reading (b) would let the compat tail wag the
format dog and forfeit structured knobs; reading (a) is (c) with worse
messaging.

**Verdict: assume reading (c).** The renamed file parses via the
always-present compat parser; dstow announces loudly that it read stow-syntax
config and names the native equivalent. (a) remains a cosmetic re-framing if
design prefers silence — the survey recommends against silence per §1.3.

---

## 2. Format candidates

| Format | Verdict | Why |
|---|---|---|
| **TOML** ([spec v1.1.0](https://toml.io/en/v1.1.0)) | **Recommend** | Per-knob presence is syntactically obvious (`key = value`, one way to write it); comments; typed scalars/arrays/tables cover every v1 knob incl. dependency records (`[[dependency]]` with `names`/`hint`); INI-familiar to stow veterans; the de-facto Go-tool config format (hugo, chezmoi's examples); two healthy zero-dep libraries |
| **YAML** | **Runner-up** | Expresses everything TOML does; gh CLI and lazygit precedent. Costs: spec complexity and coercion surprises (the `no`→false class of traps) sit badly with "surprises are announced"; ecosystem turbulence — canonical library archived (below) |
| INI ([go-ini/ini](https://github.com/go-ini/ini), v1.67.3 2026-06-08, active) | Rule out | No standard list or nested-record syntax — additive ignores and dependency declarations become library-specific conventions (`delim`-joined strings, `key[]` hacks). The library is fine; the format is too weak for §4.2's knob set |
| Extended getopt-lines | Rule out as *native* format | See §1 reading (b). Survives as the compat layer, which is exactly what §4.3 calls it |
| JSON (stdlib) | Rule out | No comments — disqualifying for hand-edited dotfile config |
| JSON5 / JSONC | Rule out | Comments fixed, but niche parser ecosystem in Go, no convention advantage over TOML |
| HCL ([hashicorp/hcl](https://github.com/hashicorp/hcl)) | Rule out | Heavy, ecosystem-coupled to Terraform idioms; expressive power (blocks, expressions) dstow doesn't need |
| CUE ([cue-lang/cue](https://github.com/cue-lang/cue)) | Rule out | Brings its own unification/merge semantics that would fight dstow's nearest-wins-plus-additive-ignores chain; large dependency for a four-file config |

## 3. Library comparison — TOML

| | [BurntSushi/toml](https://github.com/BurntSushi/toml) | [pelletier/go-toml/v2](https://github.com/pelletier/go-toml) |
|---|---|---|
| Latest release | v1.6.0, 2025-12-18 | v2.4.3, 2026-07-05 |
| Repo activity | pushed 2026-06-27; 27 open issues; ~5.0k stars | pushed 2026-07-11; 27 open issues; ~2.0k stars |
| TOML version | v1.1.0 (README) | v1.1.0 (README) |
| Dependencies | **zero** (go.mod: module line only) | **zero** (go.mod: module line only) |
| Unknown keys | `MetaData.Undecoded()` returns them; caller decides ([meta.go](https://github.com/BurntSushi/toml/blob/master/meta.go)) | Strict mode: `Decoder.DisallowUnknownFields()` → hard error (README "Strict mode") |
| Presence detection | `MetaData.IsDefined(key...)` ([meta.go](https://github.com/BurntSushi/toml/blob/master/meta.go)) — no pointer-field gymnastics needed | Pointer fields on the target struct (idiomatic, works fine) |
| Error quality | `ParseError` with position | `DecodeError`, "human readable contextualized" with position (README) |

Both are healthy, spec-current, zero-dependency, and tried-and-true. The
domain tips it:

- **Per-knob presence is the merge's core question** (§4.1), and
  `MetaData.IsDefined` answers it directly, per level, without decorating
  every knob struct with pointers.
- **Unknown keys should warn loudly and continue**, not hard-error: dstow
  config is a *superset* story with forward-compat expectations, and §1.3
  prefers announcement over refusal. `Undecoded()` is exactly
  warn-and-continue; pelletier's strict mode is all-or-nothing.

**Verdict: BurntSushi/toml**, with pelletier/go-toml/v2 a fully adequate
library runner-up (choose it if design ends up preferring hard errors on
unknown keys and pointer-field structs).

### YAML library note (runner-up format)

- [go-yaml/yaml](https://github.com/go-yaml/yaml) (`gopkg.in/yaml.v3`):
  **archived 2025-04-01**; README opens "THIS PROJECT IS UNMAINTAINED"
  (Niemeyer). 422 issues frozen open. Disqualified by the no-near-sunset rule.
- [goccy/go-yaml](https://github.com/goccy/go-yaml): active successor in
  practice — v1.19.2 (2026-01-08), pushed 2026-04, zero-dep go.mod; 184 open
  issues (busy, not sick).
- [yaml/go-yaml](https://github.com/yaml/go-yaml): the YAML org's stewardship
  fork of go-yaml — active (pushed 2026-07-12) but **no tagged GitHub release
  as of 2026-07-14**; not yet a v1 foundation.

If YAML were chosen: **goccy/go-yaml**.

## 4. Frameworks vs plain parser

- **viper** ([spf13/viper](https://github.com/spf13/viper), v1.21.0
  2025-09-08): rule out. Force-lowercases keys, breaking format specs
  ([viper#635](https://github.com/spf13/viper/pull/635)); pulls every
  format's dependencies into the core
  ([viper#707](https://github.com/spf13/viper/issues/707)); couples parsing
  to file extensions; criticisms cataloged in
  [koanf's README](https://github.com/knadh/koanf#alternative-to-viper).
- **koanf** ([knadh/koanf](https://github.com/knadh/koanf), v2.3.5
  2026-05-30, 8 open issues, modular providers): a good library, wrong
  problem. Its value is generic multi-source map merging — but dstow's merge
  *is the domain logic*: nearest-wins per knob **except** additive ignores,
  stowrc supplement conflicts warned per-knob, fold-contradiction refusal
  (§3.3). A generic deep-merge must be overridden precisely where dstow is
  most particular, leaving koanf as indirection around one TOML call.

**Verdict: plain parser + hand-wired four-level merge.** This is not
hand-rolling a parser (the library does the parsing); the merge is ~one small
function dstow owns either way — single ownership, no framework seams.

## 5. XDG placement

- `os.UserConfigDir` (stdlib) covers `$XDG_CONFIG_HOME` → `~/.config`
  fallback, but the stdlib offers **no state-dir API** (`go doc os`:
  only `UserCacheDir`, `UserConfigDir`, `UserHomeDir`) — and the ledger is
  XDG *state* (§8.1).
- [adrg/xdg](https://github.com/adrg/xdg) (v0.5.3 2024-10-31; repo pushed
  2026-07-09; 9 open issues): full Base Directory spec — `xdg.ConfigHome`,
  `xdg.StateHome`, `xdg.ConfigDirs` search, `SearchConfigFile` helpers. The
  old release date reflects a finished spec implementation, not abandonment.

**Verdict: adrg/xdg** — one small, conventional dependency; the ledger's
`XDG_STATE_HOME` requirement alone rules out stdlib-only.

## 6. Prior art

| Tool | Format | Source |
|---|---|---|
| chezmoi | Multi-format: `chezmoi.{json,jsonc,toml,yaml}` in XDG config; errors if several present | [docs/reference/configuration-file](https://github.com/twpayne/chezmoi/blob/master/assets/chezmoi.io/docs/reference/configuration-file/index.md) |
| hugo | `hugo.toml` / `.yaml` / `.json`, **TOML first in precedence** | [configuration/introduction](https://github.com/gohugoio/hugo/blob/master/docs/content/en/configuration/introduction.md) |
| gh CLI | YAML `~/.config/gh/config.yml` (internal `yamlmap`) | [cli/go-gh pkg/config](https://github.com/cli/go-gh/blob/trunk/pkg/config/config.go) |
| lazygit | YAML `~/.config/lazygit/config.yml` + repo-level overrides | [docs/Config.md](https://github.com/jesseduffield/lazygit/blob/master/docs/Config.md) |

Pattern: single-file config in XDG; TOML and YAML split the field; the
dotfile-manager closest to dstow (chezmoi) treats TOML as its documented
default example.

---

## Recommendation

**Format: TOML (spec v1.1.0). Library: BurntSushi/toml. XDG: adrg/xdg.
Merge: hand-wired over the plain parser (no framework).**

Assumed `.stowrc`-compat reading: **(c)** — the config loader content-sniffs;
a renamed `.stowrc` (flag-line content) is routed to the stowrc-compat parser
that supplement mode (§4.3) forces dstow to ship anyway, with a loud
announcement naming the native equivalent. The native TOML grammar stays
untouched by compat concerns, and "just works" is satisfied by routing.

**Runner-up: YAML via goccy/go-yaml**, same compat reading, same hand-wired
merge — viable on precedent (gh, lazygit), docked for spec-surprise surface
and the go-yaml archival turbulence.

## Open questions for the "Config schema" design ticket

1. **Native file names and per-level placement** — global file name under
   `$XDG_CONFIG_HOME/dstow/`; the repo/package-level file names; the
   dstow-native ignore-file name mirroring `.stow-local-ignore` (§4.3
   "native mirrors").
2. **stowrc option mapping table** — `--target`→target, `--no-folding`→fold
   off (repo-level: honored unless contradictory, §3.3),
   `--dotfiles`→dot-translation, `--ignore`→additive ignores; what does
   `--dir` map to (a persistent-repo entry?); and the policy for unmappable
   options (`--override`, `--defer`, `--adopt`, verbosity): warn-and-ignore
   vs refuse.
3. **Shellwords + expansion in Go** — pick the tokenizer for the compat
   parser (e.g. a maintained shellwords port) and reimplement stow's
   `$VAR`/`${VAR}`/`~` expansion incl. backslash escapes, scoped to
   path-taking options only (stow expands only `--target`/`--dir`).
4. **Perl-regex vs RE2** for compat ignore files: defined failure mode when a
   `.stow-local-ignore` pattern is not RE2-compatible (loud skip vs refuse).
5. **Native ignore-pattern semantics** — dstow must pick ONE user-facing
   pattern language across the chain; gostow's two channels anchor patterns
   differently (audit §4, gostow#36) and stow's ignore *files* are
   replacement-semantics, native chain is additive.
6. **Unknown-key policy** — warn-and-continue via `MetaData.Undecoded()`
   (recommended, §1.3) vs strict; and whether `IsDefined` or pointer fields
   express per-level presence in the knob structs.
7. **Supplement-mode conflict detection** — per-knob comparison surface
   between the parsed stowrc option set and the native level's knob set, and
   the exact loud-warning wording (§4.3).
