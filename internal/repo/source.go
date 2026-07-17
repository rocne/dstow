package repo

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rocne/dstow/internal/name"
)

// Source is where a repo comes from (CONTEXT.md "Source"): a scheme plus a
// path-shaped coordinate. Fields hold DECODED text; String re-encodes the
// canonical qualified form. The two v1 schemes are github and local, internal
// and closed — no plugin seam until a third scheme exists (A9).
type Source struct {
	Scheme     string
	Coordinate []string // decoded /-segments; a local absolute path carries a leading empty segment
}

// String returns the canonical percent-encoded qualified source
// (scheme:coordinate), each segment encoded via the name grammar. It reuses the
// FQN formatter so source and repo-FQN spellings can never drift.
func (s Source) String() string {
	return name.FQN{Scheme: s.Scheme, Coordinate: s.Coordinate}.String()
}

// SourceError is a §1.4-style refusal from ParseSource: Reason is complete
// prose naming the expected shape or the known schemes.
type SourceError struct {
	Input  string
	Reason string
}

func (e *SourceError) Error() string {
	return fmt.Sprintf("dstow/repo: %q is not a valid source: %s", e.Input, e.Reason)
}

// ParseSource parses a QUALIFIED source (§1.1, §5.2): github:owner/name
// (exactly two non-empty segments) or local:path (an absolute filesystem path).
// It is scheme-agnostic parsing via the name grammar (percent-decoding included)
// followed by scheme-specific validation. A relative local coordinate is
// refused — canonicalization to absolute happens at add time, which is ops'
// job. An unknown scheme is refused with the known schemes named.
func ParseSource(s string) (Source, error) {
	f, err := name.ParseFQN(s)
	if err != nil {
		reason := err.Error()
		var pe *name.ParseError
		if e, ok := err.(*name.ParseError); ok {
			pe = e
			reason = pe.Reason
		}
		return Source{}, &SourceError{Input: s, Reason: reason}
	}
	if f.IsPackage() {
		return Source{}, &SourceError{Input: s, Reason: fmt.Sprintf("a source names a repo, not a package; drop the %q package tail", "::"+f.Package)}
	}

	switch f.Scheme {
	case "github":
		if len(f.Coordinate) != 2 || f.Coordinate[0] == "" || f.Coordinate[1] == "" {
			return Source{}, &SourceError{
				Input:  s,
				Reason: "a github source is github:owner/name — exactly two non-empty segments",
			}
		}
	case "local":
		if !filepath.IsAbs(coordPath(f.Coordinate)) {
			return Source{}, &SourceError{
				Input:  s,
				Reason: "a local source is an absolute filesystem path; dstow repo add canonicalizes a relative path to absolute at add time, so a bare local: source must already be absolute",
			}
		}
	default:
		return Source{}, &SourceError{
			Input:  s,
			Reason: fmt.Sprintf("unknown scheme %q; the known schemes are github and local", f.Scheme),
		}
	}
	return Source{Scheme: f.Scheme, Coordinate: f.Coordinate}, nil
}

// Classification is the pure reading of raw user input as one of the four
// source-input forms (§1.3 + §5.2). It is classification only — no filesystem
// check, no interpretation; the §1.2 confirm flow in ops consumes it.
type Classification int

const (
	// PathForm is a path operand (§1.3): starts with /, ~/, ./, or ../.
	PathForm Classification = iota
	// URLForm is a full URL (scheme://…) or an scp-like ssh form (user@host:…).
	URLForm
	// QualifiedForm carries an explicit scheme separator (scheme:coordinate).
	QualifiedForm
	// BareForm is an unqualified owner/name or a bare name.
	BareForm
)

func (c Classification) String() string {
	switch c {
	case PathForm:
		return "path"
	case URLForm:
		return "url"
	case QualifiedForm:
		return "qualified"
	case BareForm:
		return "bare"
	}
	return fmt.Sprintf("Classification(%d)", int(c))
}

// scpLike matches git's scp-style ssh source (user@host:path), which carries a
// ':' but is a URL form, not a qualified source. It requires an '@' before the
// first ':' and no '://'.
var scpLike = regexp.MustCompile(`^[^/@:]+@[^/@:]+:`)

// ClassifySourceInput classifies raw user input as data, doing NO filesystem
// checks and no guessing (§1.3 + §5.2). A path operand is a PathForm; input
// containing "://" or matching the scp-like ssh form is a URLForm; input still
// carrying a ':' is a QualifiedForm; everything else (owner/name or a bare
// name) is a BareForm.
func ClassifySourceInput(s string) Classification {
	switch {
	case name.IsPathOperand(s):
		return PathForm
	case strings.Contains(s, "://") || scpLike.MatchString(s):
		return URLForm
	case strings.Contains(s, ":"):
		return QualifiedForm
	default:
		return BareForm
	}
}
