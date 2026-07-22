# dstow v1 — Design

**What this document is.** The complete, binding design of dstow v1 — the
synthesis of every resolution on the [design map](https://github.com/rocne/dstow/issues/16).
[REQUIREMENTS.md](REQUIREMENTS.md) pins *behavior*; this document pins
everything REQUIREMENTS delegated: CLI surface, config format and schema, the
metadata directory, the naming grammar, the ledger, output design, the hook
context contract, bootstrap and installer mechanics, the Go architecture, and
release wiring. Where REQUIREMENTS speaks, this document obeys; where this
document speaks, implementation obeys. Implementation is intended to be a
decision-free execution effort.

**Pinned vs. internal.** Persistent formats are always pinned fully — the
ledger schema, the config schema, the hook contract, anything user files or
user scripts depend on (migration cost makes them non-internal). Ephemeral
internals (in-memory mechanics, file organization inside a package) belong to
implementation. Presentation specifics called *provisional* below may be
revisited at implementation without a design amendment.

**Vocabulary.** This document speaks [CONTEXT.md](../CONTEXT.md)'s canonical
language. Rationale lives in each section's ticket — see
[Provenance](#12-provenance). If this document is found to disagree with a
ticket resolution as amended, that is a bug in this document.

---

## 1. Naming grammar

*Resolution: [Naming grammar: sources, repos, packages (#20)](https://github.com/rocne/dstow/issues/20).*

### 1.1 The grammar

**`scheme:coordinate::package`** — one grammar for sources, repos, and
packages. A repo's FQN drops the `::package` tail.

- **`:` separates only the scheme; `::` separates only the package.** The
  coordinate is path-shaped (one or more `/`-segments); schemes may interpret
  theirs (`github:` reads `owner/name`; `local:` reads an absolute filesystem
  path, canonicalized at add time) but the grammar itself does not. `X:Y` is
  always scheme-then-coordinate — an unknown scheme is a clean §1.4 error
  listing known schemes.
- **Suffix resolution**: any suffix cut at a segment boundary that resolves
  uniquely is a valid name (`zsh` → `dots::zsh` → `rocne/dotfiles::zsh` →
  FQN). A leading `::` forces package-kind (`::zsh`).
- **Ambiguity is exclusively a property of user input** — the model never
  contains it; same-named packages from different sources coexist. A name
  matching multiple entities gets the §1.2 treatment: interactive explicit
  choice; non-interactive hard error naming the qualified spellings.
  Cross-kind ties (a bare name matching both a repo and a package) follow the
  same rule.
- **Naming a repo where packages are expected means all its packages.**
- **The repo set is unordered** (REQUIREMENTS §2.3–2.5 as amended): bulk spans
  all packages of all registered repos; `DSTOW_PATH` contributes session repos
  with no priority.

### 1.2 Encoding

Reserved characters percent-encode — every path is spellable, zero
exceptions. dstow always emits the canonical encoded form itself; users paste
rather than construct.

| Reserved | Encoding | Why |
|---|---|---|
| `:` | `%3A` | scheme separator |
| `%` | `%25` | the escape itself |
| `@` | `%40` | reserved scheme-interpreted suffix (below) |
| control characters | `%0A` etc. | `DSTOW_HOOK_PACKAGES` is newline-separated; a pathological name must not corrupt the list |

**The `@` reservation (decided at synthesis, 2026-07-16).** `@` is reserved
in coordinates as an **optional, scheme-interpreted suffix**:
`scheme:coordinate@suffix::package`. v1 assigns it no semantics — a literal
`@` in a name expression must be written `%40`. What an `@`-suffix *means*
belongs entirely to any scheme that later chooses to interpret it; the
anticipated lineage is revision pinning (`pkg@version`, npm/brew/go-modules),
but the reservation binds only the syntax, never that meaning. Reserving now
makes future adoption non-breaking; schemes that ignore it lose nothing.

At `repo add`, a path that would need encoding triggers an interactive
continue-or-rename confirmation (continue-affirmative polarity, §7.2);
non-interactively it proceeds with a loud announcement.

### 1.3 Names vs. paths

A **path operand** starts with `/`, `~/`, `./`, or `../` and always refers to
the *target* world. A **name expression** is anything else and resolves
against the registry. The first character decides; no command ever guesses.

### 1.4 Identifier resolution and aliasing

Identifier resolution is **one generic system**: canonical identifiers plus
optional input-side aliases, resolving through a single path with §1.2
ambiguity semantics; output is always canonical. Field names are one
application of it (naming resolution above is another). **v1 ships zero
aliases** — the system degenerates to an interface over an identity resolve,
and nothing speculative is built — but the rule binds so that v2 aliases
(field shorthands, prefix resolution) arrive in an already-specified story.
*(F12 on [#27](https://github.com/rocne/dstow/issues/27); instance-set emptied
by the dependency removal, doorway kept at synthesis.)*

### 1.5 Display and utilities

- **Display rule (O9)**: shortest-unique suffix everywhere by default; full
  FQN whenever showing a tie, an ambiguity error, or `--json` (which always
  carries FQNs). Local coordinates display with `~` abbreviation.
- **Hidden `name` utility group**: `dstow name encode` / `dstow name decode` —
  absent from top-level help, documented in the manual. Natural future home
  for `name resolve`.
- Shell brace expansion gives multi-select for free:
  `dstow stow dotfiles::{bash,zsh}`.

## 2. CLI surface

*Resolution: [CLI surface: commands, flags, help text (#22)](https://github.com/rocne/dstow/issues/22);
full decision ledger D1–D16 there. The help content approved there (prototype
v2 as amended through 2026-07-16) now lives in `docs/commands/`.*

**Help content is owned by the pages under `docs/commands/`**, not by this
document (ruled 2026-07-22 —
[Help text derives from docs/commands/ (#132)](https://github.com/rocne/dstow/issues/132)).
What help *says* — the wording of every description, prose block, and example —
is the pages'; layout and rendering stay cobra's (A2 as amended —
[#96](https://github.com/rocne/dstow/issues/96)), and the flag roster stays the
flag definitions'. DESIGN keeps the shape rule (§2.1), the standing principles
(§2.2), and the derivation mechanism (§2.3/§2.4). Two owners of the same
sentence is the condition the ruling removes: prose blocks here would be the
second one.

### 2.1 The shape rule

Every command is a **group** (subcommands only, no operands; bare group
prints its help) or a **leaf** (operands only, never subcommands) —
operand/subcommand collision is impossible by construction. Groups are nouns
(`repo`, `snippet`, `theme`); leaves are verbs plus the established query
names (`list`, `info`, `status`).

- Groups: `repo` {add, remove, update, upgrade} · `snippet` {rc} ·
  `theme` {list, slots, emit} · hidden `name` {encode, decode}.
  *(`theme` replaces the `colors` group — ruled 2026-07-19 with the
  prior-art defaults: a command's noun is the noun of its output, and
  `colors theme` emitted themes while showing no colors. `theme list`
  discovers the roster; `theme slots` describes the slot vocabulary; `theme
  emit` outputs colors — the effective stack, a named theme, or either with
  slot=value tweaks. `emit` was `show` (renamed 2026-07-20 —
  [#117](https://github.com/rocne/dstow/issues/117): the noun matches the
  output — the machine formats are the emission path and the rendered view is
  one facet of it; clean rename, no alias); `theme slots` added the same day
  ([#116](https://github.com/rocne/dstow/issues/116): the slot reference
  surface).)*
- Leaves: `stow` `unstow` `restow` `adopt` · `list` `info` `status` ·
  `check` `clean` `rebuild` · `completion` `version`.
- **The `manual` tree is carved out of the shape rule** (ruled 2026-07-21 —
  [dstow manual (#130)](https://github.com/rocne/dstow/issues/130)): the
  hidden `manual` group mirrors the tracked `docs/` folder, generated from an
  embedded FS walk — directories are nodes, markdown files are leaves. These
  nodes are **not actions**; calling them commands is close to a category
  error, so the group/leaf dichotomy does not bind them. A bare node prints
  its `index.md` — which is both the directory's table of contents and its
  content — rather than its help, so any node is discoverable by running it.
  Filenames are command spellings exactly (identity mapping, no transform);
  a directory without an `index.md`, a `format.md` beside a `format/`, and a
  non-markdown file are each errors, gated by the structural suite. Markdown
  as the only format is **tentative** and may change. The requirement this
  serves: **dstow is fully learnable from the command line alone**, so an
  agent never needs external documentation; human ergonomics is explicitly a
  non-requirement, provided the information is present and reachable. Root
  help carries one footer line naming `dstow manual` — the sole discovery
  affordance, and load-bearing: without it the requirement is unmet.
- **Two `Short`s, two sources** (ruled 2026-07-22 —
  [Help text derives from docs/commands/ (#132)](https://github.com/rocne/dstow/issues/132)):
  a docs node's `Short` is its file's first H1 — the line `dstow manual <TAB>`
  shows beside each candidate ([#130](https://github.com/rocne/dstow/issues/130)) —
  while a command's `Short` is its page's `dstow:short` region, the line
  `dstow --help` lists it by. Same word, two consumers, two sources: a page's
  H1 names the *page* (`# dstow repo add`) and its `dstow:short` describes the
  *command*. This is why the `manual` tree is exempt from §2.4's completeness
  walk — requiring `docs/commands/manual.md` would hand those nodes a second
  source for a `Short` they already derive.
- **Root `--version` flag** (ruled at
  [#81](https://github.com/rocne/dstow/issues/81), 2026-07-19): prints
  exactly what `dstow version` prints — one source of truth, two spellings.
  Exists to satisfy the release-ci D30 parse rule (the installer's
  ensure-check and the release smoke parse `dstow --version` line 1); the
  `version` subcommand remains the canonical spelling and the one the root
  command listing advertises.

### 2.2 Standing principles

- **A flag never changes a command's concept** (D5).
- **Remedy pre-acceptance** (D15): a flag may pre-accept the exact remedy its
  refusal would name (`stow --adopt`, `repo remove --unstow`).
- **Long-canonical-plus-alias naming; glossary terms canonical** (D7).
- **No CLI flag twins for config knobs** (D12): no `--target`,
  `--no-folding`, `--ignore` flags; refusals name the knob and its file.
  *(Scope ruled 2026-07-19: D12 binds deployment-behavior knobs; parameters
  of an emission — `theme emit`'s slot=value operands — are outside it.)*
- **No config-mutation commands**: declare by editing the metadata file (like
  every knob); write commands are deferred to the v2 property store, where
  they can arrive as a designed family.
- **Three read surfaces**: `list` enumerates a scope's content
  (global ⊃ repos ⊃ packages ⊃ paths — the registry is the global scope's
  content); `info` reads one scope's fields (config+metadata, never disk);
  `status` inspects reality.
- `--yes` pre-answers confirmations of *stated intent* only; it never
  resolves ambiguity, never answers the bulk prompt, never bypasses guards
  (D2/D9). `--force` exists per-command only where REQUIREMENTS names an
  override.
- `-n/--dry-run` on stow/unstow/restow/adopt (D8). `--json` per-command:
  list, info, status, check (D13).
- `clean` takes no scope flags; it executes exactly check's plan (D10).
- `restow` on a not-stowed package just stows — the unstow phase no-ops;
  restow is the idempotent refresh verb (D16).

### 2.3 Top-level help

`docs/commands/index.md` owns the root command's help: its `dstow:short`
region is the one-line description, its `dstow:long` region is the prose
`dstow --help` opens with — the title line, the naming reminder, and the
`dstow manual` footer §2.1 makes load-bearing. Everything else on that page is
manual content and never reaches help.

**The tag vocabulary.** A page's help text is extracted by namespaced HTML
comment, each region opened and closed explicitly:

| Region | cobra field | Renders as |
| --- | --- | --- |
| `dstow:short` | `Short` | the one-line description, in the parent's command listing |
| `dstow:long` | `Long` | the prose body of `<command> --help` |
| `dstow:examples` | `Example` | the `Examples:` block, interior indentation intact |
| untagged | — | manual only; help never sees it |

- **Tags, never section names or positions.** An implicit boundary — the
  section titled "Overview", everything until the next heading — would make
  every heading a mechanical commitment and every restructuring a silent
  breakage. With tags, a page's prose structure is a pure authoring decision
  with no mechanical consequence.
- **Closes are explicit.** A region ends where its close tag says, never where
  the next thing happens to start.
- **An unknown `dstow:` tag is an error.** A silent no-op on a typo would drop
  a command's help text with nothing to notice it; the namespace exists
  precisely so these are greppable.
- **Tags are not stripped from manual output.** The manual prints its file
  verbatim (§2.1), and the tags stay visible — self-describing to the agent
  audience the manual exists for.
- **Flags stay cobra's.** The flag roster is generated from the flag
  definitions, already single-owner in code; a page that restated it would be
  the second owner all over again.
- **The root listing's sections are cobra command groups**, owned in code and
  carrying §2.1's inventory in its order: `Deploy:` `Inspect:` `Maintain:`
  `Groups:` `Also:`. They are the listing's structure, not any command's help
  text, so no page carries them.

*(Ruled 2026-07-22 — [Help text derives from docs/commands/ (#132)](https://github.com/rocne/dstow/issues/132).)*

### 2.4 Per-command help

`docs/commands/` mirrors the command tree, by identity on command names: a
leaf's page is a file named for it, and a group's page is its directory's
`index.md` — the same file the manual prints for that node (§2.1's carve-out).
`docs/commands/repo/add.md` is `dstow repo add`, `docs/commands/repo/index.md`
is `dstow repo`, and `docs/commands/index.md` is `dstow` itself.

- **Every dstow-defined command has a page, and every page has a command.**
  Each page is simultaneously the command's manual page and the source of its
  help text: `dstow manual commands repo add` and `dstow repo add --help` are
  the same content, which is what makes the page the single owner rather than
  one more copy of the wording.
- **cobra's built-ins (`completion`, `help`) and the `manual` tree are exempt**,
  skipped whole. The built-ins carry cobra's own text, and manual nodes already
  derive their `Short` from their H1 (§2.1's two-`Short`s rule).
- **Completeness is gated at test time**, by a walk over the live command tree
  and the embedded pages: a page missing, orphaned, or malformed fails the
  suite. Never at startup — a docs tree that has drifted from the command tree
  is a repo defect, and refusing to start over it would ship users the breakage
  in exchange for nothing.

*(Ruled 2026-07-22 — [Help text derives from docs/commands/ (#132)](https://github.com/rocne/dstow/issues/132).)*

Design notes the pages do not carry, kept here because they are decisions
rather than help text:

*(`unstow` and `restow` are identical to `stow` in shape; bare `unstow`
confirms with the same care — it is the destructive one. `restow` on a package
that is not stowed simply stows it, the unstow phase no-ops (D16); `--adopt`
composes with stow and restow, never with unstow.)*

*(`list`'s scope operand is the info resolution's restatement — list
enumerates a scope's content; package content is raw paths, no translation
flags: the translated view belongs to `-n/--dry-run`, reality to `status`.)*

*(check additionally reports **contradicted** entries — ledger evidence disk
disagrees with; clean prunes those entries only, never touching disk. Help
names the two disk-acting classes; the full contract is
[§6.4](#64-command-contracts).)*

*(Ruled 2026-07-19, replacing the `colors` group; `emit` was `show` and
`slots` was added 2026-07-20 — [#117](https://github.com/rocne/dstow/issues/117),
[#116](https://github.com/rocne/dstow/issues/116). `emit` bare renders the
effective §7.3 stack — the composed truth of what dstow is using; a ref
renders that theme AS LOADED (its declared slots); `slot=value` operands
layer on top, the top of the stack. Overrides are **operands, not per-slot
flags** — D12 was ruled inapplicable here (re-scoped: it targets flag twins
of deployment-behavior knobs; parameters of an emission are not that), and
operands reuse the one slot=value grammar besides. `--format env|toml` —
exact flag spelling is implementation detail; a format flag doesn't change
the concept, per the `--json` precedent. The default output is the rendered
view; the machine formats are the emission path, so `theme emit` absorbs the
old `colors theme` converter whole. `theme slots` is the slot-vocabulary
reference — the noun matches its output (each slot, what it colors); it renders
each slot name in its own effective style (the swatch precedent), enumerates
the stage-2 consumers from the same code-owned Role mapping the styles use
(§7.2, so the reference cannot drift), and carries the full value-grammar
enumeration in its long help. The name renaming carries no alias — pre-1.0,
clean break.)*

## 3. Configuration

*Resolutions: [Config format survey (#17)](https://github.com/rocne/dstow/issues/17),
[Config schema (#23)](https://github.com/rocne/dstow/issues/23) — full ledger
C1–C23 as amended.*

**Format: TOML (spec v1.1.0), parsed with BurntSushi/toml; XDG paths via
adrg/xdg; four-level merge hand-wired over the plain parser — no config
framework** (viper and koanf ruled out: the four-level merge *is* domain
logic).

### 3.1 Files

| File | Location | Written by |
|---|---|---|
| Global config | `$XDG_CONFIG_HOME/dstow/config.toml` | user |
| Repo config | `<repo>/.dstow/config.toml` | user |
| Package config | `<repo>/<pkg>/.dstow/config.toml` | user |
| Repo registry | `$XDG_CONFIG_HOME/dstow/repos.toml` | **dstow** |
| User theme presets | `$XDG_CONFIG_HOME/dstow/themes/<name>.toml` | user |

- **One name at every level** (`config.toml`); the level is determined by
  placement alone; the legality matrix says which keys mean anything where.
  Naming principle: *the file name carries whatever context its location
  doesn't* (C1).
- **`config.toml` never declares repos** — the registry is dstow-written
  (single ownership; reconciles "no config-mutation commands" with §5's
  persistent registration; gh `config`/`hosts` split precedent). Entries are
  canonical qualified source strings (`repos = ["github:rocne/dotfiles"]`); a
  table with `source` is the documented growth form. The registry is *config,
  not state*: not reconstructible from disk, so it lives with intent (C3).
- Theme preset name = file basename; user presets shadow bundled embedded
  ones on name collision (C4).

### 3.2 Key grammar and the schema

- **snake_case keys** across config, JSON output, and color-slot vocabulary —
  one spelling of every term. CLI flags keep GNU kebab: a deliberate,
  documented mismatch of exactly one rule (C5).
- **Bool keys are imperative statements; value keys are nouns** (C6):
  `target` (string), `translate_dot_prefixes` (default true),
  `exclude_from_bulk` (default false), `fold_trees` (default false — stow's
  own "tree folding" term).
- **Legality matrix** (C7, as amended):

  | Key | package | repo | global |
  |---|---|---|---|
  | `target` | ✓ | ✓ | ✓ *(the built-in `$HOME` floor is the global cell's default)* |
  | `translate_dot_prefixes` | ✓ | ✓ | ✓ |
  | `ignore` (additive) | ✓ | ✓ | ✓ |
  | `exclude_from_bulk` | ✓ | ✓ | — |
  | `packages_dir` | — | ✓ | — |
  | `fold_trees` | — | — | ✓ |
  | registry (`repos.toml`) | — | — | ✓ |
  | `[color]` + `theme` | — | — | ✓ |

- **Path-valued keys expand `~` and `$VAR`/`${VAR}`** (stow's grammar),
  evaluated at use time per invocation. Unset variable = loud error naming
  variable + file + key. The expanded result must be absolute. Failures scope
  per-package (§3.2 of REQUIREMENTS) (C8).
- **Schema-wide shorthand idiom** (C9): wherever a collection entry has a
  scalar essence, a bare string is legal shorthand for its table form (the
  registry is the v1 instance).
- **`packages_dir`** (M3): opt-in, repo level only, a repo-root-relative path
  naming where the repo's packages live. Unset = packages at the repo root
  (stow compat binds the default). Deliberately outside C8's
  expand-to-absolute grammar — it names a location *inside* the repo.
  `"packages"` is the documented convention and fresh-start recommendation.
- **No schema-version key** (C11): forward evolution rides the unknown-key
  policy; breaking changes are v2 territory by definition.

### 3.3 Theming keys

- **`[color]` table** (C12, amended 2026-07-20 —
  [#115](https://github.com/rocne/dstow/issues/115)): global only; fourteen
  keys, a closed set — the **generic slot roster**, two groups. Content:
  `section1` `section2` `name1` `name2` `value1` `value2`. Messages: the
  `error` `warning` `success` `info` families, two tiers each (`error1`
  `error2` …). Variant numbers are **prominence tiers** — 1 loudest —
  naturally extendable. dstow's internal vocabulary (package states, check
  classes, severity prefixes, prose roles) never appears in theming: internals
  reach slots only through the code-owned stage-2 mapping (§7.2). Values in
  git's `color.*` grammar. One vocabulary across `[color]` / `DSTOW_COLORS` /
  theme files. An undeclared tier-2 derives from its family's tier-1 (§7.3
  derivation).
- **`theme` key** (C13): name-or-path per the operand rule; the path form
  follows C8.

### 3.4 Ignores

- **Carrier: the `ignore` key** at every level, additive per §4.1 — a level
  adds to, never silences, inherited ignores. **No native ignore file**:
  "native mirror" means the concept is expressible natively, not
  file-for-file twinning; stow-named files stay honored as compat (C15).
- **Language: gitignore-glob semantics**, matched per package against
  package-root-relative paths (no slash = basename at any depth; slash =
  anchored; trailing slash = dir-only; `**` supported) (C16).
- **Two refused-and-reserved forms**: leading `!` (negation would silence
  inherited ignores — forbidden by the additive law) and leading `//`
  (reserved for a possible future regex marker) (C16).
- **Pattern language is a property of the carrier** (C17): native carriers
  speak glob; compat carriers (`.stow-local-ignore`, `--ignore`) speak stow
  regex; never mixed in one file.

### 3.5 Unknown and misplaced keys

**Warn, never refuse** (C18): unknown key → warning naming file + key with
did-you-mean; misplaced key (legal elsewhere per the matrix) → warning naming
the legal level and its file, key ignored. These are surprise-class
announcements — they survive `--quiet`.

### 3.6 Stow compatibility

- **Routing (renamed rc)**: a native config whose content is flag-lines
  (first significant token starts with `-` — impossible in top-level TOML)
  routes to the compat parser with a loud announcement naming the native
  equivalent. The native format never bends to compat (#17).
- **rc parsing is consumed from gostow's public `stowrc` package**
  (quirk-faithful, conformance-tested). dstow keeps discovery, slotting
  (`~/.stowrc` → global; `<repo>/.stowrc` → repo, discovered via the repo,
  never cwd), knob mapping, and supplement diffing (C20).
- **Option mapping** (C19): `--target`→`target`; `--no-folding`→
  `fold_trees=false` (flag *absence* maps to nothing — dstow's default
  applies); `--dotfiles`→`translate_dot_prefixes=true`; `--ignore`→additive
  chain entry in carrier language; `--dir` in `~/.stowrc`→session-repo
  contribution, announced, `fix:` suggests `repo add`; `--dir` at repo
  level→warn-and-ignore. Unmappables (`--adopt`, `--override`, `--defer`,
  verbosity, simulate, verbs): warn-and-ignore per option, naming why + the
  native remedy. The file always runs; degradation is loud, never rejection.
- **Non-RE2 compat patterns: refuse, scoped** to the level the pattern
  governs (a package-level failure fails that package; the run continues),
  naming file, pattern, RE2 error, and both remedies (C21).
- **Supplement mode** (C22): per-knob diff where *both* carriers set the knob.
  Non-overlap silent; equal values silent; differing values → native wins,
  loud warning naming level/files/knob/values/winner + `fix:` suggesting
  removal from the rc. Ignores never conflict (additive, per-carrier
  language). **Supplement mode survives design contact — REQUIREMENTS §4.3's
  fallback is not triggered.**

### 3.7 Environment

**`DSTOW_PATH`** (C23): colon-separated **absolute local directory paths**,
PATH convention. No qualified sources (`:` collides with the separator;
remote sources need clones — `repo add`'s job); no dstow-side expansion;
relative entries refused loudly; empty entries warn-and-skip.
**`DSTOW_COLORS`** is the only other environment variable dstow reads
([§7.3](#73-theming--the-dircolors-architecture)); `DSTOW_HOOK_*` is dstow's
output lane ([§5](#5-hook-context-contract)).

## 4. Metadata directory

*Resolution: [Metadata directory name and layout (#24)](https://github.com/rocne/dstow/issues/24) —
ledger M1–M8 as amended.*

- **The name: `.dstow/`** at both levels (`<repo>/.dstow/`,
  `<repo>/<pkg>/.dstow/`); the global level's equivalent is
  `$XDG_CONFIG_HOME/dstow/`. Dot-prefixed tool-metadata convention (`.git`
  lineage); inert under dot-translation by construction; visually distinct
  from stow's `.stow` marker file (M1).
- **Package identity is locational; hidden directories are never packages**
  (M2). No marker files (migrated repos must work out-of-the-box;
  mkdir-and-go stays intact). Root mode: hidden dirs skipped silently.
  Scoped mode (`packages_dir` set): every visible directory is
  definitionally a package; hidden dirs skipped **loudly**.
- **Reserved territory** (M5): `.dstow/`'s top level is dstow's namespace —
  claimed entries are `config.toml` and `hooks/`; unknown top-level entries
  draw a C18-style warning, never a refusal. Same posture in
  `$XDG_CONFIG_HOME/dstow/` (claimed: `config.toml`, `repos.toml`, `themes/`,
  `hooks/`).
- **Hooks: `.dstow/hooks/`, eight per-event executables, git-style** (M6):
  `{pre,post}-{stow,unstow,restow,adopt}` at all three levels (global:
  `$XDG_CONFIG_HOME/dstow/hooks/`). **Restow fires only the restow pair.**
  Execution is direct `exec`; shebang decides the interpreter.
  Subdirectories of `hooks/` are inert helper territory — never fired, never
  warned — with `lib/` the documented convention. Non-hook *files* directly
  in `hooks/` warn with did-you-mean (`pre_stow`, `prestow`); a valid-named
  non-executable warns naming the `chmod +x` fix.
- **`<name>.d/` reserved** (M7): the eight `.d` spellings are
  inert-and-warned ("reserved, not yet meaningful") in v1; drop-in execution
  is committed v2 (see [§10](#10-reserved-doorways)).
- **Auto-ignore rides the engine's `IgnoreFunc` seam** (M8): the closure
  returns true for `.dstow` **anchored at the package root only** — deeper
  `.dstow` names are content. Always on, in no config, unsilencable
  (REQUIREMENTS §2.7 is law, not an ignore-chain entry). check/status/deploy
  consult the same closure, so observation and deployment can never disagree.

## 5. Hook context contract

*Resolutions: [Hook context contract (#25)](https://github.com/rocne/dstow/issues/25) —
ledger H1–H8 as amended; lifecycle is REQUIREMENTS §9.1.*

- **Mechanism: environment variables, `DSTOW_HOOK_*`, no arguments, no
  stdin-feeding** (H1). Additive evolution: new context = new variable. The
  prefix partitions the env namespace into lanes — `DSTOW_PATH` /
  `DSTOW_COLORS` are user→dstow inputs; `DSTOW_HOOK_*` is the dstow→hook
  output lane.
- **The variable set** (H2) — absent-not-empty everywhere; explicit beats
  derived:

  | Variable | Value | package | repo | global |
  |---|---|---|---|---|
  | `DSTOW_HOOK_LEVEL` | `package` \| `repo` \| `global` | ✓ | ✓ | ✓ |
  | `DSTOW_HOOK_ACTION` | `stow` \| `unstow` \| `restow` \| `adopt` | ✓ | ✓ | ✓ |
  | `DSTOW_HOOK_PHASE` | `pre` \| `post` | ✓ | ✓ | ✓ |
  | `DSTOW_HOOK_FQN` | the scope's own FQN, canonical-encoded | ✓ | ✓ | — |
  | `DSTOW_HOOK_SCHEME` | decoded (`github`) | ✓ | ✓ | — |
  | `DSTOW_HOOK_COORDINATE` | decoded (`rocne/dotfiles`; a `local:` repo's is the actual absolute path) | ✓ | ✓ | — |
  | `DSTOW_HOOK_PACKAGE` | bare package name, decoded — not the FQN | ✓ | — | — |
  | `DSTOW_HOOK_PACKAGE_DIR` | absolute path | ✓ | — | — |
  | `DSTOW_HOOK_TARGET` | effective target root, absolute | ✓ | — | — |
  | `DSTOW_HOOK_REPO_FQN` | the repo's FQN, canonical-encoded | ✓ | ✓ | — |
  | `DSTOW_HOOK_REPO_DIR` | absolute path | ✓ | ✓ | — |
  | `DSTOW_HOOK_PACKAGES` | FQNs of all packages acting under this scope | — | ✓ | ✓ |

  **Encoding stance**: `*FQN` values carry the canonical percent-encoded
  spelling (paste-able straight back into dstow commands); decomposed
  segments carry decoded real values — encoding protects the grammar, and a
  standalone segment has no grammar to protect.
- **"Coordinate" carries the always-explain rule** (H3): every doc surface
  that says the word states its lineage (Maven coordinates).
- **`DSTOW_HOOK_PACKAGES` is newline-separated**, one canonical FQN per line;
  `while IFS= read -r pkg` is the documented idiom. Canonical encoding of
  control characters makes one-per-line airtight (H4).
- **cwd = the scope's own directory** (H5): package dir / repo dir /
  `$XDG_CONFIG_HOME/dstow/`. Hooks never inherit the user's incidental cwd.
- **Both hook streams land on dstow's stderr; stdin passes through** (H6).
  Nothing a hook prints is dstow's answer to the question asked — hook
  output is commentary definitionally. A hook that needs to emit data writes
  a file or is invoked directly.
- **Write commands refuse from inside a hook; reads stay fully allowed**
  (H7). Detection: `DSTOW_HOOK_ACTION` present in the environment. Deploy
  verbs, adopt, clean, rebuild, and repo mutations refuse with a §1.4 error
  naming cause and remedy; `list` / `info` / `status` / `check` all work — an
  install hook may read dstow (`dstow status --json`, `dstow info -f target`)
  while installing tools with its own commands. No bypass in v1:
  refuse-now-allow-later is additive; the reverse breaks users.

## 6. Ledger

*Resolution: [Ledger format (#21)](https://github.com/rocne/dstow/issues/21);
trade-off record [ADR 0001 — the ledger is a current-state index, not a journal](adr/0001-ledger-is-a-current-state-index.md).*

### 6.1 Format

**A current-state index** — entries are links dstow currently believes
exist; unstow deletes entries; no history. **One JSON document per machine**
at `$XDG_STATE_HOME/dstow/ledger.json` (machine state in JSON, human config
in TOML — each format in its conventional lane). Pretty-printed as a
write-side courtesy only.

```json
{
  "version": 1,
  "targets": {
    "/home/rocne": [
      {
        "link": ".config/nvim",
        "package": "github:rocne/dotfiles::nvim",
        "source": "dot-config/nvim",
        "destination": "../repos/github/rocne/dotfiles/nvim/dot-config/nvim",
        "recorded_at": "2026-07-15T09:12:03Z"
      }
    ]
  }
}
```

- `targets`: absolute target root → entry list. The grouping is
  load-bearing: it is rebuild's replace boundary.
- `link` is target-relative and `source` package-relative — exactly how
  `stow.Expected` keys and values its results; `stow.Owner` recovers the
  same, so ledger, Expected, and Owner compose directly.
- `package`: canonical percent-encoded FQN (one field — the grammar already
  unified repo+package).
- `destination`: the literal symlink text as written — the **damaged**
  evidence, comparable against disk without recomputation.
- `recorded_at`: RFC 3339 UTC, scoped strictly to "when this entry entered
  the ledger" — it claims nothing about any file or action, so disk-is-truth
  can never contradict it.
- Deliberately absent: a fold flag (derivable), the repo directory (config's
  business; storing it invites stale copies).

### 6.2 Atomicity and locking

- Every write: marshal full document → temp file **in the same directory** →
  fsync → rename. Readers and crashes see old or new, never torn.
- Writers take an exclusive advisory `flock` on sibling `ledger.lock`, held
  for the **whole operation** (read ledger → filesystem work → write
  ledger). Pure readers take no lock. Contention **fails fast** with a clear
  error (dpkg/pacman precedent).

### 6.3 Pruning discipline

- **Reads never write.** `list`/`info`/`status`/`check` are lock-free,
  mutation-free, idempotent. §8.1's "pruned on sight" is realized by
  writers.
- **Scoped pruning.** A writer prunes contradicted entries in its scope:
  entries of the packages it operates on, plus entries at paths it touches
  anyway. Unrelated damage evidence survives.
- **`clean` is the ledger-wide broom** (its scope is by definition the whole
  ledger).

### 6.4 Command contracts

- **check** (read): no lock, writes nothing; reads ledger + config + repo
  package trees ("no tree walk" means no *target* walk; each entry costs one
  lstat/readlink). Classifies **broken** (destination gone), **orphaned**
  (resolves into a known repo, current config wouldn't produce it),
  **contradicted** (disk disagrees with the entry).
- **clean** (write): takes the lock, **recomputes check's plan fresh under
  the lock** (never a stale saved report), then: broken → remove link +
  entry, freely; orphaned → remove link + entry behind confirm /
  non-interactive error / force; contradicted → **prune entry only, never
  touches disk** — freely but loudly, each prune reported with its evidence.
  One atomic ledger write at the end.
- **rebuild** (write): walks every currently configured target root,
  recursively, lstat-based, never descending through symlinks. Each symlink
  found is tested with `stow.Owner` against known repos; owned links become
  entries (`destination` from readlink, `recorded_at` = now). Hand-made
  links into repos get ledgered — that is how §8.2's hand-made-link example
  later surfaces as an orphan. **Replace per scanned target group**: rebuild
  wholesale-replaces the entry groups of targets it scanned and leaves
  unscanned groups untouched.
- **Deploy verbs**: stow/adopt add entries for links created, unstow deletes
  entries for links removed, restow does both — each under the standard lock
  + atomic write, pruning contradicted entries in scope as they go.

### 6.5 Versioning and failure modes

- `version` marks the schema; bumped only on incompatible change.
- Missing file = empty ledger, never an error.
- Version newer than this dstow → refuse loudly ("written by a newer dstow;
  upgrade"); never guess, never rewrite down.
- Version older → migrate forward in memory; the file is rewritten in the
  new schema only when a write command commits.
- Corrupt/unparseable → refuse loudly, name the path, point at the remedy
  (`rebuild` or restore). **Corruption must never degrade into amnesia.**

## 7. Output

*Resolutions: [Output design (#26)](https://github.com/rocne/dstow/issues/26) —
ledger O1–O12; [Theming survey (#31)](https://github.com/rocne/dstow/issues/31).
Flow mockups: the output-design prototype (branch `prototype/output-design`);
presentation specifics there are provisional, the rules here bind.*

### 7.1 The three governing rules

1. **stdout is data, stderr is commentary — absolute** (O1). The thing you
   asked for (listings, status, JSON, snippets) goes to stdout, pure;
   everything dstow *says about* the work (notes, warnings, errors, prompts,
   progress) goes to stderr. `dstow snippet rc >> ~/.bashrc` and
   `dstow status --json | jq` are clean by construction.
2. **Severity is a word, color is a reinforcement** (O2). Every commentary
   line starts with a greppable prefix — `note:` / `warning:` / `error:` /
   `fix:` — colored but meaningful uncolored, padded to `warning:`'s width so
   stacked commentary aligns *(padding provisional — revisit at
   implementation if it looks weird)*. The **`fix:` line makes §1.4
   structural**: every refusal is followed by a runnable command or config
   pointer; blue, not green (green means success, and a fix appears precisely
   when nothing succeeded).
3. **Semantic slots, not colors, in the code** (O3). One owned printer maps
   CONTEXT.md vocabulary to styles; nothing outside it ever names a color.
   Test contract: `strip_ansi(styled) == plain` (O11).

### 7.2 Palette, prompts, quiet

- **The two-stage vocabulary** (ruled 2026-07-20 —
  [#115](https://github.com/rocne/dstow/issues/115), replacing the flat
  internal-term palette): theming speaks the generic roster (§3.3); dstow's
  internals reach it through a fixed mapping. Semantics first — every
  internal maps to the semantically proper slot; slots carry sensible values
  on their own merits.
  - **Stage 1 — the roster.** Content: `section1`/`section2` (section
    headings), `name1`/`name2` (canonical names / secondary names and
    aliases), `value1`/`value2` (literal values and defaults / placeholders
    and meta). Messages: `error` (breakage), `warning` (attention),
    `success` (good), `info` (neutral-notable), two prominence tiers each.
    Slots may ship before any internal consumes them (`section2` `name2`
    `value1` `success1` in v1) — roster completeness over deferral.
  - **Stage 2 — the internal mapping**: code-owned, closed in v1 (no
    per-internal override surface), one owner in code. heading→`section1` ·
    name→`name1` · muted→`value2` · error:→`error1` · warning:→`warning1` ·
    fix:→`info1` (actionable guidance, prominent) · note:→`info2` (FYI,
    quiet) · stowed→`success2` · partially_stowed→`warning2` ·
    not_stowed→`info2` · occupied→`info1` (CONTEXT: deliberately neutral) ·
    damaged→`error1` · contradicted→`error1` · drifted→`warning2` ·
    broken→`error2` · orphaned→`warning2`. Slot-sharing is the point:
    sameness that was prose discipline (contradicted ≡ damaged, orphaned ≡
    partially stowed) is now structural.
  - **Defaults**: the palette declares the seven tier-1s only; every tier-2
    derives (§7.3). section1 bold brightgreen (cargo HEADER) · name1 bold
    brightcyan (cargo LITERAL) · value1 bold cyan (tier-up of cargo
    PLACEHOLDER) · error1 bold brightred (cargo ERROR) · warning1 bold
    yellow (cargo WARN) · success1 bold green (cargo GOOD) · info1 bold
    brightblue (blue-for-info convention; fix's historically blue slot —
    blue re-enters the defaults with the info family). Prior-art grounding
    per the 2026-07-19 cargo ruling (clap-cargo `style.rs`). The tier-1
    floor binds the default palette only; themes at any layer may be sparse
    (the north-star), including sparse-by-provenance bundled presets.
- **The O4 promise**: dstow's *defaults* emit only the 16 base ANSI slots, so
  the terminal theme rethemes dstow automatically and colorblind/low-vision
  users retheme through terminal preferences (gh's Accessible pattern made
  the only default mode). User choices may exceed ANSI-16 (the grammar
  allows 0–255/hex); the promise binds defaults only.
- **`--color <when>`** requires a value: auto (default) / always / never; no
  bare `--color`, no `--no-color` sugar — NO_COLOR covers the env side (O6).
- **Help is colorized** (ruled 2026-07-19 —
  [#96](https://github.com/rocne/dstow/issues/96)): help renders through the
  same stack as everything else — section headings via `section1`, command
  and flag names via `name1`, placeholder/annotation text via `value2` —
  strictly downstream of the §7.3 enable chain, so piped, `NO_COLOR`, and
  `--color=never` help is plain text. One slot vocabulary: no help-specific
  slots.
- **`--quiet` drops routine chatter only** (O7): no-op lines, all-good
  summaries, progress. Announcements (§1.3), warnings, errors, and `fix:`
  lines always survive.
- **Per-package run lines** (O8): `verb name result` one-liners, failure
  detail indented under its line, summary last, exit per §3.2.
- **Confirm polarity** (O12): destructive/bulk defaults No (`[y/N]`);
  benign-continue defaults Yes (`[Y/n]`, e.g. the encoding confirm).
- **JSON conventions** (O10): per-command shapes; lower_snake keys;
  CONTEXT.md state strings verbatim (including the space in
  `"partially stowed"`); FQNs always present. `info --json` is a flat object
  keyed by canonical field name (an array of scope objects under `-r`).

### 7.3 Theming — the dircolors architecture

Layered, top wins, all strictly downstream of the enable/disable chain
(`--color` > `NO_COLOR` > `CLICOLOR_FORCE` > `CLICOLOR` > TTY+`TERM=dumb`) —
theme config can never re-enable color the chain turned off:

1. **`DSTOW_COLORS`** — the only theming env var: packed per-slot overrides,
   LS_COLORS-family syntax (`error1=bold red:success2=#a6e3a1`), values in
   git's `color.*` grammar. Populated by hand or by generator:
   `export DSTOW_COLORS=$(dstow theme emit catppuccin-mocha --format env)`.
   There is no
   `DSTOW_THEME`.
2. **`[color]` TOML table** — global config, one key per slot, same grammar.
3. **`theme` config key** — name or path per the operand rule: a bare string
   is a theme *name* (user themes dir first, then bundled presets — TOML
   files embedded via go:embed, one loader); a path form is a theme *file*
   anywhere — including inside a repo, which makes repo-shipped themes free.
4. **The default palette** (§7.2 — the seven tier-1 declarations, ANSI-16).

**Tier derivation** (ruled 2026-07-20 —
[#115](https://github.com/rocne/dstow/issues/115)): after the stack composes
per-slot (top wins), any slot still undeclared derives from its family's
effective tier-1 by attribute step-down — **remove bold if present, else add
dim**. Attribute-only, so it works identically for named ANSI, 256, and hex
values. A declared value at any layer always beats derivation; the default
palette's tier-1 floor guarantees every slot resolves. Sparse themes get
whole-family coherence from tier-1 declarations alone.

**North-star invariants (bound in v1)**: a theme file is exactly the bare
`[color]` schema, no wrapper keys; the packed string, the config table, and
theme files share one slot vocabulary and one value grammar — losslessly
convertible between representations, forever. v1 ships
`dstow theme emit [<name>] [slot=value ...] [--format env|toml]` (the
emission path), `dstow theme slots` (the slot-vocabulary reference), and
`dstow theme list` (the roster).

## 8. Go architecture

*Resolution: [Go architecture (#29)](https://github.com/rocne/dstow/issues/29) —
ledger A1–A20 as amended. Altitude: module boundaries, public interfaces,
gostow-consumption seams; ephemeral internals are implementation's.*

### 8.1 Layout and wiring

- **Module `github.com/rocne/dstow`**; `cmd/dstow/main.go` contains only
  wiring (streams, version via goreleaser ldflags, `os.Exit`); everything
  else under `internal/` — no public Go API in v1. **Eleven packages**:
  `cli, ui, name, config, repo, engine, ignore, ledger, hooks, git, ops`
  (A1 as amended — `deps` removed with the dependency concept).
- **`cli.Run(args, version, stdin, stdout, stderr) int`** —
  constructor-injected commands over an app struct, no package-level command
  globals. Environment read via `os.Getenv` at point of use. cobra
  (v1.10.x, no viper) with `SilenceUsage`+`SilenceErrors`; all `RunE` errors
  are typed domain errors that `cli` alone maps to exit codes and renders
  through the printer — one owner for §1.4 wording. Help stays on stdout
  (help *is* the requested data) (A2). **Help is cobra-generated** (A2,
  amended per the content-not-bytes ruling —
  [#96](https://github.com/rocne/dstow/issues/96), 2026-07-19): every
  command carries its Short/Long/Example derived from its `docs/commands/`
  page (§2.3/§2.4) and its flag usage strings from its own flag definitions;
  the root's command sections are cobra command groups. No bespoke help
  renderer, no verbatim byte-pinning — the parser surface and the help
  surface share one definition, so a flag appears in help by construction. Help renders themed through `ui` (§7.2), strictly
  downstream of the §7.3 enable chain; disabled color emits plain text (O11
  strip contract).
- **Exit-code map** (A3), mapped in exactly one place in `cli`:
  `0` success · `1` negative answer (any package failed; a requested field
  applicable but unset/empty; check found findings) · `2` usage error ·
  `3` refusal/environment (non-interactive ambiguity, corrupt ledger, lock
  contention, ledger-from-newer-dstow).

### 8.2 Output plumbing

- **`ui` is the only module that touches the streams** (A4). Every other
  module returns data — diagnostics included (config warnings come back as
  values; `ui` renders them). This law makes `--json`, `--quiet`, and
  strip-roundtrip structural.
- **Printer internals** (A5): fatih/color per-instance (never the
  package-global `NoColor`); the per-stream enable precedence of §7.3;
  theming stack resolved strictly downstream of enable; go:embed presets +
  user themes dir behind one theme loader, inside `ui`; the git-grammar
  color-value parser lives in `ui` — one owner for the one value grammar.
- **Interactivity seam** (A6): `ui.Interactive()` = stdin TTY **&&** stderr
  TTY (prompts live on stderr); injectable for tests.
- **fang is ruled out** (A18): experimental, lipgloss v2 + colorprofile is a
  second styling engine (the two-engines smell), auto commands collide with
  the pinned leaves. (Its former third ground — restyling "help approved
  verbatim" — retired with the A2 amendment; the rejection stands on the
  other two.) Engine unification: **fatih/color alone**.

### 8.3 Domain modules

- **`name` is pure** (A7): FQN parse/format, percent encode/decode,
  segment-boundary suffix matching, `::` kind-forcing, path-vs-name
  classification. Zero I/O, zero deps.
- **`config`** (A8): the four-level chain, legality matrix, use-time path
  expansion, warnings-as-data, stowrc discovery/slotting/mapping/supplement
  diff consuming `gostow/stowrc`, content-sniff routing. The metadata
  location is hidden behind one accessor.
- **`repo`** (A9): the repo set (registry read/write with the same
  temp+fsync+rename discipline as the ledger; `DSTOW_PATH`), managed-dir
  layout, scheme dispatch (github/local, internal — no exported plugin seam
  until a third scheme exists), package enumeration, name resolution over
  the set (via `name`). Cross-domain guards compose in `ops`, not here.
- **`ledger`** (A10): [§6](#6-ledger) verbatim. `Load` = lock-free read
  snapshot; `Update(scope, fn)` = flock for the whole operation → scoped
  pruning → atomic write.
- **`hooks`** (A11): discovery via `config`'s metadata accessor, nested/LIFO
  ordering, once-per-invocation firing, no-op semantics, §9.1.4 failure
  rules; the context contract of [§5](#5-hook-context-contract).
- **`ops` is the app core** (A13): the verbs as deep modules returning
  structured results — deploy (per-package independence loop, nested/LIFO
  hook ordering, ledger transactions), maintenance (clean recomputes check's
  plan under the lock), views, repo ops, snippet, colors. One package;
  internal file organization is implementation's.

### 8.4 Seams to third parties

- **gostow** (A14): pin `>= v0.3.0`, moving to `>= v0.4.0` when cut (the
  `IgnoreFunc` seam ships in it). `NoGlobalIgnoreFile: true` always;
  `FixQuirks: true` for engine operations; stowrc *compat parsing* stays
  quirk-faithful; `Options.Log = io.Discard` (dstow reports from `Result`,
  never engine prose); per-package `Apply` calls so §3.2 independence holds;
  `Expected`/`Owner`/`Conflict{Path,Kind}` mapped into dstow's typed results
  in `engine`, nowhere else.
- **Ignore languages stay in their lanes** (A15): compat stow-regex chains
  ride gostow's own `Options.Ignore`; native gitignore-glob chains are
  matched dstow-side by `ignore`, wrapping go-git's
  `plumbing/format/gitignore` (verified full C16 semantics; doublestar
  verified not gitignore-semantic — disqualified). Negation and `//` never
  reach the matcher (refused at config parse).
- **Engine-ignore seam** (A16): `stow.Options.IgnoreFunc` — landed upstream
  (gostow#41); lstat-based `isDir`; additive-only (exclude more, never
  resurrect what stow ignores). The pre-match fallback is moot — do not
  implement it.
- **git = system git behind a port** (A17): `git.Port` in repo's terms
  (Clone, Fetch, FFApply old→new, AheadBehind, HasLocalWork), exec adapter
  for production, fake for tests. go-git rejected — credential-helper and
  ssh-config fidelity is exactly what dotfiles users depend on. Missing git
  binary = clean §1.4 error at remote-scheme operations only.

### 8.5 Pins and completion

- **Managed directory** (A19):
  `$XDG_DATA_HOME/dstow/repos/<scheme>/<owner>/<name>` — data, not state,
  not cache: links point into it. Directory names use the canonical
  percent-encoded segments, filesystem-safe by construction.
- **Dynamic completion** (A20): `ValidArgsFunction` completes package/repo
  names and schemes through the same `repo` resolver, best-effort-silent
  (any error → no completions, never diagnostics), config loaded in quiet
  mode, never fires hooks or network.

## 9. Bootstrap and distribution

*Resolutions: [Bootstrap snippet and installer design (#28)](https://github.com/rocne/dstow/issues/28) —
ledger B1–B9; [Release-ci and installer wiring survey (#19)](https://github.com/rocne/dstow/issues/19).*

### 9.1 The rc snippet

Emitted by `dstow snippet rc`, to stdout, composable (`>> ~/.bashrc`); dstow
never edits rc files. **Present ⇒ silent, invisible, offline** — the
installer is never even fetched.

- **Canonical text is the vendored `snippet.sh`** (B1, amended per
  release-ci D26 — [#64](https://github.com/rocne/dstow/issues/64)): the
  snippet is *authored* in release-ci and vendored at dstow's repo root
  beside `install.sh`, propagated the same way (B3). The contract the text
  keeps, whatever its bytes: the PATH line bakes the contractual default
  install dir (`~/.local/bin`, §9.2 B6) and is prepended *before* the guard;
  the guard short-circuits before any network when the tool is present; the
  snippet never breaks shell startup and leaves no variables behind.
- **Every snippet emission is a go:embed document** — real files, diffable,
  shellcheck-able in CI; never string literals (B2). `dstow snippet rc`
  embeds the vendored `snippet.sh` itself — one file, one owner, zero
  transcription drift.

### 9.2 The installer

- **Vendored canonical** (B3): `install.sh` at dstow's repo root, served
  raw-on-main from dstow's own URL; *authored* in release-ci, propagated
  downstream by push-based PRs. **dstow's first release blocks on
  [release-ci#1](https://github.com/rocne/release-ci/issues/1)** — no
  fallback design.
- **Invocation** (B4): bare `curl -fsSL …/rocne/dstow/main/install.sh | sh`;
  the repo slug is baked as the vendored default; no arguments.
- **The silence matrix** (B5): rc + present = nothing; rc + absent = installs
  and announces; direct invocation always speaks (one status line + exit 0
  when present — "exits cleanly" means exit 0, not muteness).
- **The installer contract is a floor** (B6, amended per the release-ci
  counter-offers dstow accepts — [#64](https://github.com/rocne/dstow/issues/64)):
  presence is the **dual check, install path first** (release-ci D16 —
  `command -v` alone never converges for an off-PATH install dir; it remains
  correct only inside the rc snippet, which fixes PATH first); `--force`;
  `--version vX.Y.Z` is **ensure-exactly** (D28: already at that version ⇒
  exit 0, otherwise install; it does *not* imply force); the install dir is
  **tunable with the default as the contract** (D9/M1: flag > namespaced env >
  `XDG_BIN_HOME` > `~/.local/bin` — the *default* is what the snippet bakes);
  checksum mandatory, cosign opportunistic. Anything above the floor is
  release-ci's to grow freely; the snippet depends on nothing above it.

### 9.3 Release wiring

Inherited wholesale from release-ci (proven by gostow and dot-dagger;
copy-paste-and-rename): the single reusable `workflow_call` workflow
(GoReleaser build → SLSA attestation → cosign/Fulcio verify → Cloudsmith
apt/dnf smoke), a `.goreleaser/dstow.yaml`, a release-please workflow plus
break-glass tag-push workflow, and three repo secrets set manually before
first release: `GPG_PRIVATE_KEY`, `HOMEBREW_TAP_GITHUB_TOKEN`,
`CLOUDSMITH_API_KEY`.

## 10. Reserved doorways

v1 behavior deliberately keeps these doors open. Each binds a *refusal or
reservation* in v1; none ships semantics.

| Doorway | v1 binds | Future |
|---|---|---|
| `@` coordinate suffix | literal `@` encodes as `%40` ([§1.2](#12-encoding)) | scheme-interpreted suffix; semantics belong to the interpreting scheme ([#20](https://github.com/rocne/dstow/issues/20), decided 2026-07-16) |
| Identifier aliasing | resolution rule stated; zero instances ([§1.4](#14-identifier-resolution-and-aliasing)) | v2 field aliases, prefix resolution (F12) |
| `//` ignore prefix | refused as reserved (C16) | possible regex marker (maybe-v2) |
| `!` ignore prefix | refused (additive law) | — (negation stays forbidden) |
| `<hook>.d/` dirs | eight spellings inert-and-warned (M7) | **committed v2**: run-parts drop-ins with a main-file delegation point |
| Bare field namespace | all of it dstow's; built-ins + aliases reserved (F14) | v2 user properties arrive dot-qualified, never bare |
| `.dstow/` top level & global config dir | unknown entries warn, never refuse (M5) | future kinds addable without a schema version |
| `fix:` line | structural, machine-stable runnable remedy (O2) | maybe-v2 fix-runner |
| Theming convertibility | one slot vocabulary + one value grammar across env/table/file (§7.3) | converter/builder family; first-class repo-shipped themes (maybe-v2) |
| Hook write-refusal | H7 refuses writes inside hooks | maybe-v2 hook-orchestrator (refuse-now-allow-later is additive) |
| `DSTOW_HOOK_*` namespace | absent-not-empty contract (H2) | scheme-specific variables (maybe-v2) |
| `packages_dir` kind-first convention | `"packages"` documented convention (M3) | repo aggregator sibling room (maybe-v2) |
| `snippet` group | `rc` only (B8) | `snippet hook <name>` skeletons — candidate v1.1 |
| Interactive selection | ranked candidate & ambiguity lists named as `fix:` remedies; no interactive picker ships, in any context ([§2.4](#24-per-command-help) adopt, [§1.2](#12-encoding)) | a terminal picker that reads the choice interactively (maybe-v2) |

ADRs: [0001 — the ledger is a current-state index](adr/0001-ledger-is-a-current-state-index.md);
[0002 — no dependency concept](adr/0002-no-dependency-concept.md) (superseded
design preserved in [dev/attic/dependencies.md](attic/dependencies.md)).
Every other resolution ran the ADR three-part test and minted none.

## 11. Provenance

Each section's binding detail and full rationale live in its ticket's
resolution (as amended — read resolution comments *and* addenda):

| Section | Ticket(s) |
|---|---|
| §1 Naming grammar | [Naming grammar (#20)](https://github.com/rocne/dstow/issues/20); `@` reservation decided on [Draft DESIGN.md (#30)](https://github.com/rocne/dstow/issues/30) |
| §2 CLI surface | [CLI surface (#22)](https://github.com/rocne/dstow/issues/22); [info: the property read (#27)](https://github.com/rocne/dstow/issues/27); [CLI framework survey (#18)](https://github.com/rocne/dstow/issues/18) |
| §3 Configuration | [Config format survey (#17)](https://github.com/rocne/dstow/issues/17); [Config schema (#23)](https://github.com/rocne/dstow/issues/23) |
| §4 Metadata directory | [Metadata directory (#24)](https://github.com/rocne/dstow/issues/24) |
| §5 Hook context contract | [Hook context contract (#25)](https://github.com/rocne/dstow/issues/25) |
| §6 Ledger | [Ledger format (#21)](https://github.com/rocne/dstow/issues/21); [ADR 0001](adr/0001-ledger-is-a-current-state-index.md) |
| §7 Output | [Output design (#26)](https://github.com/rocne/dstow/issues/26); [Theming survey (#31)](https://github.com/rocne/dstow/issues/31) |
| §8 Go architecture | [Go architecture (#29)](https://github.com/rocne/dstow/issues/29); [CLI framework survey (#18)](https://github.com/rocne/dstow/issues/18) |
| §9 Bootstrap and distribution | [Bootstrap design (#28)](https://github.com/rocne/dstow/issues/28); [Release/installer survey (#19)](https://github.com/rocne/dstow/issues/19) |
| §10 Reserved doorways | the map's Out of scope section + the tickets cited per row |
| Dependency removal (throughout) | [Bootstrap design (#28)](https://github.com/rocne/dstow/issues/28); [ADR 0002](adr/0002-no-dependency-concept.md); [consistency sweep (#32)](https://github.com/rocne/dstow/issues/32); [judgment calls (#33)](https://github.com/rocne/dstow/issues/33) |

Synthesized 2026-07-16 on [Draft DESIGN.md (#30)](https://github.com/rocne/dstow/issues/30)
from the [dstow v1 design map (#16)](https://github.com/rocne/dstow/issues/16).
