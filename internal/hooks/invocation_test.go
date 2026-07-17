package hooks_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/hooks"
	"github.com/rocne/dstow/internal/name"
)

// appendHook writes a hook that appends token to the shared log file (M6:
// direct exec, shebang decides the interpreter); optional trailing shell lines
// (e.g. an exit) follow the append.
func appendHook(t *testing.T, hooksDir, fileName, logPath, token string, extra ...string) {
	t.Helper()
	body := "echo " + token + " >> " + logPath
	for _, e := range extra {
		body += "\n" + e
	}
	writeExec(t, filepath.Join(hooksDir, fileName), body)
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// TestNestedLIFOSequencing drives two repos x (2,1) packages through the
// invocation and asserts the exact nested/LIFO order with once-per-invocation
// firing of global and each repo (§9.1.2/§9.1.3, A11).
func TestNestedLIFOSequencing(t *testing.T) {
	globalDir := t.TempDir()
	repo1Dir := t.TempDir()
	repo2Dir := t.TempDir()
	pkgADir := t.TempDir()
	pkgBDir := t.TempDir()
	pkgCDir := t.TempDir()
	log := filepath.Join(t.TempDir(), "log")

	r1 := repoFQN("github", []string{"rocne", "r1"})
	r2 := repoFQN("github", []string{"rocne", "r2"})
	a := pkgFQN("github", []string{"rocne", "r1"}, "a")
	b := pkgFQN("github", []string{"rocne", "r1"}, "b")
	c := pkgFQN("github", []string{"rocne", "r2"}, "c")

	appendHook(t, filepath.Join(globalDir, "hooks"), "pre-stow", log, "global-pre")
	appendHook(t, filepath.Join(globalDir, "hooks"), "post-stow", log, "global-post")
	appendHook(t, hooks.ScopeHooksDir(repo1Dir), "pre-stow", log, "repo1-pre")
	appendHook(t, hooks.ScopeHooksDir(repo1Dir), "post-stow", log, "repo1-post")
	appendHook(t, hooks.ScopeHooksDir(repo2Dir), "pre-stow", log, "repo2-pre")
	appendHook(t, hooks.ScopeHooksDir(repo2Dir), "post-stow", log, "repo2-post")
	appendHook(t, hooks.ScopeHooksDir(pkgADir), "pre-stow", log, "a-pre")
	appendHook(t, hooks.ScopeHooksDir(pkgADir), "post-stow", log, "a-post")
	appendHook(t, hooks.ScopeHooksDir(pkgBDir), "pre-stow", log, "b-pre")
	appendHook(t, hooks.ScopeHooksDir(pkgBDir), "post-stow", log, "b-post")
	appendHook(t, hooks.ScopeHooksDir(pkgCDir), "pre-stow", log, "c-pre")
	appendHook(t, hooks.ScopeHooksDir(pkgCDir), "post-stow", log, "c-post")

	inv := hooks.NewInvocation(hooks.ActionStow, quietRunner(),
		hooks.GlobalScope{Dir: globalDir, Packages: []name.FQN{a, b, c}})

	tgt := t.TempDir()
	scopeA := hooks.PackageScope{FQN: a, Dir: pkgADir, Target: tgt, RepoDir: repo1Dir}
	scopeB := hooks.PackageScope{FQN: b, Dir: pkgBDir, Target: tgt, RepoDir: repo1Dir}
	scopeC := hooks.PackageScope{FQN: c, Dir: pkgCDir, Target: tgt, RepoDir: repo2Dir}

	mustFire := func(w []hooks.Warning, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("firing: %v", err)
		}
	}

	mustFire(inv.BeforePackage(scopeA))
	mustFire(inv.AfterPackage(scopeA))
	mustFire(inv.BeforePackage(scopeB))
	mustFire(inv.AfterPackage(scopeB))
	mustFire(inv.AfterRepo(hooks.RepoScope{FQN: r1, Dir: repo1Dir, Packages: []name.FQN{a, b}}))
	mustFire(inv.BeforePackage(scopeC))
	mustFire(inv.AfterPackage(scopeC))
	mustFire(inv.AfterRepo(hooks.RepoScope{FQN: r2, Dir: repo2Dir, Packages: []name.FQN{c}}))
	mustFire(inv.Finish())

	want := []string{
		"global-pre", "repo1-pre", "a-pre", "a-post", "b-pre", "b-post",
		"repo1-post", "repo2-pre", "c-pre", "c-post", "repo2-post", "global-post",
	}
	got := readLines(t, log)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("sequence mismatch:\n got %v\nwant %v", got, want)
	}
}

// TestPreFiredOnceEvenAfterFailure: a failed repo-pre is not retried by a later
// BeforePackage for the same repo — once fired is fired (§9.1.4).
func TestPreFiredOnceEvenAfterFailure(t *testing.T) {
	globalDir := t.TempDir()
	repoDir := t.TempDir()
	pkgADir := t.TempDir()
	pkgBDir := t.TempDir()
	log := filepath.Join(t.TempDir(), "log")

	a := pkgFQN("github", []string{"rocne", "r1"}, "a")
	b := pkgFQN("github", []string{"rocne", "r1"}, "b")

	// repo-pre appends then fails.
	appendHook(t, hooks.ScopeHooksDir(repoDir), "pre-stow", log, "repo-pre", "exit 1")
	appendHook(t, hooks.ScopeHooksDir(pkgBDir), "pre-stow", log, "b-pre")

	inv := hooks.NewInvocation(hooks.ActionStow, quietRunner(),
		hooks.GlobalScope{Dir: globalDir, Packages: []name.FQN{a, b}})
	tgt := t.TempDir()

	if _, err := inv.BeforePackage(hooks.PackageScope{FQN: a, Dir: pkgADir, Target: tgt, RepoDir: repoDir}); err == nil {
		t.Fatal("expected repo-pre failure on first BeforePackage")
	}
	// ops would normally block here; the contract is that if BeforePackage is
	// called again, the already-fired repo-pre is not retried.
	if _, err := inv.BeforePackage(hooks.PackageScope{FQN: b, Dir: pkgBDir, Target: tgt, RepoDir: repoDir}); err != nil {
		t.Fatalf("second BeforePackage: %v", err)
	}

	got := readLines(t, log)
	count := 0
	for _, l := range got {
		if l == "repo-pre" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("repo-pre fired %d times, want exactly 1 (log %v)", count, got)
	}
}

// TestFinishNoOpWhenNothingActed: Finish fires global-post only if the global
// scope activated (a BeforePackage happened). With nothing acted, global-post
// must not fire (§9.1.5).
func TestFinishNoOpWhenNothingActed(t *testing.T) {
	globalDir := t.TempDir()
	log := filepath.Join(t.TempDir(), "log")
	appendHook(t, filepath.Join(globalDir, "hooks"), "post-stow", log, "global-post")

	inv := hooks.NewInvocation(hooks.ActionStow, quietRunner(),
		hooks.GlobalScope{Dir: globalDir, Packages: nil})

	warns, err := inv.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("Finish drew warnings when nothing acted: %v", warns)
	}
	if lines := readLines(t, log); len(lines) != 0 {
		t.Errorf("global-post fired when nothing acted: %v", lines)
	}
}

// TestDiscoveryErrorClassified: a hooks directory that cannot be read fails
// the firing as a *HookError carrying the Level, so ops can apply §9.1.4
// blocking — an unreadable dir must never silently skip a hook the user
// installed as a guard.
func TestDiscoveryErrorClassified(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: an unreadable directory is still readable")
	}
	globalDir := t.TempDir()
	repoDir := t.TempDir()
	pkgDir := t.TempDir()
	pfqn := pkgFQN("github", []string{"rocne", "dotfiles"}, "zsh")

	hooksDir := hooks.ScopeHooksDir(pkgDir)
	mkdir(t, hooksDir)
	if err := os.Chmod(hooksDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(hooksDir, 0o755) })

	inv := hooks.NewInvocation(hooks.ActionStow, quietRunner(),
		hooks.GlobalScope{Dir: globalDir, Packages: []name.FQN{pfqn}})
	_, err := inv.BeforePackage(hooks.PackageScope{FQN: pfqn, Dir: pkgDir, Target: t.TempDir(), RepoDir: repoDir})
	if err == nil {
		t.Fatal("expected a failure from the unreadable package hooks dir")
	}
	var he *hooks.HookError
	if !errors.As(err, &he) {
		t.Fatalf("expected *hooks.HookError, got %T: %v", err, err)
	}
	if he.Level != hooks.LevelPackage {
		t.Errorf("Level = %v, want package", he.Level)
	}
	if he.Phase != hooks.PhasePre {
		t.Errorf("Phase = %v, want pre", he.Phase)
	}
	if he.Path != hooksDir {
		t.Errorf("Path = %q, want the hooks dir %q", he.Path, hooksDir)
	}
}

// TestDiscoveryWarningsSurfaceOnce: a scope's hooks dir is discovered once per
// invocation and its warnings surface exactly once, even across repeated
// firings that reach the same directory (A11).
func TestDiscoveryWarningsSurfaceOnce(t *testing.T) {
	globalDir := t.TempDir()
	pkgDir := t.TempDir()
	repoDir := t.TempDir()
	pfqn := pkgFQN("github", []string{"rocne", "dotfiles"}, "zsh")

	// A malformed entry in the package hooks dir draws a warning on discovery.
	writeFile(t, filepath.Join(hooks.ScopeHooksDir(pkgDir), "prestow"), "oops")

	inv := hooks.NewInvocation(hooks.ActionStow, quietRunner(),
		hooks.GlobalScope{Dir: globalDir, Packages: []name.FQN{pfqn}})
	scope := hooks.PackageScope{FQN: pfqn, Dir: pkgDir, Target: t.TempDir(), RepoDir: repoDir}

	warns1, err := inv.BeforePackage(scope) // discovers pkg hooks dir
	if err != nil {
		t.Fatalf("BeforePackage: %v", err)
	}
	if len(warns1) != 1 {
		t.Fatalf("first firing should surface the discovery warning once, got %v", warns1)
	}
	warns2, err := inv.AfterPackage(scope) // same dir, already discovered
	if err != nil {
		t.Fatalf("AfterPackage: %v", err)
	}
	if len(warns2) != 0 {
		t.Errorf("warnings must surface exactly once; second firing re-surfaced %v", warns2)
	}
}
