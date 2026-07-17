package ops_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/git"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/repo"
)

// stow is a helper: deploy a package and fail the test on any error.
func (e *env) stow(pkg string) {
	e.t.Helper()
	res, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{pkg}})
	if err != nil {
		e.t.Fatalf("stow %s: %v", pkg, err)
	}
	if res.Failed() {
		e.t.Fatalf("stow %s failed: %+v", pkg, res.Packages)
	}
}

// pkgState finds a package's state in a status result by its bare package name.
func pkgState(t *testing.T, res *ops.StatusResult, pkg string) ops.PackageStatusResult {
	t.Helper()
	for _, p := range res.Packages {
		if p.FQN.Package == pkg {
			return p
		}
	}
	t.Fatalf("package %q not in status result: %+v", pkg, res.Packages)
	return ops.PackageStatusResult{}
}

// TestStatusStowed: a fully deployed package reads stowed, not drifted (§7.2).
func TestStatusStowed(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")
	e.stow("zsh")

	res, err := e.app.Status(ops.StatusRequest{Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	p := pkgState(t, res, "zsh")
	if p.State != ops.StateStowed {
		t.Errorf("state = %v, want stowed", p.State)
	}
	if p.Drifted {
		t.Error("a freshly stowed package matching config is not drifted")
	}
}

// TestStatusNotStowed: an undeployed package with empty target slots reads not
// stowed (§7.2).
func TestStatusNotStowed(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")

	res, err := e.app.Status(ops.StatusRequest{Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if p := pkgState(t, res, "zsh"); p.State != ops.StateNotStowed {
		t.Errorf("state = %v, want not stowed", p.State)
	}
}

// TestStatusPartiallyStowed: some expected links present, some missing, with no
// ledger contradiction, reads partially stowed (§7.2). Adding a file to a
// stowed package leaves its new slot unfilled.
func TestStatusPartiallyStowed(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")
	e.stow("zsh")
	// A newly-added file is expected by current config but not yet deployed.
	e.writeFile(filepath.Join(root, "zsh", "dot-aliases"), "a\n")

	res, err := e.app.Status(ops.StatusRequest{Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if p := pkgState(t, res, "zsh"); p.State != ops.StatePartiallyStowed {
		t.Errorf("state = %v, want partially stowed", p.State)
	}
}

// TestStatusOccupied: a real file where a link would go, with none of the
// package deployed, reads occupied — neutral (§7.2).
func TestStatusOccupied(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")
	e.writeFile(filepath.Join(e.target, ".zshrc"), "a real live file\n")

	res, err := e.app.Status(ops.StatusRequest{Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if p := pkgState(t, res, "zsh"); p.State != ops.StateOccupied {
		t.Errorf("state = %v, want occupied", p.State)
	}
}

// TestStatusOccupiedOutranksPartial: a package with one link stowed and another
// expected slot taken by a real file reads occupied, not partially stowed.
// CONTEXT.md defines "partially stowed" as the rest being *merely missing*, so
// an occupied slot escalates the whole package to occupied.
func TestStatusOccupiedOutranksPartial(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")
	e.stow("zsh")
	// A second expected file whose target slot is taken by a real live file.
	e.writeFile(filepath.Join(root, "zsh", "dot-aliases"), "a\n")
	e.writeFile(filepath.Join(e.target, ".aliases"), "a real live file\n")

	res, err := e.app.Status(ops.StatusRequest{Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if p := pkgState(t, res, "zsh"); p.State != ops.StateOccupied {
		t.Errorf("state = %v, want occupied (occupied outranks partially stowed)", p.State)
	}
}

// TestStatusDamagedNeedsLedgerEvidence: a link dstow recorded, now gone from
// disk, is damaged — and damaged is claimed ONLY with that ledger evidence
// (§7.2). The identical disk with no ledger record is merely not stowed.
func TestStatusDamagedNeedsLedgerEvidence(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")
	e.stow("zsh")
	// Tamper: remove the deployed link. The ledger still records it → disk
	// contradicts the ledger → damaged.
	if err := os.Remove(filepath.Join(e.target, ".zshrc")); err != nil {
		t.Fatal(err)
	}

	res, err := e.app.Status(ops.StatusRequest{Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if p := pkgState(t, res, "zsh"); p.State != ops.StateDamaged {
		t.Errorf("state = %v, want damaged (ledger evidence of a contradicted link)", p.State)
	}

	// Same disk shape without ledger evidence: not damaged, just not stowed.
	e2 := newEnv(t)
	root2 := e2.addRepo("dots")
	e2.writeFile(filepath.Join(root2, "zsh", "dot-zshrc"), "z\n")
	// No stow, so no ledger entry; the slot is simply empty.
	res2, err := e2.app.Status(ops.StatusRequest{Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if p := pkgState(t, res2, "zsh"); p.State == ops.StateDamaged {
		t.Error("damaged must never be claimed without ledger evidence")
	}
}

// TestStatusDrifted: a package still fully deployed for what config now wants,
// but carrying a leftover deployed link config no longer produces, is stowed
// with the drifted marker (§7.2).
func TestStatusDrifted(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")
	e.writeFile(filepath.Join(root, "zsh", "dot-aliases"), "a\n")
	e.stow("zsh")
	// Remove a source file: current config no longer produces its link, but the
	// deployed link (and its ledger record) linger — the deployed shape now
	// differs from what config would produce.
	if err := os.Remove(filepath.Join(root, "zsh", "dot-aliases")); err != nil {
		t.Fatal(err)
	}

	res, err := e.app.Status(ops.StatusRequest{Names: []string{"zsh"}})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	p := pkgState(t, res, "zsh")
	if p.State != ops.StateStowed {
		t.Errorf("state = %v, want stowed (every currently-expected link is present)", p.State)
	}
	if !p.Drifted {
		t.Error("a leftover deployed link config no longer produces should mark drifted")
	}
}

// TestStatusRemoteBehindAhead: a remote (managed) repo in scope reports
// behind/ahead as of the last update, from the git port, with no network
// (§7.2.1).
func TestStatusRemoteBehindAhead(t *testing.T) {
	e := newEnv(t)
	// A managed (remote) repo, rooted at a real temp dir we populate.
	clone := filepath.Join(e.base, "clone")
	e.writeFile(filepath.Join(clone, ".dstow", "config.toml"), "target = \""+e.target+"\"\n")
	e.writeFile(filepath.Join(clone, "zsh", "dot-zshrc"), "z\n")
	e.app.Repos = append(e.app.Repos, repo.Repo{
		FQN:     name.FQN{Scheme: "github", Coordinate: []string{"rocne", "dotfiles"}},
		Root:    clone,
		Managed: true,
	})
	fake := &git.Fake{Ahead: 2, Behind: 1}
	e.app.Git = fake

	res, err := e.app.Status(ops.StatusRequest{})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	var found *ops.RepoSync
	for i := range res.Repos {
		if res.Repos[i].FQN.Scheme == "github" {
			found = &res.Repos[i]
		}
	}
	if found == nil {
		t.Fatalf("remote repo sync not reported: %+v", res.Repos)
	}
	if !found.Known || found.Ahead != 2 || found.Behind != 1 {
		t.Errorf("sync = %+v, want known ahead 2 behind 1", found)
	}
	if len(fake.AheadBehindCalls) == 0 || fake.AheadBehindCalls[0] != clone {
		t.Errorf("AheadBehind should be asked about the clone dir: %v", fake.AheadBehindCalls)
	}
}

// TestStatusPerPathAdoptCandidates: the per-path view reports what occupies a
// path and the ranked adoption candidates (§7.2.4), reusing AdoptCandidates.
func TestStatusPerPathAdoptCandidates(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")
	// A real live file sitting at a path the zsh package could adopt.
	live := filepath.Join(e.target, ".zshrc")
	e.writeFile(live, "live content\n")

	res, err := e.app.Status(ops.StatusRequest{Path: live})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if res.Path == nil {
		t.Fatalf("per-path view missing: %+v", res)
	}
	if !res.Path.Exists || res.Path.Kind != "regular file" {
		t.Errorf("path view = %+v, want an existing regular file", res.Path)
	}
	var haveZsh bool
	for _, c := range res.Path.Candidates {
		if c.FQN.Package == "zsh" {
			haveZsh = true
		}
	}
	if !haveZsh {
		t.Errorf("zsh should be a ranked adoption candidate for %s: %+v", live, res.Path.Candidates)
	}
}

// TestStatusPerPathLedgerOwner: for a deployed link, the per-path view names
// the ledger owner (§7.2.4).
func TestStatusPerPathLedgerOwner(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")
	e.stow("zsh")

	res, err := e.app.Status(ops.StatusRequest{Path: filepath.Join(e.target, ".zshrc")})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if res.Path == nil || !res.Path.OwnerKnown {
		t.Fatalf("ledger owner should be known for a deployed link: %+v", res.Path)
	}
	if res.Path.Owner.Package != "zsh" {
		t.Errorf("owner = %v, want the zsh package", res.Path.Owner)
	}
}
