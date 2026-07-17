package ops_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adrg/xdg"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/hooks"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/repo"
)

// fixedNow pins entry clocks so RecordedAt is assertable.
var fixedNow = time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

// fakePrompt answers Confirm calls from a script; running out of answers
// fails the run the way a non-interactive prompter would.
type fakePrompt struct {
	answers []bool
	asked   []string
}

func (f *fakePrompt) Confirm(question string, defaultYes bool) (bool, error) {
	f.asked = append(f.asked, question)
	if len(f.answers) == 0 {
		return false, errors.New("non-interactive: no answer scripted")
	}
	a := f.answers[0]
	f.answers = f.answers[1:]
	return a, nil
}

// localRepo builds the set member for a plain local directory, the way
// BuildSet does for a DSTOW_PATH entry minus the session flag.
func localRepo(dir string) repo.Repo {
	return repo.Repo{
		FQN:  name.FQN{Scheme: "local", Coordinate: strings.Split(filepath.ToSlash(dir), "/")},
		Root: dir,
	}
}

// pkgFQN is the FQN of pkg inside the local repo at dir.
func pkgFQN(dir, pkg string) name.FQN {
	f := localRepo(dir).FQN
	f.Package = pkg
	return f
}

// env is one test's world: a base dir holding repos with fixed names (so
// canonical-FQN order is controlled), a target, a ledger path, a global
// dir, and the App over them.
type env struct {
	t      *testing.T
	base   string
	target string
	app    *ops.App
	prompt *fakePrompt
}

func newEnv(t *testing.T) *env {
	t.Helper()
	base := t.TempDir()
	e := &env{
		t:      t,
		base:   base,
		target: filepath.Join(base, "target"),
		prompt: &fakePrompt{},
	}
	e.app = &ops.App{
		LedgerPath: filepath.Join(base, "state", "ledger.json"),
		GlobalDir:  filepath.Join(base, "global"),
		Hooks:      hooks.Runner{Stdin: strings.NewReader(""), Stderr: io.Discard},
		Prompt:     e.prompt,
		Now:        func() time.Time { return fixedNow },
	}
	if err := os.MkdirAll(e.target, 0o755); err != nil {
		t.Fatal(err)
	}
	return e
}

// addRepo creates a repo directory named n under base, with its .dstow
// config pointing target at the env target, and registers it in the set.
func (e *env) addRepo(n string) string {
	e.t.Helper()
	root := filepath.Join(e.base, n)
	e.writeFile(filepath.Join(root, ".dstow", "config.toml"),
		fmt.Sprintf("target = %q\n", e.target))
	e.app.Repos = append(e.app.Repos, localRepo(root))
	return root
}

func (e *env) writeFile(path, content string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) writeExec(path, body string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		e.t.Fatal(err)
	}
}

// appendHook writes a hook under a scope's hooks dir that logs token.
func (e *env) appendHook(hooksDir, hook, logPath, token string, extra ...string) {
	e.t.Helper()
	body := "echo " + token + " >> " + logPath
	for _, x := range extra {
		body += "\n" + x
	}
	e.writeExec(filepath.Join(hooksDir, hook), body)
}

// setupXDG points XDG_CONFIG_HOME and HOME at temp dirs so the global
// level loads from a controlled location (the config package's own test
// pattern). Returns the dstow global config dir.
func setupXDG(t *testing.T) (globalDir, home string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	xdgRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgRoot)
	xdg.Reload()
	t.Cleanup(xdg.Reload)
	return filepath.Join(xdgRoot, "dstow"), home
}

// loadGlobal (re)loads the global level into the App.
func (e *env) loadGlobal() {
	e.t.Helper()
	g, _, err := config.LoadGlobal()
	if err != nil {
		e.t.Fatalf("LoadGlobal: %v", err)
	}
	e.app.Global = g
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// isLinkTo asserts path is a symlink resolving to dest.
func isLinkTo(t *testing.T, path, dest string) {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("expected %s to be a symlink: %v", path, err)
	}
	abs := got
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(filepath.Dir(path), got)
	}
	want, err := filepath.EvalSymlinks(dest)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatalf("link %s -> %s does not resolve: %v", path, got, err)
	}
	if resolved != want {
		t.Fatalf("link %s resolves to %s, want %s", path, resolved, want)
	}
}

// statuses maps FQN string -> status for compact assertions.
func statuses(res *ops.DeployResult) map[string]ops.PackageStatus {
	out := map[string]ops.PackageStatus{}
	for _, p := range res.Packages {
		key := p.FQN.String()
		if p.FQN.Package == "" && p.Operand != "" {
			key = p.Operand
		}
		out[key] = p.Status
	}
	return out
}
