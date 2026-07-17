package repo_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"

	"github.com/rocne/dstow/internal/repo"
)

// setDataHome points XDG_DATA_HOME at a fresh temp dir and returns the expected
// managed repos root under it.
func setDataHome(t *testing.T) string {
	t.Helper()
	dataRoot := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataRoot)
	xdg.Reload()
	t.Cleanup(xdg.Reload)
	return filepath.Join(dataRoot, "dstow", "repos")
}

// CloneDir is the A19 shape: <managed>/repos/<scheme>/<owner>/<name>.
func TestCloneDirA19Shape(t *testing.T) {
	root := setDataHome(t)
	s, err := repo.ParseSource("github:rocne/dotfiles")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "github", "rocne", "dotfiles")
	if got := s.CloneDir(); got != want {
		t.Errorf("CloneDir() = %q, want %q", got, want)
	}
	if got := repo.ManagedReposRoot(); got != root {
		t.Errorf("ManagedReposRoot() = %q, want %q", got, root)
	}
}

// A segment needing encoding lands encoded on the disk path (A19: filesystem-safe
// by construction).
func TestCloneDirEncodesSegments(t *testing.T) {
	setDataHome(t)
	s, err := repo.ParseSource("github:rocne/dot%3Afiles")
	if err != nil {
		t.Fatal(err)
	}
	got := s.CloneDir()
	if !strings.Contains(got, "dot%3Afiles") {
		t.Errorf("CloneDir() = %q; want the encoded segment dot%%3Afiles on the path", got)
	}
	if strings.Contains(got, "dot:files") {
		t.Errorf("CloneDir() = %q; a raw ':' must not reach the disk path", got)
	}
}
