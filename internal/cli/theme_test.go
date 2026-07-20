package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// theme list enumerates the six bundled presets, all origin "bundled", none
// active in a fresh HOME.
func TestThemeList(t *testing.T) {
	isolateXDG(t)
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
	isolateXDG(t)
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

// Bare theme emit renders the effective stack: fourteen slot lines in
// canonical §3.3 order — tier-2s filled by derivation (§7.3), so the composed
// truth is complete — plain under NO_COLOR (O11-style strip stability by
// construction).
func TestThemeEmitEffectiveRendered(t *testing.T) {
	isolateXDG(t)
	out, _, code := run(t, "theme", "emit")
	if code != 0 {
		t.Fatalf("theme emit exit = %d", code)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 14 {
		t.Fatalf("theme emit printed %d rows, want 14:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "section1") || !strings.HasPrefix(lines[13], "info2") {
		t.Errorf("rows not in canonical §3.3 order:\n%s", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("NO_COLOR output carries ANSI escapes:\n%q", out)
	}
}

// theme emit <name> shows the theme as loaded (declared slots only), and the
// env emission round-trips the old converter path.
func TestThemeEmitNamed(t *testing.T) {
	isolateXDG(t)
	out, _, code := run(t, "theme", "emit", "cargo")
	if code != 0 {
		t.Fatalf("theme emit cargo exit = %d", code)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 8 {
		t.Fatalf("cargo declares 8 slots, rendered %d:\n%s", len(lines), out)
	}

	env, _, code := run(t, "theme", "emit", "cargo", "--format", "env")
	if code != 0 {
		t.Fatalf("--format env exit = %d", code)
	}
	if !strings.Contains(env, "name1=bold brightcyan") {
		t.Errorf("env emission missing cargo's name1 slot: %q", env)
	}
}

// slot=value operands override on top; toml emission carries them.
func TestThemeEmitOverrides(t *testing.T) {
	isolateXDG(t)
	out, _, code := run(t, "theme", "emit", "cargo", "section1=bold yellow", "--format", "toml")
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
func TestThemeEmitErrors(t *testing.T) {
	isolateXDG(t)
	if _, _, code := run(t, "theme", "emit", "bogus_slot=red"); code != 2 {
		t.Errorf("unknown slot exit = %d, want 2", code)
	}
	if _, _, code := run(t, "theme", "emit", "success2=notacolor"); code != 2 {
		t.Errorf("bad value exit = %d, want 2", code)
	}
	if _, _, code := run(t, "theme", "emit", "cargo", "catppuccin-mocha"); code != 2 {
		t.Errorf("two refs exit = %d, want 2", code)
	}
	if _, errs, code := run(t, "theme", "emit", "no-such-theme"); code != 1 {
		t.Errorf("unknown theme exit = %d, want 1 (stderr: %s)", code, errs)
	}
}

// theme slots prints all fourteen slots in canonical §3.3 order on stdout, with
// descriptions sourced from the code-owned Role mapping. The column header is
// commentary: stderr, never stdout (O1).
func TestThemeSlots(t *testing.T) {
	isolateXDG(t)
	out, errs, code := run(t, "theme", "slots")
	if code != 0 {
		t.Fatalf("theme slots exit = %d", code)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 14 {
		t.Fatalf("theme slots printed %d rows, want 14:\n%s", len(lines), out)
	}
	want := []string{
		"section1", "section2", "name1", "name2", "value1", "value2",
		"error1", "error2", "warning1", "warning2", "success1", "success2", "info1", "info2",
	}
	for i, w := range want {
		if !strings.HasPrefix(lines[i], w) {
			t.Errorf("row %d = %q, want the %q slot (canonical §3.3 order)", i, lines[i], w)
		}
	}
	// The description enumerates the slot's state consumers, from the mapping.
	var error1Line string
	for _, ln := range lines {
		if strings.HasPrefix(ln, "error1") {
			error1Line = ln
		}
	}
	if !strings.Contains(error1Line, "damaged") || !strings.Contains(error1Line, "contradicted") {
		t.Errorf("error1 row omits its state consumers: %q", error1Line)
	}

	// The header is commentary: stderr, never stdout (O1).
	if !strings.Contains(errs, "Slot") || !strings.Contains(errs, "Description") {
		t.Errorf("stderr missing the two-column header: %q", errs)
	}
	if strings.Contains(out, "Description") {
		t.Errorf("header leaked onto stdout:\n%s", out)
	}
}

// --quiet drops the header (O7); the fourteen data rows survive unchanged.
func TestThemeSlotsQuiet(t *testing.T) {
	isolateXDG(t)
	out, _, _ := run(t, "theme", "slots")
	qout, qerrs, code := run(t, "-q", "theme", "slots")
	if code != 0 {
		t.Fatalf("theme slots -q exit = %d", code)
	}
	if strings.Contains(qerrs, "Slot") {
		t.Errorf("--quiet should drop the header: %q", qerrs)
	}
	if qout != out {
		t.Errorf("--quiet changed the data rows")
	}
}

// The slot names render through their own effective style when color is forced:
// section1's default is bold brightgreen, so its name is styled 1;92.
func TestThemeSlotsColorized(t *testing.T) {
	isolateXDG(t)
	cout, _, code := run(t, "--color", "always", "theme", "slots")
	if code != 0 {
		t.Fatalf("theme slots exit = %d", code)
	}
	if !strings.Contains(cout, "\x1b[1;92msection1\x1b[") {
		t.Errorf("section1 name missing its own-style swatch:\n%q", cout)
	}
}

// --json emits a per-slot object array: slot, description, and the derived
// consumer list; all fourteen present, error1 carries its state consumers, and
// a slot no internal consumes carries an empty consumers array.
func TestThemeSlotsJSON(t *testing.T) {
	isolateXDG(t)
	out, _, code := run(t, "theme", "slots", "--json")
	if code != 0 {
		t.Fatalf("theme slots --json exit = %d", code)
	}
	var rows []struct {
		Slot        string   `json:"slot"`
		Description string   `json:"description"`
		Consumers   []string `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("theme slots --json is not valid JSON: %v\n%s", err, out)
	}
	if len(rows) != 14 {
		t.Fatalf("--json has %d slots, want 14", len(rows))
	}
	byName := map[string][]string{}
	for _, r := range rows {
		byName[r.Slot] = r.Consumers
		if r.Consumers == nil {
			t.Errorf("slot %q consumers marshaled as null, want []", r.Slot)
		}
	}
	for _, want := range []string{"damaged", "contradicted", "error"} {
		if !contains(byName["error1"], want) {
			t.Errorf("error1 consumers %v missing %q", byName["error1"], want)
		}
	}
	if len(byName["section2"]) != 0 {
		t.Errorf("section2 consumers = %v, want empty", byName["section2"])
	}
}
