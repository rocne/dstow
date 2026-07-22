package cli

import (
	"strings"
	"testing"
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
