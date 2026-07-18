package ops

import (
	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/ui"
)

// ColorFormat selects how ColorsTheme serializes a theme (§2.4 colors). The
// zero value is the default env form; a format flag never changes the concept
// (the --json precedent), so cli spells the flag and hands the choice in.
type ColorFormat int

const (
	// ColorFormatEnv packs the theme as a DSTOW_COLORS string (default).
	ColorFormatEnv ColorFormat = iota
	// ColorFormatTOML emits the sixteen-slot theme-file TOML.
	ColorFormatTOML
)

// ColorsThemeResult is the resolved theme as data (A4): the ref asked for, the
// format chosen, the serialized text, and any warn-and-skip diagnostics the
// theme load raised (an unknown key, a bad value). cli writes Text.
type ColorsThemeResult struct {
	Ref      string
	Format   ColorFormat
	Text     string
	Warnings []Warning
}

// ColorsTheme loads a named theme and serializes it (§2.4 colors, A5). It
// resolves the ref through ui's single theme loader — a path, a user preset,
// or a bundled preset — and emits it AS LOADED (its declared slots) in
// canonical order, packed for env (default) or as a theme file for toml.
// A missing or unreadable theme is a refusal (error); a malformed slot inside
// a resolvable theme is a warn-and-skip, carried in Warnings.
func (a *App) ColorsTheme(ref string, format ColorFormat) (*ColorsThemeResult, error) {
	theme, warns, err := ui.LoadTheme(ref, config.UserThemesDir())
	if err != nil {
		return nil, err
	}
	res := &ColorsThemeResult{Ref: ref, Format: format, Warnings: warnUI(warns)}

	switch format {
	case ColorFormatTOML:
		text, eerr := ui.EmitThemeTOML(theme)
		if eerr != nil {
			return nil, eerr
		}
		res.Text = text
	default:
		res.Text = ui.PackDSTOWColors(theme)
	}
	return res, nil
}

// warnUI converts ui warnings (Source + Detail, no Fix) into ops warnings.
func warnUI(ws []ui.Warning) []Warning {
	out := make([]Warning, 0, len(ws))
	for _, w := range ws {
		out = append(out, Warning{Source: w.Source, Detail: w.Detail})
	}
	return out
}
