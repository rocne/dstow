// Package cli is dstow's command-line front end and composition root (A1/A2):
// the only consumer that turns the app core's data into a cobra CLI, renders
// results through the ui printer, and maps typed domain errors to exit codes.
// Every other package returns data; cli owns the streams-facing surface —
// command wiring, help text, the §1.4 error wording, and the A3 exit-code map.
package cli

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow/internal/ui"
)

// env is the composition root's shared state: the injected streams and
// version, the resolved persistent flags, the one printer, and the memoized
// heavy load. It threads through every command constructor so nothing lives in
// a package-level global (A1: constructor-injected, no command globals).
type env struct {
	version string
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer

	// Persistent (global) flags, bound on the root command.
	colorWhen string // --color: auto | always | never
	quiet     bool   // -q/--quiet
	yes       bool   // -y/--yes

	printer *ui.Printer

	// entered flips true once a command body runs — i.e. once flag parsing and
	// argument validation have passed (they run before PersistentPreRunE). It is
	// the A3 usage-error signal: an error out of Execute with entered still false
	// is a cobra flag/arg/unknown-command error, which maps to exit 2.
	entered bool
	// status is the negative-answer exit code (A3 exit 1) a handler sets when the
	// work succeeded but the answer is "no": a package failed, a requested field
	// is unset, check found findings. Handlers set it and return nil so no
	// error: line is printed over an already-rendered result.
	status int
}

// Run is the entry point cmd/dstow wires against (A2). It builds the cobra
// surface over a fresh env, executes it, and returns the process exit code —
// the one place in dstow that owns exit codes (A3).
func Run(args []string, version string, stdin io.Reader, stdout, stderr io.Writer) int {
	e := &env{
		version:   version,
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
		colorWhen: "auto",
	}

	root := e.newRootCmd()
	root.SetArgs(args[1:])
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	if err := root.Execute(); err != nil {
		return e.finish(err)
	}
	return e.status
}

// newRootCmd builds the root command with the global flags, the canonical help
// wiring, and every subcommand. SilenceUsage and SilenceErrors are set so cobra
// never prints anything itself: cli renders every error through the printer and
// maps it to an exit code (A2).
func (e *env) newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "dstow",
		Short:         "deploy dotfiles and configuration as symlinks",
		SilenceUsage:  true,
		SilenceErrors: true,
		// The bare invocation prints the top-level help on stdout (A2: help is
		// the requested data), exit 0.
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		// A leaf/group that takes no operands still errors on stray args; the
		// root takes none of its own (subcommands only).
		Args: cobra.NoArgs,
	}
	root.Annotations = map[string]string{helpKey: topLevelHelp}

	// PersistentPreRunE runs for every subcommand (only the root defines one),
	// after flag parsing and arg validation succeed. It builds the printer and
	// marks the command body entered.
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		mode, err := parseColorMode(e.colorWhen)
		if err != nil {
			return err // a bad --color value; entered stays false → usage exit 2
		}
		e.printer = ui.New(ui.Options{
			Stdin:  e.stdin,
			Stdout: e.stdout,
			Stderr: e.stderr,
			Mode:   mode,
			Quiet:  e.quiet,
			// Base theme with no config load: DSTOW_COLORS (env) over the default
			// palette. Heavy commands upgrade this to the full §7.3 stack when they
			// load global config.
			Theme: baseTheme(),
		})
		e.entered = true
		return nil
	}

	pf := root.PersistentFlags()
	pf.StringVar(&e.colorWhen, "color", "auto", "Colorize output: auto (default), always, never")
	pf.BoolVarP(&e.quiet, "quiet", "q", false, `Suppress informational output (announcements survive)`)
	pf.BoolVarP(&e.yes, "yes", "y", false, `Assume "yes" at confirmation prompts`)

	// One help func for the whole tree: it prints each command's canonical help
	// text (stored in Annotations) to stdout, verbatim (A2, §2.3/§2.4).
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if text, ok := cmd.Annotations[helpKey]; ok {
			_, _ = io.WriteString(cmd.OutOrStdout(), text)
			return
		}
		// Commands without pinned text (cobra's built-in completion/help) keep
		// cobra's default rendering.
		_ = cmd.Usage()
	})

	root.AddCommand(
		e.newStowCmd("stow", stowHelp),
		e.newStowCmd("unstow", unstowHelp),
		e.newStowCmd("restow", restowHelp),
		e.newAdoptCmd(),
		e.newListCmd(),
		e.newInfoCmd(),
		e.newStatusCmd(),
		e.newCheckCmd(),
		e.newCleanCmd(),
		e.newRebuildCmd(),
		e.newRepoCmd(),
		e.newSnippetCmd(),
		e.newColorsCmd(),
		e.newNameCmd(),
		e.newVersionCmd(),
	)
	return root
}

// helpKey names the Annotations entry carrying a command's canonical help text.
const helpKey = "dstow_help"

// staticHelp attaches a canonical help string to a command so the shared help
// func prints it verbatim.
func staticHelp(cmd *cobra.Command, text string) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[helpKey] = text
}

// baseTheme composes the no-config theme: DSTOW_COLORS (env) over the default
// palette. Lightweight commands (version, name, completion) render against it;
// heavy commands recompose the full §7.3 stack once global config is loaded.
func baseTheme() ui.Theme {
	env, _ := ui.ParseDSTOWColors(dstowColorsEnv())
	return ui.ComposeTheme(env, ui.DefaultPalette())
}
