package hooks_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/hooks"
	"github.com/rocne/dstow/internal/name"
)

// dumpEnv is a hook body that writes its whole environment to env.out in its
// cwd (which is the scope's own directory, H5).
const dumpEnv = `env > env.out`

// readEnvKV reads an env-dump file and returns key -> first value. Lines with
// no '=' (e.g. continuation lines of a newline-valued variable) are skipped.
func readEnvKV(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read env dump %s: %v", path, err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		i := strings.IndexByte(line, '=')
		if i < 0 {
			continue
		}
		k := line[:i]
		if _, seen := out[k]; !seen {
			out[k] = line[i+1:]
		}
	}
	return out
}

// hasEnvKey reports whether the dump sets key exactly (the '=' disambiguates
// DSTOW_HOOK_PACKAGE from DSTOW_HOOK_PACKAGES).
func hasEnvKey(t *testing.T, path, key string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read env dump %s: %v", path, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, key+"=") {
			return true
		}
	}
	return false
}

func quietRunner() hooks.Runner {
	return hooks.Runner{Stdin: strings.NewReader(""), Stderr: io.Discard}
}

// TestPackageLevelEnv asserts the full package-level DSTOW_HOOK_* set, the
// encoding stance (FQN encoded; decomposed segments decoded; a local:
// absolute-path coordinate), and that DSTOW_HOOK_PACKAGES is absent (H2).
func TestPackageLevelEnv(t *testing.T) {
	repoDir := t.TempDir()
	pkgDir := t.TempDir()
	target := t.TempDir()
	globalDir := t.TempDir()

	// A package name carrying a reserved byte (':') exercises the encoding
	// stance: the FQN encodes it, the bare package name does not. A local:
	// coordinate with a leading empty segment is the actual absolute path.
	pfqn := pkgFQN("local", []string{"", "home", "rocne", "dots"}, "my:pkg")
	rfqn := repoFQN("local", []string{"", "home", "rocne", "dots"})

	writeExec(t, filepath.Join(hooks.ScopeHooksDir(pkgDir), "pre-stow"), dumpEnv)

	inv := hooks.NewInvocation(hooks.ActionStow, quietRunner(),
		hooks.GlobalScope{Dir: globalDir, Packages: []name.FQN{pfqn}})
	if _, err := inv.BeforePackage(hooks.PackageScope{
		FQN: pfqn, Dir: pkgDir, Target: target, RepoDir: repoDir,
	}); err != nil {
		t.Fatalf("BeforePackage: %v", err)
	}

	dump := filepath.Join(pkgDir, "env.out")
	env := readEnvKV(t, dump)

	want := map[string]string{
		"DSTOW_HOOK_LEVEL":       "package",
		"DSTOW_HOOK_ACTION":      "stow",
		"DSTOW_HOOK_PHASE":       "pre",
		"DSTOW_HOOK_FQN":         pfqn.String(),
		"DSTOW_HOOK_SCHEME":      "local",
		"DSTOW_HOOK_COORDINATE":  "/home/rocne/dots",
		"DSTOW_HOOK_PACKAGE":     "my:pkg",
		"DSTOW_HOOK_PACKAGE_DIR": pkgDir,
		"DSTOW_HOOK_TARGET":      target,
		"DSTOW_HOOK_REPO_FQN":    rfqn.String(),
		"DSTOW_HOOK_REPO_DIR":    repoDir,
	}
	for k, v := range want {
		if got := env[k]; got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	// Encoding stance: the FQN carries the encoded ':' while the bare package
	// stays decoded.
	if !strings.Contains(env["DSTOW_HOOK_FQN"], "%3A") {
		t.Errorf("DSTOW_HOOK_FQN should carry the encoded ':' (%%3A): %q", env["DSTOW_HOOK_FQN"])
	}
	if hasEnvKey(t, dump, "DSTOW_HOOK_PACKAGES") {
		t.Error("DSTOW_HOOK_PACKAGES must be absent at package level")
	}
}

// TestRepoLevelEnv asserts the repo-level set and that PACKAGE, PACKAGE_DIR,
// and TARGET are absent (H2). Fired via AfterRepo (repo-post alone).
func TestRepoLevelEnv(t *testing.T) {
	repoDir := t.TempDir()
	globalDir := t.TempDir()
	rfqn := repoFQN("github", []string{"rocne", "dotfiles"})
	p1 := pkgFQN("github", []string{"rocne", "dotfiles"}, "zsh")
	p2 := pkgFQN("github", []string{"rocne", "dotfiles"}, "git")

	writeExec(t, filepath.Join(hooks.ScopeHooksDir(repoDir), "post-stow"), dumpEnv)

	inv := hooks.NewInvocation(hooks.ActionStow, quietRunner(),
		hooks.GlobalScope{Dir: globalDir, Packages: []name.FQN{p1, p2}})
	if _, err := inv.AfterRepo(hooks.RepoScope{FQN: rfqn, Dir: repoDir, Packages: []name.FQN{p1, p2}}); err != nil {
		t.Fatalf("AfterRepo: %v", err)
	}

	dump := filepath.Join(repoDir, "env.out")
	env := readEnvKV(t, dump)
	want := map[string]string{
		"DSTOW_HOOK_LEVEL":    "repo",
		"DSTOW_HOOK_ACTION":   "stow",
		"DSTOW_HOOK_PHASE":    "post",
		"DSTOW_HOOK_FQN":      rfqn.String(),
		"DSTOW_HOOK_SCHEME":   "github",
		"DSTOW_HOOK_REPO_FQN": rfqn.String(),
		"DSTOW_HOOK_REPO_DIR": repoDir,
	}
	for k, v := range want {
		if got := env[k]; got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if env["DSTOW_HOOK_FQN"] != env["DSTOW_HOOK_REPO_FQN"] {
		t.Errorf("at repo level FQN and REPO_FQN are the same value: %q vs %q", env["DSTOW_HOOK_FQN"], env["DSTOW_HOOK_REPO_FQN"])
	}
	if !hasEnvKey(t, dump, "DSTOW_HOOK_PACKAGES") {
		t.Error("DSTOW_HOOK_PACKAGES must be present at repo level")
	}
	for _, absent := range []string{"DSTOW_HOOK_PACKAGE", "DSTOW_HOOK_PACKAGE_DIR", "DSTOW_HOOK_TARGET"} {
		if hasEnvKey(t, dump, absent) {
			t.Errorf("%s must be absent at repo level", absent)
		}
	}
}

// TestGlobalLevelEnv asserts the global-level set is only LEVEL/ACTION/PHASE +
// PACKAGES, with every scope-specific variable absent (H2). Fired via
// global-pre (BeforePackage activates it).
func TestGlobalLevelEnv(t *testing.T) {
	globalDir := t.TempDir()
	pkgDir := t.TempDir()
	repoDir := t.TempDir()
	pfqn := pkgFQN("github", []string{"rocne", "dotfiles"}, "zsh")

	writeExec(t, filepath.Join(globalDir, "hooks", "pre-stow"), dumpEnv)

	inv := hooks.NewInvocation(hooks.ActionStow, quietRunner(),
		hooks.GlobalScope{Dir: globalDir, Packages: []name.FQN{pfqn}})
	if _, err := inv.BeforePackage(hooks.PackageScope{
		FQN: pfqn, Dir: pkgDir, Target: t.TempDir(), RepoDir: repoDir,
	}); err != nil {
		t.Fatalf("BeforePackage: %v", err)
	}

	dump := filepath.Join(globalDir, "env.out")
	env := readEnvKV(t, dump)
	if env["DSTOW_HOOK_LEVEL"] != "global" || env["DSTOW_HOOK_ACTION"] != "stow" || env["DSTOW_HOOK_PHASE"] != "pre" {
		t.Errorf("global base vars wrong: %v", env)
	}
	if !hasEnvKey(t, dump, "DSTOW_HOOK_PACKAGES") {
		t.Error("DSTOW_HOOK_PACKAGES must be present at global level")
	}
	for _, absent := range []string{
		"DSTOW_HOOK_FQN", "DSTOW_HOOK_SCHEME", "DSTOW_HOOK_COORDINATE",
		"DSTOW_HOOK_PACKAGE", "DSTOW_HOOK_PACKAGE_DIR", "DSTOW_HOOK_TARGET",
		"DSTOW_HOOK_REPO_FQN", "DSTOW_HOOK_REPO_DIR",
	} {
		if hasEnvKey(t, dump, absent) {
			t.Errorf("%s must be absent at global level", absent)
		}
	}
}

// TestPackagesNewlineIdiom asserts DSTOW_HOOK_PACKAGES is one canonical encoded
// FQN per line, newline-separated, with no trailing newline — and that a name
// carrying a literal newline is encoded (%0A) so the one-per-line list stays
// airtight (H4).
func TestPackagesNewlineIdiom(t *testing.T) {
	globalDir := t.TempDir()
	pkgDir := t.TempDir()
	repoDir := t.TempDir()

	p1 := pkgFQN("github", []string{"rocne", "dotfiles"}, "zsh")
	p2 := pkgFQN("github", []string{"rocne", "dotfiles"}, "git")
	p3 := pkgFQN("github", []string{"rocne", "dotfiles"}, "a\nb") // newline in name
	packages := []name.FQN{p1, p2, p3}

	writeExec(t, filepath.Join(globalDir, "hooks", "pre-stow"),
		`printf '%s' "$DSTOW_HOOK_PACKAGES" > pkgs.out`)

	inv := hooks.NewInvocation(hooks.ActionStow, quietRunner(),
		hooks.GlobalScope{Dir: globalDir, Packages: packages})
	if _, err := inv.BeforePackage(hooks.PackageScope{
		FQN: p1, Dir: pkgDir, Target: t.TempDir(), RepoDir: repoDir,
	}); err != nil {
		t.Fatalf("BeforePackage: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(globalDir, "pkgs.out"))
	if err != nil {
		t.Fatalf("read pkgs.out: %v", err)
	}
	got := string(data)
	if strings.HasSuffix(got, "\n") {
		t.Errorf("DSTOW_HOOK_PACKAGES has a trailing newline: %q", got)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	if lines[0] != p1.String() || lines[1] != p2.String() || lines[2] != p3.String() {
		t.Errorf("lines = %q, want the three canonical FQNs", lines)
	}
	if !strings.Contains(lines[2], "%0A") {
		t.Errorf("a newline in a name must encode as %%0A: %q", lines[2])
	}
}
