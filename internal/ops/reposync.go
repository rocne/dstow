package ops

import (
	"fmt"
	"sort"

	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// RepoSyncRequest parameterizes update and upgrade (§2.4, REQUIREMENTS §6):
// named repos, or every remote (managed) repo when none are named. Local and
// session repos have no upstream and are skipped.
type RepoSyncRequest struct {
	Names []string // empty = all remote repos
}

// RepoSyncReport is one repo's outcome in a sync run (A4). For update, Fetched
// marks a completed fetch. For upgrade, Old/New report the fast-forward and
// Changed whether it moved. Err carries a per-repo refusal (a *git.DivergedError,
// local work, or *git.NotInstalledError) — the run continues past it. Skipped
// marks a named repo with no upstream, with Note saying why.
type RepoSyncReport struct {
	FQN     name.FQN
	Fetched bool
	Old     string
	New     string
	Changed bool
	Skipped bool
	Note    string
	Err     error
}

// RepoSyncResult is the whole sync run as data.
type RepoSyncResult struct {
	Repos    []RepoSyncReport
	Warnings []Warning
}

// Failed reports whether any repo's outcome was an error (exit nonzero).
func (r *RepoSyncResult) Failed() bool {
	for _, rep := range r.Repos {
		if rep.Err != nil {
			return true
		}
	}
	return false
}

// RepoUpdate runs the fetch phase (§2.4 update, REQUIREMENTS §6.1): git.Fetch
// per remote repo, touching the network and no working tree. Each repo's
// outcome — fetched, skipped (no upstream), or errored — is data; the run
// continues past a per-repo failure.
func (a *App) RepoUpdate(req RepoSyncRequest) (*RepoSyncResult, error) {
	targets, res, err := a.syncScope(req)
	if err != nil {
		return nil, err
	}
	for _, r := range targets {
		rep := RepoSyncReport{FQN: r.FQN}
		if a.Git == nil {
			rep.Err = fmt.Errorf("git is required to fetch %s but no git port is configured", r.FQN)
			res.Repos = append(res.Repos, rep)
			continue
		}
		if ferr := a.Git.Fetch(r.Root); ferr != nil {
			rep.Err = ferr
		} else {
			rep.Fetched = true
		}
		res.Repos = append(res.Repos, rep)
	}
	return res, nil
}

// RepoUpgrade runs the apply phase (§2.4 upgrade, REQUIREMENTS §6.2):
// git.FFApply per remote repo, fast-forward only, reporting old→new. Divergence
// or local work refuses loudly as that repo's Err — no stash, merge, or rebase,
// and never a re-stow (structural drift shows up in status). A *NotInstalledError
// surfaces as the repo's outcome, never a panic.
func (a *App) RepoUpgrade(req RepoSyncRequest) (*RepoSyncResult, error) {
	targets, res, err := a.syncScope(req)
	if err != nil {
		return nil, err
	}
	for _, r := range targets {
		rep := RepoSyncReport{FQN: r.FQN}
		if a.Git == nil {
			rep.Err = fmt.Errorf("git is required to upgrade %s but no git port is configured", r.FQN)
			res.Repos = append(res.Repos, rep)
			continue
		}
		old, next, aerr := a.Git.FFApply(r.Root)
		if aerr != nil {
			rep.Err = aerr
		} else {
			rep.Old, rep.New = old, next
			rep.Changed = old != next
			if !rep.Changed {
				rep.Note = "already up to date"
			}
		}
		res.Repos = append(res.Repos, rep)
	}
	return res, nil
}

// syncScope selects the repos a sync run acts on (REQUIREMENTS §6.3): the named
// repos, or every remote (managed) repo when none are named. A named local or
// session repo resolves fine but is reported skipped (no upstream); a named
// package is a refusal. Run-level refusals (ambiguity, not-found) come back as
// error.
func (a *App) syncScope(req RepoSyncRequest) ([]repo.Repo, *RepoSyncResult, error) {
	res := &RepoSyncResult{}

	if len(req.Names) == 0 {
		var targets []repo.Repo
		for _, r := range a.Repos {
			if r.Managed {
				targets = append(targets, r)
			}
		}
		// Canonical FQN order for stable output (§2.3): the repo set is
		// unordered, so bulk sync reports must be sorted to read deterministically.
		sort.Slice(targets, func(i, j int) bool {
			return targets[i].FQN.String() < targets[j].FQN.String()
		})
		return targets, res, nil
	}

	ctxs := a.loadRepoCtxs()
	ents, byRepo := entities(ctxs)
	var targets []repo.Repo
	for _, input := range req.Names {
		ent, _, err := resolveOne(input, ents, byRepo)
		if err != nil {
			return nil, nil, err
		}
		if ent.FQN.IsPackage() {
			return nil, nil, fmt.Errorf("%q names the package %s; update and upgrade act on repos — name one of its repos", input, ent.FQN)
		}
		if !ent.Repo.Managed {
			res.Repos = append(res.Repos, RepoSyncReport{
				FQN:     ent.FQN,
				Skipped: true,
				Note:    "local repo has no upstream; nothing to sync",
			})
			continue
		}
		targets = append(targets, ent.Repo)
	}
	return targets, res, nil
}
