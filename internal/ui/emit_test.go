package ui

import (
	"reflect"
	"strings"
	"testing"
)

// --- §7.3 EmitColorValue: canonical spellings ------------------------------

// The emitter corpus. Expected strings are the canonical grammar spellings the
// value grammar mandates (§7.3), named from the spec — never read back from the
// implementation. Inputs are the parsed Styles of representative values.
func TestEmitColorValueCanonical(t *testing.T) {
	tests := []struct {
		name string
		in   string // the grammar value to parse then emit
		want string // canonical emitted spelling
	}{
		{"basic fg", "red", "red"},
		{"bright fg", "brightred", "brightred"},
		{"default fg", "default", "default"},
		{"256 fg", "231", "231"},
		{"hex fg lowercased", "#A6E3A1", "#a6e3a1"},
		{"fg then bg", "red green", "red green"},
		{"bg alone needs normal fg", "normal green", "normal green"},
		{"bg default alone", "normal default", "normal default"},
		{"bg 256 needs normal fg", "normal 231", "normal 231"},
		{"attribute only", "bold", "bold"},
		{"attribute then color order kept", "bold red", "bold red"},
		{"color then attribute order kept", "red bold", "red bold"},
		{"reset leads", "reset bold red", "reset bold red"},
		{"every attribute", "bold dim italic ul blink reverse strike",
			"bold dim italic ul blink reverse strike"},
		{"empty is empty", "", ""},
		{"normal alone emits empty", "normal", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st, err := ParseColorValue(tc.in)
			if err != nil {
				t.Fatalf("ParseColorValue(%q): %v", tc.in, err)
			}
			got, err := EmitColorValue(st)
			if err != nil {
				t.Fatalf("EmitColorValue: %v", err)
			}
			if got != tc.want {
				t.Errorf("EmitColorValue(parse(%q)) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestEmitRoundTrip is the correctness bar (§7.3): ParseColorValue(emit(st))
// reproduces st for every representative value and every bundled-preset slot.
func TestEmitRoundTrip(t *testing.T) {
	values := []string{
		"", "normal", "default", "red", "green", "blue", "yellow",
		"black", "magenta", "cyan", "white",
		"brightred", "brightblack", "brightwhite",
		"0", "255", "231", "#a6e3a1", "#000000", "#ffffff",
		"red green", "normal green", "normal default", "red 231",
		"red #a6e3a1", "black brightred",
		"bold", "dim", "italic", "ul", "blink", "reverse", "strike",
		"bold red", "red bold", "bold #f38ba8", "reset bold red",
		"bold dim italic ul blink reverse strike",
	}
	for _, v := range values {
		want, err := ParseColorValue(v)
		if err != nil {
			t.Fatalf("ParseColorValue(%q): %v", v, err)
		}
		emitted, err := EmitColorValue(want)
		if err != nil {
			t.Fatalf("EmitColorValue(parse(%q)): %v", v, err)
		}
		got, err := ParseColorValue(emitted)
		if err != nil {
			t.Fatalf("re-parse of %q (from %q): %v", emitted, v, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("round-trip of %q: parse(emit) = %+v, want %+v (emitted %q)", v, got, want, emitted)
		}
	}

	// Every bundled preset slot must round-trip too.
	for _, name := range BundledThemes() {
		theme, warns, err := LoadTheme(name, "")
		if err != nil {
			t.Fatalf("LoadTheme(%q): %v", name, err)
		}
		if len(warns) != 0 {
			t.Fatalf("LoadTheme(%q) warned: %v", name, warns)
		}
		for slot, st := range theme {
			emitted, err := EmitColorValue(st)
			if err != nil {
				t.Fatalf("EmitColorValue(%s.%s): %v", name, slot, err)
			}
			got, err := ParseColorValue(emitted)
			if err != nil {
				t.Fatalf("re-parse %q for %s.%s: %v", emitted, name, slot, err)
			}
			if !reflect.DeepEqual(got, st) {
				t.Errorf("preset %s slot %s does not round-trip: emitted %q", name, slot, emitted)
			}
		}
	}
}

// TestPackDSTOWColorsRoundTrips packs a theme then parses it back to the same
// styles, and checks canonical slotNames ordering and the ':' join.
func TestPackDSTOWColors(t *testing.T) {
	theme, _, err := LoadTheme("catppuccin-mocha", "")
	if err != nil {
		t.Fatal(err)
	}
	packed := PackDSTOWColors(theme)

	// A complete preset yields all sixteen slots, ':'-joined, in canonical §3.3
	// slot order (allSlots), so the packed string matches the authored presets.
	entries := strings.Split(packed, ":")
	if len(entries) != len(allSlots) {
		t.Fatalf("packed has %d entries, want %d: %q", len(entries), len(allSlots), packed)
	}
	for i, entry := range entries {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("entry %q not in slot=value form", entry)
		}
		if key != string(allSlots[i]) {
			t.Errorf("entry %d key = %q, want %q (canonical §3.3 order)", i, key, string(allSlots[i]))
		}
	}

	// Parsing the packed string back yields the original theme.
	back, warns := ParseDSTOWColors(packed)
	if len(warns) != 0 {
		t.Fatalf("ParseDSTOWColors warned on our own output: %v", warns)
	}
	if !reflect.DeepEqual(back, theme) {
		t.Errorf("pack then parse changed the theme")
	}
}

// TestEmitThemeTOMLRoundTrips emits a theme file then parses it back through
// the theme loader to the same styles, declared-slots only in canonical order.
func TestEmitThemeTOML(t *testing.T) {
	theme, _, err := LoadTheme("catppuccin-mocha", "")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := EmitThemeTOML(theme)
	if err != nil {
		t.Fatal(err)
	}

	// Keys appear in canonical §3.3 slot order (allSlots).
	want := make([]string, len(allSlots))
	for i, s := range allSlots {
		want[i] = string(s)
	}
	var seen []string
	for _, line := range strings.Split(strings.TrimSpace(doc), "\n") {
		key, _, ok := strings.Cut(line, " = ")
		if !ok {
			t.Fatalf("line %q not in `key = value` form", line)
		}
		seen = append(seen, key)
	}
	if !reflect.DeepEqual(seen, want) {
		t.Errorf("theme TOML keys = %v, want %v", seen, want)
	}

	// Parsing the emitted document back yields the original theme.
	back, warns, err := parseThemeContent("emitted", []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("parseThemeContent warned on our own output: %v", warns)
	}
	if !reflect.DeepEqual(back, theme) {
		t.Errorf("emit then parse changed the theme")
	}
}

// TestEmitThemeTOMLDeclaredOnly emits only the slots the theme declares.
func TestEmitThemeTOMLDeclaredOnly(t *testing.T) {
	partial := Theme{SlotStowed: mustStyle("green"), SlotError: mustStyle("bold red")}
	doc, err := EmitThemeTOML(partial)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(doc), "\n")
	if len(lines) != 2 {
		t.Fatalf("partial theme emitted %d lines, want 2: %q", len(lines), doc)
	}
	// Canonical §3.3 order: "stowed" (first state) precedes "error" (output role).
	if !strings.HasPrefix(lines[0], "stowed = ") || !strings.HasPrefix(lines[1], "error = ") {
		t.Errorf("declared slots not in canonical §3.3 order: %q", doc)
	}
}
