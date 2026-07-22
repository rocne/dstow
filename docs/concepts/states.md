# States: the vocabulary dstow reports in

dstow reports in a fixed vocabulary. Every word below means exactly one thing,
in output, in `--json`, and in this manual. Two vocabularies exist, because
two different commands ask two different questions.

- **Package states** — what `dstow status` reports about a package, by
  comparing what deploying it *now* would produce against what the target
  actually holds.
- **Finding classes** — what `dstow check` reports about a *ledger entry* that
  has gone stale, and what `dstow clean` will therefore do about it.

## Package states

A package state is computed by comparing what applying the package now, under
the **current effective config**, would produce against what the target
actually holds. It is always about the present, never about history.

| State | What it claims |
|---|---|
| `stowed` | Everything expected is present and points at this package. |
| `partially stowed` | Some expected links are present; the rest are *merely missing* — nothing is in their way. |
| `not stowed` | Nothing of the expected set is present. |
| `occupied` | An expected path holds something that is not this package's link. Deliberately neutral: the spot is taken, with no claim about how. |
| `damaged` | The ledger-attested escalation of `occupied`: dstow linked this path, and disk now disagrees. |

And one marker, which is not a state:

| Marker | What it claims |
|---|---|
| `drifted` | Set on a `stowed` package whose *deployed shape* differs from what current config would produce — for example, the fold setting changed since deployment. Configuration drift, in the ordinary ops sense. |

### Precedence

A package is in exactly one state. When more than one could apply, this order
decides — first match wins:

    damaged  >  occupied  >  stowed  >  partially stowed  >  not stowed

The two entries worth understanding:

- **`occupied` outranks the stowed/partial spectrum.** If one expected path is
  blocked, the package is `occupied` even when its other links are all
  deployed. `partially stowed` claims the remainder is *merely missing* — that
  claim is false the moment something is in the way, and a state that
  understates a blockage is worse than one that overstates progress.
- **`damaged` is only ever claimed with ledger evidence.** Without a ledger
  record saying dstow put a link there, the honest answer is `occupied`: dstow
  will not accuse itself of breakage it cannot attest to.

An empty package — nothing to deploy at all — reports `not stowed`.

### Per-link states

`status` also reports each individual link, in a parallel four-word
vocabulary: `stowed` (a symlink owned by this package sits at the slot),
`missing` (nothing exists there), `occupied` (a real file, directory, or
foreign link is there), and `damaged` (the ledger recorded a link here and
disk contradicts it).

## Finding classes

`check` walks the ledger, not the filesystem tree: it verifies each link dstow
believes it made. Every stale entry it finds is classified, and the class
decides what `clean` does. `check` and `clean` share one classifier, so the
two can never disagree — `clean` executes exactly what `check` reported.

Classification is first-match-wins in this order:

| Class | What it means | What `clean` does |
|---|---|---|
| `unobservable` | An observation the classification needs failed for a reason other than "not there" — a permission error on the link, an unreadable destination. The evidence is the OS error. | **Nothing.** It is a read-only report row. dstow will not act on a link it could not look at. |
| `contradicted` | Disk disagrees with the ledger entry: the link is gone, is not a link, or points somewhere else. | Prunes the ledger entry. **Disk is never touched** — the record was wrong, not the filesystem. |
| `broken` | The link agrees with the ledger, but its recorded destination is gone. | Removes the link and the entry, freely and without asking. |
| `orphaned` | An intact link that resolves into a known repo, but which no current configuration would produce. | Removes it **behind a confirmation** (`--yes` or `--force` to remove without asking). |

The split between `broken` and `orphaned` is the split between "this link
points at nothing" and "this link points at something real that you no longer
asked for" — the second is a judgment about your intent, which is why it asks
and the first does not.

## The relationship between the two vocabularies

They overlap in one place and it is deliberate: `damaged` (a package state)
and `contradicted` (a finding class) are the same underlying fact — dstow's
record and the disk disagree — seen from the two directions. `status` says the
*package* is damaged; `check` says the *entry* is contradicted. Likewise
`orphaned` (a finding) is the ledger-side view of links that a `partially
stowed` or reconfigured package left behind.

Both identities are structural rather than stylistic: they are wired into the
color-slot mapping, so `damaged` and `contradicted` always render alike, as do
`orphaned` and `partially stowed`. See `dstow manual theming slots`.
