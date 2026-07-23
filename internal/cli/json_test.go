package cli

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/rocne/dstow"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/ops"
)

func marshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// TestListJSONShape asserts the O10 conventions on the repos listing: lower_snake
// keys and the full FQN (never the O9 short display).
func TestListJSONShape(t *testing.T) {
	fqn := name.FQN{Scheme: "github", Coordinate: []string{"rocne", "dotfiles"}}
	res := &ops.ListResult{Kind: ops.KindRepos, Repos: []ops.RepoListing{{
		FQN: fqn, Display: "dotfiles", Source: fqn.String(), Scheme: "github",
		Root: "/x", ExcludedBulk: true, Managed: true,
	}}}
	got := marshal(t, listJSON(res))
	for _, key := range []string{`"fqn":"github:rocne/dotfiles"`, `"excluded_from_bulk":true`, `"managed":true`, `"scheme":"github"`} {
		if !strings.Contains(got, key) {
			t.Errorf("list repos JSON missing %s: %s", key, got)
		}
	}
	if strings.Contains(got, "dotfiles\"}") && !strings.Contains(got, `"fqn"`) {
		t.Errorf("listing must carry the full FQN, not the short display: %s", got)
	}
}

// TestInfoJSONSnakeKeys asserts info --json is a flat object keyed by the
// snake_case field name (O10), and that kebab tokens convert (managed-path →
// managed_path).
func TestInfoJSONSnakeKeys(t *testing.T) {
	scope := ops.InfoScope{
		FQN:  name.FQN{Scheme: "github", Coordinate: []string{"o", "n"}, Package: "zsh"},
		Kind: ops.ScopePackage,
		Fields: []ops.Field{
			{Name: "managed-path", Status: ops.FieldSet, Value: "/repo/zsh"},
			{Name: "translate", Status: ops.FieldSet, Value: true},
			{Name: "ignores", Status: ops.FieldUnset, Value: []string{}},
			{Name: "target", Status: ops.FieldUnset, Value: nil},
		},
	}
	got := marshal(t, infoJSON(&ops.InfoResult{Scopes: []ops.InfoScope{scope}}, false))
	if !strings.Contains(got, `"managed_path":"/repo/zsh"`) {
		t.Errorf("info JSON missing snake_case managed_path: %s", got)
	}
	if !strings.Contains(got, `"translate":true`) {
		t.Errorf("info JSON missing native bool: %s", got)
	}
	if !strings.Contains(got, `"ignores":[]`) {
		t.Errorf("empty list should marshal as []: %s", got)
	}
	if !strings.Contains(got, `"target":null`) {
		t.Errorf("unset scalar should marshal as null: %s", got)
	}
}

// TestInfoJSONRecurseAttributes asserts --recurse yields an array of scope
// objects, each attributed by qualified_name (§2.4, O10).
func TestInfoJSONRecurseAttributes(t *testing.T) {
	res := &ops.InfoResult{Scopes: []ops.InfoScope{
		{FQN: name.FQN{Scheme: "github", Coordinate: []string{"o", "n"}}, Kind: ops.ScopeRepo,
			Fields: []ops.Field{{Name: "target", Status: ops.FieldSet, Value: "/home"}}},
		{FQN: name.FQN{Scheme: "github", Coordinate: []string{"o", "n"}, Package: "zsh"}, Kind: ops.ScopePackage,
			Fields: []ops.Field{{Name: "target", Status: ops.FieldSet, Value: "/home"}}},
	}}
	got := marshal(t, infoJSON(res, true))
	if !strings.HasPrefix(got, "[") {
		t.Errorf("recurse JSON must be an array: %s", got)
	}
	if !strings.Contains(got, `"qualified_name":"github:o/n"`) || !strings.Contains(got, `"qualified_name":"github:o/n::zsh"`) {
		t.Errorf("each scope object must carry qualified_name attribution: %s", got)
	}
}

// TestStatusJSONStateStringsVerbatim asserts CONTEXT.md state strings ride the
// JSON verbatim — including the space in "partially stowed" (O10).
func TestStatusJSONStateStringsVerbatim(t *testing.T) {
	res := &ops.StatusResult{Packages: []ops.PackageStatusResult{{
		FQN:   name.FQN{Scheme: "github", Coordinate: []string{"o", "n"}, Package: "zsh"},
		State: ops.StatePartiallyStowed,
		Links: []ops.LinkStatus{{Link: ".zshrc", Source: "dot-zshrc", State: ops.LinkStowed}},
	}}}
	got := marshal(t, statusJSON(res))
	if !strings.Contains(got, `"state":"partially stowed"`) {
		t.Errorf("status JSON must spell the state verbatim (with the space): %s", got)
	}
	if !strings.Contains(got, `"fqn":"github:o/n::zsh"`) {
		t.Errorf("status JSON must carry the full FQN: %s", got)
	}
}

// TestCheckJSONClassStrings asserts the class strings ride the JSON verbatim.
func TestCheckJSONClassStrings(t *testing.T) {
	rep := &ops.CheckReport{Findings: []ops.Finding{{
		TargetRoot: "/home", Class: ops.ClassOrphaned, Evidence: "e",
		Entry: ledger.Entry{Link: ".x", Package: "github:o/n::zsh", Source: "x", Destination: "d"},
	}}}
	got := marshal(t, checkJSON(rep))
	if !strings.Contains(got, `"class":"orphaned"`) {
		t.Errorf("check JSON class verbatim: %s", got)
	}
	if !strings.Contains(got, `"package":"github:o/n::zsh"`) {
		t.Errorf("check JSON must carry the package FQN: %s", got)
	}
}

// TestJSONShapeKeysDocumented guards finding C3: every key the per-command
// shapes emit must be enumerated in reference/index.md's "Per-command shapes"
// section, so an agent scripting --json never has to run a command to discover
// its fields. json.go is the single owner of the shapes, so reflecting over its
// element structs (plus the top-level wrapper keys, which live in anonymous
// return structs) is the authoritative key set — the same anti-drift stance the
// slots table guard takes. info's shape is field-keyed (the Fields table), not
// struct-keyed, so it has no struct here.
func TestJSONShapeKeysDocumented(t *testing.T) {
	raw, err := fs.ReadFile(dstow.Manual, "docs/reference/index.md")
	if err != nil {
		t.Fatalf("read reference/index.md: %v", err)
	}
	doc := string(raw)
	const heading = "### Per-command shapes"
	start := strings.Index(doc, heading)
	if start < 0 {
		t.Fatalf("reference/index.md has no %q section", heading)
	}
	section := doc[start:]
	if next := strings.Index(section[len(heading):], "\n## "); next >= 0 {
		section = section[:len(heading)+next]
	}

	// Element structs json.go marshals; every json tag key must appear.
	elems := []reflect.Type{
		reflect.TypeOf(jsonRepoRow{}), reflect.TypeOf(jsonPackageRow{}),
		reflect.TypeOf(jsonLinkStatus{}), reflect.TypeOf(jsonPackageStatus{}),
		reflect.TypeOf(jsonRepoSync{}), reflect.TypeOf(jsonCandidate{}),
		reflect.TypeOf(jsonPathStatus{}), reflect.TypeOf(jsonFinding{}),
		reflect.TypeOf(jsonSlot{}),
	}
	keys := map[string]bool{}
	for _, ty := range elems {
		for i := 0; i < ty.NumField(); i++ {
			tag := ty.Field(i).Tag.Get("json")
			if tag == "" || tag == "-" {
				continue
			}
			keys[strings.Split(tag, ",")[0]] = true
		}
	}
	// Top-level wrapper keys, which live in anonymous return structs.
	for _, k := range []string{"repos", "packages", "scope", "package", "paths", "path", "findings", "qualified_name"} {
		keys[k] = true
	}

	for k := range keys {
		if !strings.Contains(section, "\""+k+"\"") {
			t.Errorf("JSON key %q is emitted by json.go but not documented in reference/index.md \"Per-command shapes\"", k)
		}
	}
}
