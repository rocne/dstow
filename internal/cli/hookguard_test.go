package cli

import (
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow/internal/hooks"
)

// h7WriteCommands is DESIGN §5 H7's refusing set, written down from the spec
// rather than read off the tree: stow / unstow / restow / adopt / clean /
// rebuild and the four repo verbs. Every other command is a read.
//
// Each carries a well-formed invocation, because cobra validates arg counts
// before any pre-run: a malformed invocation is a usage error (exit 2, A3)
// whether or not a hook is running — dstow cannot refuse a command it has not
// managed to parse.
var h7WriteCommands = []struct {
	path string
	args []string
}{
	{"dstow stow", []string{"stow", "--all"}},
	{"dstow unstow", []string{"unstow", "--all"}},
	{"dstow restow", []string{"restow", "--all"}},
	{"dstow adopt", []string{"adopt", "somefile"}},
	{"dstow clean", []string{"clean"}},
	{"dstow rebuild", []string{"rebuild"}},
	{"dstow repo add", []string{"repo", "add", "/some/path"}},
	{"dstow repo remove", []string{"repo", "remove", "somerepo"}},
	{"dstow repo update", []string{"repo", "update"}},
	{"dstow repo upgrade", []string{"repo", "upgrade"}},
}

// h7ReadCommands is a representative slice of the allowed half — the four H7
// names, plus surfaces from every other group, none of which may ever refuse.
var h7ReadCommands = [][]string{
	{"list"},
	{"info"},
	{"status"},
	{"check"},
	{"version"},
	{"theme", "list"},
	{"name", "encode", "zsh"},
}

// inHook sets the H7 detection variable for the duration of the test.
func inHook(t *testing.T) {
	t.Helper()
	t.Setenv(hooks.EnvAction, "stow")
}

// TestHookGuardWiring asserts the annotated command set is exactly H7's
// refusing set. The defect this ticket fixes was a guard that existed but was
// never wired, so the suite pins the wiring itself, not just the predicate.
func TestHookGuardWiring(t *testing.T) {
	isolateXDG(t)
	root := (&env{}).newRootCmd()

	var annotated []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Annotations[annotationWrite] != "" {
			annotated = append(annotated, c.CommandPath())
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)

	var want []string
	for _, w := range h7WriteCommands {
		want = append(want, w.path)
	}
	sort.Strings(want)
	sort.Strings(annotated)
	if strings.Join(annotated, ", ") != strings.Join(want, ", ") {
		t.Errorf("H7 write annotations = [%s], want [%s]",
			strings.Join(annotated, ", "), strings.Join(want, ", "))
	}
}

// TestHookWriteRefusal drives every write command from inside a hook and
// asserts the H7 refusal: exit 3 (A3 refusal/environment), an error: line
// naming the cause, a fix: line naming the remedy, and nothing on stdout (O1).
func TestHookWriteRefusal(t *testing.T) {
	for _, w := range h7WriteCommands {
		path := w.path
		t.Run(path, func(t *testing.T) {
			isolateXDG(t)
			inHook(t)
			out, errOut, code := run(t, w.args...)
			if code != exitRefusal {
				t.Fatalf("%s inside a hook: exit = %d, want %d\nstderr: %s",
					path, code, exitRefusal, errOut)
			}
			if out != "" {
				t.Errorf("%s refusal wrote to stdout: %q", path, out)
			}
			if !strings.Contains(errOut, "error:") || !strings.Contains(errOut, "fix:") {
				t.Errorf("%s refusal lacks error:/fix: lines:\n%s", path, errOut)
			}
			// The cause named must be the hook, never the ledger lock — the
			// incidental cover that masqueraded as this guard.
			if !strings.Contains(errOut, "hook") {
				t.Errorf("%s refusal does not name the hook as the cause:\n%s", path, errOut)
			}
			if strings.Contains(errOut, "ledger.lock") {
				t.Errorf("%s refused via the ledger lock, not the H7 guard:\n%s", path, errOut)
			}
			if !strings.Contains(errOut, hooks.EnvAction) {
				t.Errorf("%s refusal does not name %s:\n%s", path, hooks.EnvAction, errOut)
			}
		})
	}
}

// TestHookWriteRefusalIsFlagBlind asserts --dry-run refuses too: H7 enumerates
// verbs, not effects, and the membership was ruled in the retractable
// direction (#139). A plan that changes nothing still refuses.
func TestHookWriteRefusalIsFlagBlind(t *testing.T) {
	for _, verb := range []string{"stow", "unstow", "restow"} {
		t.Run(verb, func(t *testing.T) {
			isolateXDG(t)
			inHook(t)
			if _, errOut, code := run(t, verb, "--dry-run", "--all"); code != exitRefusal {
				t.Errorf("%s --dry-run inside a hook: exit = %d, want %d\nstderr: %s",
					verb, code, exitRefusal, errOut)
			}
		})
	}
}

// TestHookReadsAllowed asserts the other half of H7: reads work unchanged from
// inside a hook, so an install hook can consult dstow while it works.
func TestHookReadsAllowed(t *testing.T) {
	for _, args := range h7ReadCommands {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			isolateXDG(t)
			inHook(t)
			_, errOut, code := run(t, args...)
			if code == exitRefusal {
				t.Errorf("dstow %s refused inside a hook (exit %d):\n%s",
					strings.Join(args, " "), code, errOut)
			}
			if strings.Contains(errOut, "from inside a dstow hook") {
				t.Errorf("dstow %s raised the H7 refusal:\n%s", strings.Join(args, " "), errOut)
			}
		})
	}
}

// TestHookGuardOffOutsideHook asserts the guard is inert with the environment
// unset — a write command outside a hook fails (or succeeds) on its own terms
// and never mentions hooks.
func TestHookGuardOffOutsideHook(t *testing.T) {
	isolateXDG(t)
	t.Setenv(hooks.EnvAction, "")
	_, errOut, _ := run(t, "clean")
	if strings.Contains(errOut, "from inside a dstow hook") {
		t.Errorf("clean raised the H7 refusal outside a hook:\n%s", errOut)
	}
}

// TestHookWriteHelpAllowed asserts --help on a write command still renders
// from inside a hook: help is a read, and cobra serves it before the guard's
// pre-run ever fires.
func TestHookWriteHelpAllowed(t *testing.T) {
	isolateXDG(t)
	inHook(t)
	out, errOut, code := run(t, "stow", "--help")
	if code != exitOK {
		t.Fatalf("stow --help inside a hook: exit = %d, want 0\nstderr: %s", code, errOut)
	}
	if !strings.Contains(out, "stow") {
		t.Errorf("stow --help inside a hook printed no help:\n%s", out)
	}
}
