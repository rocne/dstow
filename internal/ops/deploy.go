package ops

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/hooks"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/name"
)

// DeployRequest parameterizes one stow/unstow/restow run (§2.4). Empty
// Names means bulk — the whole registered set; the interactive "stow
// everything?" gate and --all are cli's, upstream of ops (D2/D9).
type DeployRequest struct {
	Verb   engine.Verb
	Names  []string // name expressions; empty = bulk
	Adopt  bool     // --adopt (D15): stow/restow only
	DryRun bool     // -n: plan, change nothing (D8)
}

// PackageStatus is one package's outcome in a deploy run (§3.2).
type PackageStatus int

const (
	// StatusSucceeded includes the no-op: a package whose plan was empty
	// succeeded with zero actions (§9.1.5 keeps its hooks quiet too).
	StatusSucceeded PackageStatus = iota
	StatusFailed
	// StatusBlocked marks a package a failed repo- or global-pre hook
	// blocked (§9.1.4); its Err carries the blocking HookError.
	StatusBlocked
	// StatusNotFound marks an operand that resolved to nothing (§3.2:
	// "per-package status line (not-found included)").
	StatusNotFound
)

func (s PackageStatus) String() string {
	switch s {
	case StatusSucceeded:
		return "succeeded"
	case StatusFailed:
		return "failed"
	case StatusBlocked:
		return "blocked"
	case StatusNotFound:
		return "not found"
	}
	return fmt.Sprintf("PackageStatus(%d)", int(s))
}

// PackageResult is one package's line in the run (O8 renders it).
type PackageResult struct {
	Operand  string   // the operand this result stands for, when no FQN resolved
	FQN      name.FQN // zero when the operand never resolved
	Status   PackageStatus
	Actions  []engine.Action // executed — or planned, under dry-run
	Err      error           // typed: *engine.ConflictError, *hooks.HookError, config errors…
	Notes    []string        // announcements (§1.3): created target, …
	Warnings []Warning
}

// DeployResult is the whole run as data.
type DeployResult struct {
	Packages  []PackageResult
	Warnings  []Warning // run-level warnings (config chain, enumeration, …)
	Notes     []string  // run-level announcements (first-run folding note, …)
	RunErrors []error   // repo/global post-hook failures (§9.1.4: scope failed, work stays)
	Pruned    []ledger.Pruned
	DryRun    bool
}

// Failed reports whether the run exits nonzero (§3.2, A3 exit 1): any
// package failed, was blocked, or was not found — or a post hook marked a
// wider scope failed.
func (r *DeployResult) Failed() bool {
	for _, p := range r.Packages {
		if p.Status != StatusSucceeded {
			return true
		}
	}
	return len(r.RunErrors) > 0
}

// FoldSource names one repo's effective fold value and where it came from:
// a migrated .stowrc (honored per-repo, REQUIREMENTS §3.3) or the global
// setting every rc-less repo inherits.
type FoldSource struct {
	Repo name.FQN
	File string
}

// FoldConflictError refuses a run whose repos' effective fold values
// contradict (REQUIREMENTS §3.3): folding is a property of a target
// subtree, so a run mixing folded and unfolded repos cannot be honored.
// With --no-folding stow's only fold flag, the live case is a repo rc
// declaring false while the global fold_trees says true. The remedy is
// the global knob.
type FoldConflictError struct {
	True  []FoldSource // repos whose effective fold is on
	False []FoldSource // repos whose effective fold is off
}

func (e *FoldConflictError) Error() string {
	return fmt.Sprintf(
		"repos in this invocation contradict each other on tree folding (%s vs %s); folding is global-only — set fold_trees in %s and remove the fold flags from the repo .stowrc files",
		foldNames(e.True), foldNames(e.False), config.GlobalConfigFile())
}

func foldNames(srcs []FoldSource) string {
	out := ""
	for i, s := range srcs {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%s (%s)", s.Repo, s.File)
	}
	return out
}

// errGlobalBlocked aborts the ledger transaction when a failed global-pre
// hook blocked every action before anything was applied or recorded —
// nothing happened, so nothing is written (§6.2: fn error = no write).
var errGlobalBlocked = errors.New("ops: global-pre hook blocked the run before any change")

// hookAction maps a deploy verb to its hook action. A flag never changes a
// command's concept (D5): stow --adopt fires the stow hooks; only the adopt
// leaf fires the adopt pair.
func hookAction(v engine.Verb) hooks.Action {
	switch v {
	case engine.VerbUnstow:
		return hooks.ActionUnstow
	case engine.VerbRestow:
		return hooks.ActionRestow
	}
	return hooks.ActionStow
}

// prepared is one package's planned run: everything decided before any
// mutation, so failures scope per-package (§3.2) and the acting set is
// known before hooks fire (§9.1.5).
type prepared struct {
	w         work
	target    string
	op        engine.Op // Simulate false; toggled per call
	expected  map[string]string
	plan      []engine.Action
	status    PackageStatus
	err       error
	notes     []string
	notesWarn []Warning
	acting    bool // the plan holds actions: hooks fire, Apply runs
}

// Deploy runs stow, unstow, or restow (§2.4, A13): per-package
// independence, nested/LIFO hooks over the acting set, and the ledger
// transaction of §6.4 — all as data. Run-level refusals (ambiguous
// operand, fold contradiction, ledger refusals) return as error; every
// per-package outcome is a PackageResult.
func (a *App) Deploy(req DeployRequest) (*DeployResult, error) {
	if req.Adopt && req.Verb == engine.VerbUnstow {
		// D15: adoption pre-accepts stow's occupied-path refusal, which
		// unstow never raises.
		return nil, fmt.Errorf("--adopt applies only to stow and restow: adoption resolves an occupied path on deployment, and unstow deploys nothing")
	}

	works, preResults, warns, err := a.selectWork(req.Names)
	if err != nil {
		return nil, err
	}
	res := &DeployResult{DryRun: req.DryRun, Warnings: warns, Packages: preResults}

	// First run (ruled 2026-07-17 on #44: the ledger file not existing yet
	// detects it): announce the folding default loudly (§3.6), naming
	// exactly where the global setting lives.
	if _, serr := os.Stat(a.LedgerPath); os.IsNotExist(serr) {
		res.Notes = append(res.Notes, fmt.Sprintf(
			"first run: tree folding is off — dstow's predictable default; stow veterans can enable it by setting fold_trees = true in %s",
			config.GlobalConfigFile()))
	}

	if ferr := a.foldGuard(works); ferr != nil {
		return nil, ferr
	}

	preps := a.prepare(req, works)

	if req.DryRun {
		for i := range preps {
			p := &preps[i]
			res.Packages = append(res.Packages, PackageResult{
				FQN: p.w.pkg.FQN, Status: p.status, Actions: p.plan,
				Err: p.err, Notes: p.notes,
			})
		}
		return res, nil
	}

	if err := a.execute(req, preps, res); err != nil {
		return nil, err
	}
	return res, nil
}

// foldGuard refuses a run whose repos' effective fold values contradict
// (REQUIREMENTS §3.3). Each repo's effective fold is its migrated rc value
// when set, else the global setting; a run where they differ mixes folded
// and unfolded behavior over one target subtree, which folding's
// global-only nature forbids.
func (a *App) foldGuard(works []work) error {
	globalFold := config.Effective{Global: a.Global}.FoldTrees()
	seen := map[string]bool{}
	conflict := &FoldConflictError{}
	for _, w := range works {
		if w.rc == nil {
			continue
		}
		key := w.rc.r.FQN.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		value, file := globalFold, config.GlobalConfigFile()
		if w.rc.level != nil {
			if v, f, set := w.rc.level.CompatFoldTrees(); set {
				value, file = v, f
			}
		}
		src := FoldSource{Repo: w.rc.r.FQN, File: file}
		if value {
			conflict.True = append(conflict.True, src)
		} else {
			conflict.False = append(conflict.False, src)
		}
	}
	if len(conflict.True) > 0 && len(conflict.False) > 0 {
		return conflict
	}
	return nil
}

// prepare plans every package without mutating anything except target
// creation on a real run (auto-created and announced, §3.1).
func (a *App) prepare(req DeployRequest, works []work) []prepared {
	preps := make([]prepared, 0, len(works))
	createdTargets := map[string]bool{}
	for _, w := range works {
		p := prepared{w: w, status: StatusSucceeded}
		switch {
		case w.rc.loadErr != nil:
			p.status, p.err = StatusFailed, w.rc.loadErr
		case w.pkgErr != nil:
			p.status, p.err = StatusFailed, w.pkgErr
		}
		if p.status == StatusFailed {
			preps = append(preps, p)
			continue
		}

		eff := a.eff(w)
		target, terr := eff.Target()
		if terr != nil {
			p.status, p.err = StatusFailed, terr
			preps = append(preps, p)
			continue
		}
		p.target = target
		p.op = engineOp(w.rc, eff, target, w.pkg.FQN.Package)
		p.op.Adopt = req.Adopt && req.Verb != engine.VerbUnstow

		targetExists := true
		if _, serr := os.Stat(target); os.IsNotExist(serr) {
			targetExists = false
		}

		switch {
		case !targetExists && req.Verb == engine.VerbUnstow:
			// Nothing can be linked under a target that does not exist:
			// unstow is trivially done, and no directory is invented for it.
			p.plan = nil
		case !targetExists && req.DryRun:
			// The plan against a missing target is Expected against an empty
			// one — a pure computation, so dry-run stays mutation-free.
			exp, eerr := engine.Expected(p.op)
			if eerr != nil {
				p.status, p.err = StatusFailed, eerr
				break
			}
			p.expected = exp
			p.notes = append(p.notes, fmt.Sprintf("would create target directory %s", target))
			links := make([]string, 0, len(exp))
			for l := range exp {
				links = append(links, l)
			}
			sort.Strings(links)
			for _, l := range links {
				p.plan = append(p.plan, engine.Action{Kind: engine.LinkCreated, Path: l})
			}
		default:
			if !targetExists {
				if merr := os.MkdirAll(target, 0o755); merr != nil {
					p.status, p.err = StatusFailed, merr
					break
				}
				if !createdTargets[target] {
					createdTargets[target] = true
					p.notes = append(p.notes, fmt.Sprintf("created target directory %s", target))
				}
			}
			sim := p.op
			sim.Simulate = true
			simRes, serr := engine.Apply(req.Verb, sim)
			if serr != nil {
				p.status, p.err = StatusFailed, serr
				break
			}
			p.plan = simRes.Actions
			if req.Verb != engine.VerbUnstow {
				exp, eerr := engine.Expected(p.op)
				if eerr != nil {
					p.status, p.err = StatusFailed, eerr
					break
				}
				p.expected = exp
			}
			// Adopt confirmations fire only on a real run: dry-run shows the
			// plan and changes nothing (D8), prompts included.
			if p.op.Adopt && !req.DryRun {
				if aerr := a.confirmAdoptions(&p); aerr != nil {
					p.status, p.err = StatusFailed, aerr
				}
			}
		}
		p.acting = p.status == StatusSucceeded && len(p.plan) > 0
		preps = append(preps, p)
	}
	return preps
}

// confirmAdoptions applies the adopt rules to a plan's file moves (D15,
// §8.5): where the live file differs from the package's existing content,
// confirmation is required — live content always wins, but never silently
// over differing package content.
func (a *App) confirmAdoptions(p *prepared) error {
	for _, act := range p.plan {
		if act.Kind != engine.FileMoved {
			continue
		}
		source, ok := p.expected[act.Path]
		if !ok {
			continue
		}
		pkgFile := filepath.Join(p.op.Dir, p.op.Package, source)
		liveFile := filepath.Join(p.target, act.Path)
		differs, derr := filesDiffer(pkgFile, liveFile)
		if derr != nil {
			return derr
		}
		if !differs {
			continue
		}
		ok, perr := a.Prompt.Confirm(fmt.Sprintf(
			"adopt %s into %s, overwriting the package's differing content at %s?",
			liveFile, p.w.pkg.FQN, source), false)
		if perr != nil {
			return perr
		}
		if !ok {
			return fmt.Errorf("adoption of %s declined: the package content at %s differs and was not confirmed", liveFile, source)
		}
	}
	return nil
}

// filesDiffer reports whether both paths exist as files with different
// content. A missing package file is not a difference — adopting into an
// empty slot overwrites nothing.
func filesDiffer(pkgFile, liveFile string) (bool, error) {
	pkgData, err := os.ReadFile(pkgFile)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	liveData, err := os.ReadFile(liveFile)
	if err != nil {
		return false, err
	}
	return !bytes.Equal(pkgData, liveData), nil
}

// execute runs the acting packages under the ledger transaction (§6.4):
// one flock for the whole operation, hooks interleaved nested/LIFO over
// the acting set, per-package independence throughout.
func (a *App) execute(req DeployRequest, preps []prepared, res *DeployResult) error {
	actingFQNs := make([]name.FQN, 0, len(preps))
	scope := ledger.Scope{}
	touch := false
	for i := range preps {
		p := &preps[i]
		if p.status != StatusSucceeded {
			continue
		}
		reconcile := req.Verb == engine.VerbRestow
		if p.acting || reconcile {
			scope.Packages = append(scope.Packages, p.w.pkg.FQN.String())
			touch = true
		}
		if p.acting {
			actingFQNs = append(actingFQNs, p.w.pkg.FQN)
			for _, act := range p.plan {
				if act.Kind == engine.LinkCreated || act.Kind == engine.LinkRemoved {
					scope.Paths = append(scope.Paths, filepath.Join(p.target, act.Path))
				}
			}
		}
	}

	if !touch {
		// Nothing acts and nothing reconciles: reads never write (§6.3).
		a.assemble(preps, res)
		return nil
	}

	mutated := false
	pruned, uerr := ledger.Update(a.LedgerPath, scope, func(l *ledger.Ledger) error {
		inv := hooks.NewInvocation(hookAction(req.Verb), a.Hooks,
			hooks.GlobalScope{Dir: a.GlobalDir, Packages: actingFQNs})

		globalBlocked := false
		repoBlocked := map[string]bool{}
		anyApplied := false

		// Group by repo in the already-ordered walk, so repo-post fires
		// after a repo's last package (nested/LIFO, §9.1.3).
		for start := 0; start < len(preps); {
			end := start
			repoKey := preps[start].w.pkg.FQN.Repo().String()
			for end < len(preps) && preps[end].w.pkg.FQN.Repo().String() == repoKey {
				end++
			}
			group := preps[start:end]
			start = end

			repoActed := false
			var repoScope hooks.RepoScope
			for i := range group {
				p := &group[i]
				if p.status != StatusSucceeded {
					continue
				}
				reconcile := req.Verb == engine.VerbRestow

				if p.acting {
					// A failed pre hook blocks its whole scope (§9.1.4).
					switch {
					case globalBlocked:
						p.status = StatusBlocked
						p.err = fmt.Errorf("blocked: a global pre-%s hook failed", hookAction(req.Verb))
						continue
					case repoBlocked[repoKey]:
						p.status = StatusBlocked
						p.err = fmt.Errorf("blocked: a repo pre-%s hook failed", hookAction(req.Verb))
						continue
					}

					pkgScope := hooks.PackageScope{
						FQN:     p.w.pkg.FQN,
						Dir:     p.w.rc.pkgRoot(p.w.pkg.FQN.Package),
						Target:  p.target,
						RepoDir: p.w.rc.r.Root,
					}
					repoScope = hooks.RepoScope{
						FQN: p.w.pkg.FQN.Repo(), Dir: p.w.rc.r.Root,
						Packages: packagesUnder(actingFQNs, repoKey),
					}

					hw, herr := inv.BeforePackage(pkgScope)
					p.warn(hw)
					if herr != nil {
						var he *hooks.HookError
						level := hooks.LevelPackage
						if errors.As(herr, &he) {
							level = he.Level
						}
						p.status, p.err = StatusBlocked, herr
						switch level {
						case hooks.LevelGlobal:
							globalBlocked = true
						case hooks.LevelRepo:
							repoBlocked[repoKey] = true
						default:
							// A failed package-pre blocks that package only;
							// the run continues (§9.1.4 + §3.2).
							p.status = StatusFailed
						}
						continue
					}

					applyRes, aerr := engine.Apply(req.Verb, p.op)
					if aerr != nil {
						// The action did not complete, so its post does not
						// fire — post reports on completed work (§9.1.3).
						p.status, p.err = StatusFailed, aerr
						continue
					}
					p.plan = applyRes.Actions
					a.recordEntries(l, req.Verb, p)
					mutated, anyApplied, repoActed = true, true, true

					hw, herr = inv.AfterPackage(pkgScope)
					p.warn(hw)
					if herr != nil {
						// Completed work stays done; the scope is failed (§9.1.4).
						p.status, p.err = StatusFailed, herr
					}
				} else if reconcile {
					// An intact restow surfaces no actions (stow cancels the
					// opposing pairs), but what IS deployed is Expected — the
					// entry set comes from it, never from Actions (#42 flag).
					a.recordEntries(l, req.Verb, p)
					mutated = true
				}
			}

			if repoActed {
				hw, herr := inv.AfterRepo(repoScope)
				res.Warnings = append(res.Warnings, hookWarnings(hw)...)
				if herr != nil {
					res.RunErrors = append(res.RunErrors, herr)
				}
			}
		}

		if anyApplied {
			hw, herr := inv.Finish()
			res.Warnings = append(res.Warnings, hookWarnings(hw)...)
			if herr != nil {
				res.RunErrors = append(res.RunErrors, herr)
			}
		}

		if !mutated {
			return errGlobalBlocked
		}
		return nil
	})

	if uerr != nil && !errors.Is(uerr, errGlobalBlocked) {
		return uerr
	}
	res.Pruned = append(res.Pruned, pruned...)
	a.assemble(preps, res)
	return nil
}

// warn appends hook warnings onto the package's result.
func (p *prepared) warn(ws []hooks.Warning) {
	for _, w := range ws {
		p.notesWarn = append(p.notesWarn, Warning{Source: w.Source, Detail: w.Detail, Fix: w.Fix})
	}
}

// packagesUnder selects the acting packages of one repo, in order.
func packagesUnder(acting []name.FQN, repoKey string) []name.FQN {
	var out []name.FQN
	for _, f := range acting {
		if f.Repo().String() == repoKey {
			out = append(out, f)
		}
	}
	return out
}

// recordEntries applies §6.4's entry bookkeeping for one package under the
// open transaction: stow adds entries for links created; unstow deletes
// entries for links removed, keyed on path (gostow remove tasks carry no
// source); restow does both and then reconciles the package's entry set to
// Expected — what is deployed, not what changed.
func (a *App) recordEntries(l *ledger.Ledger, verb engine.Verb, p *prepared) {
	group := l.Targets[p.target]
	fqn := p.w.pkg.FQN.String()
	now := a.now().UTC()

	removeAt := func(link string) {
		kept := group[:0:0]
		for _, e := range group {
			if e.Link != link {
				kept = append(kept, e)
			}
		}
		group = kept
	}

	for _, act := range p.plan {
		switch act.Kind {
		case engine.LinkRemoved:
			removeAt(act.Path)
		case engine.LinkCreated:
			if verb == engine.VerbRestow {
				continue // the reconcile below owns restow's additions
			}
			removeAt(act.Path)
			group = append(group, ledger.Entry{
				Link:        act.Path,
				Package:     fqn,
				Source:      p.expected[act.Path],
				Destination: act.Dest,
				RecordedAt:  now,
			})
		}
	}

	if verb == engine.VerbRestow {
		existing := map[string]ledger.Entry{}
		kept := group[:0:0]
		for _, e := range group {
			if e.Package == fqn {
				existing[e.Link] = e
				continue // the package's set is rebuilt from Expected
			}
			kept = append(kept, e)
		}
		group = kept
		links := make([]string, 0, len(p.expected))
		for link := range p.expected {
			links = append(links, link)
		}
		sort.Strings(links)
		for _, link := range links {
			dest, rerr := os.Readlink(filepath.Join(p.target, link))
			if rerr != nil {
				continue // not a link on disk: nothing true to record
			}
			entry := ledger.Entry{
				Link:        link,
				Package:     fqn,
				Source:      p.expected[link],
				Destination: dest,
				RecordedAt:  now,
			}
			if old, ok := existing[link]; ok &&
				old.Source == entry.Source && old.Destination == entry.Destination {
				entry.RecordedAt = old.RecordedAt // unchanged: keep its history
			}
			removeAt(link)
			group = append(group, entry)
		}
	}

	sort.Slice(group, func(i, j int) bool {
		if group[i].Link != group[j].Link {
			return group[i].Link < group[j].Link
		}
		return group[i].Package < group[j].Package
	})
	if l.Targets == nil {
		l.Targets = map[string][]ledger.Entry{}
	}
	l.Targets[p.target] = group
}

// assemble folds the prepared outcomes into the result.
func (a *App) assemble(preps []prepared, res *DeployResult) {
	for i := range preps {
		p := &preps[i]
		actions := p.plan
		if !p.acting && p.status == StatusSucceeded {
			actions = nil
		}
		res.Packages = append(res.Packages, PackageResult{
			FQN: p.w.pkg.FQN, Status: p.status, Actions: actions,
			Err: p.err, Notes: p.notes, Warnings: p.notesWarn,
		})
	}
}
