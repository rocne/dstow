package ops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/name"
)

// StatusRequest parameterizes a status run (§2.4). Names scope to packages or
// whole repos; empty Names is every package. Path selects the per-path view
// (mutually exclusive with Names; cli routes a path operand here per §1.3).
// --json is cli's rendering choice.
type StatusRequest struct {
	Names []string
	Path  string
}

// RepoSync is a remote repo's sync state as of the last update (REQUIREMENTS
// §7.2.1): behind/ahead from git.Port.AheadBehind, no network. Known is false
// when the count could not be determined (no git port, or an error), with Err
// carrying the cause.
type RepoSync struct {
	FQN    name.FQN
	Ahead  int
	Behind int
	Known  bool
	Err    error
}

// PathStatus is the per-path view (REQUIREMENTS §7.2.4): what occupies a path,
// who owns it per the ledger, and — if occupied — the ranked adoption
// candidates. Path is the absolute path inspected.
type PathStatus struct {
	Path       string
	Exists     bool
	IsSymlink  bool
	LinkDest   string
	Kind       string
	Owner      name.FQN
	OwnerKnown bool
	Candidates []Candidate
	Warnings   []Warning
}

// StatusResult is a status run as data (A4). For the names/bulk view Packages
// and Repos are populated; for the per-path view Path is set.
type StatusResult struct {
	Packages []PackageStatusResult
	Repos    []RepoSync
	Path     *PathStatus
	Warnings []Warning
}

// Status inspects reality (§2.4 — the only view that lstats targets):
// expected-vs-actual against current effective config. Names scope to packages
// or whole repos (empty is every package); a single Path selects the per-path
// view. Remote repos in scope also report behind/ahead as of the last update.
// Ambiguity is a run-level refusal (error); everything else is data.
func (a *App) Status(req StatusRequest) (*StatusResult, error) {
	if req.Path != "" {
		return a.statusPath(req.Path)
	}

	led, err := ledger.Load(a.LedgerPath)
	if err != nil {
		return nil, err // corrupt / newer-version refusals pass through (§6.5)
	}

	ctxs := a.loadRepoCtxs()
	ents, byRepo := entities(ctxs)
	res := &StatusResult{}
	for _, c := range sortedCtxs(ctxs) {
		res.Warnings = append(res.Warnings, c.warns...)
	}

	// Resolve scope into a set of (repoCtx, package) pairs plus the repos whose
	// sync should be reported.
	type pkgRef struct {
		c   *repoCtx
		pkg string
	}
	var refs []pkgRef
	inScopeRepos := map[string]*repoCtx{}
	seen := map[string]bool{}
	addPkg := func(c *repoCtx, pkg string) {
		key := name.FQN{Scheme: c.r.FQN.Scheme, Coordinate: c.r.FQN.Coordinate, Package: pkg}.String()
		if seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, pkgRef{c: c, pkg: pkg})
		inScopeRepos[c.r.FQN.String()] = c
	}

	if len(req.Names) == 0 {
		for _, c := range sortedCtxs(ctxs) {
			for _, p := range c.packages {
				addPkg(c, p)
			}
			inScopeRepos[c.r.FQN.String()] = c // a repo with no packages still reports sync
		}
	} else {
		for _, input := range req.Names {
			ent, rc, rerr := resolveOne(input, ents, byRepo)
			if rerr != nil {
				var amb *AmbiguousNameError
				if errors.As(rerr, &amb) {
					return nil, rerr // run-level refusal (§1.2)
				}
				res.Warnings = append(res.Warnings, Warning{
					Source: input, Detail: rerr.Error(),
				})
				continue
			}
			if ent.FQN.IsPackage() {
				addPkg(rc, ent.FQN.Package)
			} else {
				for _, p := range rc.packages {
					addPkg(rc, p)
				}
				inScopeRepos[rc.r.FQN.String()] = rc
			}
		}
	}

	for _, r := range refs {
		pr := a.classifyPackage(r.c, r.pkg, led)
		res.Warnings = append(res.Warnings, pr.Warnings...)
		pr.Warnings = nil
		res.Packages = append(res.Packages, pr)
	}
	sortPackageResults(res.Packages)

	res.Repos = a.repoSyncs(inScopeRepos)
	return res, nil
}

// repoSyncs reports behind/ahead for the remote (managed) repos in scope
// (REQUIREMENTS §7.2.1), in canonical order. Local and session repos have no
// upstream and are skipped. No git port means no counts — a known-false row
// naming the gap, never a panic.
func (a *App) repoSyncs(inScope map[string]*repoCtx) []RepoSync {
	var out []RepoSync
	ctxs := make([]*repoCtx, 0, len(inScope))
	for _, c := range inScope {
		ctxs = append(ctxs, c)
	}
	for _, c := range sortedCtxs(ctxs) {
		if !c.r.Managed {
			continue
		}
		sync := RepoSync{FQN: c.r.FQN}
		if a.Git == nil {
			sync.Err = fmt.Errorf("no git port configured; behind/ahead unavailable")
			out = append(out, sync)
			continue
		}
		ahead, behind, err := a.Git.AheadBehind(c.r.Root)
		if err != nil {
			sync.Err = err
		} else {
			sync.Ahead, sync.Behind, sync.Known = ahead, behind, true
		}
		out = append(out, sync)
	}
	return out
}

// statusPath builds the per-path view (REQUIREMENTS §7.2.4): what occupies the
// path, its ledger owner, and — when something real occupies it — the ranked
// adoption candidates (reusing AdoptCandidates, the §8.5 pure computation).
func (a *App) statusPath(path string) (*StatusResult, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	ps := &PathStatus{Path: abs}

	info, lerr := os.Lstat(abs)
	switch {
	case lerr == nil:
		ps.Exists = true
		if info.Mode()&fs.ModeSymlink != 0 {
			ps.IsSymlink = true
			ps.Kind = "symlink"
			if dest, rerr := os.Readlink(abs); rerr == nil {
				ps.LinkDest = dest
			}
		} else {
			ps.Kind = ledger.KindOf(info)
		}
	case os.IsNotExist(lerr):
		ps.Kind = "nothing"
	default:
		ps.Kind = "unobservable"
		ps.Warnings = append(ps.Warnings, Warning{Source: abs, Detail: fmt.Sprintf("cannot observe %s: %v", abs, lerr)})
	}

	// Ledger owner: the entry whose absolute link path is this path.
	led, lderr := ledger.Load(a.LedgerPath)
	if lderr != nil {
		return nil, lderr
	}
	if owner, ok := ledgerOwner(led, abs); ok {
		if fqn, perr := name.ParseFQN(owner); perr == nil {
			ps.Owner, ps.OwnerKnown = fqn, true
		}
	}

	// Adoption candidates only when something real occupies the path (a missing
	// path or one this run cannot see has nothing to adopt).
	if ps.Exists {
		cands, warns, cerr := a.AdoptCandidates(abs)
		if cerr != nil {
			return nil, cerr
		}
		ps.Candidates = cands
		ps.Warnings = append(ps.Warnings, warns...)
	}

	return &StatusResult{Path: ps}, nil
}

// ledgerOwner finds the package that the ledger records as owning the absolute
// path, if any.
func ledgerOwner(led ledger.Ledger, abs string) (string, bool) {
	for root, entries := range led.Targets {
		for _, e := range entries {
			if filepath.Join(root, e.Link) == abs {
				return e.Package, true
			}
		}
	}
	return "", false
}

// sortPackageResults orders package rows canonically (§2.3 stable display).
func sortPackageResults(ps []PackageStatusResult) {
	sort.Slice(ps, func(i, j int) bool {
		return ps[i].FQN.String() < ps[j].FQN.String()
	})
}
