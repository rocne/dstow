// Package ui is dstow's sole owner of the terminal streams (A4): every other
// module returns data — diagnostics included — and ui alone renders it. This
// file holds the value grammar: the fourteen generic slots (§3.3), the parser
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

// Slot is one of the fourteen generic slots (§3.3): the closed theming
// vocabulary, spelled snake_case-compatible (family + prominence tier; 1 is
// loudest). Themes, DSTOW_COLORS, and the [color] table speak slots and
// nothing else; dstow's internal vocabulary reaches them only through the
// stage-2 Role mapping below (§7.2). Nothing outside ui ever names a color
// (O3).
type Slot string

const (
	// Content group.
	SlotSection1 Slot = "section1"
	SlotSection2 Slot = "section2"
	SlotName1    Slot = "name1"
	SlotName2    Slot = "name2"
	SlotValue1   Slot = "value1"
	SlotValue2   Slot = "value2"
	// Message group: error, warning, success, info families.
	SlotError1   Slot = "error1"
	SlotError2   Slot = "error2"
	SlotWarning1 Slot = "warning1"
	SlotWarning2 Slot = "warning2"
	SlotSuccess1 Slot = "success1"
	SlotSuccess2 Slot = "success2"
	SlotInfo1    Slot = "info1"
	SlotInfo2    Slot = "info2"
)

// allSlots is the closed fourteen, in §3.3 order (content, then the message
// families). Used to validate keys and to name the set in remedies.
var allSlots = []Slot{
	SlotSection1, SlotSection2, SlotName1, SlotName2, SlotValue1, SlotValue2,
	SlotError1, SlotError2, SlotWarning1, SlotWarning2,
	SlotSuccess1, SlotSuccess2, SlotInfo1, SlotInfo2,
}

var slotSet = func() map[Slot]struct{} {
	m := make(map[Slot]struct{}, len(allSlots))
	for _, s := range allSlots {
		m[s] = struct{}{}
	}
	return m
}()

// slotNames is the fourteen slot strings, sorted, for remedy prose.
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

// Slots returns the closed fourteen in canonical §3.3 order, a fresh copy per
// call — the one order every slot-per-line rendering and emission follows.
func Slots() []Slot {
	out := make([]Slot, len(allSlots))
	copy(out, allSlots)
	return out
}

// Role is one dstow-internal rendering role: a package state, check class,
// severity prefix, or prose role, spelled in CONTEXT.md vocabulary. Callers
// name roles, never slots and never colors (O3); roleSlot — the stage-2
// mapping (§7.2), whose one owner is this table — says which generic slot
// renders each role.
type Role string

const (
	RoleStowed          Role = "stowed"
	RolePartiallyStowed Role = "partially stowed"
	RoleNotStowed       Role = "not stowed"
	RoleOccupied        Role = "occupied"
	RoleDamaged         Role = "damaged"
	RoleDrifted         Role = "drifted"
	RoleBroken          Role = "broken"
	RoleOrphaned        Role = "orphaned"
	RoleContradicted    Role = "contradicted"
	RoleNote            Role = "note"
	RoleWarning         Role = "warning"
	RoleError           Role = "error"
	RoleFix             Role = "fix"
	RoleName            Role = "name"
	RoleHeading         Role = "heading"
	RoleMuted           Role = "muted"
)

// roleSlot is THE stage-2 mapping (§7.2): code-owned, closed in v1 — no
// per-role override surface — and adjusted only here. Slot-sharing is the
// point: sameness that was prose discipline (contradicted ≡ damaged,
// orphaned ≡ partially stowed, fix's prominence) is structural now.
var roleSlot = map[Role]Slot{
	RoleHeading:         SlotSection1,
	RoleName:            SlotName1,
	RoleMuted:           SlotValue2,
	RoleError:           SlotError1,
	RoleWarning:         SlotWarning1,
	RoleFix:             SlotInfo1, // actionable guidance: info, prominent
	RoleNote:            SlotInfo2, // FYI commentary: info, quiet
	RoleStowed:          SlotSuccess2,
	RolePartiallyStowed: SlotWarning2,
	RoleNotStowed:       SlotInfo2,
	RoleOccupied:        SlotInfo1, // CONTEXT: deliberately neutral
	RoleDamaged:         SlotError1,
	RoleContradicted:    SlotError1,
	RoleDrifted:         SlotWarning2,
	RoleBroken:          SlotError2,
	RoleOrphaned:        SlotWarning2,
}

// RoleSlot resolves a role through the stage-2 mapping. An unknown role
// resolves to the empty slot, which no theme declares — it renders plain.
func RoleSlot(r Role) Slot {
	return roleSlot[r]
}

// slotFamilyTier splits a slot into its family stem and prominence tier
// ("warning2" -> "warning", 2). The roster is closed and every slot ends in
// its single-digit tier, so this never fails for a valid slot.
func slotFamilyTier(s Slot) (family string, tier int) {
	str := string(s)
	return str[:len(str)-1], int(str[len(str)-1] - '0')
}

// stepDown is the tier-derivation step (§7.3): remove bold if present, else
// add dim. Attribute-only, so it works identically for named ANSI, 256, and
// hex values.
func (st Style) stepDown() Style {
	for i, p := range st.params {
		if p == color.Bold {
			out := make([]color.Attribute, 0, len(st.params)-1)
			out = append(out, st.params[:i]...)
			out = append(out, st.params[i+1:]...)
			return Style{params: out}
		}
	}
	out := make([]color.Attribute, 0, len(st.params)+1)
	out = append(out, color.Faint)
	out = append(out, st.params...)
	return Style{params: out}
}

// DeriveTiers fills every slot still undeclared after the stack composed
// (§7.3 tier derivation): a missing tier-N slot derives from its family's
// effective tier-1 by stepping down once per tier gap. A declared value at
// any layer always beats derivation. The input theme is not mutated. With
// the default palette in the stack, every slot resolves.
func DeriveTiers(t Theme) Theme {
	out := Theme{}
	for slot, st := range t {
		out[slot] = st
	}
	for _, slot := range allSlots {
		if _, ok := out[slot]; ok {
			continue
		}
		family, tier := slotFamilyTier(slot)
		base, ok := out[Slot(family+"1")]
		if !ok {
			continue // no family tier-1 anywhere: leave undeclared (renders plain)
		}
		for i := 1; i < tier; i++ {
			base = base.stepDown()
		}
		out[slot] = base
	}
	return out
}

// ParseSlotAssignment parses one slot=value operand (the theme show override
// grammar). Unlike the env/table/file paths this is a hard error, never
// warn-and-skip: the user typed the assignment on this very command line, so
// a bad one is a usage mistake with nothing else to salvage.
func ParseSlotAssignment(arg string) (Slot, Style, error) {
	key, value, ok := strings.Cut(arg, "=")
	if !ok {
		return "", Style{}, fmt.Errorf("operand %q is not in slot=value form", arg)
	}
	if !validSlot(key) {
		return "", Style{}, fmt.Errorf("unknown color slot %q; the valid slots are: %s", key, strings.Join(slotNames, ", "))
	}
	st, err := ParseColorValue(value)
	if err != nil {
		return "", Style{}, fmt.Errorf("slot %q has an invalid color value: %v", key, err)
	}
	return Slot(key), st, nil
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

// DefaultPalette returns a fresh copy of the default palette (§7.2): the
// seven tier-1 declarations, every tier-2 left to derivation (DeriveTiers) —
// the floor that guarantees every slot resolves. All values base ANSI-16 —
// the O4 promise, so the terminal theme rethemes dstow automatically. A fresh
// map each call keeps callers from mutating shared state.
//
// Prior-art grounding (ruled 2026-07-19/20 — #115): section1 = cargo HEADER,
// name1 = cargo LITERAL, value1 = a tier-up of cargo PLACEHOLDER, error1 /
// warning1 / success1 = their cargo namesakes (clap-cargo style.rs, the de
// facto cargo-CLI reference), info1 = bold brightblue (the blue-for-info
// convention; fix's historically blue slot).
func DefaultPalette() Theme {
	return Theme{
		SlotSection1: mustStyle("bold brightgreen"),
		SlotName1:    mustStyle("bold brightcyan"),
		SlotValue1:   mustStyle("bold cyan"),
		SlotError1:   mustStyle("bold brightred"),
		SlotWarning1: mustStyle("bold yellow"),
		SlotSuccess1: mustStyle("bold green"),
		SlotInfo1:    mustStyle("bold brightblue"),
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
