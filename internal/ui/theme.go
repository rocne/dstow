package ui

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/rocne/dstow/internal/name"
)

// Theme maps slots to styles. A theme need not be complete: absent slots fall
// through the compose stack to the default palette.
type Theme map[Slot]Style

// Warning is a diagnostic as data (A4): parsers RETURN warnings, the caller
// decides when to print them. Detail is complete prose — what is wrong and, per
// the C18 warn-and-skip posture, that the rest still applies.
type Warning struct {
	Source string // where it came from, e.g. "DSTOW_COLORS", a file path
	Detail string // complete prose: what's wrong
}

// ComposeTheme layers themes, first layer wins per slot; absent slots fall
// through to later layers. The production caller supplies the floor:
// ComposeTheme(env, table, theme, DefaultPalette()) — §7.3's top-wins stack.
func ComposeTheme(layers ...Theme) Theme {
	out := Theme{}
	for _, layer := range layers {
		for slot, style := range layer {
			if _, seen := out[slot]; !seen {
				out[slot] = style
			}
		}
	}
	return out
}

// setSlot parses one slot=value pair into t, returning a warn-and-skip Warning
// (C18) instead of aborting: an unknown slot or a bad value is skipped and the
// rest of the source still applies. An empty value is skipped silently.
func setSlot(t Theme, source, key, value string) *Warning {
	if value == "" {
		return nil
	}
	if !validSlot(key) {
		return &Warning{
			Source: source,
			Detail: fmt.Sprintf("unknown color slot %q; the valid slots are: %s (this entry is skipped, the rest still applies)", key, strings.Join(slotNames, ", ")),
		}
	}
	st, err := ParseColorValue(value)
	if err != nil {
		return &Warning{
			Source: source,
			Detail: fmt.Sprintf("slot %q has an invalid color value: %v (this entry is skipped, the rest still applies)", key, err),
		}
	}
	t[Slot(key)] = st
	return nil
}

// applyValues parses a map of slot=value pairs into a fresh Theme, warn-and-skip
// per entry. Keys are visited in sorted order so warnings are deterministic.
func applyValues(source string, values map[string]string) (Theme, []Warning) {
	t := Theme{}
	var warns []Warning
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if w := setSlot(t, source, k, values[k]); w != nil {
			warns = append(warns, *w)
		}
	}
	return t, warns
}

// ParseDSTOWColors parses the DSTOW_COLORS packed string: LS_COLORS-family
// slot=value entries split on ':'. git's color.* grammar never contains ':',
// so the split is safe. Empty entries are skipped silently; an unknown slot,
// a bad value, or a malformed entry warns and is skipped (the rest applies).
func ParseDSTOWColors(packed string) (Theme, []Warning) {
	const source = "DSTOW_COLORS"
	t := Theme{}
	var warns []Warning
	for _, entry := range strings.Split(packed, ":") {
		if entry == "" {
			continue
		}
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			warns = append(warns, Warning{
				Source: source,
				Detail: fmt.Sprintf("entry %q is not in slot=value form (this entry is skipped, the rest still applies)", entry),
			})
			continue
		}
		if w := setSlot(t, source, key, value); w != nil {
			warns = append(warns, *w)
		}
	}
	return t, warns
}

// ParseColorTable parses the [color] TOML table (values pre-extracted by
// config) over the closed sixteen-key set (§3.3), warn-and-skip per key.
func ParseColorTable(values map[string]string) (Theme, []Warning) {
	return applyValues("[color] table", values)
}

// themeFile is the bare [color] schema of a theme file (§7.3 north-star): the
// sixteen slot keys at TOML top level, no wrapper table. Decoding into this
// struct lets md.Undecoded() surface any unknown keys.
type themeFile struct {
	Stowed          string `toml:"stowed"`
	PartiallyStowed string `toml:"partially_stowed"`
	NotStowed       string `toml:"not_stowed"`
	Occupied        string `toml:"occupied"`
	Damaged         string `toml:"damaged"`
	Drifted         string `toml:"drifted"`
	Broken          string `toml:"broken"`
	Orphaned        string `toml:"orphaned"`
	Contradicted    string `toml:"contradicted"`
	Note            string `toml:"note"`
	Warning         string `toml:"warning"`
	Error           string `toml:"error"`
	Fix             string `toml:"fix"`
	Name            string `toml:"name"`
	Heading         string `toml:"heading"`
	Muted           string `toml:"muted"`
}

func (tf themeFile) toMap() map[string]string {
	return map[string]string{
		string(SlotStowed):          tf.Stowed,
		string(SlotPartiallyStowed): tf.PartiallyStowed,
		string(SlotNotStowed):       tf.NotStowed,
		string(SlotOccupied):        tf.Occupied,
		string(SlotDamaged):         tf.Damaged,
		string(SlotDrifted):         tf.Drifted,
		string(SlotBroken):          tf.Broken,
		string(SlotOrphaned):        tf.Orphaned,
		string(SlotContradicted):    tf.Contradicted,
		string(SlotNote):            tf.Note,
		string(SlotWarning):         tf.Warning,
		string(SlotError):           tf.Error,
		string(SlotFix):             tf.Fix,
		string(SlotName):            tf.Name,
		string(SlotHeading):         tf.Heading,
		string(SlotMuted):           tf.Muted,
	}
}

// parseThemeContent decodes a theme file's bytes into a Theme. A TOML syntax
// error aborts (there is no partial file to salvage); unknown keys and bad
// values warn and are skipped (C18).
func parseThemeContent(source string, content []byte) (Theme, []Warning, error) {
	var tf themeFile
	md, err := toml.Decode(string(content), &tf)
	if err != nil {
		return nil, nil, fmt.Errorf("theme %q is not valid TOML: %w", source, err)
	}
	t, warns := applyValues(source, tf.toMap())
	// Unknown top-level keys: a theme file is the bare sixteen-slot schema.
	for _, key := range md.Undecoded() {
		warns = append(warns, Warning{
			Source: source,
			Detail: fmt.Sprintf("unknown key %q; a theme file uses only the sixteen color slots (this key is skipped, the rest still applies)", key.String()),
		})
	}
	return t, warns, nil
}

// The bundled presets are the four catppuccin flavors, generated by Whiskers
// from themes/dstow.tera (ruled at #105 — one template owns the role mapping;
// the flavor TOMLs are vendored artifacts, regenerated via go:generate and
// kept fresh by CI).
//go:generate whiskers themes/dstow.tera

//go:embed themes/*.toml
var bundledFS embed.FS

const bundledDir = "themes"

// BundledThemes returns the embedded preset names, sorted (for remedies and the
// future colors command).
func BundledThemes() []string {
	entries, err := bundledFS.ReadDir(bundledDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".toml"))
	}
	sort.Strings(names)
	return names
}

// ThemeNotFoundError reports that a bare theme name resolved nowhere. Its
// message names the ref, both locations searched, and every available name —
// every refusal names its remedy (§1.4).
type ThemeNotFoundError struct {
	Ref           string
	UserThemesDir string
	Available     []string
}

func (e *ThemeNotFoundError) Error() string {
	return fmt.Sprintf(
		"theme %q not found (searched the user themes dir %q and the bundled presets); available themes: %s",
		e.Ref, e.UserThemesDir, strings.Join(e.Available, ", "),
	)
}

// LoadTheme resolves a theme ref through the one theme loader (A5). A path form
// (per name.IsPathOperand) is read as a file anywhere — production callers hand
// it absolute (C8). A bare name resolves the user themes dir first
// (<userThemesDir>/<name>.toml), then the bundled presets; the user shadows the
// bundled preset on a name collision (C4). A bare name that resolves nowhere
// yields a *ThemeNotFoundError.
func LoadTheme(ref, userThemesDir string) (Theme, []Warning, error) {
	if name.IsPathOperand(ref) {
		content, err := os.ReadFile(ref)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot read theme file %q: %w", ref, err)
		}
		t, warns, err := parseThemeContent(ref, content)
		return t, warns, err
	}

	if userThemesDir != "" {
		userPath := filepath.Join(userThemesDir, ref+".toml")
		content, err := os.ReadFile(userPath)
		switch {
		case err == nil:
			t, warns, err := parseThemeContent(userPath, content)
			return t, warns, err
		case !errors.Is(err, fs.ErrNotExist):
			// Only absence falls through to the bundled presets. Any other
			// failure (e.g. permissions) on the user's own theme file must
			// surface, never silently shadow-in the bundled preset.
			return nil, nil, fmt.Errorf("cannot read theme file %q: %w", userPath, err)
		}
	}

	if content, err := bundledFS.ReadFile(bundledDir + "/" + ref + ".toml"); err == nil {
		t, warns, err := parseThemeContent("bundled preset "+ref, content)
		return t, warns, err
	}

	return nil, nil, &ThemeNotFoundError{
		Ref:           ref,
		UserThemesDir: userThemesDir,
		Available:     availableThemes(userThemesDir),
	}
}

// availableThemes is the union of bundled preset names and any .toml basenames
// in the user themes dir, sorted and de-duplicated — the remedy list.
func availableThemes(userThemesDir string) []string {
	set := map[string]struct{}{}
	for _, n := range BundledThemes() {
		set[n] = struct{}{}
	}
	if userThemesDir != "" {
		if entries, err := os.ReadDir(userThemesDir); err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
					continue
				}
				set[strings.TrimSuffix(e.Name(), ".toml")] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
