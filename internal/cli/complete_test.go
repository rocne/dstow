package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCompleteBestEffortSilent asserts A20: with no config the completer yields
// candidates (the two scheme prefixes) and never a diagnostic or panic. It is a
// pure resolver read — no hooks, no network.
func TestCompleteBestEffortSilent(t *testing.T) {
	isolateXDG(t)

	// Empty prefix: at least the two scheme prefixes, no panic.
	got := completeEntities("", false)
	if !contains(got, "github:") || !contains(got, "local:") {
		t.Errorf("empty-registry completion should still offer schemes, got %v", got)
	}

	// Prefix filtering: only matching candidates come back.
	got = completeEntities("loc", false)
	for _, c := range got {
		if len(c) < 3 || c[:3] != "loc" {
			t.Errorf("candidate %q does not match prefix %q", c, "loc")
		}
	}
	if !contains(got, "local:") {
		t.Errorf("prefix loc should include local:, got %v", got)
	}
}

// TestCompleteSessionRepo asserts the completer resolves package and repo names
// from a real repo contributed via DSTOW_PATH (the resolver path, no network).
func TestCompleteSessionRepo(t *testing.T) {
	isolateXDG(t)

	repoDir := filepath.Join(os.Getenv("HOME"), "dots")
	mkdirs(t, filepath.Join(repoDir, "zsh"), filepath.Join(repoDir, "git"))
	t.Setenv("DSTOW_PATH", repoDir)

	got := completeEntities("", false)
	if !contains(got, "zsh") || !contains(got, "git") {
		t.Errorf("session-repo completion should offer its package names, got %v", got)
	}

	// Repos-only completion excludes package names.
	repos := completeEntities("", true)
	if contains(repos, "zsh") {
		t.Errorf("repos-only completion must not offer package names, got %v", repos)
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func mkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
}
