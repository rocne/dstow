package ops

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/name"
)

// PackageState is a package's live deployment state (REQUIREMENTS §7.2),
// expected-vs-actual against the CURRENT effective config. String returns the
// CONTEXT.md state strings verbatim (O10: including the space in "partially
// stowed").
type PackageState int

const (
	// StateNotStowed: none of the package's expected links are deployed and
	// nothing occupies their slots (also the empty-package case).
	StateNotStowed PackageState = iota
	// StateStowed: every expected link is present and owned by this package.
	StateStowed
	// StatePartiallyStowed: some but not all expected links are deployed.
	StatePartiallyStowed
	// StateOccupied: a real file or a foreign link sits where an expected link
	// would go — neutral, no claim how. Outranks the stowed/partial spectrum:
	// the slot is taken, not "merely missing" (CONTEXT.md package states), so it
	// holds even when some of the package's other links are stowed.
	StateOccupied
	// StateDamaged: dstow's ledger records a link here that disk now
	// contradicts — claimed only with ledger evidence (REQUIREMENTS §7.2).
	StateDamaged
)

func (s PackageState) String() string {
	switch s {
	case StateNotStowed:
		return "not stowed"
	case StateStowed:
		return "stowed"
	case StatePartiallyStowed:
		return "partially stowed"
	case StateOccupied:
		return "occupied"
	case StateDamaged:
		return "damaged"
	}
	return fmt.Sprintf("PackageState(%d)", int(s))
}

// LinkState is one expected (or ledgered) link's disk verdict.
type LinkState int

const (
	// LinkStowed: a symlink owned by this package sits at the slot.
	LinkStowed LinkState = iota
	// LinkMissing: nothing exists at the slot.
	LinkMissing
	// LinkOccupied: a real file/dir or a foreign link occupies the slot.
	LinkOccupied
	// LinkDamaged: the ledger recorded a link here and disk contradicts it.
	LinkDamaged
)

func (s LinkState) String() string {
	switch s {
	case LinkStowed:
		return "stowed"
	case LinkMissing:
		return "missing"
	case LinkOccupied:
		return "occupied"
	case LinkDamaged:
		return "damaged"
	}
	return fmt.Sprintf("LinkState(%d)", int(s))
}

// LinkStatus is one link's per-slot detail (REQUIREMENTS §7.2.4).
type LinkStatus struct {
	Link   string // target-relative
	Source string // package-relative source the current config expects, "" for a ledger-only link
	State  LinkState
	Detail string // evidence prose
}

// PackageStatusResult is one package's live status (REQUIREMENTS §7.2).
type PackageStatusResult struct {
	FQN      name.FQN
	State    PackageState
	Drifted  bool // §7.2 marker: a stowed package whose deployed shape differs from current config
	Links    []LinkStatus
	Warnings []Warning
}

// classifyPackage computes a package's live status against current effective
// config and the ledger snapshot (REQUIREMENTS §7.2): engine.Expected under
// the current config gives the wanted links; disk (lstat + engine.Owner) gives
// reality; the ledger is the sole source of the "damaged" and "drifted"
// evidence. It never writes.
func (a *App) classifyPackage(c *repoCtx, pkg string, led ledger.Ledger) PackageStatusResult {
	fqn := name.FQN{Scheme: c.r.FQN.Scheme, Coordinate: c.r.FQN.Coordinate, Package: pkg}
	res := PackageStatusResult{FQN: fqn}

	lvl, pw, perr := config.LoadPackageLevel(c.pkgRoot(pkg))
	res.Warnings = append(res.Warnings, warnConfig(pw)...)
	if perr != nil {
		res.Warnings = append(res.Warnings, Warning{
			Source: c.pkgRoot(pkg),
			Detail: fmt.Sprintf("package %s: config load failed: %v; its status cannot be computed", fqn, perr),
		})
		res.State = StateNotStowed
		return res
	}

	eff := config.Effective{Global: a.Global, Repo: c.level, Package: lvl}
	target, terr := eff.Target()
	if terr != nil {
		res.Warnings = append(res.Warnings, Warning{Source: fqn.String(), Detail: terr.Error()})
		res.State = StateNotStowed
		return res
	}
	op := engineOp(c, eff, target, pkg)
	expected, eerr := engine.Expected(op)
	if eerr != nil {
		res.Warnings = append(res.Warnings, Warning{
			Source: fqn.String(),
			Detail: fmt.Sprintf("cannot compute expected links: %v", eerr),
		})
		expected = map[string]string{}
	}

	// The package's ledger entries under this target root.
	var pkgEntries []ledger.Entry
	for _, e := range led.Targets[target] {
		if e.Package == fqn.String() {
			pkgEntries = append(pkgEntries, e)
		}
	}

	// Damaged evidence: any recorded link disk now contradicts (§7.2 — the
	// only path to a damaged claim). Contradicted is the single owner of the
	// disk-disagrees test, consulted, never reimplemented.
	damagedLinks := map[string]string{} // link -> evidence
	for _, e := range pkgEntries {
		if bad, prose := e.Contradicted(target); bad {
			damagedLinks[e.Link] = prose
		}
	}

	// Per-link verdict over the expected set.
	total := len(expected)
	stowed, occupied := 0, 0
	links := make([]LinkStatus, 0, total)
	for _, link := range sortedKeys(expected) {
		source := expected[link]
		ls := LinkStatus{Link: link, Source: source}
		if ev, dmg := damagedLinks[link]; dmg {
			ls.State, ls.Detail = LinkDamaged, ev
			links = append(links, ls)
			continue
		}
		state, detail := a.classifyLink(op.Dir, target, link, pkg)
		ls.State, ls.Detail = state, detail
		switch state {
		case LinkStowed:
			stowed++
		case LinkOccupied:
			occupied++
		}
		links = append(links, ls)
	}

	// Ledger-only damaged links not among the expected set (e.g. a link that
	// drifted out of config and is now clobbered) still carry damage evidence.
	for _, e := range pkgEntries {
		if _, ok := expected[e.Link]; ok {
			continue
		}
		if ev, dmg := damagedLinks[e.Link]; dmg {
			links = append(links, LinkStatus{Link: e.Link, Source: e.Source, State: LinkDamaged, Detail: ev})
		}
	}

	res.Links = links
	res.State = packageState(len(damagedLinks) > 0, total, stowed, occupied)

	// Drifted marker (§7.2): a stowed package whose deployed shape (the ledger)
	// differs from what current config would produce. Only claimed when there
	// is a deployment record to compare against.
	if res.State == StateStowed && len(pkgEntries) > 0 {
		recorded := map[string]string{}
		for _, e := range pkgEntries {
			if _, dmg := damagedLinks[e.Link]; dmg {
				continue // a contradicted entry is not "deployed shape"
			}
			recorded[e.Link] = e.Source
		}
		res.Drifted = !sameStringMap(recorded, expected)
	}
	return res
}

// classifyLink gives one expected slot's disk verdict (non-damaged path):
// stowed when a symlink owned by this package sits there, missing when nothing
// does, occupied when a real thing or a foreign link does.
func (a *App) classifyLink(dir, target, link, pkg string) (LinkState, string) {
	linkPath := filepath.Join(target, link)
	info, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return LinkMissing, fmt.Sprintf("nothing is deployed at %s", linkPath)
		}
		return LinkOccupied, fmt.Sprintf("cannot observe %s: %v", linkPath, err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		return LinkOccupied, fmt.Sprintf("%s holds a real %s, not this package's link", linkPath, ledger.KindOf(info))
	}
	owner, owned, oerr := engine.Owner(dir, linkPath)
	if oerr != nil {
		return LinkOccupied, fmt.Sprintf("cannot resolve the owner of %s: %v", linkPath, oerr)
	}
	if owned && owner == pkg {
		return LinkStowed, ""
	}
	// A link into another package (even in this repo) or out of this repo does
	// not count as stowed for this package (REQUIREMENTS §7.2.3 attribution).
	dest, _ := os.Readlink(linkPath)
	return LinkOccupied, fmt.Sprintf("%s is a symlink to %q, not owned by %s", linkPath, dest, pkg)
}

// packageState applies the CONTEXT.md package-state precedence:
// damaged > occupied > stowed > partially stowed > not stowed. damaged (ledger
// evidence) wins; then any foreign-occupied slot makes the package occupied —
// "partially stowed" is defined as some present with the rest *merely missing*,
// so an occupied slot disqualifies it; only then do the present/missing counts
// decide stowed / partially / not stowed.
func packageState(damaged bool, total, stowed, occupied int) PackageState {
	if damaged {
		return StateDamaged
	}
	if occupied > 0 {
		return StateOccupied
	}
	if total == 0 {
		return StateNotStowed
	}
	if stowed == total {
		return StateStowed
	}
	if stowed > 0 {
		return StatePartiallyStowed
	}
	return StateNotStowed
}

// sortedKeys returns a map's keys in lexical order.
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sameStringMap reports whether two string maps are equal.
func sameStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}
