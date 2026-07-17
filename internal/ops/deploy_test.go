package ops_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/ops"
)

// TestStowCreatesLinksAndLedger: a stow run links the package into its
// target (dot-translation on by default, §3.4), records each link as a
// ledger entry (§6.4), and succeeds.
func TestStowCreatesLinksAndLedger(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "zsh config\n")

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if res.Failed() {
		t.Fatalf("run failed: %+v", res.Packages)
	}

	isLinkTo(t, filepath.Join(e.target, ".zshrc"), filepath.Join(root, "zsh", "dot-zshrc"))

	led, err := ledger.Load(e.app.LedgerPath)
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	entries := led.Targets[e.target]
	if len(entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got %+v", entries)
	}
	en := entries[0]
	if en.Link != ".zshrc" || en.Source != "dot-zshrc" {
		t.Errorf("entry = %+v, want link .zshrc source dot-zshrc", en)
	}
	if en.Package != pkgFQN(root, "zsh").String() {
		t.Errorf("entry package = %q, want %q", en.Package, pkgFQN(root, "zsh"))
	}
	if !en.RecordedAt.Equal(fixedNow) {
		t.Errorf("RecordedAt = %v, want the pinned clock", en.RecordedAt)
	}
}

// TestPerPackageIndependence: an occupied path fails its package with a
// typed conflict; the other package succeeds; the run reports both and
// Failed() is true (§3.2).
func TestPerPackageIndependence(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "pkg\n")
	e.writeFile(filepath.Join(root, "git", "dot-gitconfig"), "pkg\n")
	e.writeFile(filepath.Join(e.target, ".zshrc"), "live\n") // occupies zsh's path

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh", "git"}})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	st := statuses(res)
	if st[pkgFQN(root, "zsh").String()] != ops.StatusFailed {
		t.Errorf("zsh should fail on the occupied path: %v", st)
	}
	if st[pkgFQN(root, "git").String()] != ops.StatusSucceeded {
		t.Errorf("git should succeed independently: %v", st)
	}
	if !res.Failed() {
		t.Error("run with a failed package must report failure (§3.2)")
	}
	var pkgRes ops.PackageResult
	for _, p := range res.Packages {
		if p.FQN.Package == "zsh" {
			pkgRes = p
		}
	}
	var ce *engine.ConflictError
	if !errors.As(pkgRes.Err, &ce) {
		t.Errorf("zsh's error should be a typed conflict, got %T: %v", pkgRes.Err, pkgRes.Err)
	}
	isLinkTo(t, filepath.Join(e.target, ".gitconfig"), filepath.Join(root, "git", "dot-gitconfig"))
}

// TestNotFoundOperand: an operand resolving to nothing is a per-package
// not-found line, and the run fails (§3.2: not-found included).
func TestNotFoundOperand(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"nope", "zsh"}})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	st := statuses(res)
	if st["nope"] != ops.StatusNotFound {
		t.Errorf("expected not-found for %q: %v", "nope", st)
	}
	if !res.Failed() {
		t.Error("a not-found operand fails the run")
	}
}

// TestAmbiguousOperand: a name matching packages in two repos refuses the
// run with the sorted qualified spellings (§1.2).
func TestAmbiguousOperand(t *testing.T) {
	e := newEnv(t)
	rootA := e.addRepo("aaa")
	rootB := e.addRepo("bbb")
	e.writeFile(filepath.Join(rootA, "zsh", "f"), "x\n")
	e.writeFile(filepath.Join(rootB, "zsh", "f"), "x\n")

	_, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}})
	var amb *ops.AmbiguousNameError
	if !errors.As(err, &amb) {
		t.Fatalf("expected AmbiguousNameError, got %T: %v", err, err)
	}
	if len(amb.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %v", amb.Matches)
	}
	if amb.Matches[0].String() >= amb.Matches[1].String() {
		t.Errorf("matches must be sorted canonically: %v", amb.Matches)
	}
	if !strings.Contains(amb.Error(), "qualified") {
		t.Errorf("the refusal names its remedy (§1.4): %q", amb.Error())
	}
}

// TestBulkExcludeAndExplicitOverride: exclude_from_bulk drops a package
// from the bulk run silently; naming it explicitly overrides (§2.5).
func TestBulkExcludeAndExplicitOverride(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	e.writeFile(filepath.Join(root, "private", "dot-private"), "x\n")
	e.writeFile(filepath.Join(root, "private", ".dstow", "config.toml"), "exclude_from_bulk = true\n")

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow}) // bulk
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	st := statuses(res)
	if _, present := st[pkgFQN(root, "private").String()]; present {
		t.Errorf("excluded package must not appear in a bulk run: %v", st)
	}
	if st[pkgFQN(root, "zsh").String()] != ops.StatusSucceeded {
		t.Errorf("zsh should stow in bulk: %v", st)
	}

	res, err = e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"private"}})
	if err != nil {
		t.Fatalf("Deploy explicit: %v", err)
	}
	if statuses(res)[pkgFQN(root, "private").String()] != ops.StatusSucceeded {
		t.Errorf("explicit naming overrides bulk exclusion (§2.5): %+v", res.Packages)
	}
}

// TestCanonicalOrder: results come back in canonical-FQN order regardless
// of operand order (ruled 2026-07-17 on #44).
func TestCanonicalOrder(t *testing.T) {
	e := newEnv(t)
	rootA := e.addRepo("aaa")
	rootB := e.addRepo("bbb")
	e.writeFile(filepath.Join(rootA, "beta", "b"), "x\n")
	e.writeFile(filepath.Join(rootA, "alpha", "a"), "x\n")
	e.writeFile(filepath.Join(rootB, "gamma", "g"), "x\n")

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow,
		Names: []string{"gamma", "beta", "alpha"}})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	var got []string
	for _, p := range res.Packages {
		got = append(got, p.FQN.Package)
	}
	want := []string{"alpha", "beta", "gamma"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want canonical %v", got, want)
	}
}

// TestUnstowRemovesLinksAndEntries: unstow deletes the links and their
// entries, keyed on path (§6.4; gostow remove tasks carry no source).
func TestUnstowRemovesLinksAndEntries(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")

	if _, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}}); err != nil {
		t.Fatal(err)
	}
	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbUnstow, Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("unstow: %v", err)
	}
	if res.Failed() {
		t.Fatalf("unstow failed: %+v", res.Packages)
	}
	if _, err := os.Lstat(filepath.Join(e.target, ".zshrc")); !os.IsNotExist(err) {
		t.Error("the link must be removed")
	}
	led, err := ledger.Load(e.app.LedgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(led.Targets[e.target]) != 0 {
		t.Errorf("entries must be deleted with their links: %+v", led.Targets[e.target])
	}
}

// TestRestowIntactIsIdempotent: restow of an intact package reports no
// actions (stow cancels the opposing pairs) yet the ledger still holds the
// package's entry set — reconciled from Expected, never from Actions
// (D16 + the #42 flag). RecordedAt of unchanged entries is preserved.
func TestRestowIntactIsIdempotent(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")

	if _, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}}); err != nil {
		t.Fatal(err)
	}
	later := fixedNow.Add(time.Hour)
	e.app.Now = func() time.Time { return later }

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbRestow, Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("restow: %v", err)
	}
	if res.Failed() {
		t.Fatalf("restow failed: %+v", res.Packages)
	}
	for _, p := range res.Packages {
		if len(p.Actions) != 0 {
			t.Errorf("an intact restow surfaces no actions, got %+v", p.Actions)
		}
	}
	led, err := ledger.Load(e.app.LedgerPath)
	if err != nil {
		t.Fatal(err)
	}
	entries := led.Targets[e.target]
	if len(entries) != 1 || entries[0].Link != ".zshrc" {
		t.Fatalf("the entry set must survive an intact restow (from Expected): %+v", entries)
	}
	if !entries[0].RecordedAt.Equal(fixedNow) {
		t.Errorf("an unchanged entry keeps its history: RecordedAt = %v, want %v",
			entries[0].RecordedAt, fixedNow)
	}
	isLinkTo(t, filepath.Join(e.target, ".zshrc"), filepath.Join(root, "zsh", "dot-zshrc"))
}

// TestRestowNotStowedJustStows: restow on a not-stowed package stows it —
// the unstow phase no-ops (D16).
func TestRestowNotStowedJustStows(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbRestow, Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("restow: %v", err)
	}
	if res.Failed() {
		t.Fatalf("restow failed: %+v", res.Packages)
	}
	isLinkTo(t, filepath.Join(e.target, ".zshrc"), filepath.Join(root, "zsh", "dot-zshrc"))
	led, err := ledger.Load(e.app.LedgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(led.Targets[e.target]) != 1 {
		t.Errorf("restow-as-stow records its entries: %+v", led.Targets[e.target])
	}
}

// TestDryRunChangesNothing: -n reports the plan and mutates neither the
// target nor the ledger (D8), and the first-run note appears when the
// ledger does not exist yet (ruled 2026-07-17 on #44).
func TestDryRunChangesNothing(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if res.Failed() {
		t.Fatalf("dry-run failed: %+v", res.Packages)
	}
	var plan []engine.Action
	for _, p := range res.Packages {
		plan = append(plan, p.Actions...)
	}
	if len(plan) == 0 {
		t.Error("dry-run must report the plan")
	}
	if _, err := os.Lstat(filepath.Join(e.target, ".zshrc")); !os.IsNotExist(err) {
		t.Error("dry-run must not create links")
	}
	if _, err := os.Stat(e.app.LedgerPath); !os.IsNotExist(err) {
		t.Error("dry-run must not write the ledger")
	}
	found := false
	for _, n := range res.Notes {
		if strings.Contains(n, "folding") && strings.Contains(n, "fold_trees") {
			found = true
		}
	}
	if !found {
		t.Errorf("first run announces the folding default, naming the knob (§3.6): %v", res.Notes)
	}
}

// TestFirstRunNoteRetires: once the ledger exists, the folding
// announcement stops.
func TestFirstRunNoteRetires(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")

	if _, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}}); err != nil {
		t.Fatal(err)
	}
	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbRestow, Names: []string{"zsh"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range res.Notes {
		if strings.Contains(n, "first run") {
			t.Errorf("the first-run note must retire once the ledger exists: %v", res.Notes)
		}
	}
}

// TestMissingTargetCreatedAndAnnounced: a missing target directory is
// auto-created and announced (§3.1, REQ §1.3).
func TestMissingTargetCreatedAndAnnounced(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	sub := filepath.Join(e.target, "deep", "cfg")
	e.writeFile(filepath.Join(root, ".dstow", "config.toml"),
		"target = \""+sub+"\"\n")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if res.Failed() {
		t.Fatalf("failed: %+v", res.Packages)
	}
	isLinkTo(t, filepath.Join(sub, ".zshrc"), filepath.Join(root, "zsh", "dot-zshrc"))
	announced := false
	for _, p := range res.Packages {
		for _, n := range p.Notes {
			if strings.Contains(n, "created target directory") {
				announced = true
			}
		}
	}
	if !announced {
		t.Error("creating a missing target must be announced (§1.3)")
	}
}

// TestUnstowMissingTargetNoOps: unstow under a target that does not exist
// is trivially done — no directory invented, no failure.
func TestUnstowMissingTargetNoOps(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	sub := filepath.Join(e.target, "never-created")
	e.writeFile(filepath.Join(root, ".dstow", "config.toml"),
		"target = \""+sub+"\"\n")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbUnstow, Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if res.Failed() {
		t.Fatalf("failed: %+v", res.Packages)
	}
	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Error("unstow must not invent the target directory")
	}
}

// TestAdoptFlagComposition: stow --adopt moves a differing live file into
// the package behind confirmation (D15) and links its place; declining
// fails the package, and unstow --adopt is refused outright.
func TestAdoptFlagComposition(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "pkg content\n")
	e.writeFile(filepath.Join(e.target, ".zshrc"), "live content\n")

	e.prompt.answers = []bool{true}
	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}, Adopt: true})
	if err != nil {
		t.Fatalf("stow --adopt: %v", err)
	}
	if res.Failed() {
		t.Fatalf("failed: %+v", res.Packages)
	}
	if len(e.prompt.asked) != 1 {
		t.Fatalf("differing content asks once, got %v", e.prompt.asked)
	}
	data, err := os.ReadFile(filepath.Join(root, "zsh", "dot-zshrc"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "live content\n" {
		t.Errorf("live content always wins (§8.5): package now holds %q", data)
	}
	isLinkTo(t, filepath.Join(e.target, ".zshrc"), filepath.Join(root, "zsh", "dot-zshrc"))

	// unstow --adopt is conceptually void (D15 pre-accepts stow's refusal).
	if _, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbUnstow, Adopt: true}); err == nil {
		t.Error("unstow --adopt must refuse")
	}
}

// TestAdoptDeclinedFailsPackage: declining the differing-content confirm
// fails the package before anything mutates.
func TestAdoptDeclinedFailsPackage(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "pkg content\n")
	e.writeFile(filepath.Join(e.target, ".zshrc"), "live content\n")

	e.prompt.answers = []bool{false}
	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}, Adopt: true})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !res.Failed() {
		t.Error("a declined adoption fails the package")
	}
	data, _ := os.ReadFile(filepath.Join(e.target, ".zshrc"))
	if string(data) != "live content\n" {
		t.Error("declining must leave the live file untouched")
	}
	pkg, _ := os.ReadFile(filepath.Join(root, "zsh", "dot-zshrc"))
	if string(pkg) != "pkg content\n" {
		t.Error("declining must leave the package untouched")
	}
}

// TestFoldContradictionRefuses: a repo rc declaring --no-folding while the
// global fold_trees says true contradicts within one invocation — the run
// refuses loudly, pointing at the global setting (REQ §3.3). With rc
// values agreeing with the global value, the run proceeds.
func TestFoldContradictionRefuses(t *testing.T) {
	e := newEnv(t)
	globalDir, _ := setupXDG(t)
	rootA := e.addRepo("aaa")
	rootB := e.addRepo("bbb")
	e.writeFile(filepath.Join(rootA, "p1", "f1"), "x\n")
	e.writeFile(filepath.Join(rootB, "p2", "f2"), "x\n")
	e.writeFile(filepath.Join(rootA, ".stowrc"), "--no-folding\n")

	// Global fold on: repo A's rc-declared off now contradicts repo B's
	// inherited on.
	e.writeFile(filepath.Join(globalDir, "config.toml"), "fold_trees = true\n")
	e.loadGlobal()

	_, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow})
	var fc *ops.FoldConflictError
	if !errors.As(err, &fc) {
		t.Fatalf("expected FoldConflictError, got %T: %v", err, err)
	}
	if len(fc.True) != 1 || len(fc.False) != 1 {
		t.Fatalf("one repo per side: %+v", fc)
	}
	if !strings.Contains(fc.Error(), "fold_trees") {
		t.Errorf("the refusal points at the global setting: %q", fc.Error())
	}

	// Global fold off (the default): repo A's rc agrees; no contradiction.
	e.writeFile(filepath.Join(globalDir, "config.toml"), "")
	e.loadGlobal()
	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow})
	if err != nil {
		t.Fatalf("agreeing fold values must not refuse: %v", err)
	}
	if res.Failed() {
		t.Fatalf("failed: %+v", res.Packages)
	}
}

// TestHookLifecycleAcrossRepos: hooks fire nested/LIFO over the acting
// set — global once, each repo once, packages inside (§9.1.2/§9.1.3) — in
// canonical repo order.
func TestHookLifecycleAcrossRepos(t *testing.T) {
	e := newEnv(t)
	rootA := e.addRepo("aaa")
	rootB := e.addRepo("bbb")
	e.writeFile(filepath.Join(rootA, "p1", "f1"), "x\n")
	e.writeFile(filepath.Join(rootA, "p2", "f2"), "x\n")
	e.writeFile(filepath.Join(rootB, "p3", "f3"), "x\n")
	log := filepath.Join(e.base, "hook.log")

	e.appendHook(filepath.Join(e.app.GlobalDir, "hooks"), "pre-stow", log, "global-pre")
	e.appendHook(filepath.Join(e.app.GlobalDir, "hooks"), "post-stow", log, "global-post")
	e.appendHook(filepath.Join(rootA, ".dstow", "hooks"), "pre-stow", log, "repoA-pre")
	e.appendHook(filepath.Join(rootA, ".dstow", "hooks"), "post-stow", log, "repoA-post")
	e.appendHook(filepath.Join(rootB, ".dstow", "hooks"), "pre-stow", log, "repoB-pre")
	e.appendHook(filepath.Join(rootB, ".dstow", "hooks"), "post-stow", log, "repoB-post")
	e.appendHook(filepath.Join(rootA, "p1", ".dstow", "hooks"), "pre-stow", log, "p1-pre")
	e.appendHook(filepath.Join(rootA, "p1", ".dstow", "hooks"), "post-stow", log, "p1-post")

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if res.Failed() {
		t.Fatalf("failed: %+v", res.Packages)
	}
	want := []string{
		"global-pre", "repoA-pre", "p1-pre", "p1-post", "repoA-post",
		"repoB-pre", "repoB-post", "global-post",
	}
	got := readLines(t, log)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("hook order:\n got %v\nwant %v", got, want)
	}
}

// TestFailedRepoPreBlocksItsPackages: a failed repo-pre blocks everything
// under that repo (§9.1.4); the other repo proceeds; blocked packages are
// classified, not silently skipped.
func TestFailedRepoPreBlocksItsPackages(t *testing.T) {
	e := newEnv(t)
	rootA := e.addRepo("aaa")
	rootB := e.addRepo("bbb")
	e.writeFile(filepath.Join(rootA, "p1", "f1"), "x\n")
	e.writeFile(filepath.Join(rootA, "p2", "f2"), "x\n")
	e.writeFile(filepath.Join(rootB, "p3", "f3"), "x\n")
	e.writeExec(filepath.Join(rootA, ".dstow", "hooks", "pre-stow"), "exit 1")

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	st := statuses(res)
	if st[pkgFQN(rootA, "p1").String()] != ops.StatusBlocked {
		t.Errorf("p1 must be blocked: %v", st)
	}
	if st[pkgFQN(rootA, "p2").String()] != ops.StatusBlocked {
		t.Errorf("p2 must be blocked (everything under the repo): %v", st)
	}
	if st[pkgFQN(rootB, "p3").String()] != ops.StatusSucceeded {
		t.Errorf("the other repo proceeds (§3.2): %v", st)
	}
	if !res.Failed() {
		t.Error("a blocked package fails the run")
	}
	if _, err := os.Lstat(filepath.Join(e.target, "f1")); !os.IsNotExist(err) {
		t.Error("a blocked package must not deploy")
	}
	isLinkTo(t, filepath.Join(e.target, "f3"), filepath.Join(rootB, "p3", "f3"))
}

// TestFailedPackagePreBlocksThatPackageOnly: §9.1.4's narrowest rule.
func TestFailedPackagePreBlocksThatPackageOnly(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "p1", "f1"), "x\n")
	e.writeFile(filepath.Join(root, "p2", "f2"), "x\n")
	e.writeExec(filepath.Join(root, "p1", ".dstow", "hooks", "pre-stow"), "exit 1")

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	st := statuses(res)
	if st[pkgFQN(root, "p1").String()] != ops.StatusFailed {
		t.Errorf("p1 fails on its own pre hook: %v", st)
	}
	if st[pkgFQN(root, "p2").String()] != ops.StatusSucceeded {
		t.Errorf("p2 continues (§3.2): %v", st)
	}
	if _, err := os.Lstat(filepath.Join(e.target, "f1")); !os.IsNotExist(err) {
		t.Error("p1 must not deploy")
	}
}

// TestNoOpScopesStayQuiet: hooks fire only for scopes that actually change
// something (§9.1.5) — a second identical stow run fires nothing.
func TestNoOpScopesStayQuiet(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	log := filepath.Join(e.base, "hook.log")
	e.appendHook(filepath.Join(root, ".dstow", "hooks"), "pre-stow", log, "repo-pre")
	e.appendHook(filepath.Join(root, ".dstow", "hooks"), "post-stow", log, "repo-post")

	if _, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}}); err != nil {
		t.Fatal(err)
	}
	first := len(readLines(t, log))
	if first == 0 {
		t.Fatal("the first run must fire hooks")
	}
	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed() {
		t.Fatalf("an already-stowed stow succeeds as a no-op: %+v", res.Packages)
	}
	if got := len(readLines(t, log)); got != first {
		t.Errorf("no-op scopes stay quiet (§9.1.5): hooks grew %d -> %d", first, got)
	}
}

// TestFailedPostReported: a failed package-post marks the package failed
// but completed work stays done (§9.1.4).
func TestFailedPostReported(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	e.writeExec(filepath.Join(root, "zsh", ".dstow", "hooks", "post-stow"), "exit 3")

	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !res.Failed() {
		t.Error("a failed post marks its scope failed")
	}
	isLinkTo(t, filepath.Join(e.target, ".zshrc"), filepath.Join(root, "zsh", "dot-zshrc"))
	led, err := ledger.Load(e.app.LedgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(led.Targets[e.target]) != 1 {
		t.Error("completed work stays done: the entry is recorded")
	}
}
