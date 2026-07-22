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
never as a distinct term. The repo set (global config + DSTOW_PATH) is unordered
— registration order never affects what a name means (amended 2026-07-14 with
the Naming grammar resolution; was an ordered search path).

A repo's packages are its visible directories — at the repo root by default, or
inside a designated packages directory (an opt-in repo-level setting; decided
2026-07-15 with the Metadata directory resolution). Hidden directories are never
packages: package identity is locational, never declared by marker files.

### Repo registry

The persistent record of registered repos — configuration, not state, though
commands write it rather than an editor (`git remote add` lineage: intent
entered through a porcelain is still intent). The line it teaches: **config is
intent — what should be managed; state is record — what actually happened.**
The registry is not reconstructible from disk, so it lives with configuration;
the ledger is the record and lives in state.

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

### Coordinate

The path-shaped middle of a source or fully qualified name — one or more
`/`-separated segments between the scheme and the package. A scheme may
interpret its coordinates (`github:` reads `owner/name`; `local:` reads a
filesystem path) but the grammar itself does not. Reserved characters inside
a coordinate are percent-encoded (`%3A` for `:`, `%25` for `%`, `%40` for
`@`, and control characters) — the same URI lineage as the scheme; dstow
always emits the encoded canonical form itself, so users paste rather than
construct it.

`@` is reserved as an optional, **scheme-interpreted suffix** on the
coordinate (`coordinate@suffix`): v1 assigns it no semantics — what a suffix
means belongs entirely to any scheme that later chooses to interpret it
(anticipated lineage: `pkg@version` pinning). Decided at DESIGN.md synthesis,
2026-07-16.

### Fully qualified name (FQN)

`scheme:coordinate::package` — the complete, globally unambiguous name of a
package (drop the `::package` tail for a repo's FQN). `:` separates only the
scheme; `::` separates only the package. **Any suffix cut at a segment
boundary that resolves uniquely is a valid name** (`zsh`, `dots::zsh`,
`rocne/dotfiles::zsh`); a leading `::` forces package-kind (`::zsh` — "the
package zsh, whatever repo"). A name matching more than one entity is
ambiguous *input*, resolved by confirm-unless-unambiguous — ambiguity exists
only in user input, never in the model.

*Retired: "shadowing" and "resolved set"* (2026-07-14) — with FQNs and
suffix resolution, no name is ever silently resolved by registration order;
bulk operations span all packages of all registered repos.

### Name expression / path operand

The two operand worlds: a **path operand** starts with `/`, `~/`, `./`, or
`../` and always refers to the *target* world (a real file or directory); a
**name expression** is anything else and resolves against registered FQNs —
the *registry* world. The first character decides; no command ever guesses.

### Ledger

The record of every link dstow creates and removes (XDG state) — a
**current-state index**, never a history. The word is accounting's own
distinction, kept deliberately: the *journal* is the chronological log of
transactions; the *ledger* is the organized current state posted from it.
dstow keeps the ledger and no journal ([ADR 0001](dev/adr/0001-ledger-is-a-current-state-index.md)).
Disk is always the truth; the ledger is an index, pruned wherever disk
contradicts it.

### Contradicted entry

A ledger entry disk disagrees with: the recorded path is gone, holds a
non-link, or holds a different link than dstow wrote. Never trusted over
reality; pruned by the next *write* command whose scope covers it (reads
report, never prune). A contradicted entry is the evidence behind the
**damaged** package state.

### Broken / orphaned links

- **broken** — a symlink whose destination no longer exists (established Unix
  usage).
- **orphaned** — a symlink that resolves into a known repo but that no current
  package config would produce; nothing owns it (established package-manager
  usage: apt/pacman orphans).

### Check / clean / rebuild

The maintenance concepts: **check** verifies ledgered links and classifies
(broken / orphaned / contradicted); **clean** executes exactly check's plan;
**rebuild** reconstructs a lost ledger by full target walk — rare and
explicit. Command spellings are design.

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

### Field

A named, readable property of a scope (package, repo, or global). One field
vocabulary is spoken everywhere a field appears — human output, field
selection, machine output. Every field is either inherent or configured:

- **Inherent field** — a fact of the thing as it exists: fixed by *which*
  thing it is or *how it came to be*. No config file authors it; changing one
  means re-making or re-registering the thing (or it cannot change at all).
  Permanently read-only — no future property store will ever write one.
- **Configured field** — an effective value from the config chain: the user's
  declared intent for the thing, editable in files. The only territory a
  future property write could ever touch.

The test: an inherent field reads what the thing *is*; a configured field
reads what the user *wants for it*. Chosen-once facts (a repo's source) are
inherent — changing one means re-registering a different thing. (Decided
with the info resolution; "identity field" was rejected because inherent
facts needn't identify — e.g. a hypothetical clone-time tool version.)

A field is data *about* a scope, located in or derived from the scope's
location and the metadata directories in scope — never the scope's content
(what is *inside* it: packages, hook executables) and never the target.
Content enumeration and target inspection are other reads' jobs. The one
bend: the global scope's inherent fields are derived from the installation
itself.

### Managed directory

The directory dstow owns, where remote-sourced repos are cloned
(`<managed>/repos/<scheme>/<owner>/<name>`). Not a cache: links point into it;
its contents are load-bearing.

### Metadata directory

The never-stowed, dstow-claimed directory carrying a scope's metadata:
`.dstow/` at a repo or package root; the global level's equivalent is dstow's
XDG config directory. Auto-ignored by the engine at package roots — its
contents are never deployed. Its top level is reserved territory: dstow claims
the names inside it; unknown entries draw warnings, never refusals.
