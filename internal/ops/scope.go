package ops

import (
	"fmt"
	"sort"

	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// NotFoundError reports a view operand that resolved to no entity in the set
// (§1.4). ops returns it as data; cli renders the §1.4 refusal.
type NotFoundError struct {
	Input string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%q matches no repo or package in the set", e.Input)
}

// resolveOne resolves a single name operand to exactly one entity over the
// already-built entity set, returning it with its repo context. Zero matches
// is a *NotFoundError; multiple is an *AmbiguousNameError (§1.2) — the same
// run-level refusals selectWork raises, so the views speak one resolution
// language.
func resolveOne(input string, ents []repo.Entity, byRepo map[string]*repoCtx) (repo.Entity, *repoCtx, error) {
	matches, err := repo.Resolve(input, ents)
	if err != nil {
		return repo.Entity{}, nil, err
	}
	switch len(matches) {
	case 0:
		return repo.Entity{}, nil, &NotFoundError{Input: input}
	case 1:
		m := matches[0]
		return m, byRepo[m.FQN.Repo().String()], nil
	default:
		fqns := make([]name.FQN, len(matches))
		for i, m := range matches {
			fqns[i] = m.FQN
		}
		sort.Slice(fqns, func(i, j int) bool { return fqns[i].String() < fqns[j].String() })
		return repo.Entity{}, nil, &AmbiguousNameError{Input: input, Matches: fqns}
	}
}

// sortedCtxs returns the repo contexts in canonical FQN order — the stable
// display order every view uses (the set itself is unordered, §2.3).
func sortedCtxs(ctxs []*repoCtx) []*repoCtx {
	out := make([]*repoCtx, len(ctxs))
	copy(out, ctxs)
	sort.Slice(out, func(i, j int) bool {
		return out[i].r.FQN.String() < out[j].r.FQN.String()
	})
	return out
}
