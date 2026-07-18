package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/ops"
)

// newVersionCmd builds the version leaf (§2.1): prints the version to stdout.
// Lightweight — it skips the heavy load entirely.
func (e *env) newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e.pr().Out().Println(e.version)
			return nil
		},
	}
	staticHelp(cmd, "Print dstow's version.\n\nUsage:\n  dstow version\n")
	return cmd
}

// newSnippetCmd builds the snippet group (§2.4). Bare group prints its help.
func (e *env) newSnippetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snippet",
		Short: firstLine(snippetHelp),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	staticHelp(cmd, snippetHelp)

	rc := &cobra.Command{
		Use:   "rc",
		Short: "Print the shell-rc bootstrap snippet",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// SnippetRC is compiled-in text and cannot fail (A4); no heavy load.
			res := (&ops.App{}).SnippetRC()
			e.pr().Out().Printf("%s", res.Text)
			return nil
		},
	}
	staticHelp(rc, snippetHelp)
	cmd.AddCommand(rc)
	return cmd
}

// newColorsCmd builds the colors group (§2.4). theme emits a named theme as a
// packed DSTOW_COLORS string (default) or a theme file (--format toml).
func (e *env) newColorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "colors",
		Short: firstLine(colorsHelp),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	staticHelp(cmd, colorsHelp)

	var format string
	theme := &cobra.Command{
		Use:   "theme",
		Short: "Emit a named theme",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := parseColorFormat(format)
			if err != nil {
				return &usageError{err}
			}
			res, err := (&ops.App{}).ColorsTheme(args[0], f)
			if err != nil {
				return err
			}
			e.renderWarnings(res.Warnings)
			e.pr().Out().Printf("%s", res.Text)
			if f == ops.ColorFormatEnv {
				e.pr().Out().Printf("\n")
			}
			return nil
		},
	}
	staticHelp(theme, colorsHelp)
	theme.Flags().StringVar(&format, "format", "env", "Output format: env (packed DSTOW_COLORS) or toml (theme file)")
	cmd.AddCommand(theme)
	return cmd
}

// parseColorFormat maps the --format value to an ops.ColorFormat.
func parseColorFormat(f string) (ops.ColorFormat, error) {
	switch f {
	case "env", "":
		return ops.ColorFormatEnv, nil
	case "toml":
		return ops.ColorFormatTOML, nil
	default:
		return ops.ColorFormatEnv, fmt.Errorf("invalid --format value %q: use env or toml", f)
	}
}

// newNameCmd builds the hidden name group (§1.5): encode/decode, absent from
// top-level help but documented in the manual.
func (e *env) newNameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "name",
		Short:  "Encode and decode name coordinate segments",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	encode := &cobra.Command{
		Use:    "encode",
		Short:  "Percent-encode a coordinate segment",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e.pr().Out().Println(name.Encode(args[0]))
			return nil
		},
	}
	decode := &cobra.Command{
		Use:    "decode",
		Short:  "Percent-decode a coordinate segment",
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
