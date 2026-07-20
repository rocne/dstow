package ops

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// repoCtx is one repo's per-invocation load: its config level and package
// enumeration, each loaded at most once per run.
type repoCtx struct {
	r        repo.Repo
	level    *config.RepoLevel // nil when the load failed
	packages []string
	warns    []Warning
	loadErr  error // repo-level config load failure
	enumErr  error // package enumeration failure
}

// pkgRoot is the package's directory: under the repo root, or under the
// repo's packages_dir when set (M3).
func (c *repoCtx) pkgRoot(pkg string) string {
	return joinRoot(c.r.Root, c.packagesDir(), pkg)
}

// stowDir is the directory the engine stows from — engine.Op.Dir.
func (c *repoCtx) stowDir() string {
	return joinRoot(c.r.Root, c.packagesDir(), "")
}

func (c *repoCtx) packagesDir() string {
	if c.level == nil {
		return ""
	}
	return c.level.PackagesDir()
}

func joinRoot(root, packagesDir, pkg string) string {
	parts := []string{root}
	if packagesDir != "" {
		parts = append(parts, packagesDir)
	}
	if pkg != "" {
		parts = append(parts, pkg)
	}
	return filepath.Join(parts...)
}

// work is one package selected for a run, with its repo context and the
// explicit-naming flag (explicit overrides bulk exclusion, §2.5).
type work struct {
	pkg      repo.Entity
	rc       *repoCtx
	explicit bool

	// Loaded during selection/prep; carried so nothing loads twice.
	pkgLevel *config.PackageLevel
	pkgErr   error // package-level config load failure
}

// eff is the package's four-level chain view.
func (a *App) eff(w work) config.Effective {
	var rl *config.RepoLevel
	if w.rc != nil {
		rl = w.rc.level
	}
	return config.Effective{Global: a.Global, Repo: rl, Package: w.pkgLevel}
}

// engineOp is the one place an Effective chain becomes an engine.Op, so every
// observer (status, check) and deployer (stow/unstow/restow, adopt)
// parameterizes the engine identically and can never disagree about what a
// package means. Verb-specific fields (Adopt, Simulate) are the caller's to
// set on the returned value.
func engineOp(rc *repoCtx, eff config.Effective, target, pkg string) engine.Op {
	return engine.Op{
		Dir:                  rc.stowDir(),
		Target:               target,
		Package:              pkg,
		Fold:                 eff.FoldTrees(),
		TranslateDotPrefixes: eff.TranslateDotPrefixes(),
		Ignores:              eff.Ignores(),
	}
}

// AmbiguousNameError reports an operand matching more than one entity
// (§1.2). ops always returns it as data: the interactive explicit-choice
// treatment is cli's — it renders the sorted qualified spellings as a
// selection and re-invokes with the chosen one — and non-interactively the
// error renders as the §1.2 hard refusal naming those spellings.
type AmbiguousNameError struct {
	Input   string
	Matches []name.FQN
}

func (e *AmbiguousNameError) Error() string {
	spellings := make([]string, len(e.Matches))
	for i, m := range e.Matches {
		spellings[i] = m.String()
	}
	return fmt.Sprintf("%q is ambiguous: it matches %s; use a qualified name",
		e.Input, strings.Join(spellings, ", "))
}

// loadRepoCtxs loads every repo's level and enumeration once. The set is
// unordered; ordering happens only in orderWork.
func (a *App) loadRepoCtxs() []*repoCtx {
	ctxs := make([]*repoCtx, 0, len(a.Repos))
	for _, r := range a.Repos {
		c := &repoCtx{r: r}
		level, warns, err := config.LoadRepoLevel(r.Root)
		c.warns = append(c.warns, warnConfig(warns)...)
		if err != nil {
			c.loadErr = err
		} else {
			c.level = level
		}
		pkgs, pwarns, perr := r.Packages(c.packagesDir())
		c.warns = append(c.warns, warnRepo(pwarns)...)
		if perr != nil {
			c.enumErr = perr
		} else {
			c.packages = pkgs
		}
		ctxs = append(ctxs, c)
	}
	return ctxs
}

// entities builds the resolvable entities over the loaded contexts,
// keeping each entity's repo context at hand.
func entities(ctxs []*repoCtx) ([]repo.Entity, map[string]*repoCtx) {
	var ents []repo.Entity
	byRepo := make(map[string]*repoCtx, len(ctxs))
	for _, c := range ctxs {
		byRepo[c.r.FQN.String()] = c
		ents = append(ents, repo.Entity{FQN: c.r.FQN, Repo: c.r})
		for _, p := range c.packages {
			ents = append(ents, repo.Entity{
				FQN:  name.FQN{Scheme: c.r.FQN.Scheme, Coordinate: c.r.FQN.Coordinate, Package: p},
				Repo: c.r,
			})
		}
	}
	return ents, byRepo
}

// selectWork resolves a deploy request's operands into the ordered
// worklist. preResults carries the per-operand outcomes that never became
// work (not-found operands, unparseable operands, empty repos) — REQ §3.2's
// "per-package status line (not-found included)". A run-level refusal
// (ambiguity) comes back as err.
func (a *App) selectWork(names []string) (works []work, preResults []PackageResult, warnings []Warning, err error) {
	ctxs := a.loadRepoCtxs()
	ents, byRepo := entities(ctxs)
	for _, c := range ctxs {
		warnings = append(warnings, c.warns...)
		if c.loadErr != nil {
			warnings = append(warnings, Warning{
				Source: c.r.Root,
				Detail: fmt.Sprintf("repo %s: config load failed: %v; its packages are unavailable this run", c.r.FQN, c.loadErr),
			})
		}
		if c.enumErr != nil {
			// A repo that cannot enumerate fails loudly and the run
			// continues past it (§3.2 independence in spirit): bulk skips
			// it, and its named packages come back not-found.
			warnings = append(warnings, Warning{
				Source: c.r.Root,
				Detail: fmt.Sprintf("repo %s: cannot enumerate packages: %v; its packages are unavailable this run", c.r.FQN, c.enumErr),
			})
		}
	}

	seen := map[string]int{}
	add := func(e repo.Entity, explicit bool) {
		key := e.FQN.String()
		if i, ok := seen[key]; ok {
			works[i].explicit = works[i].explicit || explicit
			return
		}
		seen[key] = len(works)
		works = append(works, work{pkg: e, rc: byRepo[e.FQN.Repo().String()], explicit: explicit})
	}

	if len(names) == 0 {
		// Bulk (§2.5): all packages of all repos, minus the excluded.
		for _, e := range ents {
			if e.FQN.Package == "" {
				continue
			}
			add(e, false)
		}
	} else {
		for _, input := range names {
			matches, rerr := repo.Resolve(input, ents)
			if rerr != nil {
				preResults = append(preResults, PackageResult{
					Operand: input, Status: StatusFailed, Err: rerr,
				})
				continue
			}
			switch len(matches) {
			case 0:
				preResults = append(preResults, PackageResult{Operand: input, Status: StatusNotFound})
			case 1:
				m := matches[0]
				if m.FQN.IsPackage() {
					add(m, true)
					continue
				}
				// A repo where packages are expected means all its packages
				// (§1.1), explicit — stronger intent wins over exclusion.
				c := byRepo[m.FQN.String()]
				if c.enumErr != nil {
					preResults = append(preResults, PackageResult{
						Operand: input, FQN: m.FQN, Status: StatusFailed, Err: c.enumErr,
					})
					continue
				}
				if len(c.packages) == 0 {
					preResults = append(preResults, PackageResult{
						Operand: input, FQN: m.FQN, Status: StatusSucceeded,
						Notes: []string{fmt.Sprintf("repo %s has no packages", m.FQN)},
					})
					continue
				}
				for _, p := range c.packages {
					add(repo.Entity{
						FQN:  name.FQN{Scheme: m.FQN.Scheme, Coordinate: m.FQN.Coordinate, Package: p},
						Repo: m.Repo,
					}, true)
				}
			default:
				fqns := make([]name.FQN, len(matches))
				for i, m := range matches {
					fqns[i] = m.FQN
				}
				sort.Slice(fqns, func(i, j int) bool { return fqns[i].String() < fqns[j].String() })
				return nil, nil, warnings, &AmbiguousNameError{Input: input, Matches: fqns}
			}
		}
	}

	// Load package levels; drop bulk-excluded packages (explicit naming
	// overrides exclusion, §2.5 — nearer wins is config's law).
	kept := works[:0]
	for i := range works {
		w := works[i]
		lvl, pw, perr := config.LoadPackageLevel(w.rc.pkgRoot(w.pkg.FQN.Package))
		warnings = append(warnings, warnConfig(pw)...)
		if perr != nil {
			w.pkgErr = perr
		} else {
			w.pkgLevel = lvl
		}
		if !w.explicit && w.pkgErr == nil && a.eff(w).ExcludeFromBulk() {
			continue // excluded from bulk, silently — that is the knob's meaning
		}
		kept = append(kept, w)
	}
	works = kept

	orderWork(works)
	return works, preResults, warnings, nil
}
