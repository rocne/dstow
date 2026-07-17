package ops_test

import (
	"path/filepath"
	"testing"

	"github.com/rocne/dstow/internal/ops"
)

// findField returns the field named n in a scope, or a zero Field with a
// false ok.
func findField(s ops.InfoScope, n string) (ops.Field, bool) {
	for _, f := range s.Fields {
		if f.Name == n {
			return f, true
		}
	}
	return ops.Field{}, false
}

// TestInfoGlobalGroups: no name details the global scope — version and the
// known system paths (inherent), plus the effective config chain (§2.4).
func TestInfoGlobalGroups(t *testing.T) {
	e := newEnv(t)
	e.addRepo("dots")
	e.app.Version = "9.9.9"

	res, err := e.app.Info(ops.InfoRequest{})
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if len(res.Scopes) != 1 || res.Scopes[0].Kind != ops.ScopeGlobal {
		t.Fatalf("scopes = %+v, want one global scope", res.Scopes)
	}
	g := res.Scopes[0]

	ver, ok := findField(g, "version")
	if !ok || ver.Status != ops.FieldSet || ver.Value != "9.9.9" {
		t.Errorf("version = %+v, want set to 9.9.9", ver)
	}
	if ver.Group != ops.GroupInherent {
		t.Errorf("version should be an inherent fact, got group %v", ver.Group)
	}
	ledPath, ok := findField(g, "ledger-path")
	if !ok || ledPath.Status != ops.FieldSet {
		t.Errorf("ledger-path should be a set inherent path: %+v", ledPath)
	}
	target, ok := findField(g, "target")
	if !ok || target.Status != ops.FieldSet || target.Group != ops.GroupConfigured {
		t.Errorf("target should be a set configured value (the $HOME floor): %+v", target)
	}
}

// TestInfoGlobalVersionUnset: an empty version is applicable-but-unset (exit 1
// territory), distinct from an unknown field (§2.4).
func TestInfoGlobalVersionUnset(t *testing.T) {
	e := newEnv(t)
	e.addRepo("dots")
	// Version left empty.

	res, err := e.app.Info(ops.InfoRequest{Fields: []string{"version"}})
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	f, ok := findField(res.Scopes[0], "version")
	if !ok || f.Status != ops.FieldUnset {
		t.Errorf("empty version = %+v, want FieldUnset", f)
	}
}

// TestInfoFieldSelect: -f selects only the named fields, pinned §2.4 tokens
// (source, scheme) with their values.
func TestInfoFieldSelect(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")

	res, err := e.app.Info(ops.InfoRequest{Name: "zsh", Fields: []string{"source", "scheme"}})
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	s := res.Scopes[0]
	if s.Kind != ops.ScopePackage {
		t.Fatalf("scope kind = %v, want package", s.Kind)
	}
	if len(s.Fields) != 2 {
		t.Fatalf("got %d fields, want exactly the 2 selected: %+v", len(s.Fields), s.Fields)
	}
	scheme, _ := findField(s, "scheme")
	if scheme.Status != ops.FieldSet || scheme.Value != "local" {
		t.Errorf("scheme = %+v, want set to local", scheme)
	}
	source, _ := findField(s, "source")
	if source.Status != ops.FieldSet || source.Value == "" {
		t.Errorf("source = %+v, want a set value", source)
	}
}

// TestInfoUnsetUnknownIllegal: the three per-field distinctions §2.4's exit
// codes need — unknown field (2), field illegal for the scope (2), and
// applicable-but-empty (1).
func TestInfoUnsetUnknownIllegal(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")

	res, err := e.app.Info(ops.InfoRequest{
		Name:   "zsh",
		Fields: []string{"bogus", "version", "ignores"},
	})
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	s := res.Scopes[0]

	bogus, _ := findField(s, "bogus")
	if bogus.Status != ops.FieldUnknown {
		t.Errorf("bogus = %+v, want FieldUnknown", bogus)
	}
	if bogus.Suggestion == "" {
		t.Error("an unknown field should carry a suggestion")
	}
	// version is a real field, but global-only — illegal for a package scope.
	ver, _ := findField(s, "version")
	if ver.Status != ops.FieldIllegal {
		t.Errorf("version on a package = %+v, want FieldIllegal", ver)
	}
	// No ignores configured anywhere: applicable but empty.
	ig, _ := findField(s, "ignores")
	if ig.Status != ops.FieldUnset {
		t.Errorf("empty ignores = %+v, want FieldUnset", ig)
	}
}

// TestInfoRecurseSkipsInapplicable: under -r, a field that does not apply to a
// scope is marked illegal for that scope (cli silently skips it) while applying
// scopes carry a value (§2.4).
func TestInfoRecurseSkipsInapplicable(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")

	res, err := e.app.Info(ops.InfoRequest{Recurse: true, Fields: []string{"exclude-from-bulk"}})
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	var global, pkg *ops.InfoScope
	for i := range res.Scopes {
		switch res.Scopes[i].Kind {
		case ops.ScopeGlobal:
			global = &res.Scopes[i]
		case ops.ScopePackage:
			pkg = &res.Scopes[i]
		}
	}
	if global == nil || pkg == nil {
		t.Fatalf("recurse should visit global and package scopes: %+v", res.Scopes)
	}
	gf, _ := findField(*global, "exclude-from-bulk")
	if gf.Status != ops.FieldIllegal {
		t.Errorf("exclude-from-bulk on global = %+v, want FieldIllegal (skipped under -r)", gf)
	}
	pf, _ := findField(*pkg, "exclude-from-bulk")
	if pf.Status != ops.FieldSet {
		t.Errorf("exclude-from-bulk on a package = %+v, want a set value", pf)
	}
}

// TestInfoRecurseVisitsContainmentTree: -r with no name visits the global
// scope, then every repo, then every package (§2.4).
func TestInfoRecurseVisitsContainmentTree(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")
	e.writeFile(filepath.Join(root, "git", "dot-gitconfig"), "g\n")

	res, err := e.app.Info(ops.InfoRequest{Recurse: true})
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if len(res.Scopes) != 4 {
		t.Fatalf("scopes = %d, want global + repo + 2 packages", len(res.Scopes))
	}
	if res.Scopes[0].Kind != ops.ScopeGlobal || res.Scopes[1].Kind != ops.ScopeRepo {
		t.Errorf("order should be global, repo, then packages: %+v", res.Scopes)
	}
	if res.Scopes[2].Kind != ops.ScopePackage || res.Scopes[3].Kind != ops.ScopePackage {
		t.Errorf("packages should follow their repo: %+v", res.Scopes)
	}
}
