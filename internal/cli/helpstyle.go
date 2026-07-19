package cli

import (
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow/internal/ui"
)

// Help rendering (A2 as amended — issue #96): cobra generates the help from
// each command's own definition (Long, Example, groups, flags — one surface
// with the parser), and cli styles the generated text through the ui slots
// (§7.2: heading / name / muted only). Styling is strictly presentational:
// with color disabled every Style call is the identity, so the emitted bytes
// are exactly the generated text (O11 strip contract).

// helpText composes a command's help the way cobra's default help template
// does — Long (or Short) followed by the cobra-generated usage sections.
func helpText(cmd *cobra.Command) string {
	long := strings.TrimRight(cmd.Long, "\n")
	if long == "" {
		long = strings.TrimRight(cmd.Short, "\n")
	}
	usage := cmd.UsageString()
	if long == "" {
		return usage
	}
	return long + "\n\n" + usage
}

// Line grammar of the generated help, for slot mapping.
var (
	// A section heading: an unindented, short line ending in ":" —
	// "Usage:", "Flags:", "Examples:", the §2.3 group titles, and content
	// headings inside Long prose ("Environment:", "Exit status:").
	headingRe = regexp.MustCompile(`^[A-Za-z][A-Za-z ]{0,28}:$`)
	// An entry in a command-list section: two-space indent, a name column,
	// a two-plus-space gap, a description.
	entryRe = regexp.MustCompile(`^(  )(\S+)(\s{2,}.*)$`)
	// A flag spec at the head of a flag line, with its optional value name.
	flagRe = regexp.MustCompile(`^(\s+)(-[A-Za-z], --[\w-]+|--[\w-]+|-[A-Za-z])( [a-z][\w-]*)?(\s{2,}.*)$`)
	// A <placeholder> token in a usage line.
	placeholderRe = regexp.MustCompile(`<[^>]+>`)
	// A trailing # comment in an example line.
	commentRe = regexp.MustCompile(`(\s)(#.*)$`)
)

// styleHelp maps the generated help's line grammar onto the ui slots, styling
// against the given face's own enablement. It never adds, drops, or reorders
// a byte of content: strip(styled) == input.
func styleHelp(f *ui.Face, text string) string {
	lines := strings.Split(text, "\n")
	section := ""
	for i, ln := range lines {
		switch {
		case ln == "":
			continue
		case headingRe.MatchString(ln):
			section = ln
			lines[i] = f.Style(ui.SlotHeading, ln)
		default:
			lines[i] = styleLine(f, ln, section)
		}
	}
	return strings.Join(lines, "\n")
}

// styleLine styles one non-heading line according to its section.
func styleLine(f *ui.Face, ln, section string) string {
	switch section {
	case "Usage:":
		return placeholderRe.ReplaceAllStringFunc(ln, func(ph string) string {
			return f.Style(ui.SlotMuted, ph)
		})
	case "Flags:", "Global Flags:":
		if m := flagRe.FindStringSubmatch(ln); m != nil {
			valueName := m[3]
			if valueName != "" {
				valueName = f.Style(ui.SlotMuted, valueName)
			}
			return m[1] + f.Style(ui.SlotName, m[2]) + valueName + m[4]
		}
		return ln
	case "Examples:":
		return commentRe.ReplaceAllStringFunc(ln, func(c string) string {
			gap := c[:1]
			return gap + f.Style(ui.SlotMuted, c[1:])
		})
	case "":
		// Long prose before the first heading stays unstyled.
		return ln
	default:
		// Command-list sections (the §2.3 groups, Available/Additional
		// Commands) and entry-shaped content sections (Environment:, Exit
		// status:): the name column gets the name slot.
		if m := entryRe.FindStringSubmatch(ln); m != nil {
			return m[1] + f.Style(ui.SlotName, m[2]) + m[3]
		}
		return ln
	}
}

// helpPrinter returns the printer help renders through. The help path can run
// before PersistentPreRunE builds the session printer (cobra resolves --help
// during flag parsing), so it builds an equivalent one on demand; an invalid
// --color value falls back to auto here — the run path still refuses it.
func (e *env) helpPrinter() *ui.Printer {
	if e.printer != nil {
		return e.printer
	}
	mode, err := parseColorMode(e.colorWhen)
	if err != nil {
		mode = ui.ColorAuto
	}
	return ui.New(ui.Options{
		Stdin:  e.stdin,
		Stdout: e.stdout,
		Stderr: e.stderr,
		Mode:   mode,
		Quiet:  e.quiet,
		Theme:  baseTheme(),
	})
}
