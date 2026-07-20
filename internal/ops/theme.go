package ops

import (
	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/ui"
)

// ColorFormat selects how ThemeEmit serializes its result (§2.4 theme). The
// zero value is the rendered human view (cli styles each slot from the result
// theme); a format flag never changes the concept (the --json precedent), so
// cli spells the flag and hands the choice in.
type ColorFormat int

const (
	// ColorFormatRendered is the default human view: cli renders Theme, one
	// slot per line, each value in its own style.
	ColorFormatRendered ColorFormat = iota
	// ColorFormatEnv packs the theme as a DSTOW_COLORS string.
	ColorFormatEnv
	// ColorFormatTOML emits the theme-file TOML schema.
	ColorFormatTOML
)

// ThemeRow is one roster entry for theme list: the name, where it resolves
// from (both origins true = the user file shadows the bundled preset, C4),
// and whether the global theme config key names it.
type ThemeRow struct {
	Name    string
	Bundled bool
	User    bool
	Active  bool
}

// ThemeListResult is the theme roster as data (A4). Rows are sorted by name.
type ThemeListResult struct {
	Rows []ThemeRow
}

// ThemeList enumerates the themes the loader can resolve by bare name (§2.4
// theme list): the bundled presets unioned with the user themes dir. Active
// marks the row the global theme config key names; a path-form theme key
// marks no row (it resolves outside the named roster). Requires a.Global.
func (a *App) ThemeList() *ThemeListResult {
	active := ""
	if a.Global != nil {
		if ref, err := a.Global.Theme(); err == nil {
			active = ref
		}
	}
	presences := ui.ListThemes(config.UserThemesDir())
	rows := make([]ThemeRow, len(presences))
	for i, p := range presences {
		rows[i] = ThemeRow{Name: p.Name, Bundled: p.Bundled, User: p.User, Active: p.Name == active}
	}
	return &ThemeListResult{Rows: rows}
}

// ThemeEmitResult is a theme's colors as data (A4): the ref asked for ("" =
// the effective stack), the format chosen, the resulting slot→style map (what
// cli renders for the default view), the serialized text for the machine
// formats, and any warn-and-skip diagnostics the theme load raised.
type ThemeEmitResult struct {
	Ref      string
	Format   ColorFormat
	Theme    ui.Theme
	Text     string
	Warnings []Warning
}

// ThemeEmit resolves the colors to emit (§2.4 theme emit, A5) and serializes
// them per format. A named ref loads through ui's single theme loader — a
// path, a user preset, or a bundled preset — and shows the theme AS LOADED
// (its declared slots); an empty ref shows the effective §7.3 stack, which
// the caller composes and hands in (cli is the stack's one composition owner;
// all fourteen slots present). Overrides layer on top, first — the top of the
// stack, like DSTOW_COLORS. A missing or unreadable theme is a refusal
// (error); a malformed slot inside a resolvable theme is a warn-and-skip,
// carried in Warnings.
func (a *App) ThemeEmit(ref string, effective ui.Theme, overrides ui.Theme, format ColorFormat) (*ThemeEmitResult, error) {
	base := effective
	res := &ThemeEmitResult{Ref: ref, Format: format}
	if ref != "" {
		theme, warns, err := ui.LoadTheme(ref, config.UserThemesDir())
		if err != nil {
			return nil, err
		}
		res.Warnings = warnUI(warns)
		base = theme
	}
	res.Theme = ui.ComposeTheme(overrides, base)

	switch format {
	case ColorFormatTOML:
		text, err := ui.EmitThemeTOML(res.Theme)
		if err != nil {
			return nil, err
		}
		res.Text = text
	case ColorFormatEnv:
		res.Text = ui.PackDSTOWColors(res.Theme)
	}
	return res, nil
}

// ThemeSlotRow is one slot in the theme slots reference (#116): the slot name,
// what it colors, and the stage-2 roles that render through it. cli styles Name
// in the slot's own effective style — the name doubles as a live swatch (the
// theme list precedent).
type ThemeSlotRow struct {
	Slot        string
	Description string
	Consumers   []string
}

// ThemeSlotsResult is the slot reference as data (A4): all fourteen slots in
// canonical §3.3 order.
type ThemeSlotsResult struct {
	Rows []ThemeSlotRow
}

// ThemeSlots returns the fourteen-slot vocabulary reference (§2.4 theme slots,
// #116): each generic slot, what it colors, and its consumers — sourced from
// ui's code-owned Role mapping so the reference cannot drift from it. It is pure
// vocabulary; no config or repo set is needed.
func (a *App) ThemeSlots() *ThemeSlotsResult {
	docs := ui.SlotReference()
	rows := make([]ThemeSlotRow, len(docs))
	for i, d := range docs {
		cons := make([]string, len(d.Consumers))
		for j, r := range d.Consumers {
			cons[j] = string(r)
		}
		rows[i] = ThemeSlotRow{Slot: string(d.Slot), Description: d.Description, Consumers: cons}
	}
	return &ThemeSlotsResult{Rows: rows}
}

// warnUI converts ui warnings (Source + Detail, no Fix) into ops warnings.
func warnUI(ws []ui.Warning) []Warning {
	out := make([]Warning, 0, len(ws))
	for _, w := range ws {
		out = append(out, Warning{Source: w.Source, Detail: w.Detail})
	}
	return out
}
