package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/ui"
)

// newVersionCmd builds the version leaf (§2.1): prints the version to stdout.
// Lightweight — it skips the heavy load entirely.
func (e *env) newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		GroupID: groupAlso,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e.pr().Out().Println(e.version)
			return nil
		},
	}
	return cmd
}

// newSnippetCmd builds the snippet group (§2.4). Bare group prints its help.
func (e *env) newSnippetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "snippet",
		GroupID: groupGroups,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	rc := &cobra.Command{
		Use:  "rc",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// SnippetRC is compiled-in text and cannot fail (A4); no heavy load.
			res := (&ops.App{}).SnippetRC()
			e.pr().Out().Printf("%s", res.Text)
			return nil
		},
	}
	cmd.AddCommand(rc)
	return cmd
}

// newThemeCmd builds the theme group (§2.4): list enumerates the roster, slots
// describes the vocabulary, emit renders colors — the effective stack bare, a
// named theme by ref, slot=value overrides on top — and emits them for machines
// via --format env|toml.
func (e *env) newThemeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "theme",
		GroupID: groupGroups,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(e.newThemeListCmd(), e.newThemeSlotsCmd(), e.newThemeEmitCmd())
	return cmd
}

// newThemeListCmd builds theme list: the roster of names the loader resolves —
// bundled presets and the user themes dir — with origin and the active marker.
func (e *env) newThemeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "list",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			global, warnings, err := e.loadGlobal()
			e.renderWarnings(warnings)
			if err != nil {
				return err
			}
			res := (&ops.App{Global: global}).ThemeList()
			out := e.pr().Out()
			rows := make([]tableRow, 0, len(res.Rows))
			for _, row := range res.Rows {
				origin := "bundled" // origin styling deferred (Rocne, 2026-07-19)
				switch {
				case row.Bundled && row.User:
					origin = "user (shadows bundled)"
				case row.User:
					origin = "user"
				}
				if row.Active {
					origin += "  (active)"
				}
				rows = append(rows, tableRow{name: row.Name, styled: out.Style(ui.RoleName, row.Name), rest: origin})
			}
			e.renderNameTable("Theme Name", "Source", rows)
			return nil
		},
	}
}

// newThemeSlotsCmd builds theme slots: the slot vocabulary reference (#116) —
// all fourteen generic slots in canonical §3.3 order, each name rendered in its
// own effective style (a live swatch), with what it colors and its stage-2
// consumers. The long help carries the value-grammar enumeration; --json emits
// the reference for machines.
func (e *env) newThemeSlotsCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:  "slots",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// The slot names render in their own effective style, so the full
			// §7.3 stack is composed just as the bare emit view does.
			global, warnings, err := e.loadGlobal()
			if err != nil {
				e.renderWarnings(warnings)
				return err
			}
			effective := e.composeStack(global, &warnings)
			e.renderWarnings(warnings)

			res := (&ops.App{}).ThemeSlots()

			if asJSON {
				return e.writeJSON(slotsJSON(res))
			}

			// Each slot name renders in its own effective style — a live swatch.
			out := e.pr().Out()
			rows := make([]tableRow, 0, len(res.Rows))
			for _, row := range res.Rows {
				st := effective[ui.Slot(row.Slot)]
				rows = append(rows, tableRow{name: row.Slot, styled: out.StyleWith(st, row.Slot), rest: row.Description})
			}
			e.renderNameTable("Slot", "Description", rows)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Machine-readable slot reference")
	return cmd
}

// newThemeEmitCmd builds theme emit. Operands: at most one bare ref (a theme
// name or path; absent = the effective §7.3 stack) plus any number of
// slot=value overrides, layered on top. The default output renders each slot's
// value in its own style; --format env|toml emits for machines.
func (e *env) newThemeEmitCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:               "emit [theme] [slot=value ...]",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeThemeEmit,
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := parseColorFormat(format)
			if err != nil {
				return &usageError{err}
			}
			ref := ""
			overrides := ui.Theme{}
			for _, arg := range args {
				if !strings.Contains(arg, "=") {
					if ref != "" {
						return &usageError{fmt.Errorf("at most one theme operand: got %q and %q", ref, arg)}
					}
					ref = arg
					continue
				}
				slot, st, aerr := ui.ParseSlotAssignment(arg)
				if aerr != nil {
					return &usageError{aerr}
				}
				overrides[slot] = st
			}

			var effective ui.Theme
			var warnings []ops.Warning
			if ref == "" {
				global, ws, lerr := e.loadGlobal()
				warnings = ws
				if lerr != nil {
					e.renderWarnings(warnings)
					return lerr
				}
				effective = e.composeStack(global, &warnings)
			}

			res, err := (&ops.App{}).ThemeEmit(ref, effective, overrides, f)
			if err != nil {
				e.renderWarnings(warnings)
				return err
			}
			warnings = append(warnings, res.Warnings...)
			e.renderWarnings(warnings)

			switch f {
			case ops.ColorFormatRendered:
				out := e.pr().Out()
				for _, slot := range ui.Slots() {
					st, ok := res.Theme[slot]
					if !ok {
						continue
					}
					value, verr := ui.EmitColorValue(st)
					if verr != nil {
						return verr
					}
					if value == "" {
						value = "normal"
					}
					out.Printf("%-17s %s\n", string(slot), out.StyleWith(st, value))
				}
			case ops.ColorFormatEnv:
				e.pr().Out().Printf("%s\n", res.Text)
			default:
				e.pr().Out().Printf("%s", res.Text)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "", "Emit for machines: env (packed DSTOW_COLORS) or toml (theme file); default is the rendered view")
	return cmd
}

// completeThemeEmit completes theme emit operands (A20, best-effort-silent):
// theme names (only while no bare ref is present yet) and slot= override
// stems.
func completeThemeEmit(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var out []string
	hasRef := false
	for _, a := range args {
		if !strings.Contains(a, "=") {
			hasRef = true
		}
	}
	if !hasRef {
		for _, p := range ui.ListThemes(config.UserThemesDir()) {
			out = append(out, p.Name)
		}
	}
	for _, slot := range ui.Slots() {
		out = append(out, string(slot)+"=")
	}
	return out, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
}

// parseColorFormat maps the --format value to an ops.ColorFormat. Absent means
// the rendered human view; a format flag never changes the concept (§2.4).
func parseColorFormat(f string) (ops.ColorFormat, error) {
	switch f {
	case "":
		return ops.ColorFormatRendered, nil
	case "env":
		return ops.ColorFormatEnv, nil
	case "toml":
		return ops.ColorFormatTOML, nil
	default:
		return ops.ColorFormatRendered, fmt.Errorf("invalid --format value %q: use env or toml", f)
	}
}

// newNameCmd builds the hidden name group (§1.5): encode/decode, absent from
// top-level help but documented in the manual.
func (e *env) newNameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "name",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	encode := &cobra.Command{
		Use:    "encode <segment>",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e.pr().Out().Println(name.Encode(args[0]))
			return nil
		},
	}
	decode := &cobra.Command{
		Use:    "decode <segment>",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dec, err := name.Decode(args[0])
			if err != nil {
				return err
			}
			e.pr().Out().Println(dec)
			return nil
		},
	}
	cmd.AddCommand(encode, decode)
	return cmd
}
