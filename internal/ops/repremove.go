package ops

import (
	"fmt"
	"os"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// RepoRemoveRequest parameterizes repo remove (§2.4, REQUIREMENTS §5.3): the
// repo operand and the two guard bypasses. --unstow unstows first without
// prompting; --force overrides both guards (unsaved work will be lost).
type RepoRemoveRequest struct {
	Repo   string // name expression resolving to one repo
	Unstow bool   // --unstow: unstow the repo's packages first, no prompt
	Force  bool   // --force: override both guards
}

// RepoRemoveResult is the removal as data. Deleted marks a managed clone whose
// directory was removed; a local-path repo is only forgotten (its directory is
// never touched). Unstowed carries the composed unstow run when one happened.
type RepoRemoveResult struct {
	FQN      name.FQN
	Managed  bool
	Deleted  bool          // the managed clone directory was deleted
	Unstowed *DeployResult // populated when packages were unstowed first
	Notes    []string
	Warnings []Warning
}

// StillStowedError refuses a removal that would orphan or dangle stowed links
// (REQUIREMENTS §5.3): the repo still has ledgered links. The remedy names the
// two bypasses. Applies to local and managed repos alike.
type StillStowedError struct {
	FQN name.FQN
}

func (e *StillStowedError) Error() string {
	return fmt.Sprintf(
		"repo %s still has stowed links; removing it would leave them dangling. Unstow first: rerun with --unstow to unstow then remove, or --force to remove and leave the links",
		e.FQN)
}

// UnsavedWorkError refuses deleting a managed clone that holds work not present
// at its source (REQUIREMENTS §5.3): the prose names what would be lost. Only
// managed clones hit this — a local path is never deleted.
type UnsavedWorkError struct {
	FQN   name.FQN
	Dir   string
	Prose string // git's account of what would be lost
}

func (e *UnsavedWorkError) Error() string {
	return fmt.Sprintf(
		"managed clone %s at %s holds work not present at its source, and dstow will not delete it: %s. Push or discard the work, or rerun with --force to delete it anyway",
		e.FQN, e.Dir, e.Prose)
}

// RepoRemove unregisters a repo (§2.4, REQUIREMENTS §5.3). It resolves the
// operand to one repo, applies the still-stowed guard (both repo kinds) and,
// for a managed clone, the unsaved-work guard — each prompt-or-refuse, both
// bypassable by --force. A local-path repo is forgotten (directory untouched);
// a managed clone is also deleted. The registry is saved last.
func (a *App) RepoRemove(req RepoRemoveRequest) (*RepoRemoveResult, error) {
	ctxs := a.loadRepoCtxs()
	ents, byRepo := entities(ctxs)
	ent, _, err := resolveOne(req.Repo, ents, byRepo)
	if err != nil {
		return nil, err
	}
	if ent.FQN.IsPackage() {
		return nil, fmt.Errorf("%q names the package %s; remove operates on repos — name one of its repos", req.Repo, ent.FQN)
	}
	r := ent.Repo
	res := &RepoRemoveResult{FQN: r.FQN, Managed: r.Managed}

	// Still-stowed guard (both kinds): prompt-or-refuse, bypassable by --force,
	// resolved without prompting by --unstow.
	stowed, serr := a.repoHasStowedLinks(r.FQN)
	if serr != nil {
		return nil, serr
	}
	if stowed && !req.Force {
		doUnstow := req.Unstow
		if !doUnstow {
			ok, perr := a.Prompt.Confirm(fmt.Sprintf(
				"repo %s still has stowed links; unstow them, then remove?", r.FQN), false)
			if perr != nil {
				return nil, &StillStowedError{FQN: r.FQN}
			}
			if !ok {
				return nil, &StillStowedError{FQN: r.FQN}
			}
			doUnstow = true
		}
		if doUnstow {
			dr, derr := a.Deploy(DeployRequest{Verb: engine.VerbUnstow, Names: []string{r.FQN.String()}})
			if derr != nil {
				return nil, derr
			}
			res.Unstowed = dr
		}
	}

	// Unsaved-work guard (managed clones only): a local path is never deleted,
	// so nothing of the user's could be lost.
	if r.Managed && !req.Force {
		if a.Git == nil {
			return nil, fmt.Errorf("git is required to check %s for unsaved work but no git port is configured", r.FQN)
		}
		dirty, prose, herr := a.Git.HasLocalWork(r.Root)
		if herr != nil {
			return nil, herr // e.g. *git.NotInstalledError — surfaced, never a panic
		}
		if dirty {
			return nil, &UnsavedWorkError{FQN: r.FQN, Dir: r.Root, Prose: prose}
		}
	}

	// Effect: forget the registry entry; delete the clone dir for a managed
	// repo only.
	if rerr := a.forgetSource(r.FQN, res); rerr != nil {
		return nil, rerr
	}
	if r.Managed {
		if derr := os.RemoveAll(r.Root); derr != nil {
			return nil, fmt.Errorf("removed %s from the registry but could not delete its clone at %s: %w", r.FQN, r.Root, derr)
		}
		res.Deleted = true
		res.Notes = append(res.Notes, fmt.Sprintf("deleted managed clone %s", r.Root))
	} else {
		res.Notes = append(res.Notes, fmt.Sprintf("forgot local repo %s; its directory %s is untouched", r.FQN, r.Root))
	}

	return res, nil
}

// repoHasStowedLinks reports whether the ledger holds any entry whose package
// belongs to the given repo (the still-stowed guard's test, REQUIREMENTS §5.3).
func (a *App) repoHasStowedLinks(repoFQN name.FQN) (bool, error) {
	l, err := ledger.Load(a.LedgerPath)
	if err != nil {
		return false, err
	}
	want := repoFQN.String()
	for _, entries := range l.Targets {
		for _, e := range entries {
			f, perr := name.ParseFQN(e.Package)
			if perr != nil {
				continue // a malformed ledger entry cannot pin this repo
			}
			if f.Repo().String() == want {
				return true, nil
			}
		}
	}
	return false, nil
}

// forgetSource drops the registry entry whose source matches the repo FQN and
// saves the registry. A repo not in the registry (e.g. a DSTOW_PATH session
// repo) is noted, not an error — there is nothing to unregister.
func (a *App) forgetSource(repoFQN name.FQN, res *RepoRemoveResult) error {
	reg, rwarns, err := repo.LoadRegistry(config.RegistryFile())
	if err != nil {
		return err
	}
	res.Warnings = append(res.Warnings, warnRepo(rwarns)...)
	want := repoFQN.String()
	kept := reg.Sources[:0]
	found := false
	for _, s := range reg.Sources {
		if s.String() == want {
			found = true
			continue
		}
		kept = append(kept, s)
	}
	if !found {
		res.Notes = append(res.Notes, fmt.Sprintf("repo %s was not in the registry (a session repo?); nothing to unregister", repoFQN))
		return nil
	}
	reg.Sources = kept
	return reg.Save(config.RegistryFile())
}
