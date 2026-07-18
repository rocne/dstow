package ops_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/ui"
)

// colors theme resolves a bundled preset and packs it (default env form): a
// complete preset yields all sixteen slots, ':'-joined, parseable back.
func TestColorsThemeEnv(t *testing.T) {
	app := &ops.App{}
	res, err := app.ColorsTheme("catppuccin-mocha", ops.ColorFormatEnv)
	if err != nil {
		t.Fatalf("ColorsTheme: %v", err)
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

// colors theme --format toml emits the theme-file schema, reloadable.
func TestColorsThemeTOML(t *testing.T) {
	app := &ops.App{}
	res, err := app.ColorsTheme("catppuccin-mocha", ops.ColorFormatTOML)
	if err != nil {
		t.Fatalf("ColorsTheme: %v", err)
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

// An unresolvable theme name refuses (error), naming the remedy in ui's terms.
func TestColorsThemeNotFound(t *testing.T) {
	app := &ops.App{}
	if _, err := app.ColorsTheme("no-such-theme", ops.ColorFormatEnv); err == nil {
		t.Fatal("expected an error for an unknown theme")
	}
}
