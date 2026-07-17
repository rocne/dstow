package hooks_test

import (
	"path/filepath"
	"testing"

	"github.com/rocne/dstow/internal/hooks"
)

// TestActionString: the four action spellings are the env values (H2).
func TestActionString(t *testing.T) {
	cases := map[hooks.Action]string{
		hooks.ActionStow:   "stow",
		hooks.ActionUnstow: "unstow",
		hooks.ActionRestow: "restow",
		hooks.ActionAdopt:  "adopt",
	}
	for a, want := range cases {
		if got := a.String(); got != want {
			t.Errorf("Action(%d).String() = %q, want %q", int(a), got, want)
		}
	}
}

// TestPhaseString: pre|post (H2).
func TestPhaseString(t *testing.T) {
	if hooks.PhasePre.String() != "pre" || hooks.PhasePost.String() != "post" {
		t.Errorf("Phase spellings wrong: %q %q", hooks.PhasePre, hooks.PhasePost)
	}
}

// TestLevelString: package|repo|global (H2).
func TestLevelString(t *testing.T) {
	cases := map[hooks.Level]string{
		hooks.LevelPackage: "package",
		hooks.LevelRepo:    "repo",
		hooks.LevelGlobal:  "global",
	}
	for l, want := range cases {
		if got := l.String(); got != want {
			t.Errorf("Level(%d).String() = %q, want %q", int(l), got, want)
		}
	}
}

// TestHookFileNames: the eight names are Phase.String()+"-"+Action.String()
// (M6).
func TestHookFileNames(t *testing.T) {
	want := map[hooks.Hook]string{
		{Phase: hooks.PhasePre, Action: hooks.ActionStow}:    "pre-stow",
		{Phase: hooks.PhasePost, Action: hooks.ActionStow}:   "post-stow",
		{Phase: hooks.PhasePre, Action: hooks.ActionUnstow}:  "pre-unstow",
		{Phase: hooks.PhasePost, Action: hooks.ActionUnstow}: "post-unstow",
		{Phase: hooks.PhasePre, Action: hooks.ActionRestow}:  "pre-restow",
		{Phase: hooks.PhasePost, Action: hooks.ActionRestow}: "post-restow",
		{Phase: hooks.PhasePre, Action: hooks.ActionAdopt}:   "pre-adopt",
		{Phase: hooks.PhasePost, Action: hooks.ActionAdopt}:  "post-adopt",
	}
	for h, name := range want {
		if got := h.FileName(); got != name {
			t.Errorf("Hook{%v,%v}.FileName() = %q, want %q", h.Phase, h.Action, got, name)
		}
	}
}

// TestScopeHooksDir: <scopeRoot>/.dstow/hooks via config's metadata accessor
// (M6/A11).
func TestScopeHooksDir(t *testing.T) {
	root := filepath.Join("some", "repo")
	want := filepath.Join(root, ".dstow", "hooks")
	if got := hooks.ScopeHooksDir(root); got != want {
		t.Errorf("ScopeHooksDir(%q) = %q, want %q", root, got, want)
	}
}

// TestInHook: InHook is DSTOW_HOOK_ACTION present in the environment (H7), read
// at the point of use (A2).
func TestInHook(t *testing.T) {
	if hooks.InHook() {
		t.Fatal("InHook true with DSTOW_HOOK_ACTION unset")
	}
	t.Setenv("DSTOW_HOOK_ACTION", "stow")
	if !hooks.InHook() {
		t.Error("InHook false with DSTOW_HOOK_ACTION set")
	}
	t.Setenv("DSTOW_HOOK_ACTION", "")
	if hooks.InHook() {
		t.Error("InHook true with DSTOW_HOOK_ACTION empty (absent-not-empty: empty is not in-hook)")
	}
}
