package ops_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/ops"
)

// TestAdoptSingleFile: adopt moves the live file into the package under
// its dot-translated source spelling, leaves a link, and records the entry
// (§2.4 adopt, REQ §8.5).
func TestAdoptSingleFile(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "keep"), "x\n") // the package exists
	live := filepath.Join(e.target, ".zshrc")
	e.writeFile(live, "live content\n")

	res, err := e.app.Adopt(ops.AdoptRequest{File: live, Package: "zsh"})
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if res.Failed() {
		t.Fatalf("failed: %+v", res.Errs)
	}
	if len(res.Moves) != 1 || res.Moves[0].Source != "dot-zshrc" {
		t.Fatalf("the source spelling is the package's convention (dot-translation on): %+v", res.Moves)
	}
	data, err := os.ReadFile(filepath.Join(root, "zsh", "dot-zshrc"))
	if err != nil {
		t.Fatalf("the file must move into the package: %v", err)
	}
	if string(data) != "live content\n" {
		t.Errorf("live content wins: %q", data)
	}
	isLinkTo(t, live, filepath.Join(root, "zsh", "dot-zshrc"))

	led, err := ledger.Load(e.app.LedgerPath)
	if err != nil {
		t.Fatal(err)
	}
	entries := led.Targets[e.target]
	if len(entries) != 1 || entries[0].Link != ".zshrc" || entries[0].Source != "dot-zshrc" {
		t.Errorf("the adoption is ledgered: %+v", entries)
	}
}

// TestAdoptDifferingContentConfirms: overwriting differing package content
// asks first; --force skips the question (§8.5).
func TestAdoptDifferingContentConfirms(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "pkg content\n")
	live := filepath.Join(e.target, ".zshrc")
	e.writeFile(live, "live content\n")

	e.prompt.answers = []bool{false}
	res, err := e.app.Adopt(ops.AdoptRequest{File: live, Package: "zsh"})
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if len(res.Moves) != 0 || len(res.Skipped) != 1 {
		t.Fatalf("declined: the move is skipped, not forced: %+v / %+v", res.Moves, res.Skipped)
	}
	pkg, _ := os.ReadFile(filepath.Join(root, "zsh", "dot-zshrc"))
	if string(pkg) != "pkg content\n" {
		t.Error("declining leaves the package untouched")
	}

	res, err = e.app.Adopt(ops.AdoptRequest{File: live, Package: "zsh", Force: true})
	if err != nil {
		t.Fatalf("Adopt --force: %v", err)
	}
	if len(res.Moves) != 1 {
		t.Fatalf("--force adopts without asking: %+v", res)
	}
	pkg, _ = os.ReadFile(filepath.Join(root, "zsh", "dot-zshrc"))
	if string(pkg) != "live content\n" {
		t.Error("live content wins under --force")
	}
}

// TestAdoptDryRun: the plan is shown, nothing moves, nothing is written.
func TestAdoptDryRun(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "keep"), "x\n")
	live := filepath.Join(e.target, ".zshrc")
	e.writeFile(live, "live\n")

	res, err := e.app.Adopt(ops.AdoptRequest{File: live, Package: "zsh", DryRun: true})
	if err != nil {
		t.Fatalf("Adopt -n: %v", err)
	}
	if len(res.Moves) != 1 {
		t.Fatalf("dry-run shows the plan: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(root, "zsh", "dot-zshrc")); !os.IsNotExist(err) {
		t.Error("dry-run must not move the file")
	}
	if info, err := os.Lstat(live); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Error("dry-run must leave the live file a real file")
	}
	if _, err := os.Stat(e.app.LedgerPath); !os.IsNotExist(err) {
		t.Error("dry-run must not write the ledger")
	}
}

// TestAdoptOutsideTargetRefuses: a path the package's effective target
// does not cover refuses with a remedy (§1.4).
func TestAdoptOutsideTargetRefuses(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "keep"), "x\n")
	outside := filepath.Join(e.base, "elsewhere", "file")
	e.writeFile(outside, "x\n")

	_, err := e.app.Adopt(ops.AdoptRequest{File: outside, Package: "zsh"})
	if err == nil {
		t.Fatal("a path outside the target must refuse")
	}
	if !strings.Contains(err.Error(), "target") {
		t.Errorf("the refusal explains the target rule: %v", err)
	}
}

// TestAdoptOccupied: --occupied adopts every expected path a real file
// occupies, and only those (§2.4).
func TestAdoptOccupied(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "live zshrc\n")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshenv"), "pkg zshenv\n")
	e.writeFile(filepath.Join(root, "zsh", "free"), "never occupied\n")
	// .zshrc occupied with identical content; .zshenv occupied differing.
	e.writeFile(filepath.Join(e.target, ".zshrc"), "live zshrc\n")
	e.writeFile(filepath.Join(e.target, ".zshenv"), "live zshenv\n")

	e.prompt.answers = []bool{true} // confirm the differing .zshenv
	res, err := e.app.Adopt(ops.AdoptRequest{Package: "zsh", Occupied: true})
	if err != nil {
		t.Fatalf("Adopt --occupied: %v", err)
	}
	if res.Failed() {
		t.Fatalf("failed: %+v", res.Errs)
	}
	if len(res.Moves) != 2 {
		t.Fatalf("both occupied paths adopt: %+v", res.Moves)
	}
	isLinkTo(t, filepath.Join(e.target, ".zshrc"), filepath.Join(root, "zsh", "dot-zshrc"))
	isLinkTo(t, filepath.Join(e.target, ".zshenv"), filepath.Join(root, "zsh", "dot-zshenv"))
	if data, _ := os.ReadFile(filepath.Join(root, "zsh", "dot-zshenv")); string(data) != "live zshenv\n" {
		t.Errorf("live content wins: %q", data)
	}
	if _, err := os.Lstat(filepath.Join(e.target, "free")); !os.IsNotExist(err) {
		t.Error("--occupied never touches unoccupied paths")
	}
}

// TestAdoptFiresAdoptHooks: the adopt leaf fires the adopt pair (M6:
// restow fires only restow's; adopt fires only adopt's).
func TestAdoptFiresAdoptHooks(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "keep"), "x\n")
	live := filepath.Join(e.target, ".zshrc")
	e.writeFile(live, "live\n")
	log := filepath.Join(e.base, "hook.log")
	e.appendHook(filepath.Join(root, "zsh", ".dstow", "hooks"), "pre-adopt", log, "pre-adopt")
	e.appendHook(filepath.Join(root, "zsh", ".dstow", "hooks"), "post-adopt", log, "post-adopt")
	e.appendHook(filepath.Join(root, "zsh", ".dstow", "hooks"), "pre-stow", log, "pre-stow")

	if _, err := e.app.Adopt(ops.AdoptRequest{File: live, Package: "zsh"}); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	got := readLines(t, log)
	want := []string{"pre-adopt", "post-adopt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("adopt fires exactly the adopt pair: got %v", got)
	}
}

// TestAdoptCandidatesRankedByNeighbors: candidates are packages whose
// target covers the path, ranked by ledgered neighbors, canonical-FQN
// tie-break (REQ §8.5 + the #44 ordering ruling).
func TestAdoptCandidatesRankedByNeighbors(t *testing.T) {
	e := newEnv(t)
	rootA := e.addRepo("aaa")
	rootB := e.addRepo("bbb")
	e.writeFile(filepath.Join(rootA, "shell", "dot-profile"), "x\n")
	e.writeFile(filepath.Join(rootB, "zsh", "dot-zshenv"), "x\n")

	// Stow zsh so it owns a neighbor of ~/.zshrc in the ledger.
	if _, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}}); err != nil {
		t.Fatal(err)
	}

	live := filepath.Join(e.target, ".zshrc")
	e.writeFile(live, "live\n")
	cands, _, err := e.app.AdoptCandidates(live)
	if err != nil {
		t.Fatalf("AdoptCandidates: %v", err)
	}
	if len(cands) != 2 {
		t.Fatalf("both packages' targets cover the path: %+v", cands)
	}
	if cands[0].FQN.Package != "zsh" {
		t.Errorf("the package owning neighbors ranks first: %+v", cands)
	}
	if cands[0].Neighbors != 1 || cands[1].Neighbors != 0 {
		t.Errorf("neighbor counts: %+v", cands)
	}
	for _, c := range cands {
		if c.Source != "dot-zshrc" {
			t.Errorf("candidate sources carry the translated spelling: %+v", c)
		}
	}
}

// TestAdoptCandidatesRespectIgnores: a package whose ignore chain excludes
// the mapped source is not a candidate (§8.5: "not ignored").
func TestAdoptCandidatesRespectIgnores(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "keep"), "x\n")
	e.writeFile(filepath.Join(root, "zsh", ".dstow", "config.toml"),
		"ignore = [\"dot-zshrc\"]\n")
	e.writeFile(filepath.Join(root, "shell", "keep"), "x\n")

	live := filepath.Join(e.target, ".zshrc")
	e.writeFile(live, "live\n")
	cands, _, err := e.app.AdoptCandidates(live)
	if err != nil {
		t.Fatalf("AdoptCandidates: %v", err)
	}
	if len(cands) != 1 || cands[0].FQN.Package != "shell" {
		t.Errorf("the ignoring package is excluded: %+v", cands)
	}
}
