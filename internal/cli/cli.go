// Package cli is dstow's command-line front end and composition root (A1/A2):
// the only consumer that turns the app core's data into a cobra CLI, renders
// results through the ui printer, and maps typed domain errors to exit codes.
// Every other package returns data; cli owns the streams-facing surface —
// command wiring, help text, the §1.4 error wording, and the A3 exit-code map.
package cli

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow"
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
		version: version,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
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
		Use: "dstow",
		// Short, Long, and Example are absent by design: applyHelpDocs assigns
		// them, and every other command's, from docs/commands/ once the tree is
		// built (§2.3/§2.4).
		//
		// Version enables cobra's root --version flag — the D30 contract
		// (release-ci#15): the installer's ensure-check and the dry-run's
		// assert-version-contract.sh both parse `dstow --version` line 1. The
		// template below makes it print exactly what `dstow version` prints.
		Version:       e.version,
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
	root.SetVersionTemplate("{{.Version}}\n")

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
		// H7: a write command refuses from inside a hook (DESIGN §5). The check
		// sits here because it is the one point every command passes through, and
		// after `entered` so the refusal renders as a domain error (exit 3), not
		// as a cobra usage failure (exit 2).
		return refuseInHook(cmd)
	}

	pf := root.PersistentFlags()
	pf.StringVar(&e.colorWhen, "color", "", "Colorize output: auto (default), always, never")
	pf.BoolVarP(&e.quiet, "quiet", "q", false, `Suppress informational output (announcements survive)`)
	pf.BoolVarP(&e.yes, "yes", "y", false, `Assume "yes" at confirmation prompts`)
	// Defining the help and version flags ourselves gives them the canonical
	// wording; cobra sees them and adds no differently-worded twins.
	pf.BoolP("help", "h", false, "Help for dstow or any command")
	root.Flags().BoolP("version", "v", false, "Print version")

	// One help func for the whole tree: cobra generates each command's help
	// from its own definition (Long, Example, groups, flags — the same surface
	// the parser runs), and cli styles the generated text through the ui slots
	// (A2 as amended — issue #96).
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		f := e.helpPrinter().Out()
		f.Printf("%s", styleHelp(f, helpText(cmd)))
	})

	// The §2.3 sections are cobra command groups, in canonical order.
	root.AddGroup(
		&cobra.Group{ID: groupDeploy, Title: "Deploy:"},
		&cobra.Group{ID: groupInspect, Title: "Inspect:"},
		&cobra.Group{ID: groupMaintain, Title: "Maintain:"},
		&cobra.Group{ID: groupGroups, Title: "Groups:"},
		&cobra.Group{ID: groupAlso, Title: "Also:"},
	)
	root.AddCommand(
		e.newStowCmd("stow"),
		e.newStowCmd("unstow"),
		e.newStowCmd("restow"),
		e.newAdoptCmd(),
		e.newListCmd(),
		e.newInfoCmd(),
		e.newStatusCmd(),
		e.newCheckCmd(),
		e.newCleanCmd(),
		e.newRebuildCmd(),
		e.newRepoCmd(),
		e.newSnippetCmd(),
		e.newThemeCmd(),
		e.newNameCmd(),
		e.newVersionCmd(),
	)
	// The manual tree is generated from the embedded docs/ (§2.1's carve-out).
	// A malformed tree is a repo defect the unit suite gates, never a state a
	// built binary reaches, so the surface simply goes unwired rather than
	// failing an unrelated command at startup.
	if manual, err := e.newManualCmd(); err == nil {
		root.AddCommand(manual)
	}
	root.SetHelpCommandGroupID(groupAlso)
	// Materialize cobra's completion command now so it carries dstow's wording
	// and sits in its §2.3 section. It is one of the built-ins the docs
	// derivation skips (§2.4), so its Short is owned here rather than by a page.
	root.InitDefaultCompletionCmd()
	for _, c := range root.Commands() {
		if c.Name() == "completion" {
			c.Short = "Generate shell completion (bash, zsh, fish, powershell)"
			c.GroupID = groupAlso
		}
	}
	// Help text comes from docs/commands/, in one post-pass over the finished
	// tree (§2.3/§2.4): the pages are the single owner, and the mapping is a
	// property of the tree's shape, so it runs once here rather than in fifteen
	// constructors. Same posture as the manual tree above — a malformed or
	// incomplete docs/ tree is a repo defect the unit suite gates, never a state
	// a built binary reaches, so checking it at startup would only ship users a
	// broken binary in exchange for nothing.
	_ = applyHelpDocs(dstow.Manual, root)
	return root
}

// Command listings keep §2.3's canonical order (AddCommand order), not
// cobra's alphabetical sort.
func init() {
	cobra.EnableCommandSorting = false
}

// The §2.3 section ids (cobra group ids), in canonical order.
const (
	groupDeploy   = "deploy"
	groupInspect  = "inspect"
	groupMaintain = "maintain"
	groupGroups   = "groups"
	groupAlso     = "also"
)

// baseTheme composes the no-config theme: DSTOW_COLORS (env) over the default
// palette. Lightweight commands (version, name, completion) render against it;
// heavy commands recompose the full §7.3 stack once global config is loaded.
func baseTheme() ui.Theme {
	env, _ := ui.ParseDSTOWColors(dstowColorsEnv())
	return ui.DeriveTiers(ui.ComposeTheme(env, ui.DefaultPalette()))
}
