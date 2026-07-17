package ops

// check (§6.4): the read half of maintenance. A lock-free ledger.Load,
// nothing written, nothing mutated — the shared classifier over the snapshot,
// findings in the deterministic report order. Ledger refusal errors (corrupt
// / newer version) return untouched; their remedies ride the ledger package's
// error strings (§6.5).

import "github.com/rocne/dstow/internal/ledger"

// CheckReport is a check run as data (A4): every classified entry and every
// diagnostic the classification raised.
type CheckReport struct {
	Findings []Finding
	Warnings []Warning
}

// Check classifies every ledgered link against config and disk without taking
// the lock or writing anything (§6.4). clean recomputes this same
// classification under its lock, so the two can never disagree.
func (a *App) Check() (*CheckReport, error) {
	led, err := ledger.Load(a.LedgerPath)
	if err != nil {
		return nil, err // corrupt / newer-version refusals pass through (§6.5)
	}
	c := a.newClassifier()
	findings := c.classify(led)
	sortFindings(findings)
	return &CheckReport{Findings: findings, Warnings: c.warnings}, nil
}
