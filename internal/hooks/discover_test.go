package hooks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/hooks"
)

// The eight canonical hook file names (M6).
var eightNames = []string{
	"pre-stow", "post-stow",
	"pre-unstow", "post-unstow",
	"pre-restow", "post-restow",
	"pre-adopt", "post-adopt",
}

// TestDiscoverEightNames: all eight executables are discovered and mapped to
// their absolute paths, with no warnings (M6).
func TestDiscoverEightNames(t *testing.T) {
	dir := t.TempDir()
	for _, n := range eightNames {
		writeExec(t, filepath.Join(dir, n), "exit 0")
	}

	set, warns, err := hooks.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(warns) != 0 {
		t.Fatalf("expected no warnings, got %v", warns)
	}
	if len(set) != 8 {
		t.Fatalf("expected 8 hooks, got %d: %v", len(set), set)
	}
	for _, p := range []hooks.Phase{hooks.PhasePre, hooks.PhasePost} {
		for _, a := range []hooks.Action{hooks.ActionStow, hooks.ActionUnstow, hooks.ActionRestow, hooks.ActionAdopt} {
			h := hooks.Hook{Phase: p, Action: a}
			want := filepath.Join(dir, h.FileName())
			if got := set[h]; got != want {
				t.Errorf("set[%s] = %q, want %q", h.FileName(), got, want)
			}
		}
	}
}

// TestDiscoverAbsentDir: an absent directory is an empty Set, no warnings, no
// error — hooks are optional (M6).
func TestDiscoverAbsentDir(t *testing.T) {
	set, warns, err := hooks.Discover(filepath.Join(t.TempDir(), "nope", "hooks"))
	if err != nil {
		t.Fatalf("Discover of absent dir errored: %v", err)
	}
	if len(warns) != 0 {
		t.Fatalf("absent dir drew warnings: %v", warns)
	}
	if len(set) != 0 {
		t.Fatalf("absent dir yielded a non-empty set: %v", set)
	}
}

// TestDiscoverReservedDotD: a <hook>.d entry (directory) is reserved and
// warned, inert, with no Fix (M7).
func TestDiscoverReservedDotD(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "pre-stow.d"))

	set, warns, err := hooks.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(set) != 0 {
		t.Fatalf("reserved .d must not be a hook, got set %v", set)
	}
	if len(warns) != 1 {
		t.Fatalf("expected 1 warning, got %v", warns)
	}
	w := warns[0]
	if w.Source != filepath.Join(dir, "pre-stow.d") {
		t.Errorf("Source = %q, want the entry path", w.Source)
	}
	if !strings.Contains(w.Detail, "reserved") {
		t.Errorf("Detail should say reserved: %q", w.Detail)
	}
	if w.Fix != "" {
		t.Errorf("reserved .d has no Fix, got %q", w.Fix)
	}
}

// TestDiscoverReservedDotDFile: the .d reservation covers a FILE named
// <hook>.d too, not only a directory (M7).
func TestDiscoverReservedDotDFile(t *testing.T) {
	dir := t.TempDir()
	writeExec(t, filepath.Join(dir, "post-adopt.d"), "exit 0")

	set, warns, err := hooks.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(set) != 0 {
		t.Fatalf("a .d file is not a hook, got %v", set)
	}
	if len(warns) != 1 || !strings.Contains(warns[0].Detail, "reserved") {
		t.Fatalf("expected one reserved warning, got %v", warns)
	}
}

// TestDiscoverChmodHint: a valid-named non-executable file warns, and the Fix
// is the exact chmod remedy (M6).
func TestDiscoverChmodHint(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pre-stow"), "echo hi")

	set, warns, err := hooks.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(set) != 0 {
		t.Fatalf("non-executable is not a hook, got %v", set)
	}
	if len(warns) != 1 {
		t.Fatalf("expected 1 warning, got %v", warns)
	}
	want := "chmod +x " + filepath.Join(dir, "pre-stow")
	if warns[0].Fix != want {
		t.Errorf("Fix = %q, want %q", warns[0].Fix, want)
	}
}

// TestDiscoverDidYouMean: both documented near-misses (pre_stow and prestow)
// suggest pre-stow (M6).
func TestDiscoverDidYouMean(t *testing.T) {
	for _, misspelling := range []string{"pre_stow", "prestow"} {
		t.Run(misspelling, func(t *testing.T) {
			dir := t.TempDir()
			writeExec(t, filepath.Join(dir, misspelling), "exit 0")

			_, warns, err := hooks.Discover(dir)
			if err != nil {
				t.Fatalf("Discover: %v", err)
			}
			if len(warns) != 1 {
				t.Fatalf("expected 1 warning, got %v", warns)
			}
			w := warns[0]
			if !strings.Contains(w.Detail, "pre-stow") {
				t.Errorf("Detail should suggest pre-stow: %q", w.Detail)
			}
			if !strings.Contains(w.Fix, "pre-stow") {
				t.Errorf("Fix should rename to pre-stow: %q", w.Fix)
			}
		})
	}
}

// TestDiscoverNonHookNoMatch: a non-hook file with no near match warns without
// a Fix (M6).
func TestDiscoverNonHookNoMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "README"), "notes")

	_, warns, err := hooks.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(warns) != 1 {
		t.Fatalf("expected 1 warning, got %v", warns)
	}
	if warns[0].Fix != "" {
		t.Errorf("no near match ⇒ no Fix, got %q", warns[0].Fix)
	}
}

// TestDiscoverOtherSubdirSilent: any subdirectory that is not <hook>.d is inert
// helper territory — never fired, never warned (M6). lib/ is the documented
// convention.
func TestDiscoverOtherSubdirSilent(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "lib"))
	writeExec(t, filepath.Join(dir, "lib", "helper.sh"), "exit 0")
	// A subdirectory that happens to share a hook name (not <hook>.d) is still
	// just a subdirectory: silent.
	mkdir(t, filepath.Join(dir, "pre-stow"))

	set, warns, err := hooks.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(warns) != 0 {
		t.Fatalf("subdirectories must be silent, got %v", warns)
	}
	if len(set) != 0 {
		t.Fatalf("subdirectories are not hooks, got %v", set)
	}
}

// TestDiscoverSymlinkToExecutable: executability is tested with Stat, not
// Lstat — a symlink to an executable is a hook (M6).
func TestDiscoverSymlinkToExecutable(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(t.TempDir(), "real-hook")
	writeExec(t, real, "exit 0")
	link := filepath.Join(dir, "pre-stow")
	if err := symlink(real, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	set, warns, err := hooks.Discover(dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(warns) != 0 {
		t.Fatalf("a symlink to an executable is a clean hook, got %v", warns)
	}
	if got := set[hooks.Hook{Phase: hooks.PhasePre, Action: hooks.ActionStow}]; got != link {
		t.Errorf("symlinked hook path = %q, want %q", got, link)
	}
}
