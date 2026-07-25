package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		{"path operand where a name is required", &usageError{&pathOperandError{verb: "repo remove", needs: "a repo name", input: "/abs/path"}}, exitUsage},
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

// TestFixForNameAndPathOperandErrors pins the two remedies #183 added, and the
// asymmetry between them: only an absolute path has a "local:" spelling to
// suggest. "local:~/foo" parses but its coordinate is a literal "~" segment
// rather than the home directory, so suggesting it would hand the user a
// spelling that resolves to nothing — the fix must not name it.
func TestFixForNameAndPathOperandErrors(t *testing.T) {
	// A path operand where a repo name is required: the fix is the runnable
	// qualified command for an absolute path.
	abs := &pathOperandError{verb: "repo remove", needs: "a repo name", input: "/home/you/dots"}
	if fix := fixFor(abs); fix != "dstow repo remove local:/home/you/dots" {
		t.Errorf("absolute path fix = %q, want the runnable qualified command", fix)
	}
	// The other three §1.3 prefixes have no qualified spelling to suggest.
	for _, in := range []string{"~/dots", "./dots", "../dots"} {
		fix := fixFor(&pathOperandError{verb: "repo remove", needs: "a repo name", input: in})
		if fix == "" {
			t.Errorf("%q: fix is empty; every refusal names a remedy", in)
		}
		if strings.Contains(fix, "local:"+in) {
			t.Errorf("%q: fix names %q, a spelling that parses but resolves to nothing", in, "local:"+in)
		}
	}
	// A parse failure on a path-shaped name operand points at the scheme it
	// lacks — the #183 scenario (a) remedy.
	pe := &name.ParseError{Input: "/abs/path::pkg", Reason: "a coordinate segment is empty"}
	if fix := fixFor(pe); !strings.Contains(fix, "local:/abs/path::pkg") {
		t.Errorf("path-shaped parse fix = %q, want the qualified spelling", fix)
	}
	// A parse failure that is not path-shaped still names a remedy, without
	// inventing a local: spelling for something that is not a path.
	bare := &name.ParseError{Input: "bad%zz", Reason: "invalid percent-escape"}
	fix := fixFor(bare)
	if fix == "" {
		t.Errorf("a parse error carries no fix; the error:/fix: pairing must not lapse on the name grammar")
	}
	if strings.Contains(fix, "local:") {
		t.Errorf("non-path parse fix = %q, want no invented local: spelling", fix)
	}
}

// TestRepoRemoveRefusesEveryPathOperand pins the §1.3 operand rule at repo
// remove (ruled #183). remove takes a repo *name*; a path operand refers to the
// target world, which remove has no reading of. add's acceptance of a path is
// not a counter-example — add takes a *source*, a different grammar.
//
// All four §1.3 prefixes must behave alike. Before the fix they did not: a
// leading "/" produced a raw parser message, while "./", "~/" and "../" parsed
// as ordinary names and fell through to a silent "not found" — two different
// wrong answers to one question.
func TestRepoRemoveRefusesEveryPathOperand(t *testing.T) {
	for _, operand := range []string{"/home/you/dots", "./dots", "~/dots", "../dots"} {
		t.Run(operand, func(t *testing.T) {
			isolateXDG(t)
			_, errs, code := run(t, "repo", "remove", operand)
			if code != exitUsage {
				t.Errorf("exit = %d, want %d — an operand-kind mismatch is a malformed invocation, not a lookup failure", code, exitUsage)
			}
			if !strings.Contains(errs, "is a path") {
				t.Errorf("stderr does not name the kind mismatch:\n%s", errs)
			}
			if !strings.Contains(errs, "fix:") {
				t.Errorf("stderr carries no fix: line:\n%s", errs)
			}
			if strings.Contains(errs, "not found") {
				t.Errorf("a path operand was resolved as a name and reported not found:\n%s", errs)
			}
		})
	}
}

// TestRepoRemoveStillTakesNames guards the refusal above against overreach: the
// spellings that always worked must keep working. §1.4 ships zero input-side
// aliases in v1, so the canonical name is the spelling, and a name that is
// simply absent is still the not-found family (exit 1), never usage.
func TestRepoRemoveStillTakesNames(t *testing.T) {
	isolateXDG(t)
	dir := filepath.Join(t.TempDir(), "dots")
	if err := os.MkdirAll(filepath.Join(dir, "zsh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, code := run(t, "repo", "add", dir); code != 0 {
		t.Fatalf("repo add %s exit = %d", dir, code)
	}
	if _, errs, code := run(t, "repo", "remove", "local:"+dir); code != 0 {
		t.Fatalf("repo remove by canonical name exit = %d:\n%s", code, errs)
	}
	if _, _, code := run(t, "repo", "remove", "no-such-repo"); code != exitNegative {
		t.Errorf("absent name exit = %d, want %d (the not-found family)", code, exitNegative)
	}
}

// TestNameErrorsExposeNoInternalPrefix is #183's acceptance criterion: no
// user-facing error carries the internal "dstow/name:" module identifier, and
// the error:/fix: pairing does not lapse on the name grammar. Driven black-box
// through Run, which is the only surface that can make the claim — the message
// is what a user sees, not what a package returns.
func TestNameErrorsExposeNoInternalPrefix(t *testing.T) {
	// The surfaces a *ParseError actually reaches. status is deliberately absent:
	// it takes either operand kind, so it classifies "/abs/path::pkg" as a path
	// (§1.3) and answers the per-path question instead of parsing a name.
	cases := []struct {
		args []string
		// wantFix is false for the deploy run-line surface, which reports a
		// per-package failure rather than returning the error fixFor consumes.
		wantFix bool
	}{
		{[]string{"info", "/abs/path::pkg"}, true},
		{[]string{"list", "/abs/path::pkg"}, true},
		{[]string{"stow", "/abs/path::pkg"}, false},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			isolateXDG(t)
			out, errs, _ := run(t, tc.args...)
			for _, stream := range []string{out, errs} {
				if strings.Contains(stream, "dstow/name") {
					t.Errorf("output leaks the internal package prefix:\n%s", stream)
				}
			}
			if !strings.Contains(out+errs, "cannot parse") {
				t.Fatalf("expected a parse failure to reach the user:\n%s%s", out, errs)
			}
			if tc.wantFix && !strings.Contains(errs, "fix:") {
				t.Errorf("no fix: line — the error:/fix: pairing lapsed:\n%s", errs)
			}
		})
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
