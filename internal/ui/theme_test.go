package ui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fatih/color"
)

// --- §7.3 DSTOW_COLORS packed string -----------------------------------------

func TestParseDSTOWColors(t *testing.T) {
	theme, warns := ParseDSTOWColors("error1=bold red:success2=#a6e3a1")
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	if got := theme[SlotError1].params; !reflect.DeepEqual(got, []color.Attribute{color.Bold, 31}) {
		t.Errorf("error1 = %v, want bold red", got)
	}
	if got := theme[SlotSuccess2].params; !reflect.DeepEqual(got, []color.Attribute{38, 2, 0xa6, 0xe3, 0xa1}) {
		t.Errorf("success2 = %v, want #a6e3a1", got)
	}
}

func TestParseDSTOWColorsEmptyEntriesSkipped(t *testing.T) {
	theme, warns := ParseDSTOWColors(":success2=green::warning1=yellow:")
	if len(warns) != 0 {
		t.Fatalf("empty entries must be skipped silently, got warnings: %v", warns)
	}
	if len(theme) != 2 {
		t.Errorf("want 2 slots parsed, got %d", len(theme))
	}
}

// Warn-and-skip (C18): an unknown slot warns with Source DSTOW_COLORS, but the
// rest of the string still applies.
func TestParseDSTOWColorsUnknownSlotWarnsAndContinues(t *testing.T) {
	theme, warns := ParseDSTOWColors("bogus=green:success2=green")
	if len(warns) != 1 {
		t.Fatalf("want 1 warning, got %d: %v", len(warns), warns)
	}
	if warns[0].Source != "DSTOW_COLORS" {
		t.Errorf("warning Source = %q, want DSTOW_COLORS", warns[0].Source)
	}
	if _, ok := theme[SlotSuccess2]; !ok {
		t.Error("remainder should still apply: success2 missing")
	}
}

// A bad value warns and is skipped; the remainder still applies.
func TestParseDSTOWColorsBadValueWarnsAndContinues(t *testing.T) {
	theme, warns := ParseDSTOWColors("success2=chartreuse:warning1=yellow")
	if len(warns) != 1 {
		t.Fatalf("want 1 warning, got %d: %v", len(warns), warns)
	}
	if _, ok := theme[SlotSuccess2]; ok {
		t.Error("bad value must be skipped, success2 should be absent")
	}
	if _, ok := theme[SlotWarning1]; !ok {
		t.Error("remainder should still apply: warning1 missing")
	}
}

func TestParseDSTOWColorsMalformedEntryWarns(t *testing.T) {
	_, warns := ParseDSTOWColors("success2green:warning1=yellow")
	if len(warns) != 1 {
		t.Fatalf("want 1 warning for a malformed entry, got %d: %v", len(warns), warns)
	}
}

// --- §3.3 [color] table ------------------------------------------------------

func TestParseColorTable(t *testing.T) {
	theme, warns := ParseColorTable(map[string]string{
		"success2": "green",
		"warning1": "yellow",
	})
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	if len(theme) != 2 {
		t.Errorf("want 2 slots, got %d", len(theme))
	}
}

func TestParseColorTableWarnAndSkip(t *testing.T) {
	theme, warns := ParseColorTable(map[string]string{
		"success2":   "green", // valid
		"not_a_slot": "green", // unknown slot
		"warning1":   "bogus", // bad value
	})
	if len(warns) != 2 {
		t.Fatalf("want 2 warnings (unknown slot + bad value), got %d: %v", len(warns), warns)
	}
	if _, ok := theme[SlotSuccess2]; !ok {
		t.Error("valid entry should survive")
	}
	if len(theme) != 1 {
		t.Errorf("only the valid entry should be applied, got %d", len(theme))
	}
}

// The closed fourteen-key set: an empty value is skipped silently.
func TestParseColorTableEmptyValueSkipped(t *testing.T) {
	theme, warns := ParseColorTable(map[string]string{"success2": ""})
	if len(warns) != 0 || len(theme) != 0 {
		t.Errorf("empty value must be skipped silently, got theme=%v warns=%v", theme, warns)
	}
}

// --- §7.3 ComposeTheme: top wins, absent slots fall through ------------------

func TestComposeThemePrecedence(t *testing.T) {
	env := Theme{SlotSuccess2: mustStyle("red")}
	table := Theme{SlotSuccess2: mustStyle("green"), SlotWarning1: mustStyle("green")}
	theme := Theme{SlotWarning1: mustStyle("blue"), SlotError2: mustStyle("cyan")}
	def := DefaultPalette()

	got := ComposeTheme(env, table, theme, def)

	// env beats table beats theme for success2.
	if !reflect.DeepEqual(got[SlotSuccess2].params, []color.Attribute{31}) {
		t.Errorf("success2 = %v, want env's red (31)", got[SlotSuccess2].params)
	}
	// table beats theme for warning1.
	if !reflect.DeepEqual(got[SlotWarning1].params, []color.Attribute{32}) {
		t.Errorf("warning1 = %v, want table's green (32)", got[SlotWarning1].params)
	}
	// theme wins where earlier layers are silent.
	if !reflect.DeepEqual(got[SlotError2].params, []color.Attribute{36}) {
		t.Errorf("error2 = %v, want theme's cyan (36)", got[SlotError2].params)
	}
	// A slot no layer overrides falls through to the default palette.
	if !reflect.DeepEqual(got[SlotName1].params, DefaultPalette()[SlotName1].params) {
		t.Errorf("name1 = %v, want the default palette value", got[SlotName1].params)
	}
}

// --- §7.3 / C4 / C8 LoadTheme ------------------------------------------------

func writeTheme(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadThemeUserHit(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "mine", "success2 = \"blue\"\n")
	theme, warns, err := LoadTheme("mine", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	if !reflect.DeepEqual(theme[SlotSuccess2].params, []color.Attribute{34}) {
		t.Errorf("success2 = %v, want blue", theme[SlotSuccess2].params)
	}
}

func TestLoadThemeBundledHit(t *testing.T) {
	theme, warns, err := LoadTheme("catppuccin-mocha", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("bundled preset must load warning-free, got: %v", warns)
	}
	// success2 = "#a6e3a1" per the shipped preset.
	if !reflect.DeepEqual(theme[SlotSuccess2].params, []color.Attribute{38, 2, 0xa6, 0xe3, 0xa1}) {
		t.Errorf("success2 = %v, want #a6e3a1", theme[SlotSuccess2].params)
	}
}

// C4: a user preset shadows the bundled one on a name collision.
func TestLoadThemeUserShadowsBundled(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "catppuccin-mocha", "success2 = \"red\"\n")
	theme, _, err := LoadTheme("catppuccin-mocha", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(theme[SlotSuccess2].params, []color.Attribute{31}) {
		t.Errorf("success2 = %v, want the user file's red (31), not the bundled hex", theme[SlotSuccess2].params)
	}
}

// C8: a path form is read as a file anywhere.
func TestLoadThemePathForm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repo-theme.toml")
	if err := os.WriteFile(path, []byte("section1 = \"bold\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	theme, _, err := LoadTheme(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(theme[SlotSection1].params, []color.Attribute{color.Bold}) {
		t.Errorf("section1 = %v, want bold", theme[SlotSection1].params)
	}
}

// Only ABSENCE of a user theme falls through to the bundled presets: any other
// read failure (e.g. permissions) on the user's own file must surface — never
// silently shadow-in the bundled preset.
func TestLoadThemeUnreadableUserFileSurfaces(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not bind root")
	}
	dir := t.TempDir()
	writeTheme(t, dir, "catppuccin-mocha", "success2 = \"blue\"\n")
	if err := os.Chmod(filepath.Join(dir, "catppuccin-mocha.toml"), 0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadTheme("catppuccin-mocha", dir); err == nil {
		t.Fatal("LoadTheme() = nil error, want the read failure surfaced (not a fall-through to the bundled preset)")
	}
}

// A bare name that resolves nowhere is a *ThemeNotFoundError naming the ref,
// both locations, and the available names.
func TestLoadThemeNotFound(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "userpreset", "success2 = \"red\"\n")
	_, _, err := LoadTheme("nope", dir)
	if err == nil {
		t.Fatal("want an error for an unresolved name")
	}
	var nf *ThemeNotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("error is %T, want *ThemeNotFoundError", err)
	}
	msg := err.Error()
	if !contains(msg, "nope") {
		t.Errorf("message should name the ref: %q", msg)
	}
	if !contains(msg, dir) {
		t.Errorf("message should name the user themes dir: %q", msg)
	}
	if !contains(msg, "catppuccin-mocha") || !contains(msg, "userpreset") {
		t.Errorf("message should list available themes (bundled + user): %q", msg)
	}
}

// Unknown keys in a theme file warn via md.Undecoded() and are skipped; the
// known slots still apply.
func TestLoadThemeUnknownKeyWarns(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "extra", "success2 = \"green\"\nbogus_key = \"red\"\n")
	theme, warns, err := LoadTheme("extra", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 {
		t.Fatalf("want 1 unknown-key warning, got %d: %v", len(warns), warns)
	}
	if !contains(warns[0].Detail, "bogus_key") {
		t.Errorf("warning should name the unknown key: %q", warns[0].Detail)
	}
	if _, ok := theme[SlotSuccess2]; !ok {
		t.Error("known slot should still apply")
	}
}

// A bad value in a theme file warns and is skipped.
func TestLoadThemeBadValueWarns(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "bad", "success2 = \"chartreuse\"\nwarning1 = \"yellow\"\n")
	theme, warns, err := LoadTheme("bad", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 {
		t.Fatalf("want 1 bad-value warning, got %d: %v", len(warns), warns)
	}
	if _, ok := theme[SlotSuccess2]; ok {
		t.Error("bad value should be skipped")
	}
	if _, ok := theme[SlotWarning1]; !ok {
		t.Error("remainder should still apply")
	}
}

// Every embedded preset round-trips through the loader and parses
// warning-free, exercising the same loader path as user files (A5). The four
// Whiskers-generated catppuccin flavors (#105) declare all fourteen slots; the
// hand-vendored prior-art presets (cargo, fang-ansi) declare only the slots
// their source specifies — absent slots fall through the §7.3 stack by design.
func TestBundledPresetsRoundTrip(t *testing.T) {
	minSlots := map[string]int{"cargo": 8, "fang-ansi": 4}
	for _, name := range BundledThemes() {
		theme, warns, err := LoadTheme(name, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(warns) != 0 {
			t.Fatalf("%s must parse warning-free, got: %v", name, warns)
		}
		want, partial := minSlots[name]
		if !partial {
			want = 14
		}
		if len(theme) != want {
			t.Errorf("%s has %d slots, want %d", name, len(theme), want)
		}
	}
}

func TestBundledThemes(t *testing.T) {
	got := BundledThemes()
	want := []string{"cargo", "catppuccin-frappe", "catppuccin-latte", "catppuccin-macchiato", "catppuccin-mocha", "fang-ansi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BundledThemes() = %v, want %v", got, want)
	}
}
