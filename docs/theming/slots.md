# The slot vocabulary

Fourteen slots, in two groups. This page is the reference; the live version,
with each slot name rendered in its own effective style and each consumer
named, is:

    dstow theme slots

## The roster

**Content slots** — the parts of dstow's output that are structure rather than
message:

| Slot | What it colors |
|---|---|
| `section1` | Section headings |
| `section2` | Secondary section headings |
| `name1` | Canonical names — packages, repos, fields |
| `name2` | Secondary names and aliases |
| `value1` | Literal values and defaults |
| `value2` | Muted, secondary text — placeholders and metadata |

**Message slots** — four families, two prominence tiers each:

| Slot | What it colors |
|---|---|
| `error1` | Fatal, blocking errors |
| `error2` | Lesser breakage |
| `warning1` | Warnings that want attention |
| `warning2` | Lower-prominence attention |
| `success1` | Good outcomes |
| `success2` | Quieter good outcomes |
| `info1` | Neutral-notable, prominent — actionable guidance |
| `info2` | Neutral-notable, quiet — FYI commentary |

## Tiers

The trailing number is a **prominence tier**, not an index: **1 is loudest**.
The numbering is naturally extendable — a third tier would be `error3` — and
that is why it is a number rather than a word.

A slot you leave undeclared derives from its family's tier-1 by attribute
step-down: remove `bold` if present, otherwise add `dim`. Declaring the seven
tier-1 slots therefore gives you a coherent fourteen.

Some slots have no internal consumer in v1 — `section2`, `name2`, `value1`,
and `success1` are declared, derive correctly, and are reserved for output
that does not exist yet. They are part of the vocabulary rather than of the
current output, and theming them is not wasted: it is what keeps a theme valid
as dstow's output grows.

## What consumes what

dstow's internal vocabulary never appears in theming. Instead a fixed,
code-owned mapping sends each internal concept to a slot:

| Slot | dstow concepts that render in it |
|---|---|
| `section1` | `heading` |
| `name1` | `name` |
| `value2` | `muted` |
| `error1` | `damaged`, `contradicted`, `error` |
| `error2` | `broken` |
| `warning1` | `warning` |
| `warning2` | `partially stowed`, `drifted`, `orphaned` |
| `success2` | `stowed` |
| `info1` | `occupied`, `fix` |
| `info2` | `not stowed`, `note` |

Two identities in that table are structural rather than incidental:

- **`damaged` and `contradicted` are the same fact** seen from the package
  side and the ledger side, so they always render alike.
- **`orphaned` and `partially stowed`** are likewise two views of the same
  shortfall.

Because the mapping lives in code beside the descriptions `dstow theme slots`
prints, the two cannot drift: the printed consumer list *is* the mapping.

## Why two stages

The alternative — themes naming dstow's concepts directly, with an `orphaned`
key and a `contradicted` key — was rejected. It would mean:

- Every theme has to learn dstow's domain vocabulary before it can set a
  color.
- Every new concept dstow reports is a breaking change to the theme schema.
- Two concepts that must look alike can silently be given different colors,
  and nothing detects it.

With the mapping in code, a theme sets fourteen generic slots, dstow decides
what wears them, and the identities that must hold are held structurally.

The mapping is **closed for v1**. If a concept should move to a different
slot, that is a dstow change, not a theme's business.

## Defaults

The default palette declares the seven tier-1 slots and lets the rest derive.
It is grounded in prior art rather than invented: the prose and severity
styling follows Cargo's help styling, the de facto reference for CLI color in
this family of tools.

Defaults stay within the 16 base ANSI colors, so your terminal's own theme
supplies the actual colors. That is deliberate — it means dstow looks like
the rest of your terminal by default, and retheming your terminal rethemes
dstow.

    dstow theme emit                the effective palette right now
    dstow theme emit --format toml  as a theme file, to edit and keep
