package ops

// rebuild (§6.4): dstow's one full tree walk, explicit and rare. It scans
// every currently configured target root for symlinks into known repos and
// wholesale-replaces those roots' ledger groups, leaving unscanned roots
// untouched. Under one ledger.Update with an empty Scope{}: wholesale
// replacement supersedes pruning, and unscanned groups must survive. The walk
// is lstat-based and never descends through symlinks (filepath.WalkDir does
// not follow them).

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// RebuildResult is a rebuild run as data (A4): the per-root entry counts of
// the roots it scanned, and every diagnostic the scan raised.
type RebuildResult struct {
	Counts   map[string]int // scanned target root → entries recorded
	Warnings []Warning
}

// repoOwner is one repo's ownership probe: its FQN and the stow directory
// engine.Owner tests links against (deploy.go's engine.Op.Dir, packages_dir
// included).
type repoOwner struct {
	fqn     name.FQN
	stowDir string
	rc      *repoCtx
}

// Rebuild reconstructs the ledger by scanning configured targets (§6.4). The
// target set is the union of effective targets over all packages of all
// registered repos; each scanned root's group is replaced with exactly the
// owned links found there, and roots whose walk fails are left untouched.
func (a *App) Rebuild() (*RebuildResult, error) {
	res := &RebuildResult{Counts: map[string]int{}}

	ctxs := a.loadRepoCtxs()
	sort.Slice(ctxs, func(i, j int) bool {
		return ctxs[i].r.FQN.String() < ctxs[j].r.FQN.String()
	})
	owners := make([]repoOwner, 0, len(ctxs))
	for _, c := range ctxs {
		owners = append(owners, repoOwner{fqn: c.r.FQN, stowDir: c.stowDir(), rc: c})
	}

	targets := a.rebuildTargets(ctxs, res)

	// Empty Scope{}: no pruning — rebuild replaces scanned groups wholesale
	// and every unscanned group survives untouched.
	_, uerr := ledger.Update(a.LedgerPath, ledger.Scope{}, func(l *ledger.Ledger) error {
		for _, root := range targets {
			entries, scanned, warns := a.scanTarget(root, owners)
			res.Warnings = append(res.Warnings, warns...)
			if !scanned {
				continue // unobservable root: its group is left untouched
			}
			sort.Slice(entries, func(i, j int) bool {
				if entries[i].Link != entries[j].Link {
					return entries[i].Link < entries[j].Link
				}
				return entries[i].Package < entries[j].Package
			})
			if len(entries) == 0 {
				delete(l.Targets, root)
			} else {
				if l.Targets == nil {
					l.Targets = map[string][]ledger.Entry{}
				}
				l.Targets[root] = entries
			}
			res.Counts[root] = len(entries)
		}
		return nil
	})
	if uerr != nil {
		return nil, uerr // corrupt / newer-version / lock refusals pass through
	}
	return res, nil
}

// rebuildTargets is the union of effective targets over every package of
// every registered repo, loaded the way selectWork loads them (per-repo +
// per-package levels). A config load failure contributes no target and a
// Warning — no unbacked claims (§1.5). The set is returned sorted.
func (a *App) rebuildTargets(ctxs []*repoCtx, res *RebuildResult) []string {
	set := map[string]bool{}
	for _, c := range ctxs {
		res.Warnings = append(res.Warnings, c.warns...)
		if c.loadErr != nil {
			res.Warnings = append(res.Warnings, Warning{Source: c.r.Root, Detail: fmt.Sprintf(
				"repo %s: config load failed: %v; its packages contribute no rebuild target", c.r.FQN, c.loadErr)})
			continue
		}
		if c.enumErr != nil {
			res.Warnings = append(res.Warnings, Warning{Source: c.r.Root, Detail: fmt.Sprintf(
				"repo %s: cannot enumerate packages: %v; its packages contribute no rebuild target", c.r.FQN, c.enumErr)})
			continue
		}
		for _, p := range c.packages {
			lvl, pw, perr := config.LoadPackageLevel(c.pkgRoot(p))
			res.Warnings = append(res.Warnings, warnConfig(pw)...)
			fqn := name.FQN{Scheme: c.r.FQN.Scheme, Coordinate: c.r.FQN.Coordinate, Package: p}
			if perr != nil {
				res.Warnings = append(res.Warnings, Warning{Source: c.pkgRoot(p), Detail: fmt.Sprintf(
					"package %s: config load failed: %v; contributes no rebuild target", fqn, perr)})
				continue
			}
			w := work{pkg: repo.Entity{FQN: fqn, Repo: c.r}, rc: c, pkgLevel: lvl}
			target, terr := a.eff(w).Target()
			if terr != nil {
				res.Warnings = append(res.Warnings, Warning{Source: c.pkgRoot(p), Detail: fmt.Sprintf(
					"package %s: effective target unresolvable: %v; contributes no rebuild target", fqn, terr)})
				continue
			}
			set[target] = true
		}
	}
	targets := make([]string, 0, len(set))
	for t := range set {
		targets = append(targets, t)
	}
	sort.Strings(targets)
	return targets
}

// scanTarget walks one target root, collecting the symlinks owned by a known
// repo. A missing root (ENOENT) is scanned and empty — absence is a sighting.
// Any walk failure that is not ENOENT makes the whole root unobservable: NOT
// scanned (its group is left untouched) and a Warning — replacing the group
// wholesale while part of the tree went unseen would claim absence without a
// sighting (REQUIREMENTS §1.5), the same posture as check's unobservable row.
func (a *App) scanTarget(root string, owners []repoOwner) (entries []ledger.Entry, scanned bool, warns []Warning) {
	if _, err := os.Lstat(root); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, true, nil // absence is a sighting: scanned and empty
		}
		return nil, false, []Warning{{Source: root, Detail: fmt.Sprintf(
			"target %s cannot be observed (%v); its ledger group is left untouched", root, err)}}
	}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return fmt.Errorf("cannot walk %s: %w", path, werr)
		}
		if path == root {
			return nil
		}
		// WalkDir uses lstat and never descends through a symlink (a symlinked
		// directory reports as a symlink, not a dir), so a folded-tree link is
		// recorded as one entry and its interior is never followed.
		if d.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		if e, ok := a.ownedEntry(root, path, owners); ok {
			entries = append(entries, e)
		}
		return nil
	})
	if walkErr != nil {
		return nil, false, []Warning{{Source: root, Detail: fmt.Sprintf(
			"target %s cannot be fully observed (%v); its ledger group is left untouched", root, walkErr)}}
	}
	return entries, true, warns
}

// ownedEntry composes the ledger entry for a symlink owned by a known repo,
// or reports not-owned. Ownership is engine.Owner against each repo's stow
// dir; the entry's fields compose exactly as §6.1 and resolve.go's entities
// do — Destination is the literal link text, Source is the destination made
// relative to the owning package's root, RecordedAt is now.
func (a *App) ownedEntry(root, path string, owners []repoOwner) (ledger.Entry, bool) {
	for _, o := range owners {
		pkg, owned, err := engine.Owner(o.stowDir, path)
		if err != nil || !owned {
			continue
		}
		link, lerr := filepath.Rel(root, path)
		if lerr != nil {
			continue
		}
		text, rerr := os.Readlink(path)
		if rerr != nil {
			continue
		}
		dest := text
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(filepath.Dir(path), dest)
		}
		dest = filepath.Clean(dest)
		source, serr := filepath.Rel(o.rc.pkgRoot(pkg), dest)
		if serr != nil {
			continue
		}
		fqn := name.FQN{Scheme: o.fqn.Scheme, Coordinate: o.fqn.Coordinate, Package: pkg}
		return ledger.Entry{
			Link:        link,
			Package:     fqn.String(),
			Source:      source,
			Destination: text,
			RecordedAt:  a.now().UTC(),
		}, true
	}
	return ledger.Entry{}, false
}
