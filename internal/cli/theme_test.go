package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"
)

// isolateThemeXDG is isolateXDG plus an xdg cache reload (the paths_test.go
// pattern): the theme verbs resolve real paths through adrg/xdg, which
// snapshots the environment, so t.Setenv alone would leave them pointed at
// the developer's real config.
func isolateThemeXDG(t *testing.T) {
	t.Helper()
	isolateXDG(t)
	xdg.Reload()
	t.Cleanup(xdg.Reload)
}

// theme list enumerates the six bundled presets, all origin "bundled", none
// active in a fresh HOME.
func TestThemeList(t *testing.T) {
	isolateThemeXDG(t)
	out, errs, code := run(t, "theme", "list")
	if code != 0 {
		t.Fatalf("theme list exit = %d", code)
	}
	// The header is commentary: stderr, never stdout (O1).
	if !strings.Contains(errs, "Theme Name") || !strings.Contains(errs, "Source") {
		t.Errorf("stderr missing the header: %q", errs)
	}
	if strings.Contains(out, "Theme Name") {
		t.Errorf("header leaked onto stdout:\n%s", out)
	}

	// --quiet drops the header (O7); rows survive.
	qout, qerrs, _ := run(t, "-q", "theme", "list")
	if strings.Contains(qerrs, "Theme Name") {
		t.Errorf("--quiet should drop the header: %q", qerrs)
	}
	if qout != out {
		t.Errorf("--quiet changed the data rows")
	}

	// Names render through the name slot when color is forced (beats NO_COLOR).
	cout, _, _ := run(t, "--color", "always", "theme", "list")
	if !strings.Contains(cout, "\x1b[1;96mcargo\x1b[") {
		t.Errorf("colorized names missing the name slot styling:\n%q", cout)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 6 {
		t.Fatalf("theme list printed %d rows, want 6:\n%s", len(lines), out)
	}
	for _, want := range []string{"cargo", "catppuccin-mocha", "fang-ansi"} {
		if !strings.Contains(out, want) {
			t.Errorf("theme list missing %q:\n%s", want, out)
		}
	}
	for _, line := range lines {
		if !strings.Contains(line, "bundled") {
			t.Errorf("fresh HOME row should be origin bundled: %q", line)
		}
		if strings.Contains(line, "(active)") {
			t.Errorf("no theme configured, but a row is active: %q", line)
		}
	}
}

// A user theme file appears in the roster; a name collision reads as shadowing
// (C4); the global theme key marks its row active.
func TestThemeListUserShadowActive(t *testing.T) {
	isolateThemeXDG(t)
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "dstow")
	themesDir := filepath.Join(cfgDir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mine", "catppuccin-mocha"} {
		if err := os.WriteFile(filepath.Join(themesDir, name+".toml"), []byte("stowed = \"red\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("theme = \"mine\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, code := run(t, "theme", "list")
	if code != 0 {
		t.Fatalf("theme list exit = %d", code)
	}
	var mineLine, mochaLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "mine") {
			mineLine = line
		}
		if strings.HasPrefix(line, "catppuccin-mocha") {
			mochaLine = line
		}
	}
	if !strings.Contains(mineLine, "user") || !strings.Contains(mineLine, "(active)") {
		t.Errorf("mine row should be user + active: %q", mineLine)
	}
	if !strings.Contains(mochaLine, "user (shadows bundled)") {
		t.Errorf("catppuccin-mocha row should read as shadowing: %q", mochaLine)
	}
}

// Bare theme show renders the effective stack: fourteen slot lines in
// canonical §3.3 order — tier-2s filled by derivation (§7.3), so the composed
// truth is complete — plain under NO_COLOR (O11-style strip stability by
// construction).
func TestThemeShowEffectiveRendered(t *testing.T) {
	isolateThemeXDG(t)
	out, _, code := run(t, "theme", "show")
	if code != 0 {
		t.Fatalf("theme show exit = %d", code)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 14 {
		t.Fatalf("theme show printed %d rows, want 14:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "section1") || !strings.HasPrefix(lines[13], "info2") {
		t.Errorf("rows not in canonical §3.3 order:\n%s", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("NO_COLOR output carries ANSI escapes:\n%q", out)
	}
}

// theme show <name> shows the theme as loaded (declared slots only), and the
// env emission round-trips the old converter path.
func TestThemeShowNamed(t *testing.T) {
	isolateThemeXDG(t)
	out, _, code := run(t, "theme", "show", "cargo")
	if code != 0 {
		t.Fatalf("theme show cargo exit = %d", code)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 8 {
		t.Fatalf("cargo declares 8 slots, rendered %d:\n%s", len(lines), out)
	}

	env, _, code := run(t, "theme", "show", "cargo", "--format", "env")
	if code != 0 {
		t.Fatalf("--format env exit = %d", code)
	}
	if !strings.Contains(env, "name1=bold brightcyan") {
		t.Errorf("env emission missing cargo's name1 slot: %q", env)
	}
}

// slot=value operands override on top; toml emission carries them.
func TestThemeShowOverrides(t *testing.T) {
	isolateThemeXDG(t)
	out, _, code := run(t, "theme", "show", "cargo", "section1=bold yellow", "--format", "toml")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "section1 = \"bold yellow\"") {
		t.Errorf("override lost in toml emission:\n%s", out)
	}
}

// Operand mistakes are usage errors (exit 2): a bad slot, a bad value, two
// bare refs. An unknown theme is a not-found refusal (exit 1, per the #47
// ruling: exit 2 is reserved for malformed invocation).
func TestThemeShowErrors(t *testing.T) {
	isolateThemeXDG(t)
	if _, _, code := run(t, "theme", "show", "bogus_slot=red"); code != 2 {
		t.Errorf("unknown slot exit = %d, want 2", code)
	}
	if _, _, code := run(t, "theme", "show", "success2=notacolor"); code != 2 {
		t.Errorf("bad value exit = %d, want 2", code)
	}
	if _, _, code := run(t, "theme", "show", "cargo", "catppuccin-mocha"); code != 2 {
		t.Errorf("two refs exit = %d, want 2", code)
	}
	if _, errs, code := run(t, "theme", "show", "no-such-theme"); code != 1 {
		t.Errorf("unknown theme exit = %d, want 1 (stderr: %s)", code, errs)
	}
}
