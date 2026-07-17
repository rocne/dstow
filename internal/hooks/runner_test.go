package hooks_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/hooks"
	"github.com/rocne/dstow/internal/name"
)

// TestBothStreamsOntoStderrAndStdinPassthrough: a hook's stdout AND stderr both
// land on the injected Stderr (H6), and the injected Stdin passes through to
// the hook.
func TestBothStreamsOntoStderrAndStdinPassthrough(t *testing.T) {
	pkgDir := t.TempDir()
	pfqn := pkgFQN("github", []string{"rocne", "dotfiles"}, "zsh")

	// echo to stdout, echo to stderr, then cat stdin through to stdout.
	writeExec(t, filepath.Join(hooks.ScopeHooksDir(pkgDir), "post-stow"),
		"echo to-stdout\necho to-stderr 1>&2\ncat")

	var stderr bytes.Buffer
	runner := hooks.Runner{Stdin: strings.NewReader("piped-input"), Stderr: &stderr}
	inv := hooks.NewInvocation(hooks.ActionStow, runner,
		hooks.GlobalScope{Dir: t.TempDir(), Packages: []name.FQN{pfqn}})

	if _, err := inv.AfterPackage(hooks.PackageScope{
		FQN: pfqn, Dir: pkgDir, Target: t.TempDir(), RepoDir: t.TempDir(),
	}); err != nil {
		t.Fatalf("AfterPackage: %v", err)
	}

	out := stderr.String()
	for _, want := range []string{"to-stdout", "to-stderr", "piped-input"} {
		if !strings.Contains(out, want) {
			t.Errorf("combined stderr missing %q; got %q", want, out)
		}
	}
}

// TestHookCwdIsScopeDir: the hook runs with cwd = the scope's own directory
// (H5), not the caller's incidental cwd.
func TestHookCwdIsScopeDir(t *testing.T) {
	pkgDir := t.TempDir()
	pfqn := pkgFQN("github", []string{"rocne", "dotfiles"}, "zsh")

	var stderr bytes.Buffer
	writeExec(t, filepath.Join(hooks.ScopeHooksDir(pkgDir), "pre-stow"), "pwd")
	runner := hooks.Runner{Stdin: strings.NewReader(""), Stderr: &stderr}
	inv := hooks.NewInvocation(hooks.ActionStow, runner,
		hooks.GlobalScope{Dir: t.TempDir(), Packages: []name.FQN{pfqn}})

	if _, err := inv.BeforePackage(hooks.PackageScope{
		FQN: pfqn, Dir: pkgDir, Target: t.TempDir(), RepoDir: t.TempDir(),
	}); err != nil {
		t.Fatalf("BeforePackage: %v", err)
	}

	// Resolve both sides through symlinks (macOS /var -> /private/var).
	gotPwd := strings.TrimSpace(stderr.String())
	got, err := filepath.EvalSymlinks(gotPwd)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", gotPwd, err)
	}
	want, err := filepath.EvalSymlinks(pkgDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("hook cwd = %q, want the package dir %q", got, want)
	}
}

// TestPreFailureClassification: a failing hook surfaces a *HookError whose
// Level/Phase/Action/Path classify the failure for ops (§9.1.4), with Unwrap
// exposing the underlying exec error.
func TestPreFailureClassification(t *testing.T) {
	repoDir := t.TempDir()
	pkgDir := t.TempDir()
	pfqn := pkgFQN("github", []string{"rocne", "dotfiles"}, "zsh")
	hookPath := filepath.Join(hooks.ScopeHooksDir(repoDir), "pre-stow")
	writeExec(t, hookPath, "exit 7")

	inv := hooks.NewInvocation(hooks.ActionStow, quietRunner(),
		hooks.GlobalScope{Dir: t.TempDir(), Packages: []name.FQN{pfqn}})
	_, err := inv.BeforePackage(hooks.PackageScope{
		FQN: pfqn, Dir: pkgDir, Target: t.TempDir(), RepoDir: repoDir,
	})
	if err == nil {
		t.Fatal("expected a failure from the repo-pre hook")
	}
	var he *hooks.HookError
	if !errors.As(err, &he) {
		t.Fatalf("expected *hooks.HookError, got %T: %v", err, err)
	}
	if he.Level != hooks.LevelRepo {
		t.Errorf("Level = %v, want repo", he.Level)
	}
	if he.Phase != hooks.PhasePre {
		t.Errorf("Phase = %v, want pre", he.Phase)
	}
	if he.Action != hooks.ActionStow {
		t.Errorf("Action = %v, want stow", he.Action)
	}
	if he.Path != hookPath {
		t.Errorf("Path = %q, want %q", he.Path, hookPath)
	}
	if he.Unwrap() == nil {
		t.Error("Unwrap should expose the underlying exec error")
	}
}
