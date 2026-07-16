package name

import (
	"slices"
	"strings"
)

// Expr is a parsed name expression (user input; possibly a suffix).
type Expr struct {
	Scheme     string   // "" when absent
	Segments   []string // decoded coordinate-suffix segments; may be empty only when HasPackage (leading ::)
	HasPackage bool     // a :: tail was present
	Package    string   // decoded; set iff HasPackage
	AtSuffix   string   // reserved @-suffix, opaque, "" when absent
}

// ParseExpr parses a name expression: a full or partial coordinate, with an
// optional scheme, an optional "::package" tail (a leading "::" forces
// package-kind), and an optional reserved "@" suffix on the coordinate.
func ParseExpr(s string) (Expr, error) {
	if s == "" {
		return Expr{}, &ParseError{s, "empty input: a name expression is scheme:coordinate, a coordinate suffix, or ::package"}
	}
	head, pkg, hasPkg, err := splitPackage(s)
	if err != nil {
		return Expr{}, err
	}
	var e Expr
	e.HasPackage = hasPkg
	e.Package = pkg

	// Scheme is optional: no raw ':' in the head means no scheme.
	coord := head
	if ci := strings.IndexByte(head, ':'); ci >= 0 {
		scheme := head[:ci]
		if verr := validateScheme(scheme, s); verr != nil {
			return Expr{}, verr
		}
		e.Scheme = scheme
		coord = head[ci+1:]
		if containsByte(coord, ':') {
			return Expr{}, &ParseError{s, "':' separates only the scheme; a literal ':' in the coordinate must be written %3A"}
		}
	}

	// A reserved "@" suffix: at most one, everything after it (opaque,
	// decoded) is the suffix.
	if ai := strings.IndexByte(coord, '@'); ai >= 0 {
		atRaw := coord[ai+1:]
		coord = coord[:ai]
		if containsByte(atRaw, '@') {
			return Expr{}, &ParseError{s, "a coordinate carries at most one '@' suffix; write a literal '@' as %40"}
		}
		at, aerr := Decode(atRaw)
		if aerr != nil {
			return Expr{}, rewrap(aerr, s)
		}
		e.AtSuffix = at
	}

	if head == "" {
		// Leading "::" — kind-forcing. Segments empty is legal here and only
		// here (splitPackage already guaranteed a non-empty package).
		e.Segments = nil
		return e, nil
	}

	segs, serr := parseSegments(coord, e.Scheme != "", s)
	if serr != nil {
		return Expr{}, serr
	}
	e.Segments = segs
	return e, nil
}

// Matches reports whether the expression names the entity f denotes (f with
// empty Package is a repo entity, otherwise a package entity). It never errors
// and embodies no knowledge of which schemes exist — scheme validity is
// another package's job. Comparison is on decoded values, aligned from the
// tail.
func (e Expr) Matches(f FQN) bool {
	// A reserved @-suffix has no v1 semantics: such an expression never
	// resolves to anything (the caller produces the error).
	if e.AtSuffix != "" {
		return false
	}
	// Scheme rule: a scheme attaches only to the FULL coordinate — there is
	// no scheme-plus-partial-suffix form.
	if e.Scheme != "" {
		if e.Scheme != f.Scheme || !slices.Equal(e.Segments, f.Coordinate) {
			return false
		}
	}

	if f.Package == "" {
		// Repo entity: a bare (unpackaged) non-empty tail of the coordinate.
		if e.HasPackage {
			return false
		}
		return len(e.Segments) >= 1 && isTail(e.Segments, f.Coordinate)
	}

	// Package entity.
	if e.HasPackage {
		// Package must match; segments (possibly empty for ::pkg kind-forcing)
		// must be a tail of the coordinate. The scheme rule above additionally
		// required full-coordinate equality when a scheme was present.
		return e.Package == f.Package && isTail(e.Segments, f.Coordinate)
	}
	// Bare-name cross-kind: a single-segment, schemeless expression can name a
	// package by its package name. Multi-segment bare expressions are
	// repo-shaped and never match a package entity.
	return e.Scheme == "" && len(e.Segments) == 1 && e.Segments[0] == f.Package
}

// isTail reports whether seg is a tail (suffix) of coord, aligned from the
// end. An empty seg is a trivial tail (true).
func isTail(seg, coord []string) bool {
	if len(seg) > len(coord) {
		return false
	}
	return slices.Equal(seg, coord[len(coord)-len(seg):])
}
