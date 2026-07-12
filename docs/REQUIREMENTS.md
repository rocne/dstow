# dstow v1 — Requirements

**What this document is.** The complete, binding statement of every
user-observable behavior and ergonomic guarantee of dstow v1. It pins
*behavior, not syntax*: exact flag names, command spellings, config formats,
schemas, and internals are design work, delegated to a later effort. Where this
document is silent, design decides; where it speaks, design obeys.

**Vocabulary.** This document speaks the canonical language defined in
[`CONTEXT.md`](../CONTEXT.md) (package, repo, source, scheme, ledger, occupied,
drifted, dependency, …). Terms are used per their glossary definitions
throughout.

**Product stance.** v1 is a fully featured, totally complete, finished product —
no deferred stubs. dstow owns all configuration; it consumes gostow as an
engine library, and gostow stays config-less and unchanged.

---

## 1. Conduct rules (cross-cutting)

These govern every operation and outrank any per-section detail.

1. **cwd is irrelevant.** dstow behaves identically from any directory:
   packages resolve from configured repos, targets from config. The current
   working directory never silently changes what a command does. (Interpreting
   an explicitly-given path argument relative to cwd is ordinary Unix path
   semantics, not a violation — dstow never redefines what a path means.)
2. **Confirm unless unambiguous.** Where an input admits more than one reading
   (see unqualified sources, §5.2), dstow interactively confirms, naming its
   interpretation — and presents an explicit choice when multiple readings are
   viable. In non-interactive contexts the ambiguous form is a hard error that
   names the unambiguous spellings. dstow never lets filesystem contents
   silently decide meaning.
3. **Surprises are announced.** Anything defensible-but-surprising (creating a
   missing target directory, choosing an interpretation, a mode in effect at
   first run) is stated loudly in output — never silent, never a prompt where
   an announcement suffices.
4. **Every refusal names its remedy.** Any error or refusal states what would
   fix it (e.g. an occupied path names adopt; an ambiguous source names its
   qualified spellings).
5. **No unbacked claims.** dstow reports only what it can know: it never
   asserts history without ledger evidence (§7.2), and never claims a
   dependency is installable — only present or absent.
6. **Explicit over implicit for consequential acts.** Nothing touches the
   network, deletes, or overwrites implicitly: sync phases, removals, cleans,
   and rebuilds run only when invoked. Guarded operations offer an explicit
   non-interactive form; a force override exists where specified and is never
   the default.
7. **Output is consistent.** One output style across all operations (unified
   I/O); machine-consumable output is provided where specified (§9.4) with
   sane exit codes.

## 2. The model

1. **Package** — a directory whose layout mirrors what it deploys into a
   target; the unit of stowing (GNU stow's concept, unchanged).
2. **Repo** — a directory of packages registered with dstow; where packages
   come from. Repos are first-class: added, listed, removed (§5, §6). Some
   repos are clones of remote sources; others are plain local directories.
   Every repo has a **source**; remote-sourced repos additionally know their
   origin and can sync (§6).
3. **The search path** — an ordered list of repos: persistent repos from
   global config, with a `$PATH`-style `DSTOW_PATH` environment variable
   *prepending* session repos (unset ⇒ no effect).
4. **Shadowing** — a bare package name resolves to the earliest repo on the
   search path containing it. Shadowed duplicates remain visible in listings
   (marked as shadowed) and reachable by repo qualification (mechanism is
   design).
5. **Bulk scope** — bulk operations (`--all`-style) span the *resolved set*:
   the union of package names across all repos after shadowing resolution —
   each name once, winner only. A repo may exclude itself from bulk via the
   repo-level exclude knob; package-level excludes also apply. Explicitly
   naming a package always overrides bulk exclusion (stronger intent wins).
6. **Managed directory** — the directory dstow owns, where remote-sourced
   repos are cloned: `<managed>/repos/<scheme>/<owner>/<name>`. Kind-first
   (`repos/`) to reserve sibling room for future kinds; scheme-namespaced so
   collisions are structurally impossible. Not a cache: links point into it.
7. **Package metadata is never stowed** — whatever in-package location carries
   hooks/config/dependencies is auto-ignored by the engine (name/layout is
   design).

## 3. Deployment (stow / unstow / restow)

1. **Default target is `$HOME`**, per-package overridable (§4). Missing target
   directories are auto-created and announced.
2. **Per-package independence.** In multi-package and bulk runs, each package
   succeeds or fails on its own, with a per-package status line (not-found
   included); the run continues past failures; **overall exit is nonzero if
   any package failed**. Within one package the engine's all-or-nothing
   planning applies.
3. **Folding is off by default** (predictable link topology; stow's most
   surprising behavior is opt-in). Folding is a **global-only** setting —
   never per-package or per-repo, since folding is a property of a target
   subtree and per-package values could declare contradictions. Fold flags
   found in a migrated repo-level `.stowrc` are honored unless repos in one
   invocation contradict each other — then dstow refuses loudly, pointing at
   the global setting (§4.3).
4. **Dot-translation is on by default** (`dot-foo → .foo`), overridable at any
   level of the config chain (package / repo / global). Translation only
   rewrites `dot-`-prefixed names; classic `.foo` layouts stow identically
   either way.
5. **Occupied paths refuse, with remedy.** Stowing over a real file or foreign
   link refuses per the engine's conflict rules and names adopt (§8) where a
   real file is the occupant.
6. **First run is non-interactive but loud**: dstow announces that folding is
   off (the predictable default) and tells stow veterans exactly where the
   global setting lives to enable it.

## 4. Configuration

1. **Four-level chain: package → repo → global → built-in.** Nearer level wins
   per knob, except **ignore patterns compose additively** across all levels
   (a package adds to, never silences, inherited ignores). The repo level
   carries the same knob set as the package level, acting as defaults for all
   packages in that repo.
2. **Package/repo-level knobs (v1 set)**: target override;
   exclude-from-bulk (bulk only — explicit naming proceeds); dot-translation;
   additive ignore patterns. **Global-only knobs**: folding; persistent repos;
   dependency declarations (§9); default-scheme behavior is fixed in v1
   (github.com, §5.2).
3. **Stow compatibility: dstow config is a strict superset of stow's.**
   dstow works out-of-the-box on an existing stow setup; renaming a `.stowrc`
   to the native config must just work. Slotting: `~/.stowrc` → global level;
   `<repo>/.stowrc` → repo level, discovered via the repo, never via cwd.
   **Supplement mode (supported soft goal)**: `.stowrc` may coexist with
   dstow-native config at a level — non-overlap is silent and intended; on an
   actual per-knob conflict the dstow value wins with a loud warning.
   Sanctioned fallback if supplement proves problematic in design: native
   config present at a level ⇒ that level's `.stowrc` is ignored, loudly.
   **Native mirrors**: every stow concept dstow honors has a dstow-native
   equivalent — including ignores (`.stow-local-ignore` has a dstow-named
   counterpart). A dstow user never *needs* to know or touch stow-named
   artifacts; `.stowrc` and friends are a compatibility layer only.
4. **Built-in floor** (out-of-the-box values):

   | Knob | Built-in default |
   |---|---|
   | Target | `$HOME` |
   | Folding | **off** (global setting to enable) |
   | Dot-translation | **on** (per-scope override: package / repo / global) |
   | Ignores | stow's standard built-in ignores (via gostow) + package metadata; dstow-native ignore mechanism at every level (stow-named files never required) |
   | Assumed scheme for bare `owner/name` | `github` (github.com) |
   | Search path | global-config repos; `DSTOW_PATH` prepends |

## 5. Repo management: add / list / remove

### 5.1 One concept

There is **no "tap"**. Adding from a git source is repo add; the result is an
ordinary first-class repo — listed, removed, bulk-excluded like any other.
Homebrew vocabulary is not used where the concept differs.

### 5.2 Sources

1. **Accepted source forms**: a local directory path (ordinary Unix path
   semantics); a full URL / ssh form; a **qualified source**
   (`github:owner/name`, `local:path/to/dir`). Qualified sources are always
   unambiguous and scriptable. Documentation and help text call the qualifier
   the **scheme** and always explain the URI-convention origin of the word.
2. **Unqualified sources** (bare `owner/name`, bare path) are governed by
   confirm-unless-unambiguous (§1.2): interactive confirmation naming the
   interpretation, an explicit choice when a matching local directory also
   exists, a hard error non-interactively. The assumed host for a bare or
   `github:`-qualified `owner/name` is github.com in v1.
3. **Remote adds clone into the managed directory** at
   `<managed>/repos/<scheme>/<owner>/<name>`; each clone is its own
   first-class repo with identity `owner/name`, scheme-qualifiable on
   collision. Re-adding a present repo is a safe, announced no-op.
4. **Add stows nothing** by default; it announces the repo's packages and any
   shadowing changes it causes. An opt-in add-and-stow form exists:
   behaviorally add followed by a bulk stow scoped to that repo, honoring
   standard bulk-exclusion rules.

### 5.3 Remove

1. **Local-path repos**: remove deletes the config entry only; the directory
   is the user's and is never touched.
2. **Managed clones**: remove also deletes the clone directory.
3. **Guards (prompt-or-refuse, both bypassable by an explicit force):**
   - **Still-stowed guard** — removal that would orphan or dangle stowed links
     prompts to unstow-then-remove (applies to local-path repos too, whose
     links would otherwise fall outside dstow's management). An explicit
     non-interactive unstow-then-remove form exists.
   - **Unsaved-work guard** — a managed clone holding local work not present
     at its source is not deleted; the refusal states what would be lost.
     Defined source-agnostically (no git vocabulary); for git sources it maps
     to dirty/unpushed.

## 6. Remote sync: fetch and apply

Two phases, apt/brew convention (update-then-upgrade); names are design.

1. **Fetch phase** — explicit, network-touching, alters no working tree.
   Afterwards status reports behind/ahead per remote repo.
2. **Apply phase** — fast-forward only, clean clones only. Refuses divergence
   or local work loudly, with no stash/merge/rebase negotiation; reports
   old→new; **never re-stows** (structural drift surfaces via status). Note
   the symlink-farm stake: applying changes already-linked live files
   instantly — which is why fetch-before-apply exists.
3. Both phases have per-repo and all-remote-repos forms; neither ever runs
   implicitly.

## 7. Observation: list and status

### 7.1 list — the configuration view ("what do I have")

Scopeable broadly or to just repos / just packages; always repo-attributed.
Shows targets, exclusion state, source (with scheme), and shadowed duplicates
marked. Never inspects target dirs; always instant and side-effect-free. list
carries no cached deployment state — deployment truth lives only in status.

### 7.2 status — the live-state view ("what is deployed")

1. Scopeable per package / per repo; inspects targets; for remote repos also
   reports sync state as of the last fetch.
2. **Package states** (expected-vs-actual against *current effective config*):
   **stowed** / **partially stowed** / **not stowed** / **occupied** /
   **damaged**, with the **drifted** marker on stowed packages whose deployed
   shape differs from what current config would produce (e.g. the folding
   setting changed since deployment). Occupied is neutral — no claim about
   how; **damaged is claimed only with ledger evidence** (dstow linked this
   path; disk now disagrees).
3. **Repo attribution**: a link pointing into another repo's same-named
   package does not count as stowed for this one.
4. **Per-path detail on demand**: what occupies each path (real file, foreign
   link, directory mismatch) and the remedy where knowable.
5. Unmet dependencies (§9) appear in package status detail.

## 8. Maintenance: ledger, check, clean, rebuild, adopt

1. **The ledger.** dstow records every link it creates and removes (XDG
   state). Disk is always the truth: entries contradicted by disk are pruned
   on sight, never trusted over reality. (Precedent: dpkg/rpm/pacman verify
   against their file databases, never by filesystem walk.)
2. **check** verifies ledgered paths — instant, no tree walk — and classifies
   stale links: **broken** (destination gone) vs **orphaned** (resolves into a
   known repo, but no current package config would produce it: an added
   ignore, a changed target, toggled translation, a hand-made link).
3. **clean** executes exactly check's plan (one owner — report and action can
   never drift): broken links removed freely; orphans behind interactive
   confirm, with an explicit non-interactive form and a force override.
4. **rebuild** is the only full tree walk: an explicit, rare operation that
   reconstructs a lost ledger by scanning configured targets for links whose
   destinations lie in known repos. Never runs implicitly.
5. **adopt** imports a real file into a package: always file→package (live
   content wins; never destroys running config), a link takes its place.
   Shows its plan; confirms when it would overwrite differing package
   content; non-interactive and force forms; scope is a named file or all
   occupied paths of a package. The **candidate helper**: for a given file,
   dstow enumerates the packages that could adopt it (effective target covers
   the path; not ignored; per-package dot-translation applied where enabled),
   ranking packages that already own neighboring paths first — pure
   config+ledger computation, no walking.

## 9. Hooks and dependencies

### 9.1 Hook lifecycle

1. **Points**: pre + post around each action — stow, unstow, restow, adopt —
   with the action kind visible to the hook.
2. **Levels**: package, repo, and global. Repo and global hooks fire **once
   per invocation** when any package under them acts.
3. **Ordering — nested/LIFO**: global-pre → repo-pre → package-pre → action →
   package-post → repo-post → global-post.
4. **Failure**: a failed package-pre blocks that package (run continues per
   §3.2); a failed repo/global-pre blocks everything under it; a failed post
   is reported and marks its scope failed, but completed work stays done.
5. **No-op semantics**: hooks fire only when their scope actually changes
   something; repeated bulk runs stay quiet and idempotent.
6. **Context contract**: hooks receive their scope's context (at minimum:
   action kind, package, target, repo); mechanism is design.

### 9.2 Dependencies (declared, verified — never installed)

1. **A dependency is a command a scope needs on PATH**, satisfiable by any of
   its **names** (`fd`/`fdfind`), optionally carrying a human-oriented
   **hint**. Declarations are declarative data, not scripts. Checks are
   presence-on-PATH: instant, local, never network.
2. **Three levels**: package and repo declarations live in the never-stowed
   metadata location; global declarations live in global config. Effective
   dependencies of a package: its own + its repo's + global.
3. **Checking is warn-only and per-level, after that level's own pre hook**
   (global-pre → check global → repo-pre → check repo → package-pre → check
   package → link) — installer pre-hooks are first-class; bootstrap flows
   (stow first, install later) never block. A missing dependency is a fact
   about the system, reported loudly with its hint — never a stow failure.
4. **Surfaces**: stow-time warnings; package status detail; and the
   **dependency query** — scoped (package / repo / global / everything),
   machine-consumable output, sane exit codes — deliberately shaped for the
   loop-and-install hook pattern. dstow itself never wires hooks and
   dependencies together.

## 10. Bootstrap

1. **The bootstrap-snippet command** prints a POSIX-sh snippet to stdout —
   composable (`>> ~/.bashrc`); dstow never edits rc files itself. The snippet
   installs dstow iff absent, and **its presence check is local**: a shell
   where dstow exists starts with zero network traffic and zero output.
2. **The hosted installer is a v1 requirement.** A curl-able install script is
   published (via the established release pipeline), so the emitted snippet is
   guaranteed to work, not merely to print. The script is itself idempotent —
   it too checks for dstow and exits cleanly when present, so direct
   `curl | sh` is always safe. URL and install mechanics are design.

## 11. Out of scope for v1

Design and implementation (CLI syntax, config schema, architecture, code) are
delegated. Additionally ruled out of v1's scope, recorded on the map:
Homebrew-style tap semantics; dstow as an installer of software; a generic
scoped-property store (deps as one property of many); an in-tool self-update
command; an emitted repo setup script (clone → run: bootstrap dstow, then stow
a package selection); changes to gostow; extraction of shared output/color
plumbing. Design may keep doorways open
(e.g. shaping the dependency query as a named-property read) but owes them
nothing.

---

## Provenance

Each section's binding detail lives in its ticket's resolution:

| Sections | Ticket |
|---|---|
| §1.1, §3.1–3.2 | [Location-independence requirements](https://github.com/rocne/dstow/issues/3) |
| §2.3–2.5 | [DSTOW_PATH and package-root requirements](https://github.com/rocne/dstow/issues/6) |
| §3.3, §3.6 | [Fold default](https://github.com/rocne/dstow/issues/10) |
| §4 | [Package-local config](https://github.com/rocne/dstow/issues/4), [Stow-config compatibility](https://github.com/rocne/dstow/issues/12) |
| §5, §6 | [Git-tap source requirements](https://github.com/rocne/dstow/issues/7) |
| §7, §8 | [Status, list, and maintenance operations](https://github.com/rocne/dstow/issues/11) |
| §9.1 | [Hook lifecycle requirements](https://github.com/rocne/dstow/issues/5) |
| §9.2 | [System-requirements (deps) shape](https://github.com/rocne/dstow/issues/13) |
| §10 | [Bootstrap snippet and hosted installer requirements](https://github.com/rocne/dstow/issues/15) |
| Vocabulary | [Terminology / ubiquitous language](https://github.com/rocne/dstow/issues/14) → `CONTEXT.md` |
| v1 operation set | [Wrapper reconstruction](https://github.com/rocne/dstow/issues/2) |
| Landscape honesty | [Landscape verification](https://github.com/rocne/dstow/issues/8) |

Dot-translation's on-by-default and this document's structure were settled on
[Draft REQUIREMENTS.md](https://github.com/rocne/dstow/issues/9) itself.
