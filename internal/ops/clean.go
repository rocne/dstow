package ops

// clean (§6.4): the write half of maintenance — the ledger-wide broom. One
// ledger.Update under Scope{All: true}: Update itself prunes the contradicted
// entries (returning their evidence), then fn recomputes the classification
// fresh on the already-pruned document (never a stale saved report) and acts
// per class — broken removed freely, orphaned behind confirmation, unobservable
// left alone. One atomic write at the end is Update's commit; a run that
// mutates nothing returns a sentinel so no write happens (like deploy.go's
// errGlobalBlocked).

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/rocne/dstow/internal/ledger"
)

// CleanRequest parameterizes a clean run.
type CleanRequest struct {
	Force bool // remove orphans without confirmation (REQUIREMENTS §1.6)
}

// CleanOutcome is what clean did with one finding.
type CleanOutcome int

const (
	// OutcomeRemoved: the link was removed and its entry deleted.
	OutcomeRemoved CleanOutcome = iota
	// OutcomePruned: a contradicted entry Update pruned (disk untouched).
	OutcomePruned
	// OutcomeDeclined: an orphan whose confirmation was declined — kept.
	OutcomeDeclined
	// OutcomeFailed: the removal or the prompt errored; the entry stays and
	// the run continues. Err carries the cause.
	OutcomeFailed
	// OutcomeUntouched: an unobservable finding — clean never acts on it.
	OutcomeUntouched
)

func (o CleanOutcome) String() string {
	switch o {
	case OutcomeRemoved:
		return "removed"
	case OutcomePruned:
		return "pruned"
	case OutcomeDeclined:
		return "declined"
	case OutcomeFailed:
		return "failed"
	case OutcomeUntouched:
		return "untouched"
	}
	return fmt.Sprintf("CleanOutcome(%d)", int(o))
}

// CleanFinding is one finding with what clean did about it.
type CleanFinding struct {
	Finding
	Outcome CleanOutcome
	Err     error // set when Outcome is OutcomeFailed
}

// CleanResult is a clean run as data (A4). Findings carries every classified
// entry with its outcome (contradicted entries appear as pruned rows, drawn
// from Pruned); Pruned is Update's raw prune evidence.
type CleanResult struct {
	Findings []CleanFinding
	Pruned   []ledger.Pruned
	Warnings []Warning
}

// Failed reports whether any finding errored (§3.2, A3 exit 1).
func (r *CleanResult) Failed() bool {
	for _, f := range r.Findings {
		if f.Outcome == OutcomeFailed {
			return true
		}
	}
	return false
}

// errCleanNoChange aborts the transaction when clean neither pruned nor
// removed anything: nothing to commit, so nothing is written (§6.2).
var errCleanNoChange = errors.New("ops: clean found nothing to change")

// Clean removes the stale links check reported, recomputing the plan fresh
// under the ledger lock (§6.4). Contradicted entries are pruned by Update;
// broken links are removed freely; orphans are confirmed per link (unless
// Force); unobservable entries are left alone.
func (a *App) Clean(req CleanRequest) (*CleanResult, error) {
	res := &CleanResult{}
	c := a.newClassifier()

	pruned, uerr := ledger.Update(a.LedgerPath, ledger.Scope{All: true}, func(l *ledger.Ledger) error {
		// Update has already pruned the contradicted entries the scope covers;
		// detect it so a prune-only run still commits (a prune IS a mutation).
		before, lerr := ledger.Load(a.LedgerPath)
		if lerr != nil {
			return lerr
		}
		mutated := entryCount(before) != entryCount(*l)

		for _, f := range c.classify(*l) {
			switch f.Class {
			case ClassBroken:
				res.Findings = append(res.Findings, a.removeFinding(l, f, &mutated))
			case ClassOrphaned:
				res.Findings = append(res.Findings, a.cleanOrphan(l, f, req.Force, &mutated))
			case ClassUnobservable:
				res.Findings = append(res.Findings, CleanFinding{Finding: f, Outcome: OutcomeUntouched})
			}
		}
		res.Warnings = c.warnings

		if !mutated {
			return errCleanNoChange
		}
		return nil
	})
	if uerr != nil && !errors.Is(uerr, errCleanNoChange) {
		return nil, uerr // corrupt / newer-version / lock refusals pass through
	}

	// Report each contradicted prune with its evidence, both as raw Pruned and
	// as a pruned finding so Findings is the full report check would also give.
	res.Pruned = append(res.Pruned, pruned...)
	for _, p := range pruned {
		res.Findings = append(res.Findings, CleanFinding{
			Finding: Finding{
				TargetRoot: p.TargetRoot, Entry: p.Entry,
				Class: ClassContradicted, Evidence: p.Evidence,
			},
			Outcome: OutcomePruned,
		})
	}
	sortCleanFindings(res.Findings)
	return res, nil
}

// removeFinding removes a broken (or confirmed orphan) link and deletes its
// entry. A removal failure records the error on the finding, keeps the entry,
// and the run continues (§6.4).
func (a *App) removeFinding(l *ledger.Ledger, f Finding, mutated *bool) CleanFinding {
	cf := CleanFinding{Finding: f}
	linkPath := filepath.Join(f.TargetRoot, f.Entry.Link)
	if err := os.Remove(linkPath); err != nil {
		cf.Outcome, cf.Err = OutcomeFailed, err
		return cf
	}
	deleteEntry(l, f.TargetRoot, f.Entry)
	cf.Outcome = OutcomeRemoved
	*mutated = true
	return cf
}

// cleanOrphan gates an orphan removal behind confirmation (unless Force). A
// declined orphan is kept and recorded, not a run error; a Prompter error
// fails that finding and the run continues (§6.4).
func (a *App) cleanOrphan(l *ledger.Ledger, f Finding, force bool, mutated *bool) CleanFinding {
	if !force {
		ok, perr := a.Prompt.Confirm(orphanPrompt(f), false)
		if perr != nil {
			return CleanFinding{Finding: f, Outcome: OutcomeFailed, Err: perr}
		}
		if !ok {
			return CleanFinding{Finding: f, Outcome: OutcomeDeclined}
		}
	}
	return a.removeFinding(l, f, mutated)
}

// orphanPrompt is the confirmation prose for one orphan: the evidence (which
// names the link path and the repo it points into) plus the consequence.
func orphanPrompt(f Finding) string {
	return fmt.Sprintf("%s. Remove the link and delete its ledger entry?", f.Evidence)
}

// deleteEntry drops the entry from its target group, dropping the group when
// it empties.
func deleteEntry(l *ledger.Ledger, root string, e ledger.Entry) {
	group := l.Targets[root]
	kept := group[:0:0]
	for _, x := range group {
		if x.Link == e.Link && x.Package == e.Package {
			continue
		}
		kept = append(kept, x)
	}
	if len(kept) == 0 {
		delete(l.Targets, root)
		return
	}
	l.Targets[root] = kept
}

// entryCount totals the entries across every target group.
func entryCount(l ledger.Ledger) int {
	n := 0
	for _, es := range l.Targets {
		n += len(es)
	}
	return n
}

// sortCleanFindings orders clean findings in the report order (§6.4).
func sortCleanFindings(fs []CleanFinding) {
	sort.Slice(fs, func(i, j int) bool {
		if fs[i].TargetRoot != fs[j].TargetRoot {
			return fs[i].TargetRoot < fs[j].TargetRoot
		}
		if fs[i].Entry.Link != fs[j].Entry.Link {
			return fs[i].Entry.Link < fs[j].Entry.Link
		}
		return fs[i].Entry.Package < fs[j].Entry.Package
	})
}
