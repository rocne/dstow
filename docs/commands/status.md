# dstow status

<!-- dstow:short -->
What is deployed: live state of packages against their targets
<!-- /dstow:short -->

<!-- dstow:long -->
What is deployed — expected links against what targets actually hold.

Names scope to packages or whole repos. Package states: stowed, partially
stowed, not stowed, occupied, damaged — plus a drifted marker when the
deployed shape differs from what current config would produce. damaged is
only ever claimed with ledger evidence. Remote repos also show
behind/ahead as of the last update.

For a path: what occupies it, who owns it (per the ledger), and — if
occupied — the packages that could adopt it, ranked.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow status
  dstow status zsh
  dstow status ~/.zshrc      # the per-path view, adoption candidates incl.
<!-- /dstow:examples -->

## Two questions, two views

status answers one of two different questions depending on what you give it:

- **A name** (or several) asks the expected-vs-actual question: does what
  deploying this package now would produce match what the target holds? That is
  the package-state comparison the summary above describes.
- **A single path** asks a location question: what is at this spot, and what
  does dstow know about it? A path view is about one location and reads only the
  ledger and config — no package comparison.

You give **one path at a time** (a path is a single location), but names may be
many. The first character decides which kind an operand is, and dstow never
guesses between them — `dstow manual concepts naming` has the path-vs-name
rule. Give more than one operand and dstow reads them all as names.

## What the path view reports

For a single path, status reports, as they apply:

- whether anything is there at all;
- if it is a symlink, where it points;
- if it is not a symlink, what kind of thing it is (a regular file, a
  directory, …);
- the package the ledger records as **owning** that path, when there is one;
- and, when something really occupies the path, the **ranked adoption
  candidates** — the packages that could adopt it, in the same ranking `dstow
  manual commands adopt` uses.

A path dstow cannot observe — a permission error on the way to it — is reported
as `unobservable` rather than guessed at, the same honesty the finding classes
keep (`dstow manual concepts states`).
