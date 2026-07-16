# The v1 dependency system (superseded 2026-07-15)

## What this was

A fully designed dependency subsystem for dstow v1: scoped declarations
(package / repo / global), presence-on-PATH verification at stow time,
unmet-dependency reporting in `status`, a machine-consumable query via
`info -f dependencies`, and a bundled loop-and-install hook. Designed across
the requirements map ([System-requirements (deps) shape, #13](https://github.com/rocne/dstow/issues/13))
and the design map ([Config schema #23](https://github.com/rocne/dstow/issues/23) C7/C10,
[Metadata directory #24](https://github.com/rocne/dstow/issues/24),
[info #27](https://github.com/rocne/dstow/issues/27) F4–F6,
[CLI surface #22](https://github.com/rocne/dstow/issues/22)).

## Why superseded

Decided on [Bootstrap snippet and installer design (#28)](https://github.com/rocne/dstow/issues/28);
rationale in [ADR 0002](../adr/0002-no-dependency-concept.md). In short:
presence-of-a-command is one member of a family of package-applicability
predicates (fonts, terminfo, OS, versions, env vars) and baking in exactly one
member is arbitrary privilege; no mature dotfiles manager (stow, chezmoi,
yadm, dotbot) bakes one in — the genre's answer is hooks; and the
mechanism-not-policy levee is defensible at "dstow has no dependency concept"
in a way it is not mid-slope.

## Where it may return

- **v2's generic scoped-property store** may carry dependencies as an
  *endorsed convention* — a documented property shape plus a maintained hook —
  but never first-class core semantics (ADR 0002 makes no claims beyond that).
- The **names + hint declaration shape** (C10 below) is good design and is the
  natural file format for any user-side or endorsed convention.
- **Convenience hook skeletons** (`dstow snippet hook <name>`, git-sample
  style) were descoped from v1 with the rest; recorded as a candidate v1.1
  addition on the map.

## The superseded texts, verbatim

### REQUIREMENTS §9.2 (deleted)

> ### 9.2 Dependencies (declared, verified — never installed)
>
> 1. **A dependency is a command a scope needs on PATH**, satisfiable by any of
>    its **names** (`fd`/`fdfind`), optionally carrying a human-oriented
>    **hint**. Declarations are declarative data, not scripts. Checks are
>    presence-on-PATH: instant, local, never network.
> 2. **Three levels**: package and repo declarations live in the never-stowed
>    metadata location; global declarations live in global config. Effective
>    dependencies of a package: its own + its repo's + global.
> 3. **Checking is warn-only and per-level, after that level's own pre hook**
>    (global-pre → check global → repo-pre → check repo → package-pre → check
>    package → link) — installer pre-hooks are first-class; bootstrap flows
>    (stow first, install later) never block. A missing dependency is a fact
>    about the system, reported loudly with its hint — never a stow failure.
> 4. **Surfaces**: stow-time warnings; package status detail; and the
>    **dependency query** — scoped (package / repo / global / everything),
>    machine-consumable output, sane exit codes — deliberately shaped for the
>    loop-and-install hook pattern. dstow itself never wires hooks and
>    dependencies together.

### REQUIREMENTS, other deleted lines

- §1.5 (No unbacked claims): "…and never claims a dependency is installable —
  only present or absent."
- §2.7: metadata carried "hooks/config/dependencies" (now hooks/config).
- §4.2: "**Global-only knobs**: … dependency declarations (§9)" — note this
  contradicted §9.2.2's three-level declarations; the contradiction was
  latent and never resolved, mooted by deletion.
- §5.5 (status): "Unmet dependencies (§9) appear in package status detail."
- §9 heading: "Hooks and dependencies" (now "Hooks").
- §11's doorway example: "(e.g. shaping the dependency query as a
  named-property read)".

### Config schema (C10, #23) — the declaration shape

```toml
dependencies = ["git", { names = ["fd", "fdfind"], hint = "cargo install fd-find" }]
```

Mixed array under the C9 shorthand idiom; no table-key primary name (all names
equal); same shape at every declaring level. C7 placed `[dependencies]`
global-only in the legality matrix (see the §4.2 contradiction above).

### info field (F4–F6, #27)

`dependencies` was a configured field at every scope, inheriting upward only
(package = own + repo + global; repo = own + global; global = own; F5), read
as pure declarations — info never evaluated presence (F6). The
everything-scope read was `dstow info -r -f dependencies`.

### CONTEXT.md glossary entry (retired)

> ### Dependency
>
> A command a scope (package, repo, or global) declares it needs present on
> PATH. dstow **declares and verifies** dependencies — it never resolves or
> installs them (v1 ruling). Checks are warn-only; a missing dependency is a
> fact about the system, never a stow failure. "Deps" is acceptable informal
> prose. *Rejected: "requirement"* — too firm for warn-only semantics.
>
> - **Names** — the command names a dependency answers to; any one present
>   satisfies it (`fd` / `fdfind`). No primary; all names are equal.
>   (*Rejected: "alternatives"* — implies a primary; *"aliases"* — collides
>   with shell aliases.)
> - **Hint** — the optional human-oriented install suggestion a dependency
>   carries.
> - **Dependency query** — the scoped, machine-consumable read of declared
>   dependencies.

### The loop-and-install hook pattern (as it stood at supersession)

A user-side pre-hook reads declared-but-unmet dependencies (`status --json`
under the final F6 design) and acts — running hints or its own install
commands. Under the superseding design this pattern survives with the
declaration source swapped: the hook itself carries the list
(`command -v <tool> || <install command>` lines), no dstow involvement.
