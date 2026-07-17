package ops_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/ops"
)

// --- shared maintenance-test helpers (check/clean/rebuild) ---

// stowPkg deploys one package the ordinary way, so the target holds a real
// link and the ledger a real entry — the honest starting point for the
// stale-entry scenarios.
func stowPkg(t *testing.T, e *env, name string) {
	t.Helper()
	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{name}})
	if err != nil {
		t.Fatalf("stow %s: %v", name, err)
	}
	if res.Failed() {
		t.Fatalf("stow %s failed: %+v", name, res.Packages)
	}
}

// loadLedger loads the ledger or fails the test.
func loadLedger(t *testing.T, path string) ledger.Ledger {
	t.Helper()
	led, err := ledger.Load(path)
	if err != nil {
		t.Fatalf("ledger load: %v", err)
	}
	return led
}

// makeLink writes a relative symlink at linkPath pointing to destAbs — a
// hand-made link, exactly what rebuild must discover and check may orphan.
func makeLink(t *testing.T, linkPath, destAbs string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		t.Fatal(err)
	}
	text, err := filepath.Rel(filepath.Dir(linkPath), destAbs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(text, linkPath); err != nil {
		t.Fatal(err)
	}
}

// skipIfRoot skips permission-based tests under uid 0, which bypasses the
// mode bits the unobservable class relies on (the conventional Go guard).
func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission-based test is meaningless as root")
	}
}

// classCounts tallies findings by class.
func classCounts(fs []ops.Finding) map[ops.Class]int {
	out := map[ops.Class]int{}
	for _, f := range fs {
		out[f.Class]++
	}
	return out
}

// oneFinding asserts exactly one finding of the given class and returns it.
func oneFinding(t *testing.T, fs []ops.Finding, class ops.Class) ops.Finding {
	t.Helper()
	var found []ops.Finding
	for _, f := range fs {
		if f.Class == class {
			found = append(found, f)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one %s finding, got %+v", class, fs)
	}
	if found[0].Evidence == "" {
		t.Errorf("every finding carries evidence prose: %+v", found[0])
	}
	return found[0]
}

// TestCheckHealthyIsQuiet: a fully deployed package produces no findings —
// only stale entries are reported.
func TestCheckHealthyIsQuiet(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")

	rep, err := e.app.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(rep.Findings) != 0 {
		t.Errorf("a healthy ledger yields no findings: %+v", rep.Findings)
	}
}

// TestCheckBroken: a link that still agrees with the ledger but whose
// destination is gone is broken (§6.4).
func TestCheckBroken(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")
	// The destination vanishes; the link text still matches the entry.
	if err := os.Remove(filepath.Join(root, "zsh", "dot-zshrc")); err != nil {
		t.Fatal(err)
	}

	rep, err := e.app.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	f := oneFinding(t, rep.Findings, ops.ClassBroken)
	if f.Entry.Link != ".zshrc" {
		t.Errorf("broken finding names the link: %+v", f)
	}
}

// TestCheckContradicted: disk disagreeing with the entry (link gone) is
// contradicted — the one owner of the disk-disagrees test decides it.
func TestCheckContradicted(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")
	// Remove the link entirely: disk no longer shows what the entry records.
	if err := os.Remove(filepath.Join(e.target, ".zshrc")); err != nil {
		t.Fatal(err)
	}

	rep, err := e.app.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	oneFinding(t, rep.Findings, ops.ClassContradicted)
}

// TestCheckOrphaned: an intact link into a known repo that the current config
// would no longer produce (its source is now ignored) is orphaned (§6.4).
func TestCheckOrphaned(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")
	// Config now ignores the source: Expected no longer maps this link, but
	// the link and its destination are both intact.
	e.writeFile(filepath.Join(root, "zsh", ".dstow", "config.toml"),
		"ignore = [\"dot-zshrc\"]\n")

	rep, err := e.app.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	oneFinding(t, rep.Findings, ops.ClassOrphaned)
}

// TestCheckOrphanUnansweredWarns: a package whose config fails to load makes
// the orphan test unanswerable — the entry is left unclassified and a Warning
// is raised instead of an unbacked claim (§1.5).
func TestCheckOrphanUnansweredWarns(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")
	// Break the package config so its Expected cannot be computed.
	e.writeFile(filepath.Join(root, "zsh", ".dstow", "config.toml"),
		"this is = not valid = toml\n")

	rep, err := e.app.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if c := classCounts(rep.Findings); c[ops.ClassOrphaned] != 0 {
		t.Errorf("no orphan claim when the test is unanswerable: %+v", rep.Findings)
	}
	if len(rep.Warnings) == 0 {
		t.Errorf("an unanswerable orphan test raises a Warning (§1.5)")
	}
}

// TestCheckUnobservable: an observation failing non-ENOENT (a chmod-000
// parent) is a read-only unobservable row (#45), never a contradiction.
func TestCheckUnobservable(t *testing.T) {
	skipIfRoot(t)
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "sub", "dot-thing"), "x\n")
	stowPkg(t, e, "zsh")

	dir := filepath.Join(e.target, "sub")
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	rep, err := e.app.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	oneFinding(t, rep.Findings, ops.ClassUnobservable)
}

// TestCheckWritesNothing: check is a pure read — the ledger file is unchanged
// byte-for-byte and no findings mutate it (§6.3/§6.4).
func TestCheckWritesNothing(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")
	// Make the entry broken so there is a finding to report.
	if err := os.Remove(filepath.Join(root, "zsh", "dot-zshrc")); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(e.app.LedgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.app.Check(); err != nil {
		t.Fatalf("Check: %v", err)
	}
	after, err := os.ReadFile(e.app.LedgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("check must not write the ledger")
	}
}

// TestCheckMissingLedgerIsEmpty: a missing ledger file is an empty ledger,
// never an error (§6.5).
func TestCheckMissingLedgerIsEmpty(t *testing.T) {
	e := newEnv(t)
	e.addRepo("dots")

	rep, err := e.app.Check()
	if err != nil {
		t.Fatalf("Check on a missing ledger must not error: %v", err)
	}
	if len(rep.Findings) != 0 {
		t.Errorf("a missing ledger yields no findings: %+v", rep.Findings)
	}
	if _, err := os.Stat(e.app.LedgerPath); !os.IsNotExist(err) {
		t.Error("check must not create the ledger file")
	}
}
