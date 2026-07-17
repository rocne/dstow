package ops

// The shared maintenance classifier (§6.4): check and clean run this one
// function over a ledger.Ledger snapshot + the App environment, so "clean
// executes exactly this report" holds by construction — check on a lock-free
// Load, clean on the fresh document inside Update's fn. Contradicted is the
// one owner of the disk-disagrees test (consulted, never reimplemented);
// orderWork's canonical sort is not needed here because findings sort on the
// ledger's own target/link/package keys.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// Class is a stale-entry classification (§6.4). The order is the precedence
// order: an entry is the first class it matches.
type Class int

const (
	// ClassUnobservable is the #45 ruling (issue comment 5005576678): an
	// observation the classification needs failed with a non-ENOENT error
	// (the link lstat/readlink, or the destination existence check). It is a
	// read-only row whose evidence is the OS error; clean never acts on it.
	ClassUnobservable Class = iota
	// ClassContradicted is Entry.Contradicted's verdict: disk disagrees with
	// the entry (link gone, non-link, or different link text). clean prunes
	// the entry only, never the disk.
	ClassContradicted
	// ClassBroken is a link that agrees with the ledger but whose recorded
	// destination is gone. clean removes the link and the entry, freely.
	ClassBroken
	// ClassOrphaned is an intact link resolving into a known repo that the
	// current config would not produce. clean removes it behind confirmation.
	ClassOrphaned
)

func (c Class) String() string {
	switch c {
	case ClassUnobservable:
		return "unobservable"
	case ClassContradicted:
		return "contradicted"
	case ClassBroken:
		return "broken"
	case ClassOrphaned:
		return "orphaned"
	}
	return fmt.Sprintf("Class(%d)", int(c))
}

// Finding is one classified ledger entry (§6.4): the target root it lives
// under, the entry itself, its class, and complete evidence prose.
type Finding struct {
	TargetRoot string
	Entry      ledger.Entry
	Class      Class
	Evidence   string
}

// classifier holds the per-invocation environment the classification reads:
// the repo set loaded once (config + enumeration), a repo lookup, and a memo
// of engine.Expected keyed by package FQN so Expected is computed at most
// once per distinct package appearing in the ledger (§6.4). Warnings
// accumulate here — no unbacked claims (REQUIREMENTS §1.5): where a repo or
// package's config cannot load, the orphan test is unanswerable and the
// entry is left unclassified with a Warning instead.
type classifier struct {
	app      *App
	ctxs     []*repoCtx
	byRepo   map[string]*repoCtx
	expected map[string]expectedSet // memo: package FQN string → its answer
	warned   map[string]bool        // repos whose base warnings were surfaced
	warnSeen map[string]bool        // Source\x00Detail dedup
	warnings []Warning
}

// expectedSet is one package's memoized orphan-test answer: exp holds its
// current engine.Expected when computable; exists is false when the package
// or its repo no longer enumerates (config cannot produce it — orphaned);
// answerable is false when config failed to load (unanswerable — claim
// nothing).
type expectedSet struct {
	exp        map[string]string
	exists     bool
	answerable bool
}

// newClassifier loads the repo set once and builds the shared classifier.
func (a *App) newClassifier() *classifier {
	ctxs := a.loadRepoCtxs()
	sort.Slice(ctxs, func(i, j int) bool {
		return ctxs[i].r.FQN.String() < ctxs[j].r.FQN.String()
	})
	byRepo := make(map[string]*repoCtx, len(ctxs))
	for _, c := range ctxs {
		byRepo[c.r.FQN.String()] = c
	}
	return &classifier{
		app:      a,
		ctxs:     ctxs,
		byRepo:   byRepo,
		expected: map[string]expectedSet{},
		warned:   map[string]bool{},
		warnSeen: map[string]bool{},
	}
}

// classify runs the precedence over every entry of a ledger snapshot,
// returning the findings in the deterministic report order (target root
// lexical, then link, then package). A healthy entry produces no row.
func (c *classifier) classify(led ledger.Ledger) []Finding {
	var findings []Finding
	for _, root := range sortedRoots(led) {
		for _, e := range sortedEntries(led.Targets[root]) {
			class, evidence, row := c.classifyEntry(root, e)
			if !row {
				continue
			}
			findings = append(findings, Finding{
				TargetRoot: root, Entry: e, Class: class, Evidence: evidence,
			})
		}
	}
	return findings
}

// classifyEntry applies the §6.4 precedence to one entry, first match wins.
// row is false for a healthy entry (no report row).
func (c *classifier) classifyEntry(targetRoot string, e ledger.Entry) (class Class, evidence string, row bool) {
	linkPath := filepath.Join(targetRoot, e.Link)

	// 1. Unobservable (#45): the link's own observation failing non-ENOENT is
	// a read-only row whose evidence is the OS error. Contradicted swallows
	// such errors (disk-is-truth claims only against a real sighting), so the
	// pre-check here owns them before consulting it.
	info, err := os.Lstat(linkPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ClassUnobservable, fmt.Sprintf("cannot observe the link at %s: %v", linkPath, err), true
	}
	if err == nil && info.Mode()&fs.ModeSymlink != 0 {
		if _, rerr := os.Readlink(linkPath); rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
			return ClassUnobservable, fmt.Sprintf("cannot read the link at %s: %v", linkPath, rerr), true
		}
	}

	// 2. Contradicted: the one owner of the disk-disagrees test.
	if bad, prose := e.Contradicted(targetRoot); bad {
		return ClassContradicted, prose, true
	}

	// The link now agrees with the ledger. 3. Broken: its recorded
	// destination is gone (resolved relative to the link's directory).
	dest := e.Destination
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(filepath.Dir(linkPath), dest)
	}
	dest = filepath.Clean(dest)
	if _, serr := os.Stat(dest); serr != nil {
		if errors.Is(serr, fs.ErrNotExist) {
			return ClassBroken, fmt.Sprintf(
				"the link at %s points to %q, but its destination %s no longer exists",
				linkPath, e.Destination, dest), true
		}
		// Non-ENOENT on the existence check ⇒ unobservable (#45, rule 1).
		return ClassUnobservable, fmt.Sprintf(
			"cannot observe the destination of the link at %s (%s): %v", linkPath, dest, serr), true
	}

	// 4. Orphaned: the destination resolves into a known repo but the
	// current config would not produce this link. A destination in no known
	// repo is none of the classes — the entry is healthy.
	repoFQN, known := c.repoOf(dest)
	if !known {
		return 0, "", false
	}
	fqn, perr := name.ParseFQN(e.Package)
	if perr != nil {
		// The recorded package name is unparseable: the orphan test cannot be
		// answered. No unbacked claims (§1.5) — warn and leave it unclassified.
		c.warn(Warning{Source: linkPath, Detail: fmt.Sprintf(
			"the ledger entry at %s records an unparseable package %q (%v); cannot decide whether it is orphaned",
			linkPath, e.Package, perr)})
		return 0, "", false
	}
	ex := c.expectedFor(fqn)
	if !ex.answerable {
		return 0, "", false // warned in expectedFor; claim nothing
	}
	if ex.exists {
		if src, ok := ex.exp[e.Link]; ok && src == e.Source {
			return 0, "", false // current config produces exactly this link
		}
	}
	return ClassOrphaned, fmt.Sprintf(
		"the link at %s resolves into repo %s, but no current package configuration produces it",
		linkPath, repoFQN), true
}

// repoOf reports the registered repo whose root is a path prefix of dest.
func (c *classifier) repoOf(dest string) (name.FQN, bool) {
	for _, ctx := range c.ctxs {
		if underRoot(ctx.r.Root, dest) {
			return ctx.r.FQN, true
		}
	}
	return name.FQN{}, false
}

// expectedFor answers the orphan test for one package, memoized so Expected
// is computed at most once per distinct package (§6.4).
func (c *classifier) expectedFor(fqn name.FQN) expectedSet {
	key := fqn.String()
	if r, ok := c.expected[key]; ok {
		return r
	}
	r := c.computeExpected(fqn)
	c.expected[key] = r
	return r
}

// computeExpected builds the package's engine.Expected exactly as deploy.go
// builds engine.Op (effective four-level config, current tree). A repo or
// package that no longer exists/enumerates means the config cannot produce
// the link (orphaned); a repo/package whose config fails to load makes the
// test unanswerable (a Warning, no claim).
func (c *classifier) computeExpected(fqn name.FQN) expectedSet {
	rc := c.byRepo[fqn.Repo().String()]
	if rc == nil {
		return expectedSet{answerable: true} // repo unregistered: config cannot produce it
	}
	c.surfaceRepoWarnings(rc)
	if rc.loadErr != nil || rc.enumErr != nil {
		cause := rc.loadErr
		if cause == nil {
			cause = rc.enumErr
		}
		c.warn(Warning{Source: rc.r.Root, Detail: fmt.Sprintf(
			"repo %s: config unavailable (%v); cannot decide whether %s's links are orphaned — leaving them unclassified",
			rc.r.FQN, cause, fqn)})
		return expectedSet{answerable: false}
	}
	if !containsStr(rc.packages, fqn.Package) {
		return expectedSet{answerable: true} // package gone from the tree
	}

	lvl, pw, perr := config.LoadPackageLevel(rc.pkgRoot(fqn.Package))
	c.warnAll(warnConfig(pw))
	if perr != nil {
		c.warn(Warning{Source: rc.pkgRoot(fqn.Package), Detail: fmt.Sprintf(
			"package %s: config load failed (%v); cannot decide whether its links are orphaned", fqn, perr)})
		return expectedSet{answerable: false}
	}
	w := work{pkg: repo.Entity{FQN: fqn, Repo: rc.r}, rc: rc, pkgLevel: lvl}
	eff := c.app.eff(w)
	target, terr := eff.Target()
	if terr != nil {
		c.warn(Warning{Source: rc.pkgRoot(fqn.Package), Detail: fmt.Sprintf(
			"package %s: effective target unresolvable (%v); cannot decide whether its links are orphaned", fqn, terr)})
		return expectedSet{answerable: false}
	}
	op := engine.Op{
		Dir:                  rc.stowDir(),
		Target:               target,
		Package:              fqn.Package,
		Fold:                 eff.FoldTrees(),
		TranslateDotPrefixes: eff.TranslateDotPrefixes(),
		Ignores:              eff.Ignores(),
	}
	exp, eerr := engine.Expected(op)
	if eerr != nil {
		c.warn(Warning{Source: rc.pkgRoot(fqn.Package), Detail: fmt.Sprintf(
			"package %s: cannot compute its expected links (%v); leaving them unclassified", fqn, eerr)})
		return expectedSet{answerable: false}
	}
	return expectedSet{exp: exp, exists: true, answerable: true}
}

// surfaceRepoWarnings emits a consulted repo's enumeration warnings once.
func (c *classifier) surfaceRepoWarnings(rc *repoCtx) {
	if c.warned[rc.r.FQN.String()] {
		return
	}
	c.warned[rc.r.FQN.String()] = true
	c.warnAll(rc.warns)
}

// warn appends a warning, deduped on (Source, Detail).
func (c *classifier) warn(w Warning) {
	key := w.Source + "\x00" + w.Detail
	if c.warnSeen[key] {
		return
	}
	c.warnSeen[key] = true
	c.warnings = append(c.warnings, w)
}

// warnAll appends a batch of warnings through warn's dedup.
func (c *classifier) warnAll(ws []Warning) {
	for _, w := range ws {
		c.warn(w)
	}
}

// underRoot reports whether p lies at or under root.
func underRoot(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// sortedRoots lists a ledger's target roots in lexical order.
func sortedRoots(led ledger.Ledger) []string {
	roots := make([]string, 0, len(led.Targets))
	for root := range led.Targets {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

// sortedEntries returns a copy of a group ordered by link, then package.
func sortedEntries(entries []ledger.Entry) []ledger.Entry {
	out := make([]ledger.Entry, len(entries))
	copy(out, entries)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Link != out[j].Link {
			return out[i].Link < out[j].Link
		}
		return out[i].Package < out[j].Package
	})
	return out
}

// sortFindings orders findings in the report order: target root lexical,
// then link, then package (§6.4).
func sortFindings(fs []Finding) {
	sort.Slice(fs, func(i, j int) bool {
		if fs[i].TargetRoot != fs[j].TargetRoot {
			return fs[i].TargetRoot < fs[j].TargetRoot
		}
		if fs[i].Entry.Link != fs[j].Entry.Link {
			return fs[i].Entry.Link < fs[j].Entry.Link
		}
		return fs[i].Entry.Package < fs[j].Entry.Package
	})
}
