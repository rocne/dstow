# 09 — Incidental findings in the Go code

**Fable 5, 2026-07-26.** The secondary charge's output
([`07-incidental-findings.md`](07-incidental-findings.md)): what a close read
of all dstow production Go (and gostow's engine/stowrc/getopt/conformance
core) surfaced, at `783c9d8`. Checked against the known list and the two
prior audits before reporting; nothing below re-reports a tracked issue,
though two entries extend one. Flag-don't-fix observed: nothing was changed,
no issues filed. Filing is mechanical from the entries below; whether any
becomes a ticket is Rocne's call.

Format: **where** · **what** · **confidence**. ★ items additionally say what
would enforce the rule in Rust (they double as port-design input, per the
charge).

---

## 1. Confirmed findings

### F1 — `repo add` mints a `github:` identity from any URL host
**Where:** `internal/ops/reposource.go:96-105` (URLForm branch) and
`githubFromURL`, `reposource.go:173-196`.
**What:** `githubFromURL` extracts `owner/name` from *any* URL — it drops the
host without checking it. `dstow repo add https://gitlab.com/foo/bar` (or
`git@gitlab.example:foo/bar`) clones from the given URL but registers the
repo as `github:foo/bar`. Two concrete consequences: (1) the registry
records a false origin — after `repo remove` + re-add from the registry
spelling, `githubHTTPS` would clone `https://github.com/foo/bar.git`, a
different (or nonexistent) repository; (2) the A19 "scheme-namespaced so
collisions are structurally impossible" property is violated — a gitlab
`foo/bar` and a github `foo/bar` collide in identity and clone directory.
The refusal message on the failure path ("v1 clones github sources — use a
github URL") shows the *intent* was github-only, but the accepting path
never checks.
**Confidence:** confirmed (by construction; no host validation exists on the
accepting path).
**Note:** distinct from
[`repo add owner/name -y` proceeding on the github guess (#72)](https://github.com/rocne/dstow/issues/72),
which is about the bare form and `-y`; this is the URL form.

### F2 — ★ The C21 refusal rides a string-prefix match on another repo's error bytes
**Where:** `internal/config/stowrc.go:36-46` (`invalidIgnorePrefix`) and
`:66-78`; downstream `internal/ops/candidates.go:151-156`.
**What:** dstow distinguishes C21's *refusal* (non-RE2 `--ignore` pattern)
from C19's *warn-and-ignore* degradation by testing whether a gostow
diagnostic string starts with `"Invalid --ignore regex "` — the bytes of
gostow's `CompilePattern` message. The contract is held by nothing: gostow's
`stowrc.Result.Errors` is `[]string`, so a rewording in gostow (whose
Stringer docs explicitly reserve the right to reword non-parity prose)
silently converts every C21 refusal into a warn-and-ignore. The failure then
resurfaces later and worse: the un-refused pattern reaches
`stow.CompilePattern` inside `candidateIgnored`, where a compile error
aborts the *entire* candidate enumeration (`AdoptCandidates` returns error) —
a per-level scoped refusal degraded into a run-level failure, the exact
[#139](https://github.com/rocne/dstow/issues/139)-shaped gap between repos
instead of between tickets. Currently holding (bytes verified matching).
**Confidence:** confirmed as a structural hazard; the misbehavior requires a
gostow wording change to trigger.
**Rust enforcement:** the engine crate's stowrc `Result` carries typed
diagnostics (an enum with an `InvalidIgnorePattern` variant); the C21/C19
routing becomes a `match`, and a new diagnostic variant is a compile error at
the routing site. This falls out for free under the port plan
([`08`](08-evaluation.md) §4) — the strongest single ★ item found.

### F3 — ★ Unnormalized session-repo paths mint FQNs whose canonical string does not re-parse
**Where:** `internal/config/paths.go:60-90` (`ParseDSTOWPath` — checks only
`filepath.IsAbs`, never cleans), `internal/repo/managed.go:42-44`
(`pathSegments` — raw split), `internal/repo/set.go:59-66`.
**What:** `DSTOW_PATH=/home/x/dots/` (trailing slash; doubled slashes
likewise) produces a session repo with
`FQN.Coordinate = ["", "home", "x", "dots", ""]`. Its canonical string is
`local:/home/x/dots/`, which `ParseFQN` **rejects** (empty non-leading
segment, `internal/name/fqn.go:144-151`), and which no name expression can
match (`Expr.Matches` tail-alignment fails against the empty last segment) —
so the repo lists, but cannot be named by any operand, and every surface
that promises a paste-able canonical FQN (`DSTOW_HOOK_FQN`/H2, the ledger's
`package` field, `--json` FQNs) emits a spelling dstow itself refuses to
parse. A migrated `~/.stowrc --dir` value flows down the same unnormalized
path (`compose.go:70-72`). By contrast, `repo add` path operands are safe —
`absLocalPath` runs `filepath.Abs`, which cleans.
**Confidence:** confirmed by construction (the parser's own rules reject the
emitted string); not exercised against a live shell.
**Rust enforcement:** an `Fqn` newtype whose constructor validates segments
(no empty non-leading segment), so an unnormalized path cannot become an FQN
without going through a cleaning constructor; `ParseDSTOWPath` cleaning
entries at the boundary fixes the Go side in one line if ticketed.

### F4 — ★ `engine.Apply`'s documented error contract has an untyped escape
**Where:** `internal/engine/engine.go:286-290` (`options()` returns
`ignore.Compile` errors raw) vs the contract at `:210-212` ("Conflicts return
a *ConflictError …; every other engine failure returns an *OpError").
**What:** `ignore.Compile` returns either a `*config.PatternError` or a bare
`fmt.Errorf` (`internal/ignore/ignore.go:44-62`); `options()` passes both
through unwrapped, so `Apply`/`Expected` can return errors that are neither
of the two documented types. Unreachable while config's loaders refuse the
patterns first (belt-and-braces), but the reachable route is exactly F2's
degradation — the two findings compose. `classifyExit` sends the escapee to
the generic exit 1.
**Confidence:** confirmed contract drift; behavior currently unreachable.
**Rust enforcement:** `Apply` returns a closed error enum
(`Conflict(…) | Op(…)`); the compiler forbids returning anything else, so
the prose contract becomes the signature.

### F5 — `mapVerb` silently defaults an unknown gostow action to `stow`
**Where:** `internal/engine/engine.go:327-336`.
**What:** the `default` arm maps any unrecognized `stow.Action` to
`VerbStow` — the same silent-zero-fallback class as
[`mapConflictKind`'s zero value (#125)](https://github.com/rocne/dstow/issues/125),
which names only the conflict-kind site. A future gostow action would render
in conflict rows as the wrong verb rather than surfacing loudly (contrast
`mapTask`, which deliberately reports unmappable). Cosmetic today (gostow
has exactly three actions).
**Confidence:** confirmed; best treated as an addendum to
[#125](https://github.com/rocne/dstow/issues/125) rather than a new ticket.

### F6 — A repo whose config fails to load enumerates packages in the wrong mode, then contradicts its own warning
**Where:** `internal/ops/resolve.go:113-134` (`loadRepoCtxs`: on `loadErr`,
`packagesDir()` returns `""`, so enumeration proceeds in *root* mode) and
`:163-179` (the warning says "its packages are unavailable this run") vs
`entities()`/bulk selection, which still consume `c.packages`.
**What:** with `packages_dir = "packages"` set and a broken repo
`config.toml`, a bulk run enumerates the repo root instead — so the
literal directory `packages` (and any other visible root directory) appears
as a package. Each such work item then fails with the config load error
(prepare marks `StatusFailed`), so nothing deploys — but the run prints
per-package failure lines for package names that don't exist, immediately
after a warning claiming the repo's packages are unavailable. Misleading
output, not wrong mutation.
**Confidence:** confirmed by reading; not executed.

### F7 — A failed symlink after a successful adopt move strands the file without naming where it went
**Where:** `internal/ops/adopt.go:201-215` (`adoptOne`).
**What:** `moveFile` relocates the live file into the package; if the
subsequent `os.Symlink` fails (e.g. the target directory's permissions
changed mid-run), the error recorded is `adopt <file>: <symlink error>` —
the live path is now empty, the content sits in the package, and neither the
message nor a `fix:` line says so. No data is lost, but the user is left to
discover the file's new location themselves, against the §1.4
every-refusal-names-its-remedy posture. The copy-fallback path in `moveFile`
is also non-atomic (partial `dst` on crash), though src survives until the
copy completes, so that half is benign.
**Confidence:** confirmed by reading (failure path not exercised). Low
frequency; UX/robustness rather than correctness.

### F8 — `clean` prompts interactively while holding the ledger flock
**Where:** `internal/ops/clean.go:94-123` (`Prompt.Confirm` inside
`ledger.Update`'s fn) — contrast deploy (`deploy.go:352-358`: adopt
confirmations resolved in `prepare`, before `execute` takes the lock) and
adopt (`adopt.go:94-125`: confirmations before `ledger.Update`).
**What:** a user idling at an orphan prompt holds the exclusive flock, so
every concurrent dstow write fails fast with exit 3 for as long as the
prompt sits. There is a real argument *for* the current shape — D10 requires
clean to act on a plan recomputed under the lock, and prompting outside it
would reintroduce the stale-plan window — so this may be a deliberate
trade-off; but it is nowhere recorded, and the deploy/adopt verbs made the
opposite choice. Worth a written ruling either way (record-retractable
territory), or a design change (classify under lock, prompt outside, re-verify
per-finding under the lock before acting).
**Confidence:** confirmed behavior; "defect" status is a judgment call.

### F9 — `Field.Value` doc-contract drift (cosmetic)
**Where:** `internal/ops/info.go:44-47` ("Value is nil unless Set") vs
`:216-218` (`FieldUnset` stored with `Value: []string{}` for `ignores`).
**What:** deliberate — `cli/json.go:99-104` relies on it so an unset list
serializes as `[]` not `null` — but the field's documented contract says
otherwise. Fix is a comment edit (or a typed value, which the port does
anyway).
**Confidence:** confirmed. Cosmetic.

### F10 — Dry-run against a missing target renders a thinner plan than against an existing one (cosmetic)
**Where:** `internal/ops/deploy.go:283-324`.
**What:** the [#146](https://github.com/rocne/dstow/issues/146) fix plans a
missing-target dry-run from `Expected`, synthesizing only `LinkCreated`
actions with no `Dest` and no `DirCreated` rows; a dry-run against an
existing target comes from simulate and carries both. Same verdict, thinner
detail, visible only in the one scenario. Residual asymmetry of the #146
design, not a regression of it.
**Confidence:** confirmed. Cosmetic.

### F11 — Grab-bag cosmetics
- `internal/cli/render.go:25-40`: `renderNameTable` pads by `len()` (bytes);
  a non-ASCII theme name misaligns the column. Confirmed, cosmetic.
- `internal/config/level.go:255-262`: when `os.UserHomeDir` fails,
  `LoadGlobal` skips `~/.stowrc` discovery silently — defensible (no home,
  no rc), but the silence contrasts with the target floor's loud
  `ExpandError` for the same missing-home condition. Confirmed, cosmetic.

---

## 2. Suspected (stated as suspicion, with what would settle it)

### S-A — `candidateIgnored` can disagree with the engine on compat patterns under dot-translation
**Where:** `internal/ops/candidates.go:124-163` vs gostow
`stow/engine.go:539-563` + `stow/ignore.go:84-112`.
**What (suspicion):** the engine matches compat `--ignore` regexes against
*target-relative* node paths whose **ancestors are already dot-translated**
(`.config/nvim`) while the leaf is untranslated; `candidateIgnored` matches
them against the *package-relative on-disk* spelling (`dot-config/nvim`).
A compat pattern written against the dot spelling (e.g. `\.config/nvim$`)
would ignore the node in the engine's walk but not in the candidate
computation — so `adopt` could offer (or `status --path` list) a candidate
whose actual adoption the ignore chain then blocks, or vice-versa. The two
code paths are intentionally different computations ("pure config, no tree
walk"), and the native-glob and `IgnoreFunc` lanes *do* agree (both speak
package-relative on-disk paths); only the compat-regex lane's frame differs.
**Would settle it:** one fixture — package with `dot-config/nvim`,
translation on, a repo-level `.stowrc` with `--ignore='\.config/nvim'`;
compare `dstow adopt <file>` candidacy against `stow`/`check` ignoredness.

### S-B — Ledger prune scope is derived from the simulate-time plan, which pre-hooks can invalidate
**Where:** `internal/ops/deploy.go:426-444` (scope built from `p.plan` =
simulate results) vs `:505-535` (pre-hooks run, then the real `Apply` may
produce different actions, which overwrite `p.plan` *after* the scope was
built).
**What (suspicion):** a pre-hook that mutates the package tree (generating
files is a plausible hook use) makes the real plan differ from the simulate
plan; `scope.Paths` then misses the new paths. Consequence is mild by
construction — pruning is belt-and-braces hygiene, and `scope.Packages`
already covers the acting packages' entries — so this may be unexploitable
in any way that matters. Noted because "the plan equals the apply" is an
invariant nothing enforces, and the ledger bookkeeping quietly assumes it.
**Would settle it:** a test with a pre-stow hook that adds a file to the
package; inspect whether the resulting entry set and prune reporting stay
coherent (I believe they do — `recordEntries` uses the *post-apply* plan —
leaving only the prune-scope narrowing).

---

## 3. ★ Guarantees carried by discipline — the port-design ledger

The charge's priority category: each row is (a) a rule the Go code keeps by
convention, (b) whether it is holding, (c) what would make it structural in
Rust. F2/F3/F4 above are the three with live findings attached; these are
the rest, all **currently holding**.

| Rule held by discipline | Holding? | Rust enforcement |
|---|---|---|
| Every typed error is mapped in `classifyExit`; unmapped errors silently exit 1 (`cli/errors.go:91-148`) | Yes (audited by hand — that being the only way to audit it is the point) | Closed error enum + exhaustive `match`; new variant ⇒ compile error |
| H7 write commands must remember `writeCommand()` annotations (`cli/hookguard.go`, per-command sites) | Yes (all ten verbs annotated) | Command-constructor takes a required `Kind` (read/write) parameter; forgetting it doesn't compile |
| "`Apply` and `Expected` build options identically" — held by the single `options()` owner; "observers and deployers parameterize identically" — held by the single `engineOp()` owner | Yes (grepped: no second construction site) | Same single-owner pattern; optionally a sealed constructor so `stow::Options` can't be built outside the seam module |
| gostow types stop at the seam | **Qualified** — three import sites (`engine`, `config/stowrc.go`, `ops/candidates.go`); the third re-derives ignore matching (see S-A) | Engine-crate API designed so consumers get `is_ignored(...)` as a *function of the crate*, not exported pattern-compiler pieces to re-assemble |
| Only `ui` touches process streams (A4) | Yes (grepped: no `os.Stdout`/`Stderr` outside `ui`, `cmd/`, and injected wiring) | Same discipline; a clippy `disallowed-methods` lint on `std::io::stdout`/`stderr` outside the printer module makes it mechanical |
| The twelve `DSTOW_HOOK_*` names exist only as constants; absent-not-empty via strip-then-layer (`hooks/env.go`) | Yes | Same; note for the port: `env_remove` × 12, **not** `env_clear` (the pack's suggestion would strip the user's whole environment — [`08`](08-evaluation.md) §3.4) |
| Ledger/registry writers share the temp-fsync-rename-dirsync discipline by parallel implementation (`ledger/update.go:183-230`, `repo/registry.go:134-165`) | Yes (two copies, currently identical in shape) | One `atomic_write` owner both call — the duplication is the drift risk, same shape as the tracked `editDistance` pair |
| `Contradicted` is the one disk-disagrees owner; check/clean/status all consult it | Yes | Same; enum-typed evidence would also fix the `#124` family's zero-value defaults (`statusclass.go:117-127` reports "not stowed" when config load fails — the package-level analog of [#124](https://github.com/rocne/dstow/issues/124)'s per-link finding, noted here as an addendum, not a new report) |
| `docs/` embed uses `all:` because plain embed drops dotfiles | Yes | Verify the Rust embedding's dotfile behavior explicitly at port time (pack flagged; endorsed) |

---

## 4. Pack errata noticed while verifying (not Go-code defects)

Recorded here because 07 asks for anything noticed while reading; details in
[`08`](08-evaluation.md) §3: (a) 02 misattributes `ops/candidates.go`'s
gostow consumption to `stow.Owner`; (b) 01's dependency graph omits
`hooks → config` and `hooks → name`; (c) 03's `env_clear()` equivalence for
the hook environment is behaviorally wrong; (d) 03/06's "re-point the
conformance harness" underestimates what re-pointing means (CLI coupling;
library introspections invisible to the oracle).

## 5. What was *not* found

For calibration: no incorrect filesystem mutation path, no ledger-corruption
path, no spec violation in the deploy/maintenance verbs' core logic, and no
shipped-docs drift beyond what the tracked issues already cover — the areas
the two prior audits plowed are clean under this read too. The yield above
is concentrated exactly where 07 predicted: seams (F1, F2, F4), invariants
nothing enforces (F3, §3), and the gaps between components rather than
inside them.
