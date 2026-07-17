package engine_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/engine"
)

// tree lays out a repo and a target under one temp root and returns an Op
// for the given package with dstow's built-in floor: folding off,
// dot-translation on (REQUIREMENTS §4.4).
func tree(t *testing.T, pkg string, files map[string]string) engine.Op {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "repo")
	target := filepath.Join(root, "target")
	for rel, content := range files {
		path := filepath.Join(dir, pkg, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, pkg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	return engine.Op{
		Dir:                  dir,
		Target:               target,
		Package:              pkg,
		Fold:                 false,
		TranslateDotPrefixes: true,
	}
}

// mustLstatLink asserts target/rel is a symlink and returns where it leads,
// resolved to an absolute path.
func mustLstatLink(t *testing.T, target, rel string) string {
	t.Helper()
	path := filepath.Join(target, filepath.FromSlash(rel))
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%s): %v", rel, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink", rel)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", rel, err)
	}
	return resolved
}

func kinds(res *engine.Result) map[engine.ActionKind][]string {
	out := map[engine.ActionKind][]string{}
	for _, a := range res.Actions {
		out[a.Kind] = append(out[a.Kind], a.Path)
	}
	return out
}

// --- Deployment verbs: per-package Apply --------------------------------------

func TestStowLinksLeaves(t *testing.T) {
	// Folding off: real directories are created, leaves are linked (§3.3).
	// dot-translation rewrites dot- prefixes (§3.4).
	op := tree(t, "zsh", map[string]string{
		"dot-config/app/conf": "x",
		"bin/tool":            "y",
	})
	res, err := engine.Apply(engine.VerbStow, op)
	if err != nil {
		t.Fatalf("Apply(stow) = %v", err)
	}

	got := kinds(res)
	wantLinks := []string{".config/app/conf", "bin/tool"}
	links := map[string]bool{}
	for _, l := range got[engine.LinkCreated] {
		links[l] = true
	}
	if len(links) != len(wantLinks) {
		t.Fatalf("LinkCreated = %v, want %v", got[engine.LinkCreated], wantLinks)
	}
	for _, l := range wantLinks {
		if !links[l] {
			t.Errorf("LinkCreated %v is missing %s", got[engine.LinkCreated], l)
		}
		resolved := mustLstatLink(t, op.Target, l)
		if !filepath.IsAbs(resolved) {
			t.Errorf("link %s resolved to non-absolute %s", l, resolved)
		}
	}
	// The literal link text rides Dest — the ledger's `destination` field.
	for _, a := range res.Actions {
		if a.Kind == engine.LinkCreated && a.Dest == "" {
			t.Errorf("LinkCreated %s carries no Dest (ledger needs the literal symlink text)", a.Path)
		}
	}
	// Real directories were made, not linked.
	for _, d := range []string{".config", ".config/app", "bin"} {
		fi, err := os.Lstat(filepath.Join(op.Target, filepath.FromSlash(d)))
		if err != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%s: want a real directory, got %v (err %v)", d, fi, err)
		}
	}
}

func TestStowFoldCollapses(t *testing.T) {
	op := tree(t, "zsh", map[string]string{"bin/tool": "y"})
	op.Fold = true
	res, err := engine.Apply(engine.VerbStow, op)
	if err != nil {
		t.Fatalf("Apply(stow, fold) = %v", err)
	}
	got := kinds(res)
	if len(got[engine.LinkCreated]) != 1 || got[engine.LinkCreated][0] != "bin" {
		t.Fatalf("LinkCreated = %v, want [bin] (folded)", got[engine.LinkCreated])
	}
	mustLstatLink(t, op.Target, "bin")
}

func TestUnstowRemoves(t *testing.T) {
	op := tree(t, "zsh", map[string]string{"bin/tool": "y"})
	if _, err := engine.Apply(engine.VerbStow, op); err != nil {
		t.Fatalf("setup stow: %v", err)
	}
	res, err := engine.Apply(engine.VerbUnstow, op)
	if err != nil {
		t.Fatalf("Apply(unstow) = %v", err)
	}
	got := kinds(res)
	if len(got[engine.LinkRemoved]) != 1 || got[engine.LinkRemoved][0] != "bin/tool" {
		t.Fatalf("LinkRemoved = %v, want [bin/tool]", got[engine.LinkRemoved])
	}
	if _, err := os.Lstat(filepath.Join(op.Target, "bin", "tool")); !os.IsNotExist(err) {
		t.Errorf("bin/tool still present after unstow (err %v)", err)
	}
}

func TestRestowRepairs(t *testing.T) {
	// One atomic unstow+stow. A hand-removed link is restored; note that an
	// unchanged link yields no surviving actions (stow cancels the opposing
	// remove/create pair), so the repair is what is observable.
	op := tree(t, "zsh", map[string]string{"bin/tool": "y", "bin/other": "z"})
	if _, err := engine.Apply(engine.VerbStow, op); err != nil {
		t.Fatalf("setup stow: %v", err)
	}
	if err := os.Remove(filepath.Join(op.Target, "bin", "tool")); err != nil {
		t.Fatal(err)
	}
	res, err := engine.Apply(engine.VerbRestow, op)
	if err != nil {
		t.Fatalf("Apply(restow) = %v", err)
	}
	got := kinds(res)
	if len(got[engine.LinkCreated]) != 1 || got[engine.LinkCreated][0] != "bin/tool" {
		t.Fatalf("LinkCreated = %v, want the repaired [bin/tool]", got[engine.LinkCreated])
	}
	mustLstatLink(t, op.Target, "bin/tool")
	mustLstatLink(t, op.Target, "bin/other")
}

func TestSimulateTouchesNothing(t *testing.T) {
	op := tree(t, "zsh", map[string]string{"bin/tool": "y"})
	op.Simulate = true
	res, err := engine.Apply(engine.VerbStow, op)
	if err != nil {
		t.Fatalf("Apply(stow, simulate) = %v", err)
	}
	if len(res.Actions) == 0 {
		t.Fatal("simulate reported no planned actions")
	}
	if _, err := os.Lstat(filepath.Join(op.Target, "bin")); !os.IsNotExist(err) {
		t.Errorf("simulate touched the target (err %v)", err)
	}
}

func TestAdoptMovesTheOccupant(t *testing.T) {
	op := tree(t, "zsh", map[string]string{"bin/tool": "packaged"})
	occupant := filepath.Join(op.Target, "bin", "tool")
	if err := os.MkdirAll(filepath.Dir(occupant), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(occupant, []byte("adopted"), 0o644); err != nil {
		t.Fatal(err)
	}
	op.Adopt = true
	res, err := engine.Apply(engine.VerbStow, op)
	if err != nil {
		t.Fatalf("Apply(stow, adopt) = %v", err)
	}
	got := kinds(res)
	if len(got[engine.FileMoved]) != 1 {
		t.Fatalf("FileMoved = %v, want exactly the occupant", got[engine.FileMoved])
	}
	// The occupant's content now lives in the package; the link points at it.
	content, err := os.ReadFile(filepath.Join(op.Dir, "zsh", "bin", "tool"))
	if err != nil || string(content) != "adopted" {
		t.Errorf("package file = %q, %v; want adopted content", content, err)
	}
	mustLstatLink(t, op.Target, "bin/tool")
}

// --- Conflicts: typed, mapped here and nowhere else (A14) ----------------------

func TestConflictExistingFile(t *testing.T) {
	op := tree(t, "zsh", map[string]string{"bin/tool": "y", "bin/other": "z"})
	occupant := filepath.Join(op.Target, "bin", "tool")
	if err := os.MkdirAll(filepath.Dir(occupant), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(occupant, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := engine.Apply(engine.VerbStow, op)
	var cerr *engine.ConflictError
	if !errors.As(err, &cerr) {
		t.Fatalf("Apply over occupied path = %v (%T), want *engine.ConflictError", err, err)
	}
	if cerr.Package != "zsh" {
		t.Errorf("ConflictError.Package = %q, want zsh", cerr.Package)
	}
	if len(cerr.Conflicts) != 1 {
		t.Fatalf("Conflicts = %v, want exactly one", cerr.Conflicts)
	}
	c := cerr.Conflicts[0]
	if c.Kind != engine.ConflictExistingFile {
		t.Errorf("Kind = %v, want ConflictExistingFile (the adoptable case)", c.Kind)
	}
	if c.Path != "bin/tool" {
		t.Errorf("Path = %q, want bin/tool", c.Path)
	}
	if c.Message == "" {
		t.Error("Message empty; want stow's prose for display")
	}
	if c.Verb != engine.VerbStow {
		t.Errorf("Verb = %v, want VerbStow", c.Verb)
	}
	// All-or-nothing within the package: the clean sibling was not linked.
	if _, err := os.Lstat(filepath.Join(op.Target, "bin", "other")); !os.IsNotExist(err) {
		t.Errorf("conflicting plan wrote bin/other anyway (err %v)", err)
	}
}

func TestConflictForeignLink(t *testing.T) {
	op := tree(t, "zsh", map[string]string{"bin/tool": "y"})
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(elsewhere, []byte("w"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(op.Target, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(op.Target, "bin", "tool")); err != nil {
		t.Fatal(err)
	}
	_, err := engine.Apply(engine.VerbStow, op)
	var cerr *engine.ConflictError
	if !errors.As(err, &cerr) {
		t.Fatalf("Apply over foreign link = %v (%T), want *engine.ConflictError", err, err)
	}
	if len(cerr.Conflicts) != 1 || cerr.Conflicts[0].Kind != engine.ConflictForeignLink {
		t.Fatalf("Conflicts = %+v, want one ConflictForeignLink", cerr.Conflicts)
	}
}

// --- Ignores: three additive layers at the seam --------------------------------

func TestMetadataAutoIgnore(t *testing.T) {
	// M8: .dstow anchored at the package root only — always on, in no
	// config. A deeper .dstow is content and deploys.
	op := tree(t, "zsh", map[string]string{
		".dstow/config.toml": "c",
		"deep/.dstow":        "content",
		"bin/tool":           "y",
	})
	res, err := engine.Apply(engine.VerbStow, op)
	if err != nil {
		t.Fatalf("Apply = %v", err)
	}
	links := kinds(res)[engine.LinkCreated]
	for _, l := range links {
		if l == ".dstow" || l == ".dstow/config.toml" {
			t.Errorf("package-root .dstow deployed as %s (M8 violated)", l)
		}
	}
	mustLstatLink(t, op.Target, "deep/.dstow")
}

func TestNativeChainRidesIgnoreFunc(t *testing.T) {
	op := tree(t, "zsh", map[string]string{
		"logs/debug.log": "l",
		"bin/tool":       "y",
	})
	op.Ignores = []config.IgnorePattern{{
		Pattern:  "*.log",
		Language: config.LangGlob,
		Level:    config.LevelPackage,
		Source:   "pkg config",
	}}
	res, err := engine.Apply(engine.VerbStow, op)
	if err != nil {
		t.Fatalf("Apply = %v", err)
	}
	for _, l := range kinds(res)[engine.LinkCreated] {
		if l == "logs/debug.log" {
			t.Error("glob-ignored file was deployed")
		}
	}
	mustLstatLink(t, op.Target, "bin/tool")
}

func TestCompatChainRidesEngineIgnore(t *testing.T) {
	// A15: stow-regex entries ride gostow's own Options.Ignore lane.
	op := tree(t, "zsh", map[string]string{
		"top.secret": "s",
		"bin/tool":   "y",
	})
	op.Ignores = []config.IgnorePattern{{
		Pattern:  `\.secret`,
		Language: config.LangStowRegex,
		Level:    config.LevelRepo,
		Source:   "repo/.stowrc",
	}}
	res, err := engine.Apply(engine.VerbStow, op)
	if err != nil {
		t.Fatalf("Apply = %v", err)
	}
	for _, l := range kinds(res)[engine.LinkCreated] {
		if l == "top.secret" {
			t.Error("compat-ignored file was deployed")
		}
	}
	mustLstatLink(t, op.Target, "bin/tool")
}

func TestBuiltinFloorStillApplies(t *testing.T) {
	// REQUIREMENTS §4.4: stow's standard built-in ignores ride gostow; the
	// IgnoreFunc seam is additive-only and must not displace them.
	op := tree(t, "zsh", map[string]string{
		"README.md": "r",
		"bin/tool":  "y",
	})
	res, err := engine.Apply(engine.VerbStow, op)
	if err != nil {
		t.Fatalf("Apply = %v", err)
	}
	for _, l := range kinds(res)[engine.LinkCreated] {
		if l == "README.md" {
			t.Error("stow's built-in README ignore was displaced")
		}
	}
}

func TestGlobalStowIgnoreFileSuppressed(t *testing.T) {
	// A14: NoGlobalIgnoreFile always — a stray ~/.stow-global-ignore must
	// not silently change engine behavior.
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".stow-global-ignore"), []byte("tool\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	op := tree(t, "zsh", map[string]string{"bin/tool": "y"})
	if _, err := engine.Apply(engine.VerbStow, op); err != nil {
		t.Fatalf("Apply = %v", err)
	}
	mustLstatLink(t, op.Target, "bin/tool")
}

func TestRefusedPatternPropagates(t *testing.T) {
	op := tree(t, "zsh", map[string]string{"bin/tool": "y"})
	op.Ignores = []config.IgnorePattern{{
		Pattern:  "!resurrect",
		Language: config.LangGlob,
		Level:    config.LevelPackage,
		Source:   "pkg config",
	}}
	_, err := engine.Apply(engine.VerbStow, op)
	var perr *config.PatternError
	if !errors.As(err, &perr) {
		t.Fatalf("Apply with refused form = %v (%T), want *config.PatternError", err, err)
	}
}

// --- Introspection: Expected and Owner ------------------------------------------

func TestExpectedMapsLinkToSource(t *testing.T) {
	// §6.1: Expected keys are target-relative, values package-relative —
	// exactly the ledger's link/source fields. Same ignores, same
	// translation as Apply.
	op := tree(t, "zsh", map[string]string{
		"dot-config/app/conf": "x",
		".dstow/config.toml":  "c",
		"logs/debug.log":      "l",
	})
	op.Ignores = []config.IgnorePattern{{
		Pattern:  "*.log",
		Language: config.LangGlob,
		Level:    config.LevelPackage,
		Source:   "pkg config",
	}}
	got, err := engine.Expected(op)
	if err != nil {
		t.Fatalf("Expected = %v", err)
	}
	want := map[string]string{".config/app/conf": "dot-config/app/conf"}
	if len(got) != len(want) {
		t.Fatalf("Expected = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("Expected[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestOwnerRecoversThePackage(t *testing.T) {
	op := tree(t, "zsh", map[string]string{"bin/tool": "y"})
	if _, err := engine.Apply(engine.VerbStow, op); err != nil {
		t.Fatalf("setup stow: %v", err)
	}
	pkg, owned, err := engine.Owner(op.Dir, filepath.Join(op.Target, "bin", "tool"))
	if err != nil {
		t.Fatalf("Owner = %v", err)
	}
	if !owned || pkg != "zsh" {
		t.Errorf("Owner = (%q, %v), want (zsh, true)", pkg, owned)
	}
}

func TestOwnerRejectsForeignLink(t *testing.T) {
	op := tree(t, "zsh", map[string]string{"bin/tool": "y"})
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(elsewhere, []byte("w"), 0o644); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(op.Target, "foreign")
	if err := os.Symlink(elsewhere, foreign); err != nil {
		t.Fatal(err)
	}
	_, owned, err := engine.Owner(op.Dir, foreign)
	if err != nil {
		t.Fatalf("Owner = %v", err)
	}
	if owned {
		t.Error("Owner claimed a foreign link")
	}
}

// --- Failure shape ---------------------------------------------------------------

func TestMissingPackageIsTyped(t *testing.T) {
	op := tree(t, "zsh", map[string]string{"bin/tool": "y"})
	op.Package = "absent"
	_, err := engine.Apply(engine.VerbStow, op)
	var oerr *engine.OpError
	if !errors.As(err, &oerr) {
		t.Fatalf("Apply(absent) = %v (%T), want *engine.OpError", err, err)
	}
	if oerr.Package != "absent" {
		t.Errorf("OpError.Package = %q, want absent", oerr.Package)
	}
}
