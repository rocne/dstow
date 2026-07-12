# dstow — Ubiquitous Language

The canonical vocabulary for dstow v1. REQUIREMENTS.md and all project prose speak
this language. Concept names only — command spellings and config schemas are design.

## Naming principle

**Established term when the concept matches; never an established term for a
mismatched concept; invented term only where no established term exists.**
Theme and brand live in the project's voice, not in its structural nouns — except
where a themed name is *also* the plainly correct word for the concept.

(Decided on the Terminology ticket, 2026-07-12. Origin: the rejection of
Homebrew's "tap" for what is simply adding a repo.)

## Terms

### Package

A directory whose layout mirrors what it deploys into a target — the unit of
stowing. (GNU stow's own concept, unchanged.)

### Repo (repository)

A directory of packages registered with dstow — where packages come from.
Package-manager lineage (apt/dnf/brew repositories), source-agnostic: some repos
are git clones, others plain local directories. Repos are first-class: added,
listed, removed.

*Retired: "root".* The former "root / package root" concept is the same referent
as repo; "root" survives only as informal prose for a repo's directory location,
never as a distinct term. The search path (DSTOW_PATH + config) is an ordered
list of repos.

### Stow / unstow / restow

The deployment verbs — GNU stow's own vocabulary, kept wholesale (the concepts
match exactly). Likewise kept from stow: **target** (where links land),
**fold / folding** (collapsing a directory to a single link), **adopt**
(importing an existing real file into a package, file→package direction).

### Source

Where a repo comes from — the whole specifier: a remote coordinate
(`owner/name` on GitHub) or a local directory path. Every repo has a source.

### Scheme

The qualifier naming a source's kind: `github:`, `local:`. The word follows URI
convention — in `https://…` and `file:…`, the part before the colon is
established Unix vocabulary as the "scheme"; dstow uses the same word for the
same slot. Documentation and help text explain this origin explicitly (decided:
the name surprises at first sight, so the docs always say why).

### Qualified / unqualified source

A **qualified source** carries an explicit scheme (`github:owner/name`,
`local:path/to/dir`) — always unambiguous, always scriptable. An **unqualified
source** (bare `owner/name`, bare path) is governed by the
confirm-unless-unambiguous rule.

### Ledger

The record of every link dstow creates and removes (XDG state). Disk is always
the truth; the ledger is an index, pruned wherever disk contradicts it.

### Broken / orphaned links

- **broken** — a symlink whose destination no longer exists (established Unix
  usage).
- **orphaned** — a symlink that resolves into a known repo but that no current
  package config would produce; nothing owns it (established package-manager
  usage: apt/pacman orphans).

### Check / clean / rebuild

The maintenance concepts: **check** verifies ledgered links and classifies
(broken / orphaned); **clean** executes exactly check's plan; **rebuild**
reconstructs a lost ledger by full target walk — rare and explicit. Command
spellings are design.

### Folding (the fold setting)

Stow's own word, kept: the tree-collapse behavior, globally **on** or **off**
(off is dstow's default). *Retired as glossary terms: "faithful" and
"predictable"* — they survive only as explanatory prose in docs, never as mode
names requirements lean on.

### Package states

Computed by comparing what applying the package now, under current effective
config, would produce against what the target actually holds:

- **stowed** — everything expected is present and points at this package.
- **partially stowed** — some expected links present, the rest merely missing.
- **not stowed** — nothing of the expected set is present.
- **occupied** — an expected path holds something that isn't this package's
  link. Deliberately neutral: the spot is taken, no claim about how.
- **damaged** — the ledger-attested escalation of occupied: dstow linked this
  path and disk now disagrees. Only ever claimed with ledger evidence.
- **drifted** — a marker on a stowed package whose deployed shape differs from
  what current config would produce (e.g. fold setting changed since
  deployment). Established ops vocabulary: configuration drift.

### Dependency

A command a scope (package, repo, or global) declares it needs present on PATH.
dstow **declares and verifies** dependencies — it never resolves or installs
them (v1 ruling). Checks are warn-only; a missing dependency is a fact about
the system, never a stow failure. "Deps" is acceptable informal prose.
*Rejected: "requirement"* — too firm for warn-only semantics.

- **Names** — the command names a dependency answers to; any one present
  satisfies it (`fd` / `fdfind`). No primary; all names are equal.
  (*Rejected: "alternatives"* — implies a primary; *"aliases"* — collides with
  shell aliases.)
- **Hint** — the optional human-oriented install suggestion a dependency
  carries.
- **Dependency query** — the scoped, machine-consumable read of declared
  dependencies.

### Managed directory

The directory dstow owns, where remote-sourced repos are cloned
(`<managed>/repos/<scheme>/<owner>/<name>`). Not a cache: links point into it;
its contents are load-bearing.
