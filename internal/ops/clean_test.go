package ops_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rocne/dstow/internal/ops"
)

// outcomeFor returns the outcome clean recorded for the entry at link, or
// fails if it is absent.
func outcomeFor(t *testing.T, res *ops.CleanResult, link string) ops.CleanFinding {
	t.Helper()
	for _, f := range res.Findings {
		if f.Entry.Link == link {
			return f
		}
	}
	t.Fatalf("no clean finding for link %q: %+v", link, res.Findings)
	return ops.CleanFinding{}
}

// TestCleanRemovesBrokenFreely: a broken link is removed and its entry
// deleted with no prompt (§6.4).
func TestCleanRemovesBrokenFreely(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")
	if err := os.Remove(filepath.Join(root, "zsh", "dot-zshrc")); err != nil {
		t.Fatal(err)
	}

	res, err := e.app.Clean(ops.CleanRequest{})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if res.Failed() {
		t.Fatalf("clean of a broken link must not fail: %+v", res.Findings)
	}
	if f := outcomeFor(t, res, ".zshrc"); f.Outcome != ops.OutcomeRemoved {
		t.Errorf("broken → removed, got %v", f.Outcome)
	}
	if _, err := os.Lstat(filepath.Join(e.target, ".zshrc")); !os.IsNotExist(err) {
		t.Error("the broken link must be removed")
	}
	if len(loadLedger(t, e.app.LedgerPath).Targets[e.target]) != 0 {
		t.Error("the entry must be deleted with its link")
	}
	if len(e.prompt.asked) != 0 {
		t.Error("broken removal asks nothing")
	}
}

// TestCleanPrunesContradicted: a contradicted entry is pruned by Update — the
// entry goes, disk is never touched — and the prune surfaces with evidence.
func TestCleanPrunesContradicted(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")
	// Replace the link with a regular file: disk contradicts the entry.
	if err := os.Remove(filepath.Join(e.target, ".zshrc")); err != nil {
		t.Fatal(err)
	}
	e.writeFile(filepath.Join(e.target, ".zshrc"), "usurper\n")

	res, err := e.app.Clean(ops.CleanRequest{})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if len(res.Pruned) != 1 || res.Pruned[0].Evidence == "" {
		t.Fatalf("the contradicted entry is pruned with evidence: %+v", res.Pruned)
	}
	if f := outcomeFor(t, res, ".zshrc"); f.Outcome != ops.OutcomePruned {
		t.Errorf("contradicted → pruned, got %v", f.Outcome)
	}
	if len(loadLedger(t, e.app.LedgerPath).Targets[e.target]) != 0 {
		t.Error("the contradicted entry must be gone from the ledger")
	}
	if data, _ := os.ReadFile(filepath.Join(e.target, ".zshrc")); string(data) != "usurper\n" {
		t.Error("clean must never touch disk for a contradicted entry")
	}
}

// TestCleanOrphanConfirmed: an orphan is removed only after confirmation; the
// accepted case removes link and entry.
func TestCleanOrphanConfirmed(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")
	e.writeFile(filepath.Join(root, "zsh", ".dstow", "config.toml"),
		"ignore = [\"dot-zshrc\"]\n")

	e.prompt.answers = []bool{true}
	res, err := e.app.Clean(ops.CleanRequest{})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if len(e.prompt.asked) != 1 {
		t.Fatalf("each orphan is confirmed once: %v", e.prompt.asked)
	}
	if f := outcomeFor(t, res, ".zshrc"); f.Outcome != ops.OutcomeRemoved {
		t.Errorf("confirmed orphan → removed, got %v", f.Outcome)
	}
	if _, err := os.Lstat(filepath.Join(e.target, ".zshrc")); !os.IsNotExist(err) {
		t.Error("a confirmed orphan link is removed")
	}
}

// TestCleanOrphanDeclined: a declined orphan is kept — recorded as declined,
// not a run error, and neither link nor entry is touched.
func TestCleanOrphanDeclined(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")
	e.writeFile(filepath.Join(root, "zsh", ".dstow", "config.toml"),
		"ignore = [\"dot-zshrc\"]\n")

	e.prompt.answers = []bool{false}
	res, err := e.app.Clean(ops.CleanRequest{})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if res.Failed() {
		t.Error("a declined orphan is not a run error")
	}
	if f := outcomeFor(t, res, ".zshrc"); f.Outcome != ops.OutcomeDeclined {
		t.Errorf("declined orphan → declined, got %v", f.Outcome)
	}
	if _, err := os.Lstat(filepath.Join(e.target, ".zshrc")); err != nil {
		t.Error("a declined orphan link stays")
	}
	if len(loadLedger(t, e.app.LedgerPath).Targets[e.target]) != 1 {
		t.Error("a declined orphan entry stays")
	}
}

// TestCleanOrphanForce: --force removes orphans without asking (REQ §1.6).
func TestCleanOrphanForce(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")
	e.writeFile(filepath.Join(root, "zsh", ".dstow", "config.toml"),
		"ignore = [\"dot-zshrc\"]\n")

	res, err := e.app.Clean(ops.CleanRequest{Force: true})
	if err != nil {
		t.Fatalf("Clean --force: %v", err)
	}
	if len(e.prompt.asked) != 0 {
		t.Error("--force asks nothing")
	}
	if f := outcomeFor(t, res, ".zshrc"); f.Outcome != ops.OutcomeRemoved {
		t.Errorf("forced orphan → removed, got %v", f.Outcome)
	}
	if _, err := os.Lstat(filepath.Join(e.target, ".zshrc")); !os.IsNotExist(err) {
		t.Error("forced orphan link is removed")
	}
}

// TestCleanOrphanPrompterError: a Prompter error (the non-interactive stance)
// fails that orphan and keeps it; the run reports failure and continues.
func TestCleanOrphanPrompterError(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")
	e.writeFile(filepath.Join(root, "zsh", ".dstow", "config.toml"),
		"ignore = [\"dot-zshrc\"]\n")

	// No scripted answers: the fake prompter errors like a non-interactive one.
	res, err := e.app.Clean(ops.CleanRequest{})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if !res.Failed() {
		t.Error("a Prompter error fails the run")
	}
	f := outcomeFor(t, res, ".zshrc")
	if f.Outcome != ops.OutcomeFailed || f.Err == nil {
		t.Errorf("prompter error → failed with Err, got %+v", f)
	}
	if _, err := os.Lstat(filepath.Join(e.target, ".zshrc")); err != nil {
		t.Error("a prompter error keeps the orphan link")
	}
}

// TestCleanNeverTouchesUnobservable: an unobservable entry is left alone —
// no removal, no prune, no disk change (#45).
func TestCleanNeverTouchesUnobservable(t *testing.T) {
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

	res, err := e.app.Clean(ops.CleanRequest{})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if f := outcomeFor(t, res, "sub/.thing"); f.Outcome != ops.OutcomeUntouched {
		t.Errorf("unobservable → untouched, got %v", f.Outcome)
	}
	if len(loadLedger(t, e.app.LedgerPath).Targets[e.target]) != 1 {
		t.Error("an unobservable entry survives untouched")
	}
}

// TestCleanRecomputesFresh: clean classifies fresh under its own lock. An
// entry that check would call broken but that has healed by clean-time is
// left alone — clean never executes a stale saved report (§6.4).
func TestCleanRecomputesFresh(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	src := filepath.Join(root, "zsh", "dot-zshrc")
	e.writeFile(src, "x\n")
	stowPkg(t, e, "zsh")

	// Break it, observe check sees broken.
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	rep, err := e.app.Check()
	if err != nil {
		t.Fatal(err)
	}
	oneFinding(t, rep.Findings, ops.ClassBroken)

	// Heal it between check and clean; clean's own recompute must see health.
	e.writeFile(src, "x\n")
	res, err := e.app.Clean(ops.CleanRequest{})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if res.Failed() {
		t.Fatalf("healed entry must not fail: %+v", res.Findings)
	}
	for _, f := range res.Findings {
		if f.Outcome == ops.OutcomeRemoved {
			t.Errorf("clean must not act on a healed entry: %+v", f)
		}
	}
	if _, err := os.Lstat(filepath.Join(e.target, ".zshrc")); err != nil {
		t.Error("the healed link stays")
	}
	if len(loadLedger(t, e.app.LedgerPath).Targets[e.target]) != 1 {
		t.Error("the healed entry stays")
	}
}

// TestCleanNoChangeDoesNotWrite: a clean that finds nothing to do writes
// nothing — the ledger file is unchanged byte-for-byte (the fn sentinel).
func TestCleanNoChangeDoesNotWrite(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh")

	before, err := os.ReadFile(e.app.LedgerPath)
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.app.Clean(ops.CleanRequest{})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if len(res.Findings) != 0 || len(res.Pruned) != 0 {
		t.Errorf("a healthy ledger has nothing to clean: %+v / %+v", res.Findings, res.Pruned)
	}
	after, err := os.ReadFile(e.app.LedgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("a no-op clean must not rewrite the ledger")
	}
}
