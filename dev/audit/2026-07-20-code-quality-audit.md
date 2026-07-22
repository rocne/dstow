# dstow code quality & consistency audit — 2026-07-20

- **Auditor:** Claude (Fable 5), single session, read-only — no code changed.
- **Commit audited:** `2157bea` (main, clean tree).
- **Scope:** internal code-with-code consistency: single ownership, unified I/O,
  duplication, library leverage, Go idiom, in-code vocabulary conformance, test
  intent. Docs-accuracy audit (README/CHANGELOG/help prose vs. behavior) is
  **explicitly out of scope** — earmarked for a lower-cost model; handoff notes
  at the bottom.
- **Method:** full read of `ui`, `engine`, `cli` (core files), `ops` (core
  files), `config/level.go`, `name`, `ledger`; targeted greps across the rest
  (~12.8k non-test LOC, ~10.2k test LOC, 344 test funcs, 7 e2e scripts);
  `gofmt`/`go vet` (both clean); full `go test ./...` run.

## Overall grade: A−

This is an unusually disciplined codebase. The architectural laws (A-numbers),
spec sections, and CONTEXT.md vocabulary are not aspirational — they are cited
at the exact code sites that implement them, and several invariants are
*structurally* enforced rather than promised in prose (e.g. the slot reference
derives from the role mapping it documents, so it cannot drift; `Contradicted`
is one method consulted by both report and prune paths). Stream ownership is
airtight: outside `internal/ui`, only `cmd/dstow/main.go` touches a real
stream. The findings below are real but small; nothing indicates rot.

| Axis | Grade | One-line verdict |
|---|---|---|
| Single ownership | A− | Structurally enforced in the hot spots; 4 duplication sites found (F2, F5, F6, F1-root-cause) |
| Unified I/O | A | Only `main.go` touches real streams; all diagnostics are data until `cli` renders them |
| Duplication vigilance | B+ | A handful of repeated logic blocks that a future change would have to touch in lockstep |
| Library leverage | A− | Good choices, well-scoped (go-git used *only* for its gitignore parser); footprint note in F11 |
| Go idiom & conventions | B+ | gofmt/vet clean, consistent style — but consistently pre-Go-1.21 idiom on a Go 1.26 module (F9) |
| Vocabulary conformance | A− | Roles/states speak CONTEXT.md verbatim; three small leaks (F4, F7, F8) |
| Test suite | A− | 344 unit tests + e2e scripts, intent-anchored assertions; one non-hermetic test **currently failing locally** (F1) |

---

## Findings (ranked)

### F1 — `TestCompleteSessionRepo` is non-hermetic and fails on this machine (HIGH)

`go test ./...` fails today: `internal/cli/complete_test.go:53`.

Root cause chain, verified:

1. `adrg/xdg` snapshots `XDG_*` at package init, so `t.Setenv` alone does not
   redirect it. The repo **knows** this — `theme_test.go`, `config/paths_test.go`,
   `ops/helpers_test.go`, `repo/managed_test.go`, and `ledger_test.go` all
   follow a documented `t.Setenv` + `xdg.Reload()` + `t.Cleanup(xdg.Reload)`
   pattern (the comment in `theme_test.go:15` even explains why).
2. `complete_test.go` hand-rolled the same env setup and **omitted
   `xdg.Reload()`**. `config.RegistryFile()` therefore still points at the real
   `~/.config/dstow/repos.toml`, which registers `local:/tmp/tmp.YlXxJ4SkxK/dots`
   (a live repo with a `zsh` package). That second `zsh` makes the bare `zsh`
   completion ambiguous, so the test's expected candidate disappears.
3. CI's clean HOME masks it — the test only fails on a machine with a real
   registry. This is exactly the "CI green must gate what it claims to gate"
   concern: the suite is green in CI while red on the dev machine.

**Deeper cause (the single-ownership angle):** the hermetic-env setup is
duplicated by hand across six-plus test files instead of being owned by one
shared helper. One copy drifted; this failure is the drift made visible.

**Later:** extract one `setHermeticEnv(t)` test helper (env + `xdg.Reload` +
cleanup) and route every test through it; fix `complete_test.go` by adoption.
Separately worth recording: dstow's own law is env-at-point-of-use (A2), and
the xdg dependency's init-time snapshot quietly deviates from it — harmless in
production (env doesn't change mid-process) but the source of this entire trap.

### F2 — `engine.Op` is hand-assembled at four sites (MEDIUM)

`ops/deploy.go:272`, `ops/adopt.go:81`, `ops/classify.go:273`,
`ops/statusclass.go:128` each build
`engine.Op{Dir, Target, Package, Fold, TranslateDotPrefixes, Ignores}` from an
`Effective` chain by hand. The engine's doc comment stakes the invariant
"observation and deployment can never disagree" — but the thing that keeps
those four constructions in agreement is currently reviewer eyeballs. Adding a
fifth effective knob means four touch points and a silent-drift risk at each.

**Later:** one constructor (e.g. `opFor(eff, dir, target, pkg) engine.Op`) so
the field mapping has a single owner. Small change, closes the invariant
structurally.

### F3 — Observation-failure epistemics differ between `status` and `check` (MEDIUM)

`check` (classify.go) has an explicit `ClassUnobservable` for entries whose
observation failed non-ENOENT — a read-only row, no claim made ("no unbacked
claims"). But `status` (statusclass.go:219–242 `classifyLink`) maps the same
situations to **`LinkOccupied`**: a failed `Lstat`, a failed `Owner`
resolution, all become "occupied" — a positive claim ("the spot is taken")
without a sighting. The two maintenance reads answer the same epistemic
question with different postures. `occupied` is defined in CONTEXT.md as
deliberately neutral about *how* the spot is taken, but it still asserts *that*
it is taken, which an EACCES error does not establish.

**Later:** decide whether `LinkState` needs an `unobservable` member (mirroring
check's class) or whether the CONTEXT.md definition of occupied should be
widened to cover "cannot observe". Either is fine; the inconsistency is the
defect.

### F4 — Spec citations leak into three user-visible error strings (LOW, easy)

- `ops/deploy.go:168` — "`--adopt does not compose with unstow (D15: it pre-accepts…)`"
- `ops/deploy.go:470` and `:474` — "`blocked: a … hook failed (§9.1.4)`"

These reach end users via error rendering. Everywhere else the error philosophy
is complete self-standing prose with the remedy named; "D15" and "§9.1.4" are
internal design-doc coordinates a user cannot follow. (Spec references in
*comments* are pervasive and excellent — this is only about strings that print.)

### F5 — `kindOf` / `kindOfMode` are the same function twice (LOW)

`ledger/ledger.go:169` and `ops/statusclass.go:270` are the identical
directory/regular-file/non-symlink-file switch, both feeding evidence prose.
Two owners of one vocabulary; if the wording ever changes, evidence strings
diverge between check/prune and status.

### F6 — Themed two-column table rendering is duplicated (LOW)

`cli/misc.go` `theme list` (~lines 95–125) and `theme slots` (~160–180) each
re-implement: width scan over plain names, heading-styled header on stderr
gated by `e.quiet`, pad-on-plain-then-style rows (the ANSI-width trick, with
the comment duplicated too). A third themed table will make it three.

**Later:** one small table-rendering helper in `cli` (or `ui`).

### F7 — `engine.mapConflictKind` fails silent where `mapTask` fails loud (LOW)

`engine/engine.go:338–354`: an unknown gostow conflict kind returns
`ConflictKind(0)` — an invalid zero value (the enum starts at `iota+1`) with a
comment acknowledging it. Its sibling `mapTask` returns `ok=false` and the
caller raises "engine planned an unmappable task" precisely so new gostow
behavior surfaces loudly. Same seam, two postures; a new gostow conflict kind
would silently render as a zero-valued kind instead of surfacing.

### F8 — Small vocabulary slips inside code (LOW)

- `cli/deploy.go:144` — a variable named `slot` holds a `ui.Role`. The
  two-stage roles→slots split is this project's own hard-won distinction; the
  misnomer is exactly the confusion the vocabulary exists to prevent.
- `cli/json.go:40` — machine-output key `"root"` on the repo row. CONTEXT.md
  retires "root", allowing it only as *informal prose* for a repo's directory
  location; a stable JSON schema key is arguably the opposite of informal
  prose. Consider `"path"`/`"dir"` before the JSON surface hardens (0.x is the
  cheap time to rename — no-backwards-compat rule applies).
- `cli/json.go:244` — `checkJSON` sits under the `--- theme slots ---` section
  marker; its type `jsonFinding` is under `--- check ---`. Organization nit.

### F9 — Consistently pre-Go-1.21 idiom on a Go 1.26 module (LOW, batch)

Hand-rolled `maxInt` (`cli/views.go:376` — builtin `max`), `containsStr`
(`ops/classify.go:328`), `contains` (`complete_test.go`), `sortedKeys`
(`statusclass.go:282` — `slices.Sorted(maps.Keys(m))`), plus `sort.Slice`/
`sort.Strings` throughout where `slices.Sort`/`slices.SortFunc` are the modern
spelling, and `containsByte` (`name/name.go:139` — `strings.IndexByte`; though
`name`'s zero-dep purity is a stated goal, stdlib `strings` is already
imported). The style is *internally consistent*, which counts for a lot — but
it's the 2019 dialect. One mechanical modernization pass would also delete
several tiny helpers outright (single-ownership win: the stdlib becomes the
owner).

### F10 — `ops/repremove.go` filename typo (TRIVIAL)

`repremove.go` is missing the "o" its siblings have (`repoadd.go`,
`reposync.go`, `reposource.go`). Rename-only.

### F11 — go-git dependency footprint (INFORMATIONAL, deliberate-looking)

`go-git/v5` is imported **only** for `plumbing/format/gitignore`
(`internal/ignore`) — correct parser reuse, and *not* in tension with the
"shell out to system git" decision (that rejection was about git operations,
not pattern-matcher reuse). But it drags go-billy, gcfg, go-context, x/net as
transitive baggage for one matcher. Fine for now; worth a line in the record
that this is a conscious trade, and worth re-checking if a lighter
gitignore-matcher library matures.

### F12 — Dual enumerations of the typed-error taxonomy in `cli/errors.go` (WATCH)

`classifyExit` and `fixFor` each enumerate the typed errors independently. The
design is right (exit codes and remedies are different concerns), but a new
typed error must be threaded through both by hand, and nothing fails if one is
forgotten — the error would exit 1 with no fix line, silently. Not worth
restructuring now; worth a test that walks a canonical list of typed errors and
asserts each classifies deliberately.

### F13 — `execute` in `ops/deploy.go` is the complexity ceiling (WATCH)

~160 lines, four levels of nesting, and the global/repo/package hook-blocking
state machine interleaved with the ledger transaction. It is *correct-looking*
and well-commented, but it is the one function in the codebase where a future
change has real regression surface. No action now; if hook semantics ever grow
another level, extract the repo-group loop first.

---

## What is exemplary (keep doing this)

- **Structural drift-prevention.** `slotGloss` lives beside `roleSlot` and the
  slot reference is *derived* by inverting the live mapping (`ui/color.go`);
  `Contradicted` is one method consulted by both check and prune;
  `Apply`/`Expected` share one `options()` builder at the gostow seam. These
  make single-ownership self-enforcing instead of reviewed-for.
- **Warnings-as-data, uniformly.** Every package returns `Warning` values;
  only `cli` renders. Verified by grep: zero stray stream writes.
- **Typed error taxonomy with field-derived remedies.** `fixFor` builds
  remedies from error *fields*, never by parsing messages — and the error
  strings themselves consistently name what's wrong and what to do.
- **The ledger package** is the best file-pair in the repo: lock-free reads,
  flock+fresh-load+atomic-rename writes, fsync'd directory, corruption refuses
  rather than degrading to amnesia, and the forward-migration seam exists
  without inventing migrations ahead of need.
- **`name` package purity**: zero-dep, byte-exact grammar with complete-prose
  parse errors; hand-rolled percent-encoding is *justified* here (custom
  reserved set — net/url's rules genuinely don't match).
- **Comment discipline**: comments state constraints and cite rulings; they are
  traceable to spec sections and dated decisions rather than narrating code.

## Handoff notes for the (smaller-model) docs audit

Incidental observations only — not investigated:

1. Verify every §-reference in code comments still points at the section it
   describes (DESIGN.md/REQUIREMENTS have been amended repeatedly).
2. `cli/cli.go` `shorts` map vs. the §2.3 wording table — mechanical diff.
3. Theme docs: confirm the four bundled catppuccin flavors and the
   `whiskers`/go:generate regeneration note in `ui/theme.go` match the CI job
   that claims to keep them fresh.
4. CHANGELOG spot-check against `git log` for the 0.4.x line.
5. `docs/agents/*` vs. actual label set and issue-tracker usage.

## Suggested sequencing (when work resumes)

1. **F1** — fix the failing test + extract the hermetic-env helper (small,
   restores local green, closes the drift class).
2. **F2 + F5 + F6** — the three mechanical de-duplications, one PR.
3. **F4 + F8 + F10** — string/naming cleanups, one PR (F8's `"root"` JSON key
   is the only one needing a decision from Rocne first).
4. **F3 + F7** — posture decisions (unobservable link state; loud vs. silent
   seam fallback), each a small ticket with a one-paragraph resolution.
5. **F9** — mechanical modernization pass, ideally with a linter rule
   (`gopls`/`modernize` or `staticcheck`) added to lint.yml so it can't regress.
