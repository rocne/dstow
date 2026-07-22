# The ledger is a current-state index, not a journal

REQUIREMENTS §8.1 ("dstow records every link it creates and removes") admits two
readings: a current-state index maintained by create/remove operations, or an
append-only journal of events with state derived by replay. We chose the index:
the ledger holds only the links dstow currently believes exist; unstow deletes
entries; there is no history.

## Considered options

A journal was genuinely attractive — appends are O(1), and retained history
would unblock hypothetical v2+/v3 conveniences (undo, audit). It lost on two
grounds:

1. **Performance is a wash at dstow scale** in both directions. A dotfiles
   estate is hundreds to low-thousands of links; whole-file rewrite and full
   replay are both single-digit milliseconds. Neither format wins on speed.
2. **dstow's own semantics corrode a journal's history.** Disk is truth:
   entries contradicted by disk are pruned, so a journal would mix *deeds*
   (what dstow did) with *sightings* (corrections observed on disk) — exactly
   what makes replayed history untrustworthy. And `rebuild` reconstructs a
   lost ledger by walking targets, producing a synthetic snapshot with no
   history at all. Any feature leaning on journal history must already
   tolerate history-free ledgers, so the history is best-effort at most.
   Best-effort history can be had later by adding a separate, non-pinned
   advisory log beside the index — the future stays reachable without paying
   for it now. A journal would also pin strictly more format semantics
   forever: event vocabulary, replay rules, and a torn-tail repair rule
   (appends are not crash-atomic), versus the index's single entry schema
   behind one atomic temp-file+rename.

## Consequences

- Promoting the ledger to a journal is ruled out; history features, if ever
  wanted, are a separate artifact.
- "Pruned on sight" (§8.1) is realized by **writers only**: read commands
  (`list`, `info`, `status`, `check`) never mutate the ledger. A literal
  reading — reads pruning as they go — would make the ledger-attested
  `damaged` state a one-shot observation: reported once, evidence deleted,
  downgraded to `occupied` on the next look. Writers prune contradicted
  entries **in their scope**; `clean` is the ledger-wide broom. Evidence of
  damage dies only when the user resolves it or is told about it.
- The word "ledger" is kept deliberately: in double-entry bookkeeping the
  *journal* is the chronological log and the *ledger* is the organized
  current state posted from it — accounting's own distinction matches the
  decision.
