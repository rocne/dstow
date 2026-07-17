package ledger_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/ledger"
)

// makeSymlink creates a symlink at root/rel pointing at dest and returns rel.
func makeSymlink(t *testing.T, root, rel, dest string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dest, full); err != nil {
		t.Fatal(err)
	}
}

// §6.3: a writer prunes contradicted entries in its scope, reports each with
// evidence, and leaves healthy and out-of-scope entries alone.
func TestScopedPruning(t *testing.T) {
	target := t.TempDir()
	// (a) healthy: real symlink whose text matches the entry.
	makeSymlink(t, target, "healthy", "dest-a")
	// (b) and (c) are contradicted (recorded path is gone) — no link created.

	healthy := ledger.Entry{Link: "healthy", Package: "pkg-a", Destination: "dest-a"}
	inScope := ledger.Entry{Link: "gone-b", Package: "pkg-b", Destination: "dest-b"}
	outScope := ledger.Entry{Link: "gone-c", Package: "pkg-c", Destination: "dest-c"}

	seed := func(t *testing.T) string {
		path := ledgerPath(t)
		if _, err := ledger.Update(path, ledger.Scope{}, func(doc *ledger.Ledger) error {
			doc.Targets[target] = []ledger.Entry{healthy, inScope, outScope}
			return nil
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
		return path
	}

	// Prune by package: only pkg-b is in scope.
	path := seed(t)
	pruned, err := ledger.Update(path, ledger.Scope{Packages: []string{"pkg-b"}}, func(*ledger.Ledger) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(pruned) != 1 || pruned[0].Entry.Link != "gone-b" {
		t.Fatalf("pruned = %+v, want exactly gone-b", pruned)
	}
	if pruned[0].TargetRoot != target {
		t.Errorf("pruned TargetRoot = %q, want %q", pruned[0].TargetRoot, target)
	}
	if !strings.Contains(pruned[0].Evidence, "nothing exists") {
		t.Errorf("evidence lacks disk description: %q", pruned[0].Evidence)
	}
	l, err := ledger.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	links := linkSet(l.Targets[target])
	if !links["healthy"] || !links["gone-c"] || links["gone-b"] {
		t.Errorf("after package-scope prune, links = %v; want healthy+gone-c, not gone-b", links)
	}

	// Scope.All prunes the out-of-scope contradicted entry too.
	path = seed(t)
	pruned, err = ledger.Update(path, ledger.Scope{All: true}, func(*ledger.Ledger) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 2 {
		t.Errorf("Scope.All pruned %d entries, want 2 (both contradicted)", len(pruned))
	}
	l, _ = ledger.Load(path)
	if links := linkSet(l.Targets[target]); !links["healthy"] || len(links) != 1 {
		t.Errorf("after Scope.All, links = %v; want only healthy", links)
	}

	// Scope.Paths covers an entry by its joined absolute path.
	path = seed(t)
	pruned, err = ledger.Update(path, ledger.Scope{Paths: []string{filepath.Join(target, "gone-c")}}, func(*ledger.Ledger) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) != 1 || pruned[0].Entry.Link != "gone-c" {
		t.Errorf("path-scope pruned = %+v, want exactly gone-c", pruned)
	}
}

func linkSet(entries []ledger.Entry) map[string]bool {
	m := map[string]bool{}
	for _, e := range entries {
		m[e.Link] = true
	}
	return m
}

// CONTEXT.md "Contradicted entry": healthy → false; the recorded path gone,
// holding a non-link, or holding a different link text → true, with evidence
// naming what disk shows.
func TestContradicted(t *testing.T) {
	target := t.TempDir()
	makeSymlink(t, target, "good", "the-dest")
	makeSymlink(t, target, "drifted", "other-dest")
	if err := os.WriteFile(filepath.Join(target, "regular"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Healthy: symlink text matches.
	good := ledger.Entry{Link: "good", Destination: "the-dest"}
	if bad, ev := good.Contradicted(target); bad {
		t.Errorf("healthy entry reported contradicted: %q", ev)
	}

	// Missing path.
	missing := ledger.Entry{Link: "absent", Destination: "the-dest"}
	if bad, ev := missing.Contradicted(target); !bad {
		t.Error("missing path not reported contradicted")
	} else if !strings.Contains(ev, "nothing exists") {
		t.Errorf("missing-path evidence = %q", ev)
	}

	// Regular file where a link is recorded.
	regular := ledger.Entry{Link: "regular", Destination: "the-dest"}
	if bad, ev := regular.Contradicted(target); !bad {
		t.Error("regular file not reported contradicted")
	} else if !strings.Contains(ev, "regular file") {
		t.Errorf("regular-file evidence = %q", ev)
	}

	// Different link text.
	drifted := ledger.Entry{Link: "drifted", Destination: "the-dest"}
	if bad, ev := drifted.Contradicted(target); !bad {
		t.Error("drifted link not reported contradicted")
	} else if !strings.Contains(ev, "other-dest") {
		t.Errorf("different-text evidence does not name disk's link: %q", ev)
	}
}

// §6.2/§6.3: pruning the last entry of a target root drops the root key from
// the written document — an index holds only what exists.
func TestEmptyGroupCleanup(t *testing.T) {
	target := t.TempDir()
	path := ledgerPath(t)
	// One contradicted entry (path gone) is the only entry under target.
	if _, err := ledger.Update(path, ledger.Scope{}, func(doc *ledger.Ledger) error {
		doc.Targets[target] = []ledger.Entry{{Link: "gone", Package: "p", Destination: "d"}}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := ledger.Update(path, ledger.Scope{All: true}, func(*ledger.Ledger) error { return nil }); err != nil {
		t.Fatalf("prune Update: %v", err)
	}
	l, err := ledger.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := l.Targets[target]; ok {
		t.Errorf("emptied target root still present: %v", l.Targets)
	}
}
