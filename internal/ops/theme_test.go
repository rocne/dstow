package ops_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/ui"
)

// theme emit <name> --format env resolves a bundled preset and packs it: a
// complete preset yields all fourteen slots, ':'-joined, parseable back.
func TestThemeEmitEnv(t *testing.T) {
	app := &ops.App{}
	res, err := app.ThemeEmit("catppuccin-mocha", nil, nil, ops.ColorFormatEnv)
	if err != nil {
		t.Fatalf("ThemeEmit: %v", err)
	}
	if res.Ref != "catppuccin-mocha" || res.Format != ops.ColorFormatEnv {
		t.Errorf("result metadata = %q/%v", res.Ref, res.Format)
	}
	entries := strings.Split(res.Text, ":")
	if len(entries) != 14 {
		t.Fatalf("packed theme has %d entries, want 14: %q", len(entries), res.Text)
	}
	// The packed string parses back to a full fourteen-slot theme.
	theme, warns := ui.ParseDSTOWColors(res.Text)
	if len(warns) != 0 {
		t.Fatalf("parsing our own packed output warned: %v", warns)
	}
	if len(theme) != 14 {
		t.Errorf("parsed theme has %d slots, want 14", len(theme))
	}
}

// theme emit <name> --format toml emits the theme-file schema, reloadable.
func TestThemeEmitTOML(t *testing.T) {
	app := &ops.App{}
	res, err := app.ThemeEmit("catppuccin-mocha", nil, nil, ops.ColorFormatTOML)
	if err != nil {
		t.Fatalf("ThemeEmit: %v", err)
	}
	if !strings.Contains(res.Text, "section1 = ") || !strings.Contains(res.Text, "info2 = ") {
		t.Errorf("TOML output missing expected slot lines:\n%s", res.Text)
	}
	// Written to a file and reloaded, it is a valid theme resolving to 14 slots.
	dir := t.TempDir()
	path := filepath.Join(dir, "mine.toml")
	if err := os.WriteFile(path, []byte(res.Text), 0o644); err != nil {
		t.Fatal(err)
	}
	theme, warns, err := ui.LoadTheme(path, "")
	if err != nil {
		t.Fatalf("reload emitted theme: %v", err)
	}
	if len(warns) != 0 {
		t.Fatalf("reload warned: %v", warns)
	}
	if len(theme) != 14 {
		t.Errorf("reloaded theme has %d slots, want 14", len(theme))
	}
}

// A bare theme emit renders the effective stack the caller composed; the
// default rendered format serializes no Text (cli styles res.Theme itself).
func TestThemeEmitEffective(t *testing.T) {
	app := &ops.App{}
	res, err := app.ThemeEmit("", ui.DeriveTiers(ui.DefaultPalette()), nil, ops.ColorFormatRendered)
	if err != nil {
		t.Fatalf("ThemeEmit: %v", err)
	}
	if len(res.Theme) != 14 {
		t.Errorf("effective theme has %d slots, want 14", len(res.Theme))
	}
	if res.Text != "" {
		t.Errorf("rendered format must not serialize Text, got %q", res.Text)
	}
}

// Overrides layer on top of the resolved base — the top of the stack.
func TestThemeEmitOverrides(t *testing.T) {
	over, warns := ui.ParseDSTOWColors("success2=red")
	if len(warns) != 0 {
		t.Fatal(warns)
	}
	app := &ops.App{}
	res, err := app.ThemeEmit("catppuccin-mocha", nil, over, ops.ColorFormatEnv)
	if err != nil {
		t.Fatalf("ThemeEmit: %v", err)
	}
	if !strings.Contains(res.Text, "success2=red") {
		t.Errorf("override lost: %q", res.Text)
	}
}

// An unresolvable theme name refuses (error), naming the remedy in ui's terms.
func TestThemeEmitNotFound(t *testing.T) {
	app := &ops.App{}
	if _, err := app.ThemeEmit("no-such-theme", nil, nil, ops.ColorFormatEnv); err == nil {
		t.Fatal("expected an error for an unknown theme")
	}
}

// ThemeSlots returns all fourteen slots in canonical §3.3 order, each with its
// derived consumer list. The consumers come from ui's code-owned Role mapping,
// so error1 must carry damaged and contradicted (the states that share it) and
// section1 must carry no state (heading is a prose role, folded into the gloss).
func TestThemeSlots(t *testing.T) {
	app := &ops.App{}
	res := app.ThemeSlots()
	if len(res.Rows) != 14 {
		t.Fatalf("slot reference has %d rows, want 14", len(res.Rows))
	}
	want := []string{
		"section1", "section2", "name1", "name2", "value1", "value2",
		"error1", "error2", "warning1", "warning2", "success1", "success2", "info1", "info2",
	}
	for i, w := range want {
		if res.Rows[i].Slot != w {
			t.Errorf("row %d = %q, want %q (canonical §3.3 order)", i, res.Rows[i].Slot, w)
		}
	}

	byName := map[string]ops.ThemeSlotRow{}
	for _, r := range res.Rows {
		byName[r.Slot] = r
	}
	// error1's consumers are derived from roleSlot: error, damaged, contradicted.
	if !hasAll(byName["error1"].Consumers, "damaged", "contradicted", "error") {
		t.Errorf("error1 consumers = %v, want error/damaged/contradicted", byName["error1"].Consumers)
	}
	// The description enumerates the state/class consumers verbatim.
	if !strings.Contains(byName["error1"].Description, "damaged") ||
		!strings.Contains(byName["error1"].Description, "contradicted") {
		t.Errorf("error1 description omits its state consumers: %q", byName["error1"].Description)
	}
	// A slot no internal consumes has an empty consumer list, never nil.
	if s2 := byName["section2"]; len(s2.Consumers) != 0 || s2.Consumers == nil {
		t.Errorf("section2 consumers = %#v, want an empty non-nil slice", s2.Consumers)
	}
	// A tier-2 slot notes its derivation (§7.3).
	if !strings.Contains(byName["error2"].Description, "derives from error1") {
		t.Errorf("error2 description omits the tier-derivation note: %q", byName["error2"].Description)
	}
}

// hasAll reports whether every want string is present in got.
func hasAll(got []string, want ...string) bool {
	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

// ThemeList enumerates the bundled roster with origins; no Global means no
// active row. XDG is isolated so a developer's real themes dir can't leak in.
func TestThemeList(t *testing.T) {
	setupXDG(t)
	app := &ops.App{}
	res := app.ThemeList()
	byName := map[string]ops.ThemeRow{}
	for _, r := range res.Rows {
		byName[r.Name] = r
		if r.Active {
			t.Errorf("no theme configured, but %q is marked active", r.Name)
		}
	}
	for _, want := range []string{"cargo", "fang-ansi", "catppuccin-mocha"} {
		row, ok := byName[want]
		if !ok {
			t.Fatalf("roster missing %q: %v", want, res.Rows)
		}
		if !row.Bundled || row.User {
			t.Errorf("%q origins = bundled:%v user:%v, want bundled only", want, row.Bundled, row.User)
		}
	}
}
