package repo_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/repo"
)

func mustSource(t *testing.T, s string) repo.Source {
	t.Helper()
	src, err := repo.ParseSource(s)
	if err != nil {
		t.Fatalf("ParseSource(%q): %v", s, err)
	}
	return src
}

// BuildSet sets Managed/Session flags, roots, and FQNs per source kind, and
// deduplicates on exact FQN identity.
func TestBuildSetFlagsRootsAndFQNs(t *testing.T) {
	setDataHome(t)
	gh := mustSource(t, "github:rocne/dotfiles")
	local := mustSource(t, "local:/srv/dots")

	set := repo.BuildSet([]repo.Source{gh, local}, []string{"/home/rocne/session"})
	if len(set) != 3 {
		t.Fatalf("BuildSet len = %d, want 3", len(set))
	}

	// github → managed, rooted at the clone dir.
	managed := set[0]
	if !managed.Managed || managed.Session {
		t.Errorf("github repo flags: managed=%v session=%v; want managed", managed.Managed, managed.Session)
	}
	if managed.Root != gh.CloneDir() {
		t.Errorf("github root = %q, want %q", managed.Root, gh.CloneDir())
	}
	if managed.FQN.String() != "github:rocne/dotfiles" {
		t.Errorf("github FQN = %q", managed.FQN.String())
	}

	// local → neither managed nor session, rooted at its path.
	loc := set[1]
	if loc.Managed || loc.Session {
		t.Errorf("local repo flags: managed=%v session=%v; want neither", loc.Managed, loc.Session)
	}
	if loc.Root != "/srv/dots" {
		t.Errorf("local root = %q, want /srv/dots", loc.Root)
	}
	if loc.FQN.String() != "local:/srv/dots" {
		t.Errorf("local FQN = %q, want local:/srv/dots", loc.FQN.String())
	}

	// session → session flag, rooted at the DSTOW_PATH dir.
	sess := set[2]
	if sess.Managed || !sess.Session {
		t.Errorf("session repo flags: managed=%v session=%v; want session", sess.Managed, sess.Session)
	}
	if sess.Root != "/home/rocne/session" {
		t.Errorf("session root = %q", sess.Root)
	}
	if sess.FQN.String() != "local:/home/rocne/session" {
		t.Errorf("session FQN = %q, want local:/home/rocne/session", sess.FQN.String())
	}
}

// A repo both registered as local and present in DSTOW_PATH is one repo: exact
// FQN identity, first wins.
func TestBuildSetExactFQNDedup(t *testing.T) {
	local := mustSource(t, "local:/srv/dots")
	set := repo.BuildSet([]repo.Source{local}, []string{"/srv/dots"})
	if len(set) != 1 {
		t.Fatalf("BuildSet len = %d, want 1 (exact-FQN dedup)", len(set))
	}
	// The registered entry wins: it is not marked as a session repo.
	if set[0].Session {
		t.Errorf("dedup kept the session entry; want the first (registered) entry")
	}
}

// Root mode: visible directories directly under Root are packages; hidden dirs
// are skipped silently and non-directories are ignored. Output is sorted.
func TestPackagesRootMode(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"zsh", "git", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "readme.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := repo.Repo{Root: root}
	pkgs, warns, err := r.Packages("")
	if err != nil {
		t.Fatalf("Packages: %v", err)
	}
	if want := []string{"git", "zsh"}; !reflect.DeepEqual(pkgs, want) {
		t.Errorf("Packages = %v, want %v", pkgs, want)
	}
	if len(warns) != 0 {
		t.Errorf("root mode should skip hidden dirs silently; got warnings %+v", warns)
	}
}

// Scoped mode: entries live under Root/packagesDir; every visible directory is a
// package and hidden dirs are skipped LOUDLY (one Warning each, M2).
func TestPackagesScopedModeHiddenLoud(t *testing.T) {
	root := t.TempDir()
	pkgDir := filepath.Join(root, "packages")
	for _, d := range []string{"zsh", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(pkgDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := repo.Repo{Root: root}
	pkgs, warns, err := r.Packages("packages")
	if err != nil {
		t.Fatalf("Packages: %v", err)
	}
	if want := []string{"zsh"}; !reflect.DeepEqual(pkgs, want) {
		t.Errorf("Packages = %v, want %v", pkgs, want)
	}
	if len(warns) != 1 {
		t.Fatalf("scoped mode should warn once for the hidden dir; got %d warnings %+v", len(warns), warns)
	}
	if !strings.Contains(warns[0].Detail, ".hidden") {
		t.Errorf("warning does not name the hidden dir: %q", warns[0].Detail)
	}
}

// A missing packages directory is an error naming the path.
func TestPackagesMissingDirErrors(t *testing.T) {
	root := t.TempDir()
	r := repo.Repo{Root: root}
	_, _, err := r.Packages("nonexistent")
	if err == nil {
		t.Fatal("Packages on a missing packages dir returned nil error")
	}
	if !strings.Contains(err.Error(), filepath.Join(root, "nonexistent")) {
		t.Errorf("error does not name the missing path: %v", err)
	}
}
