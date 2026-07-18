package ops

import (
	"fmt"
	"sort"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// RepoAddRequest parameterizes repo add (§2.4, REQUIREMENTS §5): the raw source
// operand and the opt-in add-and-stow flag. Classification and the §1.2
// confirm/ambiguity flow happen inside — cli hands the raw string in.
type RepoAddRequest struct {
	Source string // raw user input (path, URL, qualified, or bare)
	Stow   bool   // --stow: after registering, stow this repo's packages
}

// RepoAddResult is the add as data (A4). It names the resolved source and repo
// FQN, whether a clone happened, the enumerated packages, the bare names that
// now need qualification (the shadowing announcement, §5.2.4), and — under
// --stow — the composed deploy run. AlreadyPresent marks the safe, announced
// re-add no-op (§5.2.3).
type RepoAddResult struct {
	Source         repo.Source
	FQN            name.FQN
	Managed        bool          // a managed clone (remote source)
	Cloned         bool          // a clone was performed this run
	AlreadyPresent bool          // re-add no-op: the source was already registered
	Packages       []string      // the new repo's enumerated packages
	Shadowed       []string      // bare package names that now need qualification
	Deploy         *DeployResult // populated under --stow
	Notes          []string
	Warnings       []Warning
}

// RepoAdd registers a repo from a source (§2.4, REQUIREMENTS §5.1–5.2). It
// resolves the source (consulting the Prompter for the §1.2 flow), confirms
// percent-encoding when a segment needs it, clones a remote into the managed
// directory or registers a local path in place, appends to the registry
// (dedup-by-source, re-add is a no-op), and returns the packages and shadowing
// as data. Adding stows nothing unless --stow, which composes a bulk stow
// scoped to the new repo.
func (a *App) RepoAdd(req RepoAddRequest) (*RepoAddResult, error) {
	rs, err := a.resolveAddSource(req.Source)
	if err != nil {
		return nil, err
	}
	src := rs.src

	res := &RepoAddResult{
		Source:  src,
		FQN:     name.FQN{Scheme: src.Scheme, Coordinate: src.Coordinate},
		Managed: rs.remote,
	}

	// Encoding confirm (§1.2 + §2.4 add): surface the encoded form and ask
	// continue-or-rename. The polarity is continue-affirmative; a rename answer
	// cancels. A non-interactive prompter proceeds with a loud announcement
	// (§1.2), so its error is taken as "proceed and announce", not a refusal.
	if needsEncoding(src) {
		note := fmt.Sprintf("source contains characters that percent-encode; the canonical encoded source is %s", src.String())
		ok, perr := a.Prompt.Confirm(note+"; continue with the encoded form (answer no to cancel and rename first)?", true)
		switch {
		case perr != nil:
			res.Notes = append(res.Notes, note+"; continuing non-interactively with the encoded form")
		case !ok:
			return nil, &RenameRequestedError{Source: src.String()}
		}
	}

	// Registry dedup (§5.2.3): a source already present is a no-op — no clone,
	// no duplicate entry — but still announced, still stow-able under --stow.
	reg, rwarns, lerr := repo.LoadRegistry(config.RegistryFile())
	if lerr != nil {
		return nil, lerr
	}
	res.Warnings = append(res.Warnings, warnRepo(rwarns)...)
	present := false
	for _, s := range reg.Sources {
		if s.String() == src.String() {
			present = true
			break
		}
	}

	if present {
		res.AlreadyPresent = true
		res.Notes = append(res.Notes, fmt.Sprintf("repo %s is already registered; nothing to do", res.FQN))
	} else {
		// Remote → clone into the managed directory; local → registered in
		// place, never cloned or modified (§5.2.3, REQUIREMENTS §5.2).
		if rs.remote {
			if a.Git == nil {
				return nil, fmt.Errorf("git is required to clone %s but no git port is configured", res.FQN)
			}
			if cerr := a.Git.Clone(rs.cloneURL, src.CloneDir()); cerr != nil {
				return nil, cerr
			}
			res.Cloned = true
		}
		reg.Sources = append(reg.Sources, src)
		if serr := reg.Save(config.RegistryFile()); serr != nil {
			return nil, serr
		}
	}

	// Build the new repo and enumerate its packages (§5.2.4: add announces the
	// repo's packages). A faked or freshly cloned dir may not enumerate yet;
	// that is a warning, not a failure of the add.
	built := repo.BuildSet([]repo.Source{src}, nil)
	if len(built) == 0 {
		return res, nil
	}
	newRepo := built[0]

	pkgs, penum := a.enumeratePackages(newRepo)
	res.Packages = pkgs
	res.Warnings = append(res.Warnings, penum...)

	// Shadowing (§5.2.4): a package name the new repo shares with an existing
	// repo now needs qualification to resolve. Computed against the current
	// set, before any --stow append.
	res.Shadowed = a.shadowedNames(newRepo, pkgs)

	if req.Stow {
		a.ensureInSet(newRepo)
		dr, derr := a.Deploy(DeployRequest{Verb: engine.VerbStow, Names: []string{src.String()}})
		if derr != nil {
			return nil, derr
		}
		res.Deploy = dr
	}

	return res, nil
}

// enumeratePackages lists a repo's packages, honoring its repo-level
// packages_dir, returning any load/enumeration trouble as warnings.
func (a *App) enumeratePackages(r repo.Repo) ([]string, []Warning) {
	var warns []Warning
	packagesDir := ""
	level, lw, lerr := config.LoadRepoLevel(r.Root)
	warns = append(warns, warnConfig(lw)...)
	if lerr != nil {
		warns = append(warns, Warning{Source: r.Root, Detail: fmt.Sprintf("repo %s: config load failed: %v", r.FQN, lerr)})
	} else if level != nil {
		packagesDir = level.PackagesDir()
	}
	pkgs, pw, perr := r.Packages(packagesDir)
	warns = append(warns, warnRepo(pw)...)
	if perr != nil {
		warns = append(warns, Warning{Source: r.Root, Detail: fmt.Sprintf("repo %s: cannot enumerate packages yet: %v", r.FQN, perr)})
	}
	return pkgs, warns
}

// shadowedNames returns the new repo's package names that also exist in the
// current set — the bare names whose resolution now needs qualification.
func (a *App) shadowedNames(newRepo repo.Repo, newPkgs []string) []string {
	existing := map[string]bool{}
	for _, c := range a.loadRepoCtxs() {
		if c.r.FQN.String() == newRepo.FQN.String() {
			continue
		}
		for _, p := range c.packages {
			existing[p] = true
		}
	}
	var out []string
	for _, p := range newPkgs {
		if existing[p] {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// ensureInSet appends r to the App's repo set if its FQN is not already there,
// so a following Deploy has it in scope (pre-ruled: internal wiring is ops').
func (a *App) ensureInSet(r repo.Repo) {
	for _, existing := range a.Repos {
		if existing.FQN.String() == r.FQN.String() {
			return
		}
	}
	a.Repos = append(a.Repos, r)
}
