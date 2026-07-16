// Package ui is dstow's sole owner of the terminal streams (A4): every other
// module returns data — diagnostics included — and ui alone renders it. This
// file holds the value grammar: the sixteen semantic slots (§3.3), the parser
// for git's color.* value grammar (§7.3), the default ANSI-16 palette (§7.2),
// and the strip-ANSI helper that backs the O11 test contract.
//
// One vocabulary, one grammar: the same slot names and the same value grammar
// serve DSTOW_COLORS, the [color] TOML table, and theme files (§7.3
// north-star), and ParseColorValue is their single owner (A5).
package ui

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// Slot is one of the sixteen semantic slots (§3.3): a closed set spelled in
// the snake_case color vocabulary. Nothing outside ui ever names a color (O3);
// callers name slots.
type Slot string

const (
	SlotStowed          Slot = "stowed"
	SlotPartiallyStowed Slot = "partially_stowed"
	SlotNotStowed       Slot = "not_stowed"
	SlotOccupied        Slot = "occupied"
	SlotDamaged         Slot = "damaged"
	SlotDrifted         Slot = "drifted"
	SlotBroken          Slot = "broken"
	SlotOrphaned        Slot = "orphaned"
	SlotContradicted    Slot = "contradicted"
	SlotNote            Slot = "note"
	SlotWarning         Slot = "warning"
	SlotError           Slot = "error"
	SlotFix             Slot = "fix"
	SlotName            Slot = "name"
	SlotHeading         Slot = "heading"
	SlotMuted           Slot = "muted"
)

// allSlots is the closed sixteen, in §3.3 order (states, check classes,
// severities, prose). Used to validate keys and to name the set in remedies.
var allSlots = []Slot{
	SlotStowed, SlotPartiallyStowed, SlotNotStowed, SlotOccupied, SlotDamaged, SlotDrifted,
	SlotBroken, SlotOrphaned, SlotContradicted,
	SlotNote, SlotWarning, SlotError, SlotFix,
	SlotName, SlotHeading, SlotMuted,
}

var slotSet = func() map[Slot]struct{} {
	m := make(map[Slot]struct{}, len(allSlots))
	for _, s := range allSlots {
		m[s] = struct{}{}
	}
	return m
}()

// slotNames is the sixteen slot strings, sorted, for remedy prose.
var slotNames = func() []string {
	out := make([]string, len(allSlots))
	for i, s := range allSlots {
		out[i] = string(s)
	}
	sort.Strings(out)
	return out
}()

func validSlot(key string) bool {
	_, ok := slotSet[Slot(key)]
	return ok
}

// Style is one parsed git color.* value: an ordered list of raw SGR parameters
// (fatih color.Attribute is a raw SGR int). The zero value has no parameters
// and renders plain — as do a lone `normal` and lone attribute negations,
// which contribute no code (see ParseColorValue).
type Style struct {
	params []color.Attribute
}

// render wraps text in this style's SGR sequence. An empty parameter list
// (zero value, `normal`-only, negation-only) renders plain, so the caller
// never emits a bare `ESC[m`. The instance is enabled explicitly and per-call:
// the caller (a Face) has already decided color is on, and ui never touches the
// fatih package-global NoColor (A5).
func (st Style) render(text string) string {
	if len(st.params) == 0 {
		return text
	}
	c := color.New(st.params...)
	c.EnableColor()
	return c.Sprint(text)
}

// basicIdx maps the eight basic color words to their SGR offset (fg 30+idx,
// bg 40+idx; bright fg 90+idx, bg 100+idx).
var basicIdx = map[string]int{
	"black": 0, "red": 1, "green": 2, "yellow": 3,
	"blue": 4, "magenta": 5, "cyan": 6, "white": 7,
}

// attrCode maps the seven attribute words to their SGR code. Each is negatable
// with a `no`/`no-` prefix (validated below); negations render as nothing.
var attrCode = map[string]color.Attribute{
	"bold":    color.Bold,         // 1
	"dim":     color.Faint,        // 2
	"italic":  color.Italic,       // 3
	"ul":      color.Underline,    // 4
	"blink":   color.BlinkSlow,    // 5
	"reverse": color.ReverseVideo, // 7
	"strike":  color.CrossedOut,   // 9
}

// ParseColorValue parses one value in git's color.* grammar (§7.3), the single
// value-grammar owner (A5). Words are whitespace-separated, in any order:
//
//   - Colors: the first color word is the foreground, the second the
//     background, a third is an error. `normal` leaves the channel unchanged
//     (emits no code) but still occupies its slot; `default` explicitly resets
//     it (SGR 39 fg / 49 bg); the eight basics and their `bright*` variants;
//     an integer 0–255 (256-color: 38;5;n); `#RRGGBB` hex (24-bit: 38;2;r;g;b).
//   - Attributes, any number and position: bold dim ul blink reverse italic
//     strike, each negatable with `no` or `no-`. Negations are parsed and
//     validated but render as nothing: a themed slot REPLACES its default
//     wholesale (§7.3 "top wins"), so there is never a base style for a
//     negation to cancel.
//   - `reset` renders SGR 0 first.
//
// An unknown word, a third color, or a malformed hex/integer is an error that
// names the offending word. The empty string yields the zero Style.
func ParseColorValue(s string) (Style, error) {
	var params []color.Attribute
	hasReset := false
	colorSlots := 0

	for _, tok := range strings.Fields(s) {
		lt := strings.ToLower(tok)

		if lt == "reset" {
			hasReset = true
			continue
		}
		if a, ok := attrCode[lt]; ok {
			params = append(params, a)
			continue
		}
		if isNegation(lt) {
			// Validated; emits nothing (see doc comment).
			continue
		}

		codes, isColor, cerr := colorToken(lt, colorSlots)
		if isColor {
			if colorSlots >= 2 {
				return Style{}, fmt.Errorf("%q is a third color; a color value takes at most a foreground and a background", tok)
			}
			if cerr != nil {
				return Style{}, cerr
			}
			params = append(params, codes...)
			colorSlots++
			continue
		}
		return Style{}, fmt.Errorf("unrecognized color-value word %q", tok)
	}

	if hasReset {
		params = append([]color.Attribute{color.Reset}, params...)
	}
	return Style{params: params}, nil
}

// isNegation reports whether tok is a `no`/`no-` negation of a known
// attribute word (e.g. "nobold", "no-italic").
func isNegation(tok string) bool {
	rest := ""
	switch {
	case strings.HasPrefix(tok, "no-"):
		rest = tok[3:]
	case strings.HasPrefix(tok, "no"):
		rest = tok[2:]
	default:
		return false
	}
	_, ok := attrCode[rest]
	return ok
}

// colorToken classifies tok as a color word for the given position (0 =
// foreground, 1 = background). It returns (codes, true, nil) for a valid
// color, (nil, true, err) for a color-shaped-but-malformed token (bad hex or
// out-of-range integer), and (nil, false, nil) when tok is not a color at all.
func colorToken(tok string, pos int) ([]color.Attribute, bool, error) {
	base, brightBase, ext := 30, 90, 38
	if pos == 1 {
		base, brightBase, ext = 40, 100, 48
	}

	switch tok {
	case "normal":
		return nil, true, nil // leave the channel unchanged: no code
	case "default":
		return []color.Attribute{color.Attribute(base + 9)}, true, nil // 39 / 49
	}

	if idx, ok := basicIdx[tok]; ok {
		return []color.Attribute{color.Attribute(base + idx)}, true, nil
	}
	if strings.HasPrefix(tok, "bright") {
		if idx, ok := basicIdx[tok[len("bright"):]]; ok {
			return []color.Attribute{color.Attribute(brightBase + idx)}, true, nil
		}
	}
	if strings.HasPrefix(tok, "#") {
		r, g, b, err := parseHexColor(tok)
		if err != nil {
			return nil, true, err
		}
		return []color.Attribute{color.Attribute(ext), 2, color.Attribute(r), color.Attribute(g), color.Attribute(b)}, true, nil
	}
	if isAllDigits(tok) {
		n, err := strconv.Atoi(tok)
		if err != nil || n < 0 || n > 255 {
			return nil, true, fmt.Errorf("%q is not a valid color: 256-color integers must be in the 0–255 range", tok)
		}
		return []color.Attribute{color.Attribute(ext), 5, color.Attribute(n)}, true, nil
	}
	return nil, false, nil
}

// parseHexColor parses a #RRGGBB token into its three 8-bit channels.
func parseHexColor(tok string) (r, g, b int, err error) {
	if len(tok) != 7 || tok[0] != '#' {
		return 0, 0, 0, fmt.Errorf("%q is not a valid #RRGGBB hex color", tok)
	}
	vals := make([]int, 3)
	for i := 0; i < 3; i++ {
		v, e := strconv.ParseUint(tok[1+i*2:3+i*2], 16, 8)
		if e != nil {
			return 0, 0, 0, fmt.Errorf("%q is not a valid #RRGGBB hex color", tok)
		}
		vals[i] = int(v)
	}
	return vals[0], vals[1], vals[2], nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// mustStyle parses a constant grammar value, panicking on error. Used only for
// the compile-time default palette, whose inputs are fixed literals.
func mustStyle(s string) Style {
	st, err := ParseColorValue(s)
	if err != nil {
		panic("ui: bad constant color value " + strconv.Quote(s) + ": " + err.Error())
	}
	return st
}

// DefaultPalette returns a fresh copy of the full sixteen-slot default palette
// (§7.2), emitting only the base ANSI-16 slots — the O4 promise, so the
// terminal theme rethemes dstow automatically. A fresh map each call keeps
// callers from mutating shared state.
func DefaultPalette() Theme {
	return Theme{
		SlotStowed:          mustStyle("green"),
		SlotPartiallyStowed: mustStyle("yellow"),
		SlotNotStowed:       mustStyle("dim"),
		SlotOccupied:        mustStyle("magenta"),
		SlotDamaged:         mustStyle("bold red"),
		SlotDrifted:         mustStyle("cyan"),
		SlotBroken:          mustStyle("red"),
		SlotOrphaned:        mustStyle("yellow"),
		SlotContradicted:    mustStyle("bold red"),
		SlotNote:            mustStyle("cyan"),
		SlotWarning:         mustStyle("yellow"),
		SlotError:           mustStyle("bold red"),
		SlotFix:             mustStyle("blue"),
		SlotName:            mustStyle("bold"),
		SlotHeading:         mustStyle("bold"),
		SlotMuted:           mustStyle("dim"),
	}
}

// ansiSGR matches a CSI SGR sequence (ESC [ ... m) — the family every Style
// and severity prefix emits.
var ansiSGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes every CSI SGR sequence from s. It backs the O11 contract:
// StripANSI(styled) == plain for every Style the grammar can produce and every
// severity line ui prints.
func StripANSI(s string) string {
	return ansiSGR.ReplaceAllString(s, "")
}
