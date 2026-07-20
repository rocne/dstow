package ops

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/hooks"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// AdoptRequest parameterizes the adopt leaf (§2.4, REQUIREMENTS §8.5):
// import a real file into a package, a link takes its place, live content
// always wins. Exactly one of File or Occupied selects the scope.
type AdoptRequest struct {
	File     string // path operand: the real file to adopt
	Package  string // name expression; must resolve to one package
	Occupied bool   // --occupied: every occupied path of the package
	Force    bool   // overwrite differing package content without asking
	DryRun   bool
}

// AdoptMove is one adoption: the live file and the package-relative source
// that received it (a link at File takes its place).
type AdoptMove struct {
	File   string // absolute path in the target world
	Source string // package-relative destination
}

// AdoptSkip is one path adopt left alone, with its reason (a declined
// confirmation, a non-file occupant).
type AdoptSkip struct {
	File   string
	Reason string
}

// AdoptResult is the adopt run as data.
type AdoptResult struct {
	FQN      name.FQN
	Moves    []AdoptMove // executed — or planned, under dry-run
	Skipped  []AdoptSkip
	Errs     []error // move failures and post-hook failures; completed work stays
	Notes    []string
	Warnings []Warning
	Pruned   []ledger.Pruned
	DryRun   bool
}

// Failed reports whether the run exits nonzero.
func (r *AdoptResult) Failed() bool { return len(r.Errs) > 0 }

// errAdoptBlocked aborts the ledger transaction when a pre hook blocked the
// adoption before anything moved.
var errAdoptBlocked = errors.New("ops: a pre-adopt hook blocked the run before any change")

// Adopt runs the adopt leaf. Refusals — an unresolvable or ambiguous
// package, a path outside the target, a non-file occupant, an ignored
// path — return as error before anything mutates; per-move outcomes land
// in the result.
func (a *App) Adopt(req AdoptRequest) (*AdoptResult, error) {
	w, warns, err := a.resolveOnePackage(req.Package)
	res := &AdoptResult{DryRun: req.DryRun, Warnings: warns}
	if err != nil {
		return nil, err
	}
	res.FQN = w.pkg.FQN

	eff := a.eff(w)
	target, terr := eff.Target()
	if terr != nil {
		return nil, terr
	}
	op := engineOp(w.rc, eff, target, w.pkg.FQN.Package)

	var moves []AdoptMove
	if req.Occupied {
		moves, err = a.occupiedMoves(op, res)
	} else {
		moves, err = a.singleMove(req.File, target, eff.TranslateDotPrefixes(), op, w)
	}
	if err != nil {
		return nil, err
	}

	// Adopt rules (§8.5): the plan is shown (the caller renders Moves), and
	// differing package content is overwritten only behind confirmation or
	// --force. Live content always wins — the question is only whether now.
	kept := moves[:0]
	for _, m := range moves {
		pkgFile := filepath.Join(op.Dir, op.Package, m.Source)
		differs, derr := filesDiffer(pkgFile, m.File)
		if derr != nil {
			return nil, derr
		}
		if differs && !req.Force {
			ok, perr := a.Prompt.Confirm(fmt.Sprintf(
				"adopt %s into %s, overwriting the package's differing content at %s?",
				m.File, w.pkg.FQN, m.Source), false)
			if perr != nil {
				return nil, perr
			}
			if !ok {
				res.Skipped = append(res.Skipped, AdoptSkip{
					File: m.File, Reason: "declined: package content differs",
				})
				continue
			}
		}
		kept = append(kept, m)
	}
	moves = kept
	res.Moves = moves

	if req.DryRun || len(moves) == 0 {
		// Nothing changes: no hooks fire (§9.1.5) and nothing is written.
		return res, nil
	}

	scope := ledger.Scope{Packages: []string{w.pkg.FQN.String()}}
	for _, m := range moves {
		scope.Paths = append(scope.Paths, m.File)
	}
	pruned, uerr := ledger.Update(a.LedgerPath, scope, func(l *ledger.Ledger) error {
		return a.executeAdopt(l, w, op, target, moves, res)
	})
	if uerr != nil && !errors.Is(uerr, errAdoptBlocked) {
		return nil, uerr
	}
	res.Pruned = append(res.Pruned, pruned...)
	return res, nil
}

// executeAdopt performs the moves under the open transaction, wrapped in
// the adopt hook pair (§9.1).
func (a *App) executeAdopt(l *ledger.Ledger, w work, op engine.Op, target string, moves []AdoptMove, res *AdoptResult) error {
	inv := hooks.NewInvocation(hooks.ActionAdopt, a.Hooks,
		hooks.GlobalScope{Dir: a.GlobalDir, Packages: []name.FQN{w.pkg.FQN}})
	pkgScope := hooks.PackageScope{
		FQN:     w.pkg.FQN,
		Dir:     w.rc.pkgRoot(w.pkg.FQN.Package),
		Target:  target,
		RepoDir: w.rc.r.Root,
	}

	hw, herr := inv.BeforePackage(pkgScope)
	res.Warnings = append(res.Warnings, hookWarnings(hw)...)
	if herr != nil {
		res.Errs = append(res.Errs, herr)
		res.Moves = nil
		return errAdoptBlocked
	}

	moved := false
	completed := moves[:0:0]
	for _, m := range moves {
		if merr := a.adoptOne(l, w, op, target, m); merr != nil {
			res.Errs = append(res.Errs, fmt.Errorf("adopt %s: %w", m.File, merr))
			continue
		}
		completed = append(completed, m)
		moved = true
	}
	res.Moves = completed

	if moved {
		hw, herr = inv.AfterPackage(pkgScope)
		res.Warnings = append(res.Warnings, hookWarnings(hw)...)
		if herr != nil {
			res.Errs = append(res.Errs, herr)
		}
		hw, herr = inv.AfterRepo(hooks.RepoScope{
			FQN: w.pkg.FQN.Repo(), Dir: w.rc.r.Root, Packages: []name.FQN{w.pkg.FQN},
		})
		res.Warnings = append(res.Warnings, hookWarnings(hw)...)
		if herr != nil {
			res.Errs = append(res.Errs, herr)
		}
		hw, herr = inv.Finish()
		res.Warnings = append(res.Warnings, hookWarnings(hw)...)
		if herr != nil {
			res.Errs = append(res.Errs, herr)
		}
		return nil
	}
	return errAdoptBlocked // every move failed: nothing to record
}

// adoptOne moves one live file into the package and leaves a link behind,
// recording the entry. The link is relative, stow's own stance for links
// it creates — this is the one mutation ops performs itself, because
// single-file adoption is narrower than the engine's whole-package verbs
// (A13 gives adopt to ops).
func (a *App) adoptOne(l *ledger.Ledger, w work, op engine.Op, target string, m AdoptMove) error {
	srcAbs := filepath.Join(op.Dir, op.Package, m.Source)
	if err := os.MkdirAll(filepath.Dir(srcAbs), 0o755); err != nil {
		return err
	}
	if err := moveFile(m.File, srcAbs); err != nil {
		return err
	}
	linkText, err := filepath.Rel(filepath.Dir(m.File), srcAbs)
	if err != nil {
		return err
	}
	if err := os.Symlink(linkText, m.File); err != nil {
		return err
	}

	rel, err := filepath.Rel(target, m.File)
	if err != nil {
		return err
	}
	group := l.Targets[target]
	kept := group[:0:0]
	for _, e := range group {
		if e.Link != rel {
			kept = append(kept, e)
		}
	}
	group = append(kept, ledger.Entry{
		Link:        rel,
		Package:     w.pkg.FQN.String(),
		Source:      m.Source,
		Destination: linkText,
		RecordedAt:  a.now().UTC(),
	})
	sort.Slice(group, func(i, j int) bool { return group[i].Link < group[j].Link })
	if l.Targets == nil {
		l.Targets = map[string][]ledger.Entry{}
	}
	l.Targets[target] = group
	return nil
}

// singleMove plans the adoption of one named file (§2.4 adopt).
func (a *App) singleMove(file, target string, translate bool, op engine.Op, w work) ([]AdoptMove, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s does not exist: adopt imports an existing real file", abs)
		}
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file: adopt imports real files only (live content always wins; for a directory, adopt its files)", abs)
	}
	rel, err := filepath.Rel(target, abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("%s is outside %s's effective target %s; adopt into a package whose target covers it (see dstow adopt with no package for candidates)", abs, w.pkg.FQN, target)
	}
	source := sourceRelFor(rel, translate)
	ignored, ierr := candidateIgnored(op.Ignores, source)
	if ierr != nil {
		return nil, ierr
	}
	if ignored {
		return nil, fmt.Errorf("%s maps to %s, which %s's ignore chain excludes; adopting it would create a link stow does not own — adjust the ignore chain first", abs, source, w.pkg.FQN)
	}
	return []AdoptMove{{File: abs, Source: source}}, nil
}

// occupiedMoves plans --occupied: every expected path of the package whose
// target position holds a real file (§2.4). Non-file occupants are
// reported and left alone — they are stow conflicts, not adoptable files.
func (a *App) occupiedMoves(op engine.Op, res *AdoptResult) ([]AdoptMove, error) {
	expected, err := engine.Expected(op)
	if err != nil {
		return nil, err
	}
	links := make([]string, 0, len(expected))
	for link := range expected {
		links = append(links, link)
	}
	sort.Strings(links)

	var moves []AdoptMove
	for _, link := range links {
		abs := filepath.Join(op.Target, link)
		info, lerr := os.Lstat(abs)
		if lerr != nil {
			continue // nothing occupies the path
		}
		if !info.Mode().IsRegular() {
			if info.Mode()&os.ModeSymlink != 0 {
				continue // a link is stow's business (ours or a conflict), never adopted
			}
			res.Skipped = append(res.Skipped, AdoptSkip{
				File: abs, Reason: "occupant is not a regular file",
			})
			continue
		}
		moves = append(moves, AdoptMove{File: abs, Source: expected[link]})
	}
	if len(moves) == 0 && len(res.Skipped) == 0 {
		res.Notes = append(res.Notes, "no occupied paths: every expected path is free or already linked")
	}
	return moves, nil
}

// resolveOnePackage resolves a name expression to exactly one package
// entity, with its levels loaded — adopt's operand rule.
func (a *App) resolveOnePackage(input string) (work, []Warning, error) {
	ctxs := a.loadRepoCtxs()
	ents, byRepo := entities(ctxs)
	var warnings []Warning
	for _, c := range ctxs {
		warnings = append(warnings, c.warns...)
	}
	matches, err := repo.Resolve(input, ents)
	if err != nil {
		return work{}, warnings, err
	}
	switch len(matches) {
	case 0:
		return work{}, warnings, fmt.Errorf("%q matches no registered package; dstow list shows what is registered", input)
	case 1:
		m := matches[0]
		if !m.FQN.IsPackage() {
			return work{}, warnings, fmt.Errorf("%q names the repo %s; adopt imports into a package — name one of its packages", input, m.FQN)
		}
		w := work{pkg: m, rc: byRepo[m.FQN.Repo().String()], explicit: true}
		lvl, pw, perr := config.LoadPackageLevel(w.rc.pkgRoot(w.pkg.FQN.Package))
		warnings = append(warnings, warnConfig(pw)...)
		if perr != nil {
			return work{}, warnings, perr
		}
		w.pkgLevel = lvl
		return w, warnings, nil
	default:
		fqns := make([]name.FQN, len(matches))
		for i, m := range matches {
			fqns[i] = m.FQN
		}
		sort.Slice(fqns, func(i, j int) bool { return fqns[i].String() < fqns[j].String() })
		return work{}, warnings, &AmbiguousNameError{Input: input, Matches: fqns}
	}
}

// sourceRelFor maps a target-relative path to the package-relative source
// adopt writes: with dot-translation on, each ".foo" component is written
// in the package's canonical "dot-foo" spelling (§3.4 — translation only
// rewrites dot- prefixed names, and adopt writes new package content in
// the convention the package deploys under).
func sourceRelFor(rel string, translate bool) string {
	if !translate {
		return rel
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, p := range parts {
		if len(p) > 1 && strings.HasPrefix(p, ".") && p != ".." {
			parts[i] = "dot-" + p[1:]
		}
	}
	return strings.Join(parts, "/")
}

// moveFile renames src over dst, falling back to copy-and-remove when the
// rename crosses filesystems (the target and the repo commonly do).
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}

// hookWarnings converts hook warnings to ops warnings.
func hookWarnings(ws []hooks.Warning) []Warning {
	out := make([]Warning, 0, len(ws))
	for _, w := range ws {
		out = append(out, Warning{Source: w.Source, Detail: w.Detail, Fix: w.Fix})
	}
	return out
}
