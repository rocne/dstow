package cli

import (
	"io"
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow"
)

// The extractor is asserted on the properties the tag design exists to buy:
// prose structure carries no mechanical consequence, boundaries are explicit,
// and a malformed page is an error rather than silently missing help text.

func TestParseHelpDocExtractsTaggedRegions(t *testing.T) {
	page := `# stow

<!-- dstow:short -->
Link packages into their targets
<!-- /dstow:short -->

## Overview

<!-- dstow:long -->
Link packages into their targets.

Names are packages or repos, by any unambiguous suffix.
<!-- /dstow:long -->

## Examples

<!-- dstow:examples -->
  dstow stow zsh git tmux
  dstow stow dotfiles              # a repo: all of its packages
<!-- /dstow:examples -->

## Conflicts and remedies

Manual-only prose, invisible to help.
`
	doc, err := parseHelpDoc("docs/commands/stow.md", page)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Short != "Link packages into their targets" {
		t.Errorf("Short = %q", doc.Short)
	}
	if !strings.HasPrefix(doc.Long, "Link packages into their targets.\n\n") ||
		!strings.HasSuffix(doc.Long, "any unambiguous suffix.") {
		t.Errorf("Long = %q, want the region verbatim with its blank line", doc.Long)
	}
	// Example indentation is content: cobra prints the block as given.
	if !strings.HasPrefix(doc.Examples, "  dstow stow zsh git tmux\n") {
		t.Errorf("Examples = %q, want leading indentation preserved", doc.Examples)
	}
	// Untagged prose never reaches help.
	for _, field := range []string{doc.Short, doc.Long, doc.Examples} {
		if strings.Contains(field, "Manual-only prose") {
			t.Errorf("untagged prose leaked into help: %q", field)
		}
	}
}

// Headings and ordering are prose decisions with zero mechanical consequence —
// the property the tags exist to guarantee. The same regions in a different
// structure must extract identically.
func TestParseHelpDocIgnoresProseStructure(t *testing.T) {
	canonical := "# stow\n\n<!-- dstow:short -->\none-liner\n<!-- /dstow:short -->\n\n## Overview\n\n<!-- dstow:long -->\nbody\n<!-- /dstow:long -->\n"
	restructured := "# stow\n\nIntro paragraph nobody asked about.\n\n### Deep heading\n\n<!-- dstow:long -->\nbody\n<!-- /dstow:long -->\n\n#### Another\n\n<!-- dstow:short -->\none-liner\n<!-- /dstow:short -->\n"

	first, err := parseHelpDoc("a.md", canonical)
	if err != nil {
		t.Fatalf("parse canonical: %v", err)
	}
	second, err := parseHelpDoc("b.md", restructured)
	if err != nil {
		t.Fatalf("parse restructured: %v", err)
	}
	if first != second {
		t.Errorf("restructuring the page changed the help text:\n%+v\n%+v", first, second)
	}
}

// Whitespace inside the comment is tolerated so a markdown formatter cannot
// break extraction.
func TestParseHelpDocToleratesTagWhitespace(t *testing.T) {
	doc, err := parseHelpDoc("a.md", "<!--dstow:short-->\nterse\n<!--   /dstow:short   -->\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Short != "terse" {
		t.Errorf("Short = %q, want %q", doc.Short, "terse")
	}
}

// An absent optional region is empty, not an error: examples are not universal
// (a hidden utility leaf needs none), and cobra omits an empty Example block.
func TestParseHelpDocAllowsAbsentRegions(t *testing.T) {
	doc, err := parseHelpDoc("a.md", "# version\n\n<!-- dstow:short -->\nPrint version\n<!-- /dstow:short -->\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Examples != "" || doc.Long != "" {
		t.Errorf("absent regions are not empty: %+v", doc)
	}
}

// A malformed page is an error rather than silently missing help text: the
// failure mode this guards is a command shipping with no description at all.
func TestParseHelpDocRejectsMalformedPages(t *testing.T) {
	cases := map[string]string{
		"never closed":        "<!-- dstow:short -->\nno close\n",
		"closed never opened": "text\n<!-- /dstow:long -->\n",
		"mismatched close":    "<!-- dstow:short -->\nx\n<!-- /dstow:long -->\n",
		"opened twice":        "<!-- dstow:short -->\na\n<!-- /dstow:short -->\n<!-- dstow:short -->\nb\n<!-- /dstow:short -->\n",
		"nested regions":      "<!-- dstow:long -->\n<!-- dstow:short -->\nx\n<!-- /dstow:short -->\n<!-- /dstow:long -->\n",
		"unknown tag":         "<!-- dstow:sohrt -->\ntypo\n<!-- /dstow:sohrt -->\n",
	}
	for name, page := range cases {
		t.Run(name, func(t *testing.T) {
			doc, err := parseHelpDoc("docs/commands/x.md", page)
			if err == nil {
				t.Fatalf("parsed a %s page without error: %+v", name, doc)
			}
			// Every refusal names the file it came from (§1.4): a bare complaint
			// about a tag is useless across a 26-page tree.
			if !strings.Contains(err.Error(), "docs/commands/x.md") {
				t.Errorf("error does not name its source: %v", err)
			}
		})
	}
}

// Ordinary author comments are untouched — the dstow: namespace exists so the
// tags never collide with them.
func TestParseHelpDocIgnoresForeignComments(t *testing.T) {
	doc, err := parseHelpDoc("a.md", "<!-- TODO: rewrite this -->\n<!-- dstow:short -->\nkept\n<!-- /dstow:short -->\n<!-- markdownlint-disable -->\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Short != "kept" {
		t.Errorf("Short = %q", doc.Short)
	}
}

// internalRefs are the spellings that name a document the reader does not have.
// dstow's internal documents live in dev/ and never ship; shipped help that
// cites them sends the reader somewhere they cannot go.
//
// The list is explicit rather than a pattern over dev/: a bare "dev/" also
// matches /dev/null, which is legitimate content in a hooks example.
var internalRefs = []*regexp.Regexp{
	regexp.MustCompile(`§`),                                     // DESIGN/REQUIREMENTS section marks
	regexp.MustCompile(`\b(DESIGN|REQUIREMENTS|CONTEXT)\.md\b`), // the documents themselves
	regexp.MustCompile(`\bdev/`),                                // any path into the internal tree
	regexp.MustCompile(`\b[ABCDHMO][0-9]{1,2}\b`),               // the resolution-ledger codes (A3, C7, M8, H2…)
}

// TestHelpTextCitesNothingInternal asserts the rule ruled at #131: shipped
// user-facing text never cites an internal document. The manual ships inside
// the binary, so a cross-reference has a resolvable form available to it —
// "dstow manual theming values" — and a §-number does not.
//
// This is NOT a resurrection of the designBlock coupling #132 deleted. That
// asserted help against DESIGN's wording, which made a derivation assert itself
// against its own source. This asserts a property of the *audience*: whatever
// the text says, it may not point the reader at a document they do not have.
// Wording stays entirely the authors'.
//
// It gates the tagged regions specifically, because those render into --help,
// where a dangling reference costs the most. The untagged manual-only prose
// carries the same rule (docs/ is user-facing since #129) but is the author's
// to keep — see #141, which authors it across all 26 pages.
func TestHelpTextCitesNothingInternal(t *testing.T) {
	isolateXDG(t)
	e := &env{version: "v1.2.3", stdin: strings.NewReader(""), stdout: io.Discard, stderr: io.Discard}
	root := e.newRootCmd()
	if err := applyHelpDocs(dstow.Manual, root); err != nil {
		t.Fatalf("apply help docs: %v", err)
	}

	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		regions := map[string]string{
			"dstow:short":    cmd.Short,
			"dstow:long":     cmd.Long,
			"dstow:examples": cmd.Example,
		}
		for region, text := range regions {
			for _, re := range internalRefs {
				if hit := re.FindString(text); hit != "" {
					t.Errorf("%s: %s region cites %q — an internal document the reader does not have; "+
						"cite the manual instead (e.g. 'dstow manual concepts states')",
						helpDocPath(cmd), region, hit)
				}
			}
		}
		for _, child := range cmd.Commands() {
			if helpDocExempt[child.Name()] {
				continue
			}
			walk(child)
		}
	}
	walk(root)
}

// TestHelpDocsCoverTheCommandTree asserts the bijection between the live cobra
// tree and docs/commands/: every dstow-defined command has a page, every page
// is some command's page, and every page carries the regions help needs. This
// is the whole completeness gate — it runs here rather than at startup, because
// a docs tree that has drifted from the command tree is a repo defect, and
// checking it in a built binary would only ship users the breakage.
//
// Nothing about prose is asserted. Headings, ordering, and wording are the
// authors' (issue #131); the coupling this replaces asserted all three, which
// is what made it fragile.
func TestHelpDocsCoverTheCommandTree(t *testing.T) {
	isolateXDG(t)
	e := &env{version: "v1.2.3", stdin: strings.NewReader(""), stdout: io.Discard, stderr: io.Discard}
	root := e.newRootCmd()

	// Every command resolves to a page that exists and parses: applyHelpDocs is
	// the production path, so its error is the assertion — the runtime ignores
	// it precisely because this test does not.
	if err := applyHelpDocs(dstow.Manual, root); err != nil {
		t.Fatalf("docs/commands/ does not cover the live command tree: %v", err)
	}

	reached := map[string]bool{}
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		page := helpDocPath(cmd)
		reached[page] = true
		// Both regions are required of every page. A command whose help is a
		// bare Short renders nothing under `--help` but its usage, which is the
		// surface being replaced, not an acceptable outcome of replacing it.
		if cmd.Short == "" {
			t.Errorf("%s: %q has an empty dstow:short region", page, cmd.CommandPath())
		}
		if cmd.Long == "" {
			t.Errorf("%s: %q has an empty dstow:long region", page, cmd.CommandPath())
		}
		for _, child := range cmd.Commands() {
			if helpDocExempt[child.Name()] {
				continue
			}
			walk(child)
		}
	}
	walk(root)

	// And no page is an orphan. A page left behind by a deleted or renamed
	// command is help text nothing renders — drift in the direction the
	// per-command lookup could never have caught, since it only ever asked for
	// the pages it already knew about.
	if err := fs.WalkDir(dstow.Manual, commandsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !reached[p] {
			t.Errorf("%s belongs to no command: every page under %s is one command's help", p, commandsDir)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", commandsDir, err)
	}
}

// TestHelpDocPathMirrorsTheCommandTree pins the mapping itself: a group takes
// its directory's index.md — the same file the manual prints for that node —
// and a leaf takes a file named for it, at any depth.
func TestHelpDocPathMirrorsTheCommandTree(t *testing.T) {
	root := &cobra.Command{Use: "dstow"}
	leaf := &cobra.Command{Use: "stow <name>..."}
	group := &cobra.Command{Use: "repo"}
	nested := &cobra.Command{Use: "add <source>"}
	group.AddCommand(nested)
	root.AddCommand(leaf, group)

	cases := map[*cobra.Command]string{
		root:   "docs/commands/index.md",
		leaf:   "docs/commands/stow.md",
		group:  "docs/commands/repo/index.md",
		nested: "docs/commands/repo/add.md",
	}
	for cmd, want := range cases {
		if got := helpDocPath(cmd); got != want {
			t.Errorf("helpDocPath(%q) = %q, want %q", cmd.CommandPath(), got, want)
		}
	}
}
