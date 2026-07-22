package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/git"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/ui"
)

// TestClassifyExit is the A3 exit-code map's table: each typed domain error maps
// to its code (§8.1 A3). The assertions come from A3's own wording — refusal /
// environment shapes are 3, exit 2 is malformed invocation only, and everything
// unclassified (including a name or theme that resolves to nothing) is a general
// negative outcome (1) — the not-found family ruled → 1 on #47.
func TestClassifyExit(t *testing.T) {
	fqn := name.FQN{Scheme: "github", Coordinate: []string{"o", "n"}, Package: "p"}
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"corrupt ledger", &ledger.CorruptError{Path: "p", Err: errors.New("bad")}, exitRefusal},
		{"newer ledger", &ledger.NewerVersionError{Path: "p", FileVersion: 9}, exitRefusal},
		{"lock contention", &ledger.LockedError{LockPath: "l"}, exitRefusal},
		{"git not installed", &git.NotInstalledError{Binary: "git"}, exitRefusal},
		{"git diverged", &git.DivergedError{Dir: "d"}, exitRefusal},
		{"ambiguous name", &ops.AmbiguousNameError{Input: "x", Matches: []name.FQN{fqn}}, exitRefusal},
		{"source ambiguous", &ops.SourceAmbiguousError{Input: "o/n"}, exitRefusal},
		{"source declined", &ops.SourceDeclinedError{Input: "o/n"}, exitRefusal},
		{"source unresolvable", &ops.SourceUnresolvableError{Input: "x"}, exitRefusal},
		{"rename requested", &ops.RenameRequestedError{Source: "s"}, exitRefusal},
		{"still stowed", &ops.StillStowedError{FQN: fqn.Repo()}, exitRefusal},
		{"unsaved work", &ops.UnsavedWorkError{FQN: fqn.Repo(), Dir: "d"}, exitRefusal},
		{"fold conflict", &ops.FoldConflictError{}, exitRefusal},
		{"non-interactive prompt", &nonInteractiveError{question: "q?"}, exitRefusal},
		{"bulk refusal", &bulkRefusalError{verb: "stow"}, exitRefusal},
		{"usage error", &usageError{errors.New("bad flag")}, exitUsage},
		{"not found scope", &ops.NotFoundError{Input: "x"}, exitNegative},
		{"theme not found", &ui.ThemeNotFoundError{Ref: "nope"}, exitNegative},
		{"generic failure", errors.New("disk exploded"), exitNegative},
		{"wrapped corrupt", fmt.Errorf("context: %w", &ledger.CorruptError{Path: "p"}), exitRefusal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyExit(tc.err); got != tc.want {
				t.Errorf("classifyExit(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// TestFixForDerivesRunnableRemedy checks that fix lines come from the error's
// fields (O2: a machine-stable runnable remedy), not string parsing.
func TestFixForDerivesRunnableRemedy(t *testing.T) {
	corrupt := &ledger.CorruptError{Path: "p", Err: errors.New("bad")}
	if fix := fixFor(corrupt); !strings.Contains(fix, "dstow rebuild") {
		t.Errorf("corrupt fix = %q, want it to name dstow rebuild", fix)
	}
	amb := &ops.AmbiguousNameError{Input: "zsh", Matches: []name.FQN{
		{Scheme: "github", Coordinate: []string{"a", "b"}, Package: "zsh"},
		{Scheme: "local", Coordinate: []string{"", "x"}, Package: "zsh"},
	}}
	fix := fixFor(amb)
	if !strings.Contains(fix, "github:a/b::zsh") || !strings.Contains(fix, "local:/x::zsh") {
		t.Errorf("ambiguous fix = %q, want both qualified spellings", fix)
	}
	if fixFor(errors.New("plain")) != "" {
		t.Errorf("a plain error should carry no derived fix")
	}
}

// TestDeployNotFoundNamesRemedy pins §1.4 (finding C2) on the deploy path: a
// per-package not-found is a StatusNotFound run-line, not a returned error, so
// it once bypassed the fix: remedy the resolve-error path emits. The two
// not-found experiences must name the same remedy.
func TestDeployNotFoundNamesRemedy(t *testing.T) {
	isolateXDG(t)
	_, errs, code := run(t, "stow", "nonexistent")
	if code != 1 {
		t.Fatalf("stow nonexistent exit = %d, want 1", code)
	}
	if !strings.Contains(errs, "nonexistent not found") {
		t.Errorf("missing the not-found run-line:\n%s", errs)
	}
	if !strings.Contains(errs, "fix:") || !strings.Contains(errs, "dstow list") {
		t.Errorf("deploy not-found names no dstow list remedy — §1.4 unmet:\n%s", errs)
	}
	// One remedy line per run, not one per operand: two not-found operands emit
	// the fix once.
	_, errs2, _ := run(t, "stow", "fake1", "fake2")
	if n := strings.Count(errs2, "fix:"); n != 1 {
		t.Errorf("two not-found operands emitted %d fix lines, want 1:\n%s", n, errs2)
	}
}
