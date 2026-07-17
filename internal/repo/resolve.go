package repo

import "github.com/rocne/dstow/internal/name"

// Entity is one resolvable thing in the set: a repo or one of its packages,
// carrying its FQN and the Repo it belongs to. Package entities have
// FQN.Package set; repo entities do not.
type Entity struct {
	FQN  name.FQN
	Repo Repo
}

// Entities builds the repo and package entities over the set (§1.1). The
// enumerate function is injected so tests and ops control the I/O of listing a
// repo's packages; an enumeration error surfaces. Each repo contributes one
// repo entity plus one package entity per enumerated package.
func Entities(set []Repo, enumerate func(Repo) ([]string, error)) ([]Entity, error) {
	var ents []Entity
	for _, r := range set {
		ents = append(ents, Entity{FQN: r.FQN, Repo: r})
		pkgs, err := enumerate(r)
		if err != nil {
			return nil, err
		}
		for _, p := range pkgs {
			ents = append(ents, Entity{
				FQN:  name.FQN{Scheme: r.FQN.Scheme, Coordinate: r.FQN.Coordinate, Package: p},
				Repo: r,
			})
		}
	}
	return ents, nil
}

// Resolve filters the entities by a name expression (§1.1 via
// name.Expr.Matches). It parses the input and returns every entity the
// expression matches — resolution data only: zero, one, or many results are all
// non-error outcomes, and the caller (ops) applies the §1.2 ambiguity semantics.
// A parse error surfaces. A reserved @-suffix expression matches nothing (the
// name grammar guarantees it), so it returns an empty slice, not an error.
func Resolve(input string, entities []Entity) ([]Entity, error) {
	expr, err := name.ParseExpr(input)
	if err != nil {
		return nil, err
	}
	var out []Entity
	for _, e := range entities {
		if expr.Matches(e.FQN) {
			out = append(out, e)
		}
	}
	return out, nil
}
