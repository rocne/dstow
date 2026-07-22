package cli

import (
	"fmt"
	"regexp"
	"strings"
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
