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
// config) over the closed fourteen-key set (§3.3), warn-and-skip per key.
func ParseColorTable(values map[string]string) (Theme, []Warning) {
	return applyValues("[color] table", values)
}

// themeFile is the bare [color] schema of a theme file (§7.3 north-star): the
// fourteen generic slot keys at TOML top level, no wrapper table. Decoding
// into this struct lets md.Undecoded() surface any unknown keys.
type themeFile struct {
	Section1 string `toml:"section1"`
	Section2 string `toml:"section2"`
	Name1    string `toml:"name1"`
	Name2    string `toml:"name2"`
	Value1   string `toml:"value1"`
	Value2   string `toml:"value2"`
	Error1   string `toml:"error1"`
	Error2   string `toml:"error2"`
	Warning1 string `toml:"warning1"`
	Warning2 string `toml:"warning2"`
	Success1 string `toml:"success1"`
	Success2 string `toml:"success2"`
	Info1    string `toml:"info1"`
	Info2    string `toml:"info2"`
}

func (tf themeFile) toMap() map[string]string {
	return map[string]string{
		string(SlotSection1): tf.Section1,
		string(SlotSection2): tf.Section2,
		string(SlotName1):    tf.Name1,
		string(SlotName2):    tf.Name2,
		string(SlotValue1):   tf.Value1,
		string(SlotValue2):   tf.Value2,
		string(SlotError1):   tf.Error1,
		string(SlotError2):   tf.Error2,
		string(SlotWarning1): tf.Warning1,
		string(SlotWarning2): tf.Warning2,
		string(SlotSuccess1): tf.Success1,
		string(SlotSuccess2): tf.Success2,
		string(SlotInfo1):    tf.Info1,
		string(SlotInfo2):    tf.Info2,
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
	// Unknown top-level keys: a theme file is the bare fourteen-slot schema.
	for _, key := range md.Undecoded() {
		warns = append(warns, Warning{
			Source: source,
			Detail: fmt.Sprintf("unknown key %q; a theme file uses only the fourteen color slots (this key is skipped, the rest still applies)", key.String()),
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

// ThemePresence is one name in the theme roster and where it resolves from.
// Both true means the user file shadows the bundled preset (C4).
type ThemePresence struct {
	Name    string
	Bundled bool
	User    bool
}

// ListThemes enumerates the theme roster the loader resolves against: the
// bundled presets unioned with the .toml basenames in the user themes dir,
// sorted by name. An unreadable user dir contributes nothing — the same
// best-effort posture as the remedy list.
func ListThemes(userThemesDir string) []ThemePresence {
	set := map[string]*ThemePresence{}
	for _, n := range BundledThemes() {
		set[n] = &ThemePresence{Name: n, Bundled: true}
	}
	if userThemesDir != "" {
		if entries, err := os.ReadDir(userThemesDir); err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
					continue
				}
				n := strings.TrimSuffix(e.Name(), ".toml")
				if p, ok := set[n]; ok {
					p.User = true
				} else {
					set[n] = &ThemePresence{Name: n, User: true}
				}
			}
		}
	}
	out := make([]ThemePresence, 0, len(set))
	for _, p := range set {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// availableThemes is the union of bundled preset names and any .toml basenames
// in the user themes dir, sorted and de-duplicated — the remedy list.
func availableThemes(userThemesDir string) []string {
	presences := ListThemes(userThemesDir)
	out := make([]string, len(presences))
	for i, p := range presences {
		out[i] = p.Name
	}
	return out
}
