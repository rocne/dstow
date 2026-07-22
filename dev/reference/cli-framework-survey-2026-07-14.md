# CLI framework & color/output library — survey (2026-07-14)

**Question.** (1) Which CLI framework should dstow use — cobra, kong,
urfave/cli v3, or stdlib `flag`? cobra enters with burden-of-displacement
(the human's other CLIs use it), nothing more. (2) Which terminal
color/output stack, given that colorized output is high-priority UX?
One recommendation each, with runner-up. All maintenance data checked
against the live repos on 2026-07-14 via the GitHub API.

**Binding constraints (REQUIREMENTS.md, CONTEXT.md vocabulary):**

- **§1.2** — ambiguity confirms interactively, hard-errors non-interactively:
  interactivity detection (stdin/stdout TTY) must be clean and testable.
- **§1.4** — every refusal names its remedy: error/help text quality is a
  requirement, not polish.
- **§1.7** — one output style (unified I/O); machine-consumable output where
  specified; sane exit codes.
- **§3.2** — per-package status lines; overall exit nonzero if any package
  failed.
- **§7** — list/status views; **§9.2.4** — machine-consumable dependency
  query; **§10** — bootstrap-snippet prints POSIX sh to *stdout*: stdout
  discipline is load-bearing.
- **§11** — extraction of shared output/color plumbing (i.e. reusing gostow's
  `internal/ui`) is explicitly out of v1 scope; dstow picks its own stack.
- Command surface: ~15 subcommands (stow/unstow/restow, repo add/list/remove,
  fetch/apply pair, list, status, check/clean/rebuild, adopt, dependency
  query, bootstrap-snippet), non-interactive and force flag forms, shell
  completions high-value for a daily driver.
- Sibling survey ([config-format-survey-2026-07-14.md](config-format-survey-2026-07-14.md))
  already ruled out viper and chose BurntSushi/toml + adrg/xdg + hand-wired
  merge. This survey must not reintroduce viper through cobra.

---

## 1. Local precedent (verified on disk, 2026-07-14)

| Repo | Framework | Color | Evidence |
|---|---|---|---|
| **gostow** (`~/git/gostow`) | **Hand-rolled getopt**, not stdlib `flag` — `internal/getopt` clones Getopt::Long for byte-level GNU Stow conformance (`internal/cli/options.go` imports `github.com/rocne/gostow/internal/getopt`) | Hand-rolled SGR paint-pass writer (`internal/ui/color.go`): color is a rendering pass over finished lines; `StripANSI(paint(s)) == s` is the tested parity invariant; `Enabled()` checks `NO_COLOR`, `TERM=dumb`, and `os.ModeCharDevice` | zero-dep `go.mod`; `cli.Run(os.Args, version, os.Stdout, os.Stderr)` injects both streams |
| **dot-dagger** (`~/git/rocne/dot-dagger`, builds the **dotd** binary — `cmd/dotd/main.go`) | **cobra** v1.10.2 | **fatih/color** v1.19.0 behind a semantic wrapper package (`internal/ui/ui.go`): `Warnf/Errf/Hintf/OKf` + named style funcs over eight `color.New` instances; lipgloss/huh only in prompt/log paths | `go.mod`; `internal/ui/ui.go` |
| **hud** (`~/git/rocne/hud`) | **cobra** v1.10.2 | Hand-rolled ANSI (`internal/ansi/ansi.go` — "single owner of raw ANSI escape-sequence construction"); `NO_COLOR`/`TERM=dumb` checked in `internal/cli/style.go`. hud is a screen-addressing HUD (DECSC/DECRC, cursor control) — raw escapes are its domain, not a styling-library rejection | `go.mod`, source |
| **fable-surprise** (`~/git/rocne/fable-surprise`, module `tine`) | **cobra** v1.10.2 | lipgloss v1.1.0 + termenv v0.16.0 (Charm huh prompts) | `go.mod` |
| **sorta** (`~/git/rocne/sorta`) | stdlib `flag` (all three `cmd/*/main.go`) | none direct | source |

**The cobra precedent is real and pinned: dot-dagger, hud, and
fable-surprise all use cobra v1.10.2.** gostow's hand-rolled getopt is a
conformance artifact (it must parse exactly like Perl's Getopt::Long), not a
framework opinion, and sorta's stdlib `flag` serves single-verb binaries —
neither transfers to a 15-subcommand CLI.

### The dotd verdict

**dotd's color plumbing is LIBRARY-BASED, not hand-rolled** — fatih/color
behind a single-ownership semantic seam (`internal/ui`), exactly the "one
owned printer" shape dstow wants. The standing rule (2026-07-14: weight dotd
low *if* it hand-rolled color) therefore does **not** trigger:
**dotd's precedent carries full weight**, and what it models — cobra +
fatih/color + one semantic ui package owning every style decision — is a
directly reusable pattern.

---

## 2. CLI framework candidates

Maintenance snapshot (GitHub API, 2026-07-14):

| | [spf13/cobra](https://github.com/spf13/cobra) | [alecthomas/kong](https://github.com/alecthomas/kong) | [urfave/cli](https://github.com/urfave/cli) v3 | stdlib `flag` |
|---|---|---|---|---|
| Latest release | v1.10.2, 2025-12-04 | v1.15.0 (tag, 2026-04-01; no GitHub releases) | v3.10.1, 2026-06-29 | Go stdlib |
| Activity | pushed 2026-07-11; ~44.3k stars; 386 open issues | pushed 2026-07-13; ~3.1k stars; 34 open issues | pushed 2026-07-03; ~24.2k stars; 75 open issues | — |
| Bus factor | Multi-maintainer org | Effectively one: alecthomas 271 commits, next human 14 (contributors API) | Team-maintained org | — |
| Runtime deps (go.mod) | spf13/pflag, inconshreveable/mousetrap (go-md2man + yaml only for doc-gen packages) | **zero** (alecthomas/assert + repr are test-only requires) | **zero** (testify test-only) | zero |
| Subcommands | Native, nested | Native (struct tags: `cmd`) | Native, nested | none — hand-roll dispatch |
| Shell completions | **Built-in: bash, zsh, fish, powershell** ([README](https://github.com/spf13/cobra/blob/main/README.md)) | **None built-in** (README silent); third-party [willabides/kongplete](https://github.com/willabides/kongplete) last pushed **2024-07-15** (41 stars) | **Built-in, dynamic at runtime**: bash, zsh, fish, powershell via `EnableShellCompletion` + `completion` subcommand ([docs](https://cli.urfave.org/v3/examples/completions/shell-completions/)) | none |
| Help/UX extras | Auto help, command suggestions ("did you mean"), aliases, man-page gen, customizable templates (README) | Help generated from struct grammar, `help:""` tags, hooks (`BeforeApply` etc.), variable interpolation (README) | Auto help, templates | hand-roll |
| Viper coupling | **None.** Viper is "Optional seamless integration" (README); cobra's go.mod does not require viper. **Verified: cobra works fine without viper** — dot-dagger/hud/fable-surprise all use cobra with zero viper in their module graphs | n/a | n/a | n/a |

Verdicts:

- **cobra — recommend.** Everything §1.4 and the daily-driver lens want is
  built in and battle-tested: 4-shell completions, quality generated help,
  suggestions, nested subcommands for `repo add/list/remove`. The cost is two
  small runtime deps (pflag, mousetrap) — the only candidate with any — which
  is cheap against three in-house repos of precedent and the largest
  ecosystem in Go. The viper trap simply doesn't exist: config stays
  BurntSushi/toml + hand-wired merge per the sibling survey.
- **urfave/cli v3 — strong runner-up.** Zero runtime deps, active v3 line
  (stable releases since v3.1.0, 2025-03-31; 24 stable v3 releases since),
  built-in dynamic completions. Loses only on precedent (zero in-house
  repos) and the smaller template/help ecosystem; nothing here would block
  dstow.
- **kong — rule out for dstow.** The struct-tag grammar is elegant and the
  repo is healthy, but shell completions — high value for a daily driver —
  are not built in, and the third-party bridge (kongplete) has been idle
  since mid-2024. Single-maintainer bus factor (alecthomas 271 commits vs 14
  for the next human) adds risk without an offsetting win.
- **stdlib `flag` — rule out.** No subcommands, no completions, no help
  generation for a ~15-command surface; gostow's zero-dep story came from a
  hand-rolled getopt that exists for Stow conformance, which dstow neither
  needs nor may share (§11).

---

## 3. Color / output ecosystem

Maintenance snapshot (GitHub API, 2026-07-14):

| Library | Latest release | Activity | Runtime deps (go.mod) |
|---|---|---|---|
| [fatih/color](https://github.com/fatih/color) | v1.19.0, 2026-03-20 | pushed 2026-07-09; ~8.0k stars; 25 issues; maintainer: fatih (197 commits) | mattn/go-colorable, mattn/go-isatty, golang.org/x/sys |
| [muesli/termenv](https://github.com/muesli/termenv) | v0.16.0, 2025-02-21 | pushed 2025-11-21; last substantive commit 2025-09-22; still v0.x | go-osc52, go-colorful, go-isatty, uniseg, x/sys |
| [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) | **v2.0.5, 2026-07-03** | pushed 2026-07-13; ~11.6k stars | colorprofile, ultraviolet, x/ansi, x/term, displaywidth, go-colorful, uniseg, x/sys + 9 indirect |
| [mattn/go-isatty](https://github.com/mattn/go-isatty) | v0.0.22 (tags only) | pushed 2026-04-27 | golang.org/x/sys |
| [pterm/pterm](https://github.com/pterm/pterm) | v0.12.83, 2026-02-25 | pushed 2026-07-11 | large (printer framework) |

**Charm ecosystem state.** Very healthy as an org — six repos pushed within
the last 24 hours (org API) — but in **deliberate major-version churn**:
lipgloss v2.0.0 (2026-02-24, after a year of betas) moved the import path
off GitHub to `charm.land/lipgloss/v2` (go.mod: `module
charm.land/lipgloss/v2`), replaced the renderer with explicit
`lipgloss.Fprint`-style writers that downsample via
[charmbracelet/colorprofile](https://github.com/charmbracelet/colorprofile)
(v0.4.3, 2026-03-09), and made background detection explicit
([v2.0.0 release notes](https://github.com/charmbracelet/lipgloss/releases/tag/v2.0.0)).
Bubble Tea is likewise at v2 (v2.0.8, 2026-07-03). termenv — the v1-era
detection layer — is effectively superseded inside Charm by colorprofile.

Verdicts:

- **fatih/color — recommend.** Small active dependency (three-module tree),
  built-in NO_COLOR handling ("disables color output if the NO_COLOR
  environment variable is set to a non-empty string" —
  [README](https://github.com/fatih/color/blob/main/README.md)), isatty-based
  TTY auto-disable, Windows via go-colorable, per-instance
  `EnableColor`/`DisableColor` and `SprintFunc` — everything a 16-color
  semantic printer needs and nothing more. dotd already proved the exact
  seam dstow wants: a handful of `color.New` instances behind one semantic
  ui package. Known wart: the package-level `color.NoColor` global is
  process-wide, so per-stream decisions (stdout piped, stderr TTY — the
  §1.7/§9.2.4 shape) must be made in dstow's printer via per-instance
  toggles or plain-vs-colored render funcs, not via the global. The single
  owned printer absorbs this cleanly.
- **termenv — runner-up.** The best *architectural* fit for per-stream
  handling — `Output` binds a profile to a writer; `EnvColorProfile`
  "respects the NO_COLOR and CLICOLOR_FORCE environment variables"
  ([README](https://github.com/muesli/termenv/blob/master/README.md)) — plus
  color degradation dstow doesn't strictly need. Docked for momentum: still
  v0.x, last release 2025-02, last substantive commit 2025-09, and its main
  patron (Charm) has moved to colorprofile. Not near-sunset, but coasting.
- **lipgloss (v2) — rule out for dstow.** It is a styling/layout engine for
  TUI-shaped output (borders, joins, measurement) riding a fresh
  major-version and import-path migration (`charm.land`), with a dependency
  tree an order of magnitude larger than fatih/color. dstow emits
  line-oriented status text and POSIX-sh snippets; a layout engine is the
  wrong altitude. (No health objection — if dstow ever grows a TUI, Charm v2
  is the obvious place to look.)
- **pterm — rule out.** An opinionated printer *framework* (spinners, trees,
  its own themes) that would compete with, not serve, dstow's single owned
  printer seam.
- **mattn/go-isatty — take (transitively or directly).** It arrives free
  with fatih/color and answers §1.2's interactivity question
  (`isatty.IsTerminal(os.Stdin.Fd())`). `golang.org/x/term.IsTerminal` and
  gostow's `os.ModeCharDevice` stat are equivalent stdlib-adjacent options;
  design's call, zero extra weight either way once fatih/color is in.
- **Hand-rolling (gostow-style paint pass) — rule out.** gostow's approach
  is excellent *for gostow* (byte-parity mandate) but §11 forbids sharing
  the plumbing, and the philosophy memo says libraries over hand-rolling
  when good ones exist. They do.

### Machine-output coexistence (§1.7, §9.2.4)

All recommended pieces compose with `--json`/`--quiet`: the decision "is
color on for this stream?" is computed once per stream in dstow's printer
(flag > env > TTY, below), and JSON/snippet output paths simply never route
through style functions. fatih/color's `SprintFunc()` returning
identity-when-disabled makes the printer's internals trivially testable
against gostow-style strip-roundtrip assertions. termenv would do the same
via a per-writer `Output`; lipgloss v2 would force adoption of its writer
wrappers even on plain paths — another reason it's ruled out.

---

## 4. Conventions dstow must honor

- **NO_COLOR** ([no-color.org spec text](https://github.com/jcs/no_color/blob/master/index.md),
  proposed 2017): "Command-line software which adds ANSI color to its output
  by default should check for a NO_COLOR environment variable that, when
  present and not an empty string (regardless of its value), prevents the
  addition of ANSI color." Note gostow's `Enabled()` treats *any non-empty*
  value as off — spec-correct; fatih/color matches.
- **CLICOLOR / CLICOLOR_FORCE** ([bixense.com/clicolors](https://bixense.com/clicolors/)):
  `CLICOLOR_FORCE` set (and NO_COLOR unset) ⇒ "ANSI colors should be enabled
  no matter what" (even piped); `CLICOLOR` set ⇒ color when writing to a
  terminal; `NO_COLOR` overrides everything. fatih/color does *not* read
  CLICOLOR* natively — dstow's printer implements the two extra checks
  (~four lines); termenv reads them natively.
- **`--color=WHEN`** (GNU precedent,
  [grep manual](https://www.gnu.org/software/grep/manual/grep.html)):
  `never` / `always` / `auto`, where auto means "standard output is
  associated with a terminal device and the TERM environment variable's
  value suggests that the terminal supports colors". dstow should ship
  `--color=auto|always|never` with auto the default, flag beating env.
- **TTY detection**: per-stream (stdout and stderr decided independently —
  gostow already does this: `cli.go` wraps each with its own `Enabled()`),
  plus `TERM=dumb` ⇒ off (gostow and hud both check it).
- **Precedence, combined**: `--color=always|never` > `NO_COLOR` >
  `CLICOLOR_FORCE` > `CLICOLOR`/TTY-auto. (Flag over env is universal CLI
  convention; env order is bixense's.)
- **Stdout discipline** (§10, §9.2.4): machine output (bootstrap snippet,
  dependency query, any `--json`) goes to stdout unstyled; human commentary
  to stderr, styled per stderr's own decision — the gostow two-writer
  injection pattern (`Run(args, version, stdout, stderr)`) is the shape to
  copy.

---

## Recommendations

**Framework: cobra (v1.10.x).** Built-in 4-shell completions, generated
help and suggestions that serve §1.4 directly, nested subcommands for the
repo verbs, three in-house repos of precedent, and a verified-clean
no-viper story (viper is optional per cobra's README and absent from all
three local cobra users' module graphs). Cost: pflag + mousetrap, the only
runtime deps of any candidate — accepted.
**Runner-up: urfave/cli v3** — zero-dep, actively released, built-in
dynamic completions; choose it if design decides pflag's weight or cobra's
global-state `Command` style offends more than losing precedent.

**Color stack: fatih/color + mattn/go-isatty (transitive), wrapped in ONE
owned printer package** — the dotd `internal/ui` pattern with dstow's
additions: per-stream enable decision implementing
`--color=auto|always|never` > NO_COLOR > CLICOLOR_FORCE > CLICOLOR > TTY +
`TERM=dumb`, semantic style names drawn from CONTEXT.md package states
(stowed/drifted/occupied/damaged…), and identity-function styles when
disabled so machine paths and tests see plain bytes.
**Runner-up: muesli/termenv** — architecturally the cleanest per-stream
fit and natively CLICOLOR-aware, docked for v0.x coasting since early 2025.

## Open questions handed to design tickets

**→ "Output design" ticket:**

1. Exact `--color` spelling and default (`--color[=WHEN]` with bare
   `--color` = always, per grep? or require the value?), and whether
   `--no-color` sugar exists.
2. The semantic style vocabulary: one style name per CONTEXT.md state
   (stowed/partially/not-stowed/occupied/damaged/drifted, broken/orphaned)
   and per severity (error/warning/hint/announcement §1.3–1.4) — the
   printer's public surface.
3. stdout vs stderr routing table per command (esp. status/list human
   tables vs dependency-query machine output vs bootstrap-snippet's pure-sh
   stdout), and whether `--json` is global or per-command.
4. Whether the printer adopts gostow's strip-roundtrip test invariant
   (StripANSI(styled) == plain) as its own contract.
5. `--quiet` semantics vs §1.3's "surprises are announced" — what may quiet
   suppress?

**→ "Go architecture" ticket:**

6. Cobra wiring style: single `root.go` with constructor-injected printer
   and streams (gostow's `Run(args, version, stdout, stderr)` shape) vs
   cobra's package-level command globals; how `SetOut`/`SetErr` route
   cobra's own help/error output through the owned printer.
7. Interactivity seam for §1.2: one `Interactive()` predicate (stdin TTY
   via isatty vs x/term vs ModeCharDevice) owned by the same package as the
   color decision, injectable for tests.
8. Completion depth: static command/flag completion for free vs custom
   `ValidArgsFunction` completing package names, repo names, and schemes
   from config (high daily-driver value; needs the config layer early in
   the dependency graph).
9. Exit-code map (§1.7, §3.2, §9.2.4): reserve distinct codes for
   "some packages failed" vs "usage error" vs dependency-query's
   present/absent signaling before commands ossify around `1`.
