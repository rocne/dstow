package ops_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rocne/dstow/internal/ledger"
)

// seedLedger writes a target group directly, so rebuild's replace/untouched
// behavior can be observed against a known prior state.
func seedLedger(t *testing.T, path, root string, entries []ledger.Entry) {
	t.Helper()
	_, err := ledger.Update(path, ledger.Scope{}, func(l *ledger.Ledger) error {
		if l.Targets == nil {
			l.Targets = map[string][]ledger.Entry{}
		}
		l.Targets[root] = entries
		return nil
	})
	if err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
}

// TestRebuildLedgersOwnedLinks: rebuild discovers the symlinks pointing into
// known repos — both a hand-made link and one dstow itself made — and records
// them; a link pointing outside every repo is left unledgered (§6.4).
func TestRebuildLedgersOwnedLinks(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	e.writeFile(filepath.Join(e.base, "elsewhere", "loose"), "x\n")

	// A hand-made link into the repo, and a link into no repo at all.
	makeLink(t, filepath.Join(e.target, ".zshrc"), filepath.Join(root, "zsh", "dot-zshrc"))
	makeLink(t, filepath.Join(e.target, ".loose"), filepath.Join(e.base, "elsewhere", "loose"))

	res, err := e.app.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	entries := loadLedger(t, e.app.LedgerPath).Targets[e.target]
	if len(entries) != 1 {
		t.Fatalf("only the link into a known repo is ledgered: %+v", entries)
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
	if res.Counts[e.target] != 1 {
		t.Errorf("per-root count = %v, want 1", res.Counts[e.target])
	}
}

// TestRebuildReplacesScannedGroups: rebuild wholesale-replaces a scanned
// root's group with exactly what disk now holds — stale entries vanish, new
// ones appear (§6.4).
func TestRebuildReplacesScannedGroups(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	e.writeFile(filepath.Join(root, "bash", "dot-bashrc"), "x\n")
	stowPkg(t, e, "zsh") // ledger now records .zshrc

	// Disk changes underneath: .zshrc gone, a new .bashrc link appears.
	if err := os.Remove(filepath.Join(e.target, ".zshrc")); err != nil {
		t.Fatal(err)
	}
	makeLink(t, filepath.Join(e.target, ".bashrc"), filepath.Join(root, "bash", "dot-bashrc"))

	if _, err := e.app.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	entries := loadLedger(t, e.app.LedgerPath).Targets[e.target]
	if len(entries) != 1 || entries[0].Link != ".bashrc" {
		t.Fatalf("the scanned group is replaced with disk's truth: %+v", entries)
	}
}

// TestRebuildLeavesUnscannedGroups: a ledger group under a root no current
// config targets is not scanned — rebuild leaves it untouched (§6.4).
func TestRebuildLeavesUnscannedGroups(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	makeLink(t, filepath.Join(e.target, ".zshrc"), filepath.Join(root, "zsh", "dot-zshrc"))

	other := filepath.Join(e.base, "otherhome")
	seedLedger(t, e.app.LedgerPath, other, []ledger.Entry{{
		Link: ".vimrc", Package: "local:x::vim", Source: "dot-vimrc",
		Destination: "../repo/vim/dot-vimrc", RecordedAt: fixedNow,
	}})

	if _, err := e.app.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	led := loadLedger(t, e.app.LedgerPath)
	if len(led.Targets[other]) != 1 {
		t.Errorf("an unscanned group must survive untouched: %+v", led.Targets[other])
	}
	if len(led.Targets[e.target]) != 1 {
		t.Errorf("the scanned target is still rebuilt: %+v", led.Targets[e.target])
	}
}

// TestRebuildMissingRootEmptiesGroup: a configured target that does not exist
// is scanned and empty — absence is a sighting, so its group is emptied (§6.4).
func TestRebuildMissingRootEmptiesGroup(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	missing := filepath.Join(e.base, "nonexistent")
	e.writeFile(filepath.Join(root, ".dstow", "config.toml"),
		"target = \""+missing+"\"\n")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")

	seedLedger(t, e.app.LedgerPath, missing, []ledger.Entry{{
		Link: ".zshrc", Package: pkgFQN(root, "zsh").String(), Source: "dot-zshrc",
		Destination: "../dots/zsh/dot-zshrc", RecordedAt: fixedNow,
	}})

	if _, err := e.app.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if g := loadLedger(t, e.app.LedgerPath).Targets[missing]; len(g) != 0 {
		t.Errorf("a missing root's group is emptied: %+v", g)
	}
}

// TestRebuildDoesNotDescendSymlinkedDirs: rebuild records a folded-tree
// directory link as one entry and never follows it — the link inside the
// symlinked directory is not ledgered (§6.4, lstat semantics).
func TestRebuildDoesNotDescendSymlinkedDirs(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "app", "conf", "x"), "x\n")
	e.writeFile(filepath.Join(root, "app", "other"), "x\n")
	// A link that lives *inside* the package's conf directory. If rebuild
	// followed the folded-dir symlink below, it would surface this too.
	makeLink(t, filepath.Join(root, "app", "conf", "inner"), filepath.Join(root, "app", "other"))

	// The folded-tree directory link: target/conf -> repo/app/conf.
	makeLink(t, filepath.Join(e.target, "conf"), filepath.Join(root, "app", "conf"))

	if _, err := e.app.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	entries := loadLedger(t, e.app.LedgerPath).Targets[e.target]
	if len(entries) != 1 || entries[0].Link != "conf" {
		t.Fatalf("the folded-dir link is one entry and its interior is never walked: %+v", entries)
	}
}

// TestRebuildUnreadableSubtreeLeavesRootUntouched: a root with an unreadable
// subtree is not scanned — its group survives untouched and a Warning names
// the failure. Wholesale-replacing a partially observed root would claim
// absence without a sighting (§1.5).
func TestRebuildUnreadableSubtreeLeavesRootUntouched(t *testing.T) {
	skipIfRoot(t)
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	stowPkg(t, e, "zsh") // ledger now records .zshrc

	// The link vanishes from disk — a full scan would empty the group — and a
	// subtree goes unreadable, so the scan cannot claim to have seen it all.
	if err := os.Remove(filepath.Join(e.target, ".zshrc")); err != nil {
		t.Fatal(err)
	}
	dark := filepath.Join(e.target, "dark")
	if err := os.MkdirAll(dark, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dark, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dark, 0o755) })

	res, err := e.app.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if len(loadLedger(t, e.app.LedgerPath).Targets[e.target]) != 1 {
		t.Error("an unscannable root's group must survive untouched")
	}
	if _, counted := res.Counts[e.target]; counted {
		t.Error("an unscannable root is not a scanned root")
	}
	if len(res.Warnings) == 0 {
		t.Error("an unscannable root raises a Warning")
	}
}

// TestRebuildEmptyLedgerFromScratch: rebuild with no prior ledger builds one
// from disk — a missing ledger is simply an empty one (§6.5).
func TestRebuildEmptyLedgerFromScratch(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "x\n")
	makeLink(t, filepath.Join(e.target, ".zshrc"), filepath.Join(root, "zsh", "dot-zshrc"))

	if _, err := os.Stat(e.app.LedgerPath); !os.IsNotExist(err) {
		t.Fatal("precondition: no ledger yet")
	}
	if _, err := e.app.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if len(loadLedger(t, e.app.LedgerPath).Targets[e.target]) != 1 {
		t.Error("rebuild from scratch records the discovered link")
	}
}
