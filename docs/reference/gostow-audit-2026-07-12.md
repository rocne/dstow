# gostow library audit — 2026-07-12

**Purpose.** Audit of the gostow library (local repo `~/git/gostow`, package
`github.com/rocne/gostow/stow`) against what dstow's
[`REQUIREMENTS.md`](../REQUIREMENTS.md) assumes of its symlink engine. This is
a factual capability inventory; gaps are inputs to the design effort, since
changes to gostow are out of the requirements effort's scope (§11).

**The public surface** (all in `stow/`, per the package doc in
`stow/paths.go:1-6`): `Apply`, `Stow`, `Unstow`, `Restow`; types `Options`,
`Request`, `Action`, `Result`, `Task`, `TaskAction`, `TaskType`, `Conflict`,
`ConflictError`, `FatalError`; helpers `CompilePattern`, `Gerund`, and the
anchor constants `IgnoreAnchor`/`PrefixAnchor`. Everything else — the engine,
the planner, ignore resolution, ownership resolution — is unexported.

---

## 1. Core apply operations — **present**

- `stow.Apply(opts Options, reqs ...Request) (*Result, error)` —
  `stow/engine.go:140`. A `Request` is `{Action, Packages}` with
  `ActionStow` / `ActionUnstow` / `ActionRestow` (`engine.go:16-42`).
- Sugar: `Stow`, `Unstow`, `Restow` (`engine.go:124-134`).
- `Options.FixQuirks` exists exactly as the concept notes claimed
  (`engine.go:76`, documented in gostow's `docs/DIVERGENCES.md` §2): it turns
  off stow bug-parity at three enumerated sites (the `--dotfiles` `.stow`-guard
  bypass, the RMDIR colon, two CLI-only fixes). It does **not** fix stow's two
  documented algorithmic bugs (empty-directory problem; folding across stow
  directories). dstow should always pass `FixQuirks: true`.

Note: `Options` also carries `Compat`, `Defer`, `Override`, `Verbosity`, `Log`
— the full stow CLI feature set is reachable from the library.

## 2. Folding control — **present**

- `Options.Fold bool` per invocation (`engine.go:48`); `false` is stow's
  `--no-folding`. With `Fold: false`, stow-side directories are materialized
  with `MKDIR` + per-file links (`engine.go:537-539`), and unstow's re-fold
  check short-circuits (`foldable`, `engine.go:841-844`).
- dstow's §3.3 global folding knob maps directly: one boolean per `Apply`.
- Caveat for the **drifted** marker (§7.2.2): toggling `Fold` against an
  already-deployed package does not restructure it on restow in every case —
  an existing fold link whose destination equals what stow would write is
  skipped as already-stowed (`stowOverExistingLink`, `engine.go:576-578`).
  Detecting fold-drift is dstow's job (see item 5).

## 3. Dot-translation — **present**

- `Options.Dotfiles bool` per invocation (`engine.go:49`); translation is
  `adjustDotfile` (`stow/dotfiles.go:11-17`), applied per path segment, exactly
  stow's `s/^dot-([^.])/.$1/`.
- Per-package control (§3.4) falls out naturally because dstow will call
  `Apply` once per package (see item 8); no per-package parameter is needed
  inside one call.
- Matches §3.4's "translation only rewrites `dot-`-prefixed names": classic
  `.foo` layouts are untouched either way.

## 4. Ignore mechanism — **partial**

- **Built-in standard ignores: present.** `defaultIgnoreData` is stow's
  `__DATA__` block verbatim (`stow/ignore.go:22-46`) — RCS/CVS/git/hg/svn,
  emacs droppings, `^/README.*` etc.
- **Caller-added patterns: present.** `Options.Ignore []string`
  (`engine.go:54`) is additive and checked *before* any file-based source
  (`ignore()`, `stow/ignore.go:60-69`). This is the injection point for the
  never-stowed metadata dir (§2.7) and for dstow's config-chain patterns.
- **Package-local ignore file: present.** `.stow-local-ignore` per package,
  plus `$HOME/.stow-global-ignore` (`ignore.go:14-17`, `ignoreRegexps`,
  `ignore.go:92-126`).
- **The gaps:**
  1. **File sources are exclusive, not additive** (`ignore.go:56-59`): the
     first existing file among local → global **replaces** the built-in
     defaults entirely. dstow §4.1 demands ignores that *compose additively*
     across all four levels — gostow's file semantics are stow's, and stow's
     are replacement semantics. dstow cannot get its additive chain from
     gostow's file mechanism; it must resolve its own chain and feed the result
     through `Options.Ignore`.
  2. **Pattern semantics differ by channel.** `Options.Ignore` patterns are
     suffix-anchored regexes on the whole target path (`IgnoreAnchor "(%s)$"`,
     `engine.go:237-239`); ignore-*file* patterns get stow's segment/path
     split with different anchoring (`compileIgnorePatterns`,
     `ignore.go:223-251`). dstow's design must pick one user-facing pattern
     dialect and translate; naively forwarding file-style patterns into
     `Options.Ignore` changes their meaning.
  3. **`$HOME/.stow-global-ignore` is read implicitly** from `os.Getenv("HOME")`
     (`ignore.go:94`) and cannot be disabled per call. dstow's "dstow owns all
     configuration" stance meets a knob the engine reads on its own.
  4. The built-in default list is an **unexported** const — dstow cannot obtain
     it programmatically to re-inject it additively alongside a user's
     `.stow-local-ignore` (the §4.4 built-in floor says built-ins + metadata +
     chain always apply).
- Metadata-dir injection itself (§2.7) works today: an `Options.Ignore` entry
  matching the metadata name is checked first and cannot be silenced by any
  ignore file.

## 5. Planning / dry-run — **partial**

- **Present:** `Options.Simulate bool` (`engine.go:52`). Apply always plans
  first; with `Simulate` set, `processTasks` is skipped entirely
  (`engine.go:172-176`) and the caller still gets `Result.Tasks` — the ordered
  `[]Task{Action, Type, Path, Source, Dest}` (`stow/tasks.go:45-69`), paths
  relative to the target, `Source` being the link destination. Planning reads
  the filesystem but never mutates it.
- **The gap:** Simulate computes a **delta against the current disk**, not the
  pure expected link set. Already-correct links yield no task
  (`engine.go:576-578` "Skipping ... already points to"); a fold link that
  still satisfies the package yields no task even under `Fold: false`. So:
  - "empty plan + no conflicts" does prove **stowed** (usable for §7.2 status),
  - but the **expected shape** needed for the drifted marker (§7.2.2), for
    check/clean's orphan classification (§8.2 — "no current package config
    would produce it"), and for adopt candidates (§8.5) is *not* directly
    exposed. There is no "plan against an empty target" mode, and the two
    ingredients dstow would need to compute the expected set itself — the
    ignore verdict (`engine.ignore`, unexported) and the translation/folding
    walk — are internal.
  - Workarounds exist (simulate against a temp empty target dir; reimplement
    the walk in dstow), but each either double-implements engine semantics or
    contorts the engine. This is the load-bearing gap of the audit.

## 6. Conflict reporting — **partial**

- **Present:** conflicts are structured per conflict, collected across the
  whole plan, and returned without touching disk: `Conflict{Action, Package,
  Message}` (`engine.go:80-84`), `Result.Conflicts` +
  `ConflictError` (`engine.go:88-92`, `engine.go:168-170`).
- **The gap:** the occupied **path** and the **occupant kind** live only inside
  the prose `Message`, formatted at the conflict site (e.g.
  `"cannot stow %s over existing target %s since neither a link nor a
  directory and --adopt not specified"`, `engine.go:523-525`; `"existing
  target is not owned by stow: %s"`, `engine.go:562`; `"existing target is
  stowed to a different package: %s => %s"`, `engine.go:605-606`; `"cannot
  stow non-directory %s over existing directory target %s"`,
  `engine.go:516-517`). There is no `Path` field and no machine-readable
  occupant classification (real file / foreign link / directory mismatch —
  exactly the §7.2.4 taxonomy). The message set is small, enumerable, and
  byte-pinned by gostow's parity contract, so parsing is *stable* — but it is
  still parsing prose to recover structure the engine had in hand.

## 7. Adopt — **present** (engine-level)

- `Options.Adopt bool` (`engine.go:50`): a real file occupying a link's place
  is moved into the package (`doMv`, `engine.go:533`) and a link takes its
  place (`engine.go:534`) — file→package, live content wins, matching §8.5's
  direction. The move survives cross-filesystem layouts (`moveFile`,
  `stow/move.go`, porting `File::Copy::move`).
- What dstow layers on top (as §8.5 already assumes): plan display,
  confirmation when package content would be overwritten (the engine
  overwrites unconditionally), named-single-file scope (engine adopt is
  per-Apply, i.e. all occupied paths of the package — narrowing to one file
  would take `Ignore`-pattern gymnastics), and the candidate helper (pure
  dstow config+ledger computation; nothing for the engine to provide).

## 8. Per-package atomicity — **present** (with one execution-phase caveat)

- `Apply` plans **every** request, collects **all** conflicts, and only then
  touches the filesystem; any conflict aborts the whole invocation before the
  first write (`engine.go:140-176`, `ConflictError` doc `engine.go:87-88`).
  One package per `Apply` gives dstow §3.2's per-package all-or-nothing
  planning plus per-package independence for free.
- Caveat: atomicity is a *planning* guarantee. If task **execution** fails
  midway (I/O error after planning succeeded — `processTask`,
  `tasks.go:345-373`), completed tasks are not rolled back; the error is
  returned with the target partially mutated. Same as stow. dstow's ledger and
  status model should treat a failed-execution package as damaged/partial, not
  assume disk purity.

## 9. Target/dir handling — **partial**

- **Arbitrary target per call: present.** `Options.Dir` and `Options.Target`
  (`engine.go:46-47`); no chdir ever happens — the engine explicitly joins
  target-relative paths instead of mutating process state (`real()`,
  `engine.go:115-121`). Relative `Dir`/`Target` are resolved against process
  cwd via `filepath.Abs` (`canonPath`, `engine.go:281-294`), so dstow satisfies
  §1.1 by always passing absolute paths — trivial and sufficient.
- **Missing target: the engine refuses.** `canonPath` stats the path and
  returns a fatal `"canon_path: cannot chdir to ..."` error when target (or
  dir) does not exist or lacks search permission (`engine.go:281-304`).
  §3.1's auto-create-and-announce is therefore **caller-side**: dstow creates
  the target directory (and announces it) before calling `Apply`.
  Intermediate directories *inside* an existing target are created by the plan
  itself (`doMkdir` tasks) — only the target root is the caller's problem.

## 10. Link ownership introspection — **absent** (from the public API)

- The capability exists but is unexported: `findStowedPath` decides whether a
  link destination points into a stow package and which
  (`engine.go:744-759`), with `linkDestWithinStowDir` (`engine.go:761-768`),
  `findContainingMarkedStowDir` (`.stow`-marker support, `engine.go:770-784`)
  and `linkOwnedByPackage` (`engine.go:832-835`) behind it. All are methods on
  the unexported `engine`; no public helper answers "does this symlink belong
  to package P in repo R?".
- For status attribution (§7.2.3) and rebuild (§8.4), dstow must implement the
  check itself: readlink, resolve relative to the link's parent, test
  containment in a known repo/package path. That is a small amount of code,
  but it re-derives semantics gostow already has pinned (relative-link
  resolution via stow's own `joinPaths` rules, `.stow` markers) — a drift risk
  if implemented twice.

---

## Gaps & recommendations

Changes to gostow are out of the requirements effort's scope (§11), so every
gap below is an **input to the design effort**, which must decide per gap:
build it in dstow, or file a gostow work item and design dstow around its
eventual presence (with an interim dstow-side implementation if needed).

### dstow builds it (engine is fine as-is)

- **Target auto-create + announcement** (item 9): create the target root
  before `Apply`. Caller-side by clean design; the engine refusing a missing
  target is correct engine behavior.
- **The additive ignore chain** (item 4): dstow resolves package → repo →
  global → built-in itself and feeds the merged result through
  `Options.Ignore`. gostow's replacement-semantics ignore *files* are the
  compatibility layer (§4.3), not the mechanism.
- **Adopt UX** (item 7): plan display, differing-content confirmation,
  single-file scope, candidate helper — all dstow-layer per §8.5; the engine's
  `Adopt` move+link primitive is sufficient underneath.
- **Per-package orchestration** (items 1, 3, 8): one `Apply` per package with
  per-package `Options` gives per-package fold/dotfiles/ignore settings and
  per-package atomicity without any engine change.

### Gaps that warrant gostow work items (design-effort inputs)

*All four are now filed as gostow feature requests:
[gostow#33](https://github.com/rocne/gostow/issues/33) (expected-set planning),
[gostow#34](https://github.com/rocne/gostow/issues/34) (structured conflicts),
[gostow#35](https://github.com/rocne/gostow/issues/35) (ownership helper),
[gostow#36](https://github.com/rocne/gostow/issues/36) (ignore seams).*

1. **Pure expected-set planning** (item 5) — *the most consequential gap.*
   dstow's status states, drifted marker, check/clean orphan classification,
   and adopt candidates all compute expected-vs-actual; `Simulate` yields a
   disk-relative delta, and the ignore verdict + translation walk needed to
   compute the expected set independently are unexported. Design options:
   (a) gostow work item — a plan-only entry point that walks the package under
   the given options and returns the expected link set without consulting the
   target; (b) interim: dstow simulates against an empty scratch target dir
   (correct but ugly, and folding decisions depend on target state);
   (c) dstow reimplements the walk (double implementation of pinned
   semantics — worst option). The design should prefer (a) with (b) as bridge.
2. **Structured conflicts** (item 6) — a `Path` field and an occupant-kind
   enum on `Conflict`, so §7.2.4's per-path detail and §3.5's adopt remedy
   hints don't come from parsing pinned prose. Interim: parse the enumerated
   message set (stable by parity contract, but still prose-parsing).
3. **Exported ownership helper** (item 10) — a public
   "resolve this symlink; is it owned by stow dir D / package P?" function
   wrapping `findStowedPath`, for status attribution and rebuild. Interim:
   ~30 lines in dstow, accepting the semantic-drift risk.
4. **Ignore-mechanism seams** (item 4, lesser): export the built-in default
   pattern list (so the §4.4 floor can be re-injected additively when a user's
   ignore file would otherwise silence it), and a way to suppress the implicit
   `$HOME/.stow-global-ignore` read (it is engine-read configuration outside
   dstow's ownership). Interim: document the interaction; treat
   `.stow-global-ignore` as part of the `.stowrc`-style compatibility surface.

Nothing found contradicts REQUIREMENTS.md's core bet: gostow really is a
config-less, cwd-independent, per-call-parameterized engine with all three
apply operations, per-call fold and dotfiles control, additive caller ignores,
plan-first all-or-nothing semantics, an adopt primitive, and `FixQuirks` as
advertised. The work the design effort must plan for is observational
(expected-set planning, structured conflicts, ownership introspection), not
operational.
