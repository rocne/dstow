package name

import "strings"

// ShortestUnique returns, for each FQN in fqns, the shortest name-expression
// spelling that resolves uniquely to that FQN within the set — the O9 display
// rule (DESIGN.md §1.5: "shortest-unique suffix everywhere by default; full
// FQN whenever showing a tie"). The result is parallel to the input.
//
// Each FQN's candidate spellings are walked shortest-first along the
// segment-boundary suffix ladder the naming grammar accepts (§1.1): a package
// climbs bare-name → coordinate-tail::package → … → full FQN; a repo climbs
// coordinate-tail → … → full FQN. The first candidate that, read as a name
// expression, matches exactly one FQN in the set is chosen. The full FQN is
// always the last rung and is always unique, so every FQN gets a spelling.
//
// Uniqueness is decided with Expr.Matches — the same resolution the CLI uses —
// so a displayed short name always resolves back to the entity it names. A
// tie (two identical FQNs) degenerates to the full FQN for both.
//
// This is pure display over the grammar (A7): the local-coordinate "~"
// abbreviation of §1.5 is a presentation concern that needs the home
// directory (an OS fact) and is deliberately left to the rendering layer.
func ShortestUnique(fqns []FQN) []string {
	out := make([]string, len(fqns))
	for i, f := range fqns {
		out[i] = shortestFor(f, fqns)
	}
	return out
}

// shortestFor picks the shortest uniquely-matching spelling for f within set.
func shortestFor(f FQN, set []FQN) string {
	for _, cand := range ladder(f) {
		if matchesExactlyOne(cand.expr, set) {
			return cand.spelling
		}
	}
	// Unreachable in practice: the full FQN is always a candidate. Kept as a
	// total function — a degenerate duplicate FQN falls back to its full form.
	return f.String()
}

// candidate is one rung of the suffix ladder: the expression to test for
// uniqueness and the canonical spelling to display when it wins.
type candidate struct {
	expr     Expr
	spelling string
}

// ladder builds a FQN's suffix-ladder candidates, shortest first, ending in
// the always-unique full FQN.
func ladder(f FQN) []candidate {
	var cands []candidate
	if f.IsPackage() {
		// Bare package name (the §1.1 cross-kind bare form).
		cands = append(cands, candidate{
			expr:     Expr{Segments: []string{f.Package}},
			spelling: Encode(f.Package),
		})
		// Growing coordinate tails, qualified with the package.
		for j := 1; j <= len(f.Coordinate); j++ {
			tail := f.Coordinate[len(f.Coordinate)-j:]
			cands = append(cands, candidate{
				expr:     Expr{Segments: tail, HasPackage: true, Package: f.Package},
				spelling: encodeTail(tail) + "::" + Encode(f.Package),
			})
		}
	} else {
		// Growing coordinate tails (repo entity).
		for j := 1; j <= len(f.Coordinate); j++ {
			tail := f.Coordinate[len(f.Coordinate)-j:]
			cands = append(cands, candidate{
				expr:     Expr{Segments: tail},
				spelling: encodeTail(tail),
			})
		}
	}
	// The full FQN with scheme — always unique.
	cands = append(cands, candidate{expr: fullExpr(f), spelling: f.String()})
	return cands
}

// fullExpr is the expression form of a complete FQN (scheme + full coordinate
// + package), which Matches accepts only against that exact FQN.
func fullExpr(f FQN) Expr {
	return Expr{
		Scheme:     f.Scheme,
		Segments:   f.Coordinate,
		HasPackage: f.IsPackage(),
		Package:    f.Package,
	}
}

// encodeTail joins encoded coordinate segments with "/".
func encodeTail(tail []string) string {
	parts := make([]string, len(tail))
	for i, s := range tail {
		parts[i] = Encode(s)
	}
	return strings.Join(parts, "/")
}

// matchesExactlyOne reports whether expr matches exactly one FQN in set.
func matchesExactlyOne(expr Expr, set []FQN) bool {
	n := 0
	for _, g := range set {
		if expr.Matches(g) {
			n++
			if n > 1 {
				return false
			}
		}
	}
	return n == 1
}
