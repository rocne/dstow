package name

import "strings"

// FQN is a fully qualified name: scheme:coordinate::package. Fields hold
// DECODED text; String re-encodes canonically.
type FQN struct {
	Scheme string // non-empty
	// Coordinate holds decoded /-segments, len >= 1. A LEADING empty segment
	// is legal and represents an absolute-path coordinate
	// (local:/home/x -> ["", "home", "x"]); any other empty segment is invalid.
	Coordinate []string
	Package    string // decoded; "" means this is a repo FQN
}

// IsPackage reports whether the FQN names a package (as opposed to a repo).
func (f FQN) IsPackage() bool { return f.Package != "" }

// Repo returns the same FQN with the ::package tail dropped.
func (f FQN) Repo() FQN {
	return FQN{Scheme: f.Scheme, Coordinate: f.Coordinate, Package: ""}
}

// String returns the canonical percent-encoded form:
// scheme + ":" + Encode(segments) joined by "/" + ("::" + Encode(package)
// when Package != "").
func (f FQN) String() string {
	var b strings.Builder
	b.WriteString(f.Scheme)
	b.WriteByte(':')
	for i, seg := range f.Coordinate {
		if i > 0 {
			b.WriteByte('/')
		}
		b.WriteString(Encode(seg))
	}
	if f.Package != "" {
		b.WriteString("::")
		b.WriteString(Encode(f.Package))
	}
	return b.String()
}

// ParseFQN parses a full canonical FQN (scheme mandatory).
func ParseFQN(s string) (FQN, error) {
	if s == "" {
		return FQN{}, &ParseError{s, "empty input: an FQN is scheme:coordinate[::package]"}
	}
	// v1 canonical names carry no @-suffix (§1.2 reservation): a raw '@'
	// anywhere is rejected outright; a literal '@' must be written %40.
	if containsByte(s, '@') {
		return FQN{}, &ParseError{s, "'@' is reserved as a scheme-interpreted coordinate suffix and has no meaning in a v1 canonical name; write a literal '@' as %40"}
	}
	head, pkg, _, err := splitPackage(s)
	if err != nil {
		return FQN{}, err
	}
	if head == "" {
		// A leading "::" forces package-kind — legal only in a name
		// expression, never in a canonical FQN.
		return FQN{}, &ParseError{s, "a leading '::' is a name expression (package kind-forcing), not a canonical FQN; an FQN needs a scheme and coordinate"}
	}
	ci := strings.IndexByte(head, ':')
	if ci < 0 {
		return FQN{}, &ParseError{s, "an FQN requires a scheme (write scheme:coordinate); a bare coordinate is a name expression"}
	}
	scheme := head[:ci]
	coord := head[ci+1:]
	if verr := validateScheme(scheme, s); verr != nil {
		return FQN{}, verr
	}
	if containsByte(coord, ':') {
		return FQN{}, &ParseError{s, "':' separates only the scheme; a literal ':' in the coordinate must be written %3A"}
	}
	segs, serr := parseSegments(coord, true, s)
	if serr != nil {
		return FQN{}, serr
	}
	return FQN{Scheme: scheme, Coordinate: segs, Package: pkg}, nil
}

// splitPackage cuts the "::" package tail. At most one "::" may appear; a
// second (or ":::") is an error. The tail is exactly one non-empty segment
// with no "/", no raw ":", and no raw "@"; it is returned decoded.
func splitPackage(s string) (head, pkg string, hasPkg bool, err error) {
	idx := strings.Index(s, "::")
	if idx < 0 {
		return s, "", false, nil
	}
	// An overlapping search from idx+1 catches both a second "::" and ":::".
	if strings.Contains(s[idx+1:], "::") {
		return "", "", false, &ParseError{s, "'::' separates only the package and may appear at most once; write a literal ':' as %3A"}
	}
	head = s[:idx]
	pkgRaw := s[idx+2:]
	if pkgRaw == "" {
		return "", "", false, &ParseError{s, "the package after '::' is empty; write scheme:coordinate::package"}
	}
	if containsByte(pkgRaw, '/') {
		return "", "", false, &ParseError{s, "the package is a single segment and contains no '/'"}
	}
	if containsByte(pkgRaw, ':') {
		return "", "", false, &ParseError{s, "':' separates only the scheme; a literal ':' in the package must be written %3A"}
	}
	if containsByte(pkgRaw, '@') {
		return "", "", false, &ParseError{s, "'@' is reserved; write a literal '@' in the package as %40"}
	}
	dec, derr := Decode(pkgRaw)
	if derr != nil {
		return "", "", false, rewrap(derr, s)
	}
	return head, dec, true, nil
}

// validateScheme enforces that a scheme is non-empty and contains no "/",
// "@", or "%". (A raw ":" cannot occur — the scheme is the text before the
// first ":".)
func validateScheme(scheme, input string) *ParseError {
	if scheme == "" {
		return &ParseError{input, "the scheme is empty; write scheme:coordinate"}
	}
	if containsByte(scheme, '/') {
		return &ParseError{input, "a scheme contains no '/'; the coordinate begins after the first ':'"}
	}
	if containsByte(scheme, '@') {
		return &ParseError{input, "a scheme contains no '@'; write a literal '@' as %40"}
	}
	if containsByte(scheme, '%') {
		return &ParseError{input, "a scheme is literal and contains no percent-escapes"}
	}
	return nil
}

// parseSegments splits a coordinate on "/" and decodes each segment. An empty
// coordinate is an error. An empty segment is an error EXCEPT a single leading
// empty segment, which is legal only when a scheme is present (an absolute-path
// coordinate, e.g. local:/home/x).
func parseSegments(coord string, schemePresent bool, input string) ([]string, *ParseError) {
	if coord == "" {
		return nil, &ParseError{input, "the coordinate is empty; scheme:coordinate needs at least one segment"}
	}
	raw := strings.Split(coord, "/")
	out := make([]string, len(raw))
	for j, r := range raw {
		if r == "" {
			if j == 0 && schemePresent && len(raw) > 1 {
				out[j] = "" // absolute-path leading segment
				continue
			}
			return nil, &ParseError{input, "a coordinate segment is empty; only a single leading empty segment (an absolute-path coordinate such as local:/home/x, with a scheme) is allowed"}
		}
		d, derr := Decode(r)
		if derr != nil {
			return nil, rewrap(derr, input)
		}
		out[j] = d
	}
	return out, nil
}
