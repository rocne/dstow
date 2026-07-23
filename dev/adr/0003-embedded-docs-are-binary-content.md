# Embedded docs are binary content, not project documentation

Every file under `docs/` is compiled into the binary — `embed.go` does
`//go:embed all:docs` — and is the source of two user-facing surfaces of a
released build: the hidden **`manual`** command group
(`internal/cli/manual.go`, `manualDir = "docs"`) and every command's
**`--help`** text (`internal/cli/helpdoc.go` — `docs/commands/*.md` *is* the
help page). Editing a file under `docs/` therefore changes what a released
binary prints.

Such a change ships as `fix(manual):` (a correction — patch) or
`feat(manual):` (new or materially expanded manual/help content — minor).
`docs:` is reserved for documentation that ships *outside* the artifact —
`dev/**`, `README.md`, `CLAUDE.md`, `CONTEXT.md`, `CHANGELOG.md`. Decided on
[Embedded docs/ changes don't trigger a release (#175)](https://github.com/rocne/dstow/issues/175);
the embed boundary is the classification boundary, and it is enforced in CI by
`.github/workflows/docs-release-guard.yml`.

## Considered options

The tempting fix is a release-please setting that makes `docs` bump. It does
not exist. Confirmed against release-please's source: version bumps are
hard-coded to `feat`/`feature` (minor) and breaking (major) with a patch
fallback, a release PR opens only when a *triggering* commit exists, and `docs`
sits in the `hidden: true`, non-triggering set. `changelog-sections` controls
changelog *visibility* only, never trigger or bump behaviour. The sole
per-change overrides are a `Release-As: x.y.z` footer or using a triggering
commit type. So this cannot be a config tweak — it is a classification problem,
and the honest classification is that an embedded page is a source input to a
user-facing binary feature, not documentation about the project. Conventional
Commits reserves `docs:` for the latter.

Relying on muscle memory to type `fix(manual):` was rejected as the *whole*
answer: the failure mode is a reflex (`edit docs/foo.md` → type `docs:`), and
this repo turns "remember to" into red/green signals (the vendored-installer
drift check, the Whiskers preset freshness check). So the convention is backed
by a path-based guard: a PR that touches `docs/**` whose title is a
non-releasing type fails, with a message naming this convention. The guard keys
off the PR **title** because squash-merge lands the title as the conventional
commit release-please reads.

Pushing the guard up into the shared `rocne/release-ci` pipeline was rejected:
dstow is the only consumer that embeds its docs, and encoding one consumer's
specifics upstream is exactly the coupling release-ci#29 warns against. It
graduates to a canonical release-ci check only if another tool embeds docs.

## Consequences

- `docs/**` edits are `fix(manual):` / `feat(manual):`; the `(manual)` scope
  keeps them attributable in the changelog (they land under Bug Fixes /
  Features, clearly tagged as manual rather than general code).
- `dev/**`, `README.md`, `CLAUDE.md`, `CONTEXT.md`, `CHANGELOG.md` stay `docs:`
  and correctly cut no release — they are not embedded (`docs/` is the
  user-facing tree, `dev/` is author-facing, per #129).
- CI (`docs-release-guard.yml`) fails a `docs/**` PR with a non-releasing
  title and points at this ADR; retitling re-runs the guard alone.
- The rare one-off — a `docs/**` change shipped without reclassifying — uses a
  `Release-As: x.y.z` footer on the squash body. It is a manual lever, not the
  default.
- Pre-guard `docs:`-typed manual corrections (e.g. #166, #171, #173, #178)
  already merged remain as-is: they ride whatever release their sibling
  `feat`/`fix` commits trigger, unattributed in that changelog. History is not
  rewritten to relabel them; the guard prevents recurrence from here.
