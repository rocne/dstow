package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/git"
	"github.com/rocne/dstow/internal/hooks"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/repo"
	"github.com/rocne/dstow/internal/ui"
)

// parseColorMode maps a --color <when> value to a ui.ColorMode. The value is
// required (no bare --color, no --no-color; O6) and must be one of the three
// words; anything else is a usage error.
func parseColorMode(when string) (ui.ColorMode, error) {
	switch when {
	// The flag's declared default is "" so pflag prints no default suffix in
	// help; absent means auto (§7.2 O6).
	case "auto", "":
		return ui.ColorAuto, nil
	case "always":
		return ui.ColorAlways, nil
	case "never":
		return ui.ColorNever, nil
	default:
		return ui.ColorAuto, fmt.Errorf("invalid --color value %q: use auto, always, or never", when)
	}
}

// dstowColorsEnv reads DSTOW_COLORS at point of use (A2). It is the only
// theming environment variable dstow reads (§7.3).
func dstowColorsEnv() string { return os.Getenv("DSTOW_COLORS") }

// load builds the ops.App the heavy commands run against, once per invocation
// (A1): global config, the registry, and the DSTOW_PATH session repos compose
// into the repo set; the ledger path, global dir, hook streams, prompter, and
// git port complete the environment. It also upgrades the printer's theme to
// the full §7.3 stack now that global config is available. Every composition
// diagnostic comes back as a warning value for the caller to render.
//
// assumeYes selects whether this command's confirmations honor -y/--yes: only
// commands whose prompts are confirmations of stated intent (clean, repo add)
// pass it through; guard prompts (adopt, repo remove) never do (§2.2).
func (e *env) load(assumeYes bool) (*ops.App, []ops.Warning, error) {
	var warnings []ops.Warning

	global, gwarns, err := config.LoadGlobal()
	warnings = appendConfigWarnings(warnings, gwarns)
	if err != nil {
		return nil, warnings, err
	}

	reg, rwarns, err := repo.LoadRegistry(config.RegistryFile())
	warnings = appendRepoWarnings(warnings, rwarns)
	if err != nil {
		return nil, warnings, err
	}

	sessionDirs, dwarns, derr := config.ParseDSTOWPath(os.Getenv("DSTOW_PATH"))
	warnings = appendConfigWarnings(warnings, dwarns)
	if derr != nil {
		return nil, warnings, derr
	}
	// A migrated ~/.stowrc --dir contributes a session repo too (C19).
	if dir := global.SessionRepoDir(); dir != "" {
		sessionDirs = append(sessionDirs, dir)
	}

	repos := repo.BuildSet(reg.Sources, sessionDirs)

	// Upgrade the printer theme to the full stack (§7.3): DSTOW_COLORS over the
	// [color] table over the theme key over the default palette. A theme-load
	// failure degrades to a warning, never a refusal — a bad theme key must not
	// kill every command.
	e.upgradeTheme(global, &warnings)

	app := &ops.App{
		Global:     global,
		Repos:      repos,
		LedgerPath: ledger.Path(),
		GlobalDir:  config.GlobalConfigDir(),
		Hooks:      hooks.Runner{Stdin: e.stdin, Stderr: e.stderr},
		Prompt:     e.newPrompter(assumeYes),
		Now:        func() time.Time { return time.Now() },
		Version:    e.version,
		Git:        git.Exec{},
	}
	return app, warnings, nil
}

// loadGlobal is the light loader for commands that need only global config
// (the theme verbs): no registry, no session repos, no App environment.
func (e *env) loadGlobal() (*config.GlobalLevel, []ops.Warning, error) {
	global, gwarns, err := config.LoadGlobal()
	return global, appendConfigWarnings(nil, gwarns), err
}

// composeStack composes the full §7.3 theming stack — DSTOW_COLORS over the
// [color] table over the theme key over the default palette — the one
// composition owner both the printer upgrade and theme show's effective view
// use. A theme-load failure degrades to a warning, never a refusal.
func (e *env) composeStack(global *config.GlobalLevel, warnings *[]ops.Warning) ui.Theme {
	envTheme, ewarns := ui.ParseDSTOWColors(dstowColorsEnv())
	*warnings = appendUIWarnings(*warnings, ewarns)

	tableTheme, twarns := ui.ParseColorTable(global.ColorTable())
	*warnings = appendUIWarnings(*warnings, twarns)

	var keyTheme ui.Theme
	if ref, terr := global.Theme(); terr != nil {
		*warnings = append(*warnings, ops.Warning{Source: config.GlobalConfigFile(), Detail: terr.Error()})
	} else if ref != "" {
		t, tw, lerr := ui.LoadTheme(ref, config.UserThemesDir())
		*warnings = appendUIWarnings(*warnings, tw)
		if lerr != nil {
			*warnings = append(*warnings, ops.Warning{
				Source: config.GlobalConfigFile(),
				Detail: fmt.Sprintf("theme %q could not be loaded: %v; falling back to the default palette", ref, lerr),
			})
		} else {
			keyTheme = t
		}
	}

	return ui.DeriveTiers(ui.ComposeTheme(envTheme, tableTheme, keyTheme, ui.DefaultPalette()))
}

// upgradeTheme recomposes the printer with the full §7.3 theming stack. Color
// enablement is unchanged (it was fixed upstream of theming); only which style
// each slot maps to is refined by the config layers.
func (e *env) upgradeTheme(global *config.GlobalLevel, warnings *[]ops.Warning) {
	theme := e.composeStack(global, warnings)

	mode, _ := parseColorMode(e.colorWhen) // already validated in PreRun
	e.printer = ui.New(ui.Options{
		Stdin:  e.stdin,
		Stdout: e.stdout,
		Stderr: e.stderr,
		Mode:   mode,
		Quiet:  e.quiet,
		Theme:  theme,
	})
}

// appendConfigWarnings folds config warnings into the ops.Warning stream.
func appendConfigWarnings(dst []ops.Warning, ws []config.Warning) []ops.Warning {
	for _, w := range ws {
		dst = append(dst, ops.Warning{Source: w.Source, Detail: w.Detail, Fix: w.Fix})
	}
	return dst
}

// appendRepoWarnings folds repo warnings into the ops.Warning stream.
func appendRepoWarnings(dst []ops.Warning, ws []repo.Warning) []ops.Warning {
	for _, w := range ws {
		dst = append(dst, ops.Warning{Source: w.Source, Detail: w.Detail, Fix: w.Fix})
	}
	return dst
}

// appendUIWarnings folds ui theming warnings (Source + Detail, no Fix) into the
// ops.Warning stream.
func appendUIWarnings(dst []ops.Warning, ws []ui.Warning) []ops.Warning {
	for _, w := range ws {
		dst = append(dst, ops.Warning{Source: w.Source, Detail: w.Detail})
	}
	return dst
}
