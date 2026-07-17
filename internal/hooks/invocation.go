package hooks

import (
	"github.com/rocne/dstow/internal/name"
)

// Invocation drives the nested/LIFO hook lifecycle for one dstow invocation
// (A11 + REQUIREMENTS §9.1.3): global-pre → repo-pre → package-pre → action →
// package-post → repo-post → global-post. It owns ordering and
// once-per-invocation firing; ops owns the iteration loop and calls the
// methods below in nesting order for each acting scope.
//
// Each hooks directory is discovered lazily, at most once per invocation, and
// its warnings are surfaced exactly once — from whichever method first reaches
// it. Each pre hook fires at most once per scope: once fired is fired, whether
// it succeeded or failed, so a later BeforePackage never retries a pre hook
// that already ran (a failed global/repo-pre means ops blocks everything under
// it per §9.1.4, and hooks must not undo that by re-firing).
type Invocation struct {
	action  Action
	runner  Runner
	global  GlobalScope
	globals bool // global-pre fired, and the global scope activated (⇒ Finish may fire global-post)

	repoPreFired  map[string]bool // repo-pre fired, keyed by RepoFQN.String()
	repoPostFired map[string]bool // repo-post fired, keyed by RepoFQN.String()

	cache map[string]cachedDir // hooks dir → its discovery, so warnings surface once
}

// cachedDir memoizes one hooks directory's discovery.
type cachedDir struct {
	set Set
	err error
}

// NewInvocation begins a hook invocation for one action, over the injected
// Runner and the global scope (its Dir supplies the global hooks dir and cwd).
func NewInvocation(action Action, r Runner, global GlobalScope) *Invocation {
	return &Invocation{
		action:        action,
		runner:        r,
		global:        global,
		repoPreFired:  map[string]bool{},
		repoPostFired: map[string]bool{},
		cache:         map[string]cachedDir{},
	}
}

// BeforePackage fires the pre hooks nesting down to one package, each at most
// once per invocation and in order: global-pre (first BeforePackage only),
// repo-pre (first BeforePackage for this repo, keyed by FQN.Repo()), then this
// package's package-pre. It stops at the first failure and returns it — the
// firing that failed is still marked fired, so a later BeforePackage will not
// retry it. Returned warnings are those newly incurred by directories this
// call discovered for the first time.
func (inv *Invocation) BeforePackage(pkg PackageScope) ([]Warning, error) {
	var warns []Warning

	// global-pre, once per invocation.
	if !inv.globals {
		inv.globals = true
		w, err := inv.fire(LevelGlobal, PhasePre, inv.global.hooksDir(), inv.global.Dir,
			globalVars(inv.action, PhasePre, inv.global.Packages))
		warns = append(warns, w...)
		if err != nil {
			return warns, err
		}
	}

	// repo-pre, once per repo.
	repoFQN := pkg.FQN.Repo()
	if repoKey := repoFQN.String(); !inv.repoPreFired[repoKey] {
		inv.repoPreFired[repoKey] = true
		w, err := inv.fire(LevelRepo, PhasePre, ScopeHooksDir(pkg.RepoDir), pkg.RepoDir,
			repoVars(inv.action, PhasePre, repoFQN, pkg.RepoDir, inv.packagesForRepo(repoFQN)))
		warns = append(warns, w...)
		if err != nil {
			return warns, err
		}
	}

	// package-pre.
	w, err := inv.fire(LevelPackage, PhasePre, ScopeHooksDir(pkg.Dir), pkg.Dir,
		packageVars(inv.action, PhasePre, pkg))
	warns = append(warns, w...)
	return warns, err
}

// AfterPackage fires this package's package-post hook.
func (inv *Invocation) AfterPackage(pkg PackageScope) ([]Warning, error) {
	return inv.fire(LevelPackage, PhasePost, ScopeHooksDir(pkg.Dir), pkg.Dir,
		packageVars(inv.action, PhasePost, pkg))
}

// AfterRepo fires a repo's repo-post hook, at most once per repo (keyed by
// FQN). A second AfterRepo for the same repo is a no-op with no error.
func (inv *Invocation) AfterRepo(repo RepoScope) ([]Warning, error) {
	repoKey := repo.FQN.String()
	if inv.repoPostFired[repoKey] {
		return nil, nil
	}
	inv.repoPostFired[repoKey] = true
	return inv.fire(LevelRepo, PhasePost, ScopeHooksDir(repo.Dir), repo.Dir,
		repoVars(inv.action, PhasePost, repo.FQN, repo.Dir, repo.Packages))
}

// Finish fires the global-post hook, at most once, and only if the global
// scope ever activated — i.e. at least one BeforePackage happened. With no
// package having acted, global-pre never fired and global-post must not either
// (§9.1.5: nothing changed, so the global scope stays quiet).
func (inv *Invocation) Finish() ([]Warning, error) {
	if !inv.globals {
		return nil, nil
	}
	return inv.fire(LevelGlobal, PhasePost, inv.global.hooksDir(), inv.global.Dir,
		globalVars(inv.action, PhasePost, inv.global.Packages))
}

// packagesForRepo selects the acting packages belonging to one repo, for the
// repo level's DSTOW_HOOK_PACKAGES when firing from BeforePackage (which has no
// RepoScope). "All packages acting under this repo" is exactly the subset of
// the invocation's acting packages whose repo is this one (H2/H4).
func (inv *Invocation) packagesForRepo(repoFQN name.FQN) []name.FQN {
	key := repoFQN.String()
	var out []name.FQN
	for _, p := range inv.global.Packages {
		if p.Repo().String() == key {
			out = append(out, p)
		}
	}
	return out
}

// fire discovers hooksDir once, then execs the hook for (phase, action) with
// cwd and the composed environment. A missing hook file is a silent no-op
// (hooks are optional). Returned warnings are non-nil only the first time this
// directory is discovered; every failure — a hook failing, or discovery itself
// failing — comes back as a *HookError, so the caller always has the Level to
// apply §9.1.4 blocking (an unreadable hooks dir must never silently skip a
// hook the user installed as a guard).
func (inv *Invocation) fire(level Level, phase Phase, hooksDir, cwd string, vars []kv) ([]Warning, error) {
	set, warns, err := inv.discover(hooksDir)
	if err != nil {
		return warns, &HookError{Level: level, Action: inv.action, Phase: phase, Path: hooksDir, Err: err}
	}
	path, ok := set[Hook{Phase: phase, Action: inv.action}]
	if !ok {
		return warns, nil
	}
	if runErr := inv.runner.run(cwd, path, environ(vars)); runErr != nil {
		return warns, &HookError{Level: level, Action: inv.action, Phase: phase, Path: path, Err: runErr}
	}
	return warns, nil
}

// discover memoizes one hooks directory's Discover result: the Set persists for
// the whole invocation, and its warnings are returned only on the first call
// (surfaced exactly once). A ReadDir error is cached too, so it is not retried.
func (inv *Invocation) discover(hooksDir string) (Set, []Warning, error) {
	if c, ok := inv.cache[hooksDir]; ok {
		return c.set, nil, c.err
	}
	set, warns, err := Discover(hooksDir)
	inv.cache[hooksDir] = cachedDir{set: set, err: err}
	return set, warns, err
}
