package cli

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// This file extracts help text from the manual's markdown. A command's page in
// docs/commands/ is simultaneously its manual page and the source of its help
// text: `dstow manual commands repo add` and the source of `dstow repo add
// --help` are the same file, which is what makes docs/commands/ the single
// owner of help content.
//
// Extraction is by namespaced comment tag with an explicit close, never by
// section name or position. Heading names are then a pure prose decision with
// zero mechanical consequence, and authors may restructure a page freely; an
// implicit boundary ("until the next heading") would reintroduce exactly the
// positional coupling this design removes. Tags are not stripped from manual
// output — they stay visible, self-describing to the agent audience, and the
// manual keeps printing the file verbatim.

// helpDoc is one command's help content, as extracted from its markdown page.
// Flags are absent by design: cobra generates the flag roster from the flag
// definitions, already single-owner in code, and deriving them here would
// reintroduce two owners.
type helpDoc struct {
	Short    string // cobra Short — the one-line description
	Long     string // cobra Long — the prose body
	Examples string // cobra Example — the example block, indentation intact
}

// The tag vocabulary. An unrecognized dstow: tag is an error rather than a
// silent no-op: a typo would otherwise drop a command's help text with nothing
// to notice it, and the namespace exists precisely so these are greppable.
const (
	tagShort    = "short"
	tagLong     = "long"
	tagExamples = "examples"
)

// tagRe matches one tag, open or close: the leading slash is the close marker.
// Whitespace inside the comment is tolerated so an author's formatter cannot
// break extraction.
var tagRe = regexp.MustCompile(`<!--\s*(/?)dstow:([A-Za-z-]+)\s*-->`)

// parseHelpDoc extracts the tagged regions of a command's markdown page.
// Content comes back with surrounding blank lines trimmed but interior
// indentation intact — example blocks are indented, and that indentation is
// content.
func parseHelpDoc(source, text string) (helpDoc, error) {
	found := map[string]string{}
	openTag, openEnd := "", 0

	for _, m := range tagRe.FindAllStringSubmatchIndex(text, -1) {
		closing := text[m[2]:m[3]] == "/"
		tag := text[m[4]:m[5]]

		switch tag {
		case tagShort, tagLong, tagExamples:
		default:
			return helpDoc{}, fmt.Errorf("%s: unknown help tag %q (known: %s, %s, %s)",
				source, tag, tagShort, tagLong, tagExamples)
		}

		if !closing {
			if openTag != "" {
				return helpDoc{}, fmt.Errorf("%s: help tag %q opened inside %q (regions never nest)", source, tag, openTag)
			}
			if _, dup := found[tag]; dup {
				return helpDoc{}, fmt.Errorf("%s: help tag %q opened twice (one region per tag)", source, tag)
			}
			openTag, openEnd = tag, m[1]
			continue
		}

		if openTag == "" {
			return helpDoc{}, fmt.Errorf("%s: help tag %q closed but never opened", source, tag)
		}
		if tag != openTag {
			return helpDoc{}, fmt.Errorf("%s: help tag %q closed by %q", source, openTag, tag)
		}
		found[openTag] = strings.Trim(text[openEnd:m[0]], "\n")
		openTag = ""
	}
	if openTag != "" {
		return helpDoc{}, fmt.Errorf("%s: help tag %q is never closed (closes are explicit)", source, openTag)
	}

	return helpDoc{
		Short:    found[tagShort],
		Long:     found[tagLong],
		Examples: found[tagExamples],
	}, nil
}

// commandsDir is the docs/ subtree that mirrors the command tree (§2.4): the
// page a command derives its help from is the page the manual prints for it.
const commandsDir = manualDir + "/commands"

// helpDocExempt names the subtrees the derivation skips whole. cobra's
// built-ins carry cobra's own text, and every node of the manual tree takes its
// Short from its file's first H1 (§2.1's two-Shorts rule) — demanding pages for
// those would hand them a second source, which is the condition this design
// removes.
var helpDocExempt = map[string]bool{"completion": true, "help": true, "manual": true}

// helpDocPath is the page a command's help comes from: its position in the
// command tree, mirrored into docs/commands/. A command with subcommands takes
// its directory's index.md — the same file the manual prints for a directory
// node — and a leaf takes a file named for it. Command names map to path
// segments by identity, so the file behind any help text is readable straight
// off the command line.
func helpDocPath(cmd *cobra.Command) string {
	var segs []string
	for c := cmd; c.HasParent(); c = c.Parent() {
		segs = append(segs, c.Name())
	}
	slices.Reverse(segs) // walked child-to-root, the path reads root-to-child
	p := path.Join(append([]string{commandsDir}, segs...)...)
	if cmd.HasSubCommands() {
		return path.Join(p, manualIndex)
	}
	return p + manualExt
}

// applyHelpDocs assigns Short, Long, and Example to every dstow-defined command
// in the tree from its page. It is one post-pass over the built tree rather
// than a lookup in each constructor: the page's location is a property of the
// tree's shape, so it is derivable exactly where the shape exists, and no
// command constructor has to know that docs/ exists.
//
// Assignment is unconditional — docs/commands/ is the single owner, so a value
// left in a constructor would be a competing one, not a fallback.
func applyHelpDocs(fsys fs.FS, root *cobra.Command) error {
	var walk func(cmd *cobra.Command) error
	walk = func(cmd *cobra.Command) error {
		file := helpDocPath(cmd)
		content, err := fs.ReadFile(fsys, file)
		if err != nil {
			return fmt.Errorf("help: read %s: %w", file, err)
		}
		doc, err := parseHelpDoc(file, string(content))
		if err != nil {
			return err
		}
		cmd.Short, cmd.Long, cmd.Example = doc.Short, doc.Long, doc.Examples
		for _, child := range cmd.Commands() {
			if helpDocExempt[child.Name()] {
				continue
			}
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root)
}
