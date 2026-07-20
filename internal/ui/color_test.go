package ui

import (
	"reflect"
	"testing"

	"github.com/fatih/color"
)

// --- §7.3 ParseColorValue: the git color.* value grammar ---------------------

// The parser corpus. Expected parameter lists are the raw SGR codes the grammar
// mandates, named from the spec (fg basics 30–37, bright 90–97; bg +10;
// default 39/49; 256-color 38;5;n; truecolor 38;2;r;g;b; attributes per §7.3),
// never read back from the implementation.
func TestParseColorValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []color.Attribute
	}{
		// The eight basic foreground colors (30–37).
		{"black fg", "black", []color.Attribute{color.FgBlack}},
		{"red fg", "red", []color.Attribute{color.FgRed}},
		{"green fg", "green", []color.Attribute{color.FgGreen}},
		{"yellow fg", "yellow", []color.Attribute{color.FgYellow}},
		{"blue fg", "blue", []color.Attribute{color.FgBlue}},
		{"magenta fg", "magenta", []color.Attribute{color.FgMagenta}},
		{"cyan fg", "cyan", []color.Attribute{color.FgCyan}},
		{"white fg", "white", []color.Attribute{color.FgWhite}},
		// Bright variants (90–97).
		{"bright red fg", "brightred", []color.Attribute{color.FgHiRed}},
		{"bright white fg", "brightwhite", []color.Attribute{color.FgHiWhite}},
		{"bright black fg", "brightblack", []color.Attribute{color.FgHiBlack}},
		// default explicitly resets the channel (39 fg).
		{"default fg", "default", []color.Attribute{39}},
		// normal leaves the channel unchanged: emits no code.
		{"normal emits nothing", "normal", nil},
		// 256-color integer bounds and interior (38;5;n).
		{"256-color 0", "0", []color.Attribute{38, 5, 0}},
		{"256-color 255", "255", []color.Attribute{38, 5, 255}},
		{"256-color 231", "231", []color.Attribute{38, 5, 231}},
		// 24-bit hex (38;2;r;g;b).
		{"hex catppuccin green", "#a6e3a1", []color.Attribute{38, 2, 0xa6, 0xe3, 0xa1}},
		{"hex black", "#000000", []color.Attribute{38, 2, 0, 0, 0}},
		{"hex white", "#ffffff", []color.Attribute{38, 2, 255, 255, 255}},
		// Foreground then background ordering (fg 31, bg 42).
		{"fg then bg", "red green", []color.Attribute{color.FgRed, color.BgGreen}},
		{"bg default", "normal default", []color.Attribute{49}},
		{"bg bright", "black brightred", []color.Attribute{color.FgBlack, color.BgHiRed}},
		{"bg 256", "red 231", []color.Attribute{color.FgRed, 48, 5, 231}},
		{"bg hex", "red #a6e3a1", []color.Attribute{color.FgRed, 48, 2, 0xa6, 0xe3, 0xa1}},
		// Every attribute.
		{"bold", "bold", []color.Attribute{color.Bold}},
		{"dim", "dim", []color.Attribute{color.Faint}},
		{"ul", "ul", []color.Attribute{color.Underline}},
		{"blink", "blink", []color.Attribute{color.BlinkSlow}},
		{"reverse", "reverse", []color.Attribute{color.ReverseVideo}},
		{"italic", "italic", []color.Attribute{color.Italic}},
		{"strike", "strike", []color.Attribute{color.CrossedOut}},
		// Attribute + color, order-independent.
		{"bold red", "bold red", []color.Attribute{color.Bold, color.FgRed}},
		{"red bold", "red bold", []color.Attribute{color.FgRed, color.Bold}},
		// Negations render as nothing (validated, no base to cancel).
		{"no- negation emits nothing", "no-bold", nil},
		{"no negation emits nothing", "nobold", nil},
		{"negation alongside a color", "no-italic red", []color.Attribute{color.FgRed}},
		// reset renders SGR 0 first.
		{"reset alone", "reset", []color.Attribute{color.Reset}},
		{"reset hoisted to front", "red reset", []color.Attribute{color.Reset, color.FgRed}},
		// Empty string yields the zero Style.
		{"empty string", "", nil},
		{"whitespace only", "   ", nil},
		// Case-insensitive words.
		{"uppercase RED", "RED", []color.Attribute{color.FgRed}},
		{"mixed case Bold", "Bold", []color.Attribute{color.Bold}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseColorValue(tt.in)
			if err != nil {
				t.Fatalf("ParseColorValue(%q) unexpected error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got.params, tt.want) {
				t.Errorf("ParseColorValue(%q).params = %v, want %v", tt.in, got.params, tt.want)
			}
		})
	}
}

func TestParseColorValueErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"unknown word", "chartreuse"},
		{"bright without a basic", "brightpink"},
		{"256-color out of range", "256"},
		{"256-color far out of range", "1000"},
		{"malformed hex too short", "#a6e3a"},
		{"malformed hex non-hex digit", "#gggggg"},
		{"malformed hex no digits", "#"},
		{"third color is an error", "red green blue"},
		{"third color even with attributes", "bold red green yellow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseColorValue(tt.in)
			if err == nil {
				t.Fatalf("ParseColorValue(%q) = nil error, want an error", tt.in)
			}
		})
	}
}

// A third color must be rejected even though the first two parsed fine; the
// error names the offending word.
func TestParseColorValueThirdColorNamesWord(t *testing.T) {
	_, err := ParseColorValue("red green blue")
	if err == nil {
		t.Fatal("want error for a third color")
	}
	if !contains(err.Error(), "blue") {
		t.Errorf("third-color error should name %q, got %q", "blue", err.Error())
	}
}

// --- §7.2 default palette ----------------------------------------------------

// The default palette declares exactly the seven tier-1s (§7.2), ANSI-16 only
// (the O4 promise): every expected value is the SGR meaning of the color named
// in §7.2, not a copy of DefaultPalette's construction. Tier-2s are absent by
// design — they derive (DeriveTiers).
func TestDefaultPalette(t *testing.T) {
	want := map[Slot][]color.Attribute{
		SlotSection1: {color.Bold, color.FgHiGreen}, // bold brightgreen (cargo HEADER)
		SlotName1:    {color.Bold, color.FgHiCyan},  // bold brightcyan (cargo LITERAL)
		SlotValue1:   {color.Bold, color.FgCyan},    // bold cyan (PLACEHOLDER tier-up)
		SlotError1:   {color.Bold, color.FgHiRed},   // bold brightred (cargo ERROR)
		SlotWarning1: {color.Bold, color.FgYellow},  // bold yellow (cargo WARN)
		SlotSuccess1: {color.Bold, color.FgGreen},   // bold green (cargo GOOD)
		SlotInfo1:    {color.Bold, color.FgHiBlue},  // bold brightblue (blue-for-info)
	}
	pal := DefaultPalette()
	if len(pal) != 7 {
		t.Fatalf("DefaultPalette declares %d slots, want the 7 tier-1s", len(pal))
	}
	for slot, wantParams := range want {
		st, ok := pal[slot]
		if !ok {
			t.Errorf("DefaultPalette missing slot %q", slot)
			continue
		}
		if !reflect.DeepEqual(st.params, wantParams) {
			t.Errorf("DefaultPalette[%q].params = %v, want %v", slot, st.params, wantParams)
		}
	}
}

// --- §7.2 stage-2 mapping + §7.3 tier derivation ------------------------------

// The ruled stage-2 mapping (#115): every role resolves to its ruled generic
// slot. The table below IS the ruling — asserted role by role, never derived
// from roleSlot itself.
func TestRoleSlotMapping(t *testing.T) {
	want := map[Role]Slot{
		RoleHeading:         SlotSection1,
		RoleName:            SlotName1,
		RoleMuted:           SlotValue2,
		RoleError:           SlotError1,
		RoleWarning:         SlotWarning1,
		RoleFix:             SlotInfo1,
		RoleNote:            SlotInfo2,
		RoleStowed:          SlotSuccess2,
		RolePartiallyStowed: SlotWarning2,
		RoleNotStowed:       SlotInfo2,
		RoleOccupied:        SlotInfo1,
		RoleDamaged:         SlotError1,
		RoleContradicted:    SlotError1,
		RoleDrifted:         SlotWarning2,
		RoleBroken:          SlotError2,
		RoleOrphaned:        SlotWarning2,
	}
	for role, slot := range want {
		if got := RoleSlot(role); got != slot {
			t.Errorf("RoleSlot(%q) = %q, want %q", role, got, slot)
		}
	}
}

// Tier derivation (§7.3): an undeclared tier-2 derives from its family's
// effective tier-1 — remove bold if present, else add dim; a declared tier-2
// always wins; a family with no tier-1 anywhere stays undeclared.
func TestDeriveTiers(t *testing.T) {
	in := Theme{
		SlotError1:   mustStyle("bold brightred"), // bold present: tier-2 drops it
		SlotInfo1:    mustStyle("brightblue"),     // no bold: tier-2 adds dim
		SlotWarning1: mustStyle("bold yellow"),
		SlotWarning2: mustStyle("magenta"), // declared: beats derivation
	}
	got := DeriveTiers(in)

	if !reflect.DeepEqual(got[SlotError2].params, []color.Attribute{color.FgHiRed}) {
		t.Errorf("error2 = %v, want brightred (bold removed)", got[SlotError2].params)
	}
	if !reflect.DeepEqual(got[SlotInfo2].params, []color.Attribute{color.Faint, color.FgHiBlue}) {
		t.Errorf("info2 = %v, want dim brightblue (dim added)", got[SlotInfo2].params)
	}
	if !reflect.DeepEqual(got[SlotWarning2].params, []color.Attribute{color.FgMagenta}) {
		t.Errorf("warning2 = %v, want the declared magenta", got[SlotWarning2].params)
	}
	if _, ok := got[SlotSuccess2]; ok {
		t.Error("success2 derived with no success1 anywhere; must stay undeclared")
	}
	if _, ok := in[SlotError2]; ok {
		t.Error("DeriveTiers mutated its input")
	}
}

// The full stack composed over the default palette resolves every slot: the
// tier-1 floor guarantees whole-roster coverage after derivation.
func TestDeriveTiersOverDefaultPaletteIsComplete(t *testing.T) {
	got := DeriveTiers(DefaultPalette())
	for _, slot := range Slots() {
		if _, ok := got[slot]; !ok {
			t.Errorf("slot %q unresolved over the default palette", slot)
		}
	}
}

// The palette promises ANSI-16 only (O4): no default slot uses a 256-color or
// truecolor introducer (38/48).
func TestDefaultPaletteIsANSI16(t *testing.T) {
	for slot, st := range DefaultPalette() {
		for _, p := range st.params {
			if p == 38 || p == 48 {
				t.Errorf("slot %q uses an extended-color introducer %d; defaults must be ANSI-16", slot, p)
			}
		}
	}
}

// DefaultPalette returns a fresh map: mutating one call must not affect another.
func TestDefaultPaletteIsFresh(t *testing.T) {
	a := DefaultPalette()
	delete(a, SlotError1)
	b := DefaultPalette()
	if _, ok := b[SlotError1]; !ok {
		t.Error("DefaultPalette shares state across calls")
	}
}

// --- O11: StripANSI(styled) == plain -----------------------------------------

// Property test across the whole parser corpus and the default palette: for
// every style the grammar can produce, stripping ANSI recovers the plain text.
func TestStripANSIRoundtrip(t *testing.T) {
	const plain = "the quick brown fox"
	corpus := []string{
		"green", "bold red", "red green", "#a6e3a1", "231", "brightred",
		"default", "normal", "reset", "red reset", "no-bold red", "dim",
		"ul blink reverse italic strike", "red #a6e3a1", "normal default",
	}
	for _, spec := range corpus {
		st, err := ParseColorValue(spec)
		if err != nil {
			t.Fatalf("ParseColorValue(%q): %v", spec, err)
		}
		styled := st.render(plain)
		if got := StripANSI(styled); got != plain {
			t.Errorf("StripANSI(render(%q)) = %q, want %q", spec, got, plain)
		}
	}
	for slot, st := range DefaultPalette() {
		if got := StripANSI(st.render(plain)); got != plain {
			t.Errorf("StripANSI(render(%q)) = %q, want %q", slot, got, plain)
		}
	}
}

// A zero Style and equivalents render exactly plain (no bare ESC[m).
func TestZeroStyleRendersPlain(t *testing.T) {
	const plain = "data"
	for _, spec := range []string{"", "normal", "no-bold"} {
		st, err := ParseColorValue(spec)
		if err != nil {
			t.Fatalf("ParseColorValue(%q): %v", spec, err)
		}
		if got := st.render(plain); got != plain {
			t.Errorf("render(%q) = %q, want plain %q", spec, got, plain)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
