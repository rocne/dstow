package ops_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/ui"
)

// theme show <name> --format env resolves a bundled preset and packs it: a
// complete preset yields all sixteen slots, ':'-joined, parseable back.
func TestThemeShowEnv(t *testing.T) {
	app := &ops.App{}
	res, err := app.ThemeShow("catppuccin-mocha", nil, nil, ops.ColorFormatEnv)
	if err != nil {
		t.Fatalf("ThemeShow: %v", err)
	}
	if res.Ref != "catppuccin-mocha" || res.Format != ops.ColorFormatEnv {
		t.Errorf("result metadata = %q/%v", res.Ref, res.Format)
	}
	entries := strings.Split(res.Text, ":")
	if len(entries) != 16 {
		t.Fatalf("packed theme has %d entries, want 16: %q", len(entries), res.Text)
	}
	// The packed string parses back to a full sixteen-slot theme.
	theme, warns := ui.ParseDSTOWColors(res.Text)
	if len(warns) != 0 {
		t.Fatalf("parsing our own packed output warned: %v", warns)
	}
	if len(theme) != 16 {
		t.Errorf("parsed theme has %d slots, want 16", len(theme))
	}
}

// theme show <name> --format toml emits the theme-file schema, reloadable.
func TestThemeShowTOML(t *testing.T) {
	app := &ops.App{}
	res, err := app.ThemeShow("catppuccin-mocha", nil, nil, ops.ColorFormatTOML)
	if err != nil {
		t.Fatalf("ThemeShow: %v", err)
	}
	if !strings.Contains(res.Text, "stowed = ") || !strings.Contains(res.Text, "muted = ") {
		t.Errorf("TOML output missing expected slot lines:\n%s", res.Text)
	}
	// Written to a file and reloaded, it is a valid theme resolving to 16 slots.
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
	if len(theme) != 16 {
		t.Errorf("reloaded theme has %d slots, want 16", len(theme))
	}
}

// A bare theme show renders the effective stack the caller composed; the
// default rendered format serializes no Text (cli styles res.Theme itself).
func TestThemeShowEffective(t *testing.T) {
	app := &ops.App{}
	res, err := app.ThemeShow("", ui.DefaultPalette(), nil, ops.ColorFormatRendered)
	if err != nil {
		t.Fatalf("ThemeShow: %v", err)
	}
	if len(res.Theme) != 16 {
		t.Errorf("effective theme has %d slots, want 16", len(res.Theme))
	}
	if res.Text != "" {
		t.Errorf("rendered format must not serialize Text, got %q", res.Text)
	}
}

// Overrides layer on top of the resolved base — the top of the stack.
func TestThemeShowOverrides(t *testing.T) {
	over, warns := ui.ParseDSTOWColors("stowed=red")
	if len(warns) != 0 {
		t.Fatal(warns)
	}
	app := &ops.App{}
	res, err := app.ThemeShow("catppuccin-mocha", nil, over, ops.ColorFormatEnv)
	if err != nil {
		t.Fatalf("ThemeShow: %v", err)
	}
	if !strings.Contains(res.Text, "stowed=red") {
		t.Errorf("override lost: %q", res.Text)
	}
}

// An unresolvable theme name refuses (error), naming the remedy in ui's terms.
func TestThemeShowNotFound(t *testing.T) {
	app := &ops.App{}
	if _, err := app.ThemeShow("no-such-theme", nil, nil, ops.ColorFormatEnv); err == nil {
		t.Fatal("expected an error for an unknown theme")
	}
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
