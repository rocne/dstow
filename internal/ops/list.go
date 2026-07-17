package ops

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// ListRequest parameterizes a list run (§2.4): the read surface over a scope's
// content. Name is the scope operand — empty is the global scope (the repos).
// ReposOnly (--repos) and PackagesOnly (--packages) are the flag filters; cli
// owns their mutual exclusion and any name/flag combination rules. --json is a
// cli rendering choice over this same data.
type ListRequest struct {
	Name         string
	ReposOnly    bool
	PackagesOnly bool
}

// ListKind names which content a ListResult carries (§2.4: global ⊃ repos ⊃
// packages ⊃ paths).
type ListKind int

const (
	// KindRepos lists the global scope's repos (bare list, or --repos).
	KindRepos ListKind = iota
	// KindPackages lists packages — one repo's, or all repos' under --packages.
	KindPackages
	// KindPaths lists a package's raw file paths.
	KindPaths
)

// RepoListing is one repo row of a repos listing (§2.4: source, scheme,
// bulk-exclusion). Source is the canonical FQN spelling of where it came from;
// Root is its on-disk location; the flags carry no priority (§2.3).
type RepoListing struct {
	FQN          name.FQN
	Display      string // shortest-unique spelling among the listed repos (O9)
	Source       string // canonical scheme:coordinate — where it came from
	Scheme       string
	Root         string
	ExcludedBulk bool
	Managed      bool
	Session      bool
}

// PackageListing is one package row, always repo-attributed (REQUIREMENTS
// §7.1). Display is the O9 shortest-unique spelling among the listed set —
// bare where unambiguous, qualified where a bare name is shared.
type PackageListing struct {
	FQN     name.FQN
	Display string
	Repo    name.FQN
}

// PathListing is one raw package path, relative to the package directory
// (§2.4; ruled: no dot-translation, no ignore application — a plain walk).
type PathListing struct {
	Path string
}

// ListResult is a list run as data (A4). Exactly one of Repos/Packages/Paths
// is populated per Kind; Scope names the resolved repo or package (zero for
// the global scope).
type ListResult struct {
	Kind     ListKind
	Scope    name.FQN
	Repos    []RepoListing
	Packages []PackageListing
	Paths    []PathListing
	Warnings []Warning
}

// List enumerates a scope's configured content (§2.4, REQUIREMENTS §7.1): a
// pure config+source read — it never inspects target dirs. A named operand
// selects the scope (a repo lists its packages, a package lists its paths); no
// name lists the repos, unless --packages widens to every repo's packages.
// Run-level refusals (ambiguity, not-found) return as error; everything else
// is data.
func (a *App) List(req ListRequest) (*ListResult, error) {
	ctxs := a.loadRepoCtxs()
	ents, byRepo := entities(ctxs)

	var warnings []Warning
	for _, c := range sortedCtxs(ctxs) {
		warnings = append(warnings, c.warns...)
	}

	if req.Name != "" {
		ent, rc, err := resolveOne(req.Name, ents, byRepo)
		if err != nil {
			return nil, err
		}
		if ent.FQN.IsPackage() {
			return a.listPaths(ent, rc, warnings)
		}
		return a.listPackages(warnings, ctxs, rc), nil
	}

	if req.PackagesOnly {
		return a.listPackages(warnings, ctxs, nil), nil
	}
	return a.listRepos(warnings, ctxs), nil
}

// listRepos builds the repos listing: source, scheme, and bulk-exclusion per
// repo (§2.4), in canonical FQN order with O9 display spellings.
func (a *App) listRepos(warnings []Warning, ctxs []*repoCtx) *ListResult {
	ordered := sortedCtxs(ctxs)
	fqns := make([]name.FQN, len(ordered))
	for i, c := range ordered {
		fqns[i] = c.r.FQN
	}
	display := name.ShortestUnique(fqns)

	repos := make([]RepoListing, 0, len(ordered))
	for i, c := range ordered {
		excluded := config.Effective{Global: a.Global, Repo: c.level}.ExcludeFromBulk()
		repos = append(repos, RepoListing{
			FQN:          c.r.FQN,
			Display:      display[i],
			Source:       c.r.FQN.String(),
			Scheme:       c.r.FQN.Scheme,
			Root:         c.r.Root,
			ExcludedBulk: excluded,
			Managed:      c.r.Managed,
			Session:      c.r.Session,
		})
	}
	return &ListResult{Kind: KindRepos, Repos: repos, Warnings: warnings}
}

// listPackages builds a packages listing. When only is non-nil the listing is
// that one repo's packages (repo-attributed); when nil it spans every repo
// (--packages). Same-named entries qualify via O9 (REQUIREMENTS §7.1).
func (a *App) listPackages(warnings []Warning, ctxs []*repoCtx, only *repoCtx) *ListResult {
	var fqns []name.FQN
	var repos []name.FQN
	add := func(c *repoCtx) {
		for _, p := range c.packages {
			fqns = append(fqns, name.FQN{
				Scheme:     c.r.FQN.Scheme,
				Coordinate: c.r.FQN.Coordinate,
				Package:    p,
			})
			repos = append(repos, c.r.FQN)
		}
	}
	scope := name.FQN{}
	if only != nil {
		add(only)
		scope = only.r.FQN
	} else {
		for _, c := range sortedCtxs(ctxs) {
			add(c)
		}
	}

	// Order the whole listing canonically before computing display spellings,
	// so qualification is stable and the rows read in FQN order.
	idx := make([]int, len(fqns))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(i, j int) bool { return fqns[idx[i]].String() < fqns[idx[j]].String() })
	ordFQNs := make([]name.FQN, len(idx))
	ordRepos := make([]name.FQN, len(idx))
	for i, k := range idx {
		ordFQNs[i], ordRepos[i] = fqns[k], repos[k]
	}
	display := name.ShortestUnique(ordFQNs)

	pkgs := make([]PackageListing, len(ordFQNs))
	for i := range ordFQNs {
		pkgs[i] = PackageListing{FQN: ordFQNs[i], Display: display[i], Repo: ordRepos[i]}
	}
	return &ListResult{Kind: KindPackages, Scope: scope, Packages: pkgs, Warnings: warnings}
}

// listPaths walks a package directory and lists its files, relative to the
// package dir, raw — no dot-translation and no user-ignore application (§2.4
// ruling: the translated view is -n/--dry-run's, reality is status's). The
// structural .dstow metadata dir is excluded: it is dstow's own bookkeeping,
// never part of the package's deployable content (CONTEXT.md). An enumeration
// failure surfaces as error; a warning-worthy but non-fatal condition rides
// warnings.
func (a *App) listPaths(ent repo.Entity, rc *repoCtx, warnings []Warning) (*ListResult, error) {
	if rc == nil {
		return nil, &NotFoundError{Input: ent.FQN.String()}
	}
	root := rc.pkgRoot(ent.FQN.Package)
	metaDir := config.MetadataDir(root)
	var paths []PathListing
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if p == metaDir {
				return filepath.SkipDir // the .dstow metadata dir is dstow's own, never package content
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		paths = append(paths, PathListing{Path: filepath.ToSlash(rel)})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].Path < paths[j].Path })
	return &ListResult{Kind: KindPaths, Scope: ent.FQN, Paths: paths, Warnings: warnings}, nil
}
