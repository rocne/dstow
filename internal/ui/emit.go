package ui

// This file holds the value-grammar emitter: the inverse of ParseColorValue
// (A5, ui is the value grammar's sole owner). ui could parse git's color.*
// grammar but not emit it; the colors command needs the round trip, so a
// Style ([]color.Attribute) maps back to a canonical grammar token string.
// Correctness bar (§7.3): ParseColorValue(EmitColorValue(st)) == st for every
// Style the grammar can produce.
//
// PackDSTOWColors and EmitThemeTOML build on EmitColorValue to serialize a
// whole Theme — the two shapes the colors command emits (§2.4): a packed
// DSTOW_COLORS string and a fourteen-slot theme file. Both emit the theme AS
// LOADED (its declared slots) in the canonical §3.3 slot order (allSlots), so a
// generated theme file reads like the hand-authored presets (§7.3 north-star),
// not alphabetized.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// basicWords is the inverse of basicIdx: SGR offset (0–7) -> color word.
var basicWords = []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white"}

// attrWords is the inverse of attrCode plus reset: an SGR code -> its grammar
// word. It names only the codes ParseColorValue emits for attributes and reset.
var attrWords = map[color.Attribute]string{
	color.Reset:        "reset",
	color.Bold:         "bold",
	color.Faint:        "dim",
	color.Italic:       "italic",
	color.Underline:    "ul",
	color.BlinkSlow:    "blink",
	color.ReverseVideo: "reverse",
	color.CrossedOut:   "strike",
}

// UnemittableError reports a Style whose parameter list ParseColorValue could
// never have produced, so the emitter cannot render it back into the grammar
// (§7.3 round-trip contract). It names the offending SGR code.
type UnemittableError struct {
	Code color.Attribute
}

func (e *UnemittableError) Error() string {
	return fmt.Sprintf("ui: SGR parameter %d is outside git's color.* value grammar and cannot be emitted", int(e.Code))
}

// EmitColorValue renders a Style back into a canonical git color.* value
// string, the inverse of ParseColorValue (A5). It walks the parameter list in
// order — the order ParseColorValue produced it — so re-parsing the emitted
// string yields the identical Style. A background color with no foreground
// before it is emitted with a leading `normal` (which occupies the foreground
// slot but emits no code), exactly how such a Style must have been written.
// The empty Style yields the empty string.
func EmitColorValue(st Style) (string, error) {
	var toks []string
	colorSlots := 0
	p := st.params
	for i := 0; i < len(p); {
		c := p[i]
		if word, ok := attrWords[c]; ok {
			toks = append(toks, word)
			i++
			continue
		}
		tok, n, isBG, ok := decodeColor(p, i)
		if !ok {
			return "", &UnemittableError{Code: c}
		}
		if isBG && colorSlots == 0 {
			// The foreground slot was left unchanged (`normal`): it emitted no
			// code but must be spelled so the background lands in slot two.
			toks = append(toks, "normal")
			colorSlots++
		}
		toks = append(toks, tok)
		colorSlots++
		i += n
	}
	return strings.Join(toks, " "), nil
}

// decodeColor reads one color token starting at index i, returning the grammar
// word, how many parameters it consumed, whether it is a background color, and
// whether i even began a color. The SGR ranges are git's: fg basics 30–37,
// bright 90–97, default 39, extended 38;5;n / 38;2;r;g;b; bg the +10 family.
func decodeColor(p []color.Attribute, i int) (tok string, n int, isBG bool, ok bool) {
	c := int(p[i])
	switch {
	case c >= 30 && c <= 37:
		return basicWords[c-30], 1, false, true
	case c >= 90 && c <= 97:
		return "bright" + basicWords[c-90], 1, false, true
	case c == 39:
		return "default", 1, false, true
	case c >= 40 && c <= 47:
		return basicWords[c-40], 1, true, true
	case c >= 100 && c <= 107:
		return "bright" + basicWords[c-100], 1, true, true
	case c == 49:
		return "default", 1, true, true
	case c == 38 || c == 48:
		isBG = c == 48
		if i+1 >= len(p) {
			return "", 0, false, false
		}
		switch p[i+1] {
		case 5:
			if i+2 >= len(p) {
				return "", 0, false, false
			}
			return strconv.Itoa(int(p[i+2])), 3, isBG, true
		case 2:
			if i+4 >= len(p) {
				return "", 0, false, false
			}
			return fmt.Sprintf("#%02x%02x%02x", int(p[i+2]), int(p[i+3]), int(p[i+4])), 5, isBG, true
		}
	}
	return "", 0, false, false
}

// PackDSTOWColors serializes a theme as a packed DSTOW_COLORS string:
// slot=value entries in canonical §3.3 slot order, joined by ':', over the
// theme's declared slots (§2.4 colors, default format). A complete preset
// yields all fourteen entries. It is the inverse shape of ParseDSTOWColors.
func PackDSTOWColors(t Theme) string {
	var parts []string
	for _, s := range allSlots {
		st, ok := t[s]
		if !ok {
			continue
		}
		val, err := EmitColorValue(st)
		if err != nil {
			continue // a grammar-loaded theme cannot hit this; skip defensively
		}
		parts = append(parts, string(s)+"="+val)
	}
	return strings.Join(parts, ":")
}

// EmitThemeTOML serializes a theme as a theme file (§7.3 north-star): the bare
// slot keys at TOML top level, one per declared slot in canonical §3.3 slot
// order, values as color-grammar strings. It is the write side of the
// themeFile schema parseThemeContent reads. Color values are ASCII, so Go
// quoting is TOML-basic-string safe.
func EmitThemeTOML(t Theme) (string, error) {
	var b strings.Builder
	for _, s := range allSlots {
		st, ok := t[s]
		if !ok {
			continue
		}
		val, err := EmitColorValue(st)
		if err != nil {
			return "", fmt.Errorf("slot %q: %w", string(s), err)
		}
		fmt.Fprintf(&b, "%s = %q\n", string(s), val)
	}
	return b.String(), nil
}
