package ops

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rocne/gostow/stow"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/ignore"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// Candidate is one package that could adopt a file (REQUIREMENTS §8.5):
// its effective target covers the path, the mapped source is not ignored,
// and per-package dot-translation decided the source spelling. Neighbors
// counts the package's ledgered links already living in the file's
// directory — the ranking signal.
type Candidate struct {
	FQN       name.FQN
	Source    string // the package-relative source adoption would write
	Neighbors int
}

// AdoptCandidates enumerates the packages that could adopt file, ranked:
// packages already owning neighboring paths first, canonical-FQN order as
// the tie-break (ruled 2026-07-17 on #44). A pure config+ledger
// computation — no tree walking (§8.5).
func (a *App) AdoptCandidates(file string) ([]Candidate, []Warning, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, nil, err
	}
	led, err := ledger.Load(a.LedgerPath)
	if err != nil {
		return nil, nil, err
	}

	ctxs := a.loadRepoCtxs()
	var warnings []Warning
	var out []Candidate
	for _, c := range ctxs {
		warnings = append(warnings, c.warns...)
		if c.loadErr != nil || c.enumErr != nil {
			continue // its warning is already recorded by loadRepoCtxs
		}
		for _, p := range c.packages {
			w := work{
				pkg: repo.Entity{
					FQN:  name.FQN{Scheme: c.r.FQN.Scheme, Coordinate: c.r.FQN.Coordinate, Package: p},
					Repo: c.r,
				},
				rc: c,
			}
			lvl, pw, perr := config.LoadPackageLevel(c.pkgRoot(p))
			warnings = append(warnings, warnConfig(pw)...)
			if perr != nil {
				warnings = append(warnings, Warning{
					Source: c.pkgRoot(p),
					Detail: fmt.Sprintf("package %s: config load failed: %v; not considered as an adopt candidate", w.pkg.FQN, perr),
				})
				continue
			}
			w.pkgLevel = lvl
			eff := a.eff(w)
			target, terr := eff.Target()
			if terr != nil {
				continue // its target cannot resolve; it cannot cover anything
			}
			rel, rerr := filepath.Rel(target, abs)
			if rerr != nil || rel == "." || strings.HasPrefix(rel, "..") {
				continue // target does not cover the path
			}
			source := sourceRelFor(rel, eff.TranslateDotPrefixes())
			ignored, ierr := candidateIgnored(eff.Ignores(), source)
			if ierr != nil {
				return nil, warnings, ierr
			}
			if ignored {
				continue
			}
			out = append(out, Candidate{
				FQN:       w.pkg.FQN,
				Source:    source,
				Neighbors: neighborCount(led, target, w.pkg.FQN.String(), rel),
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Neighbors != out[j].Neighbors {
			return out[i].Neighbors > out[j].Neighbors
		}
		return out[i].FQN.String() < out[j].FQN.String()
	})
	return out, warnings, nil
}

// neighborCount counts the package's ledgered links in the file's own
// directory — the §8.5 ranking signal, straight from the ledger.
func neighborCount(led ledger.Ledger, target, fqn, rel string) int {
	dir := filepath.Dir(rel)
	n := 0
	for _, e := range led.Targets[target] {
		if e.Package == fqn && filepath.Dir(e.Link) == dir {
			n++
		}
	}
	return n
}

// candidateIgnored decides "not ignored" for a hypothetical source path
// from config alone (§8.5 is a pure config computation): the M8 metadata
// auto-ignore, the native gitignore-glob chain, and the compat stow-regex
// entries — each compat pattern compiled by gostow's own exported
// CompilePattern under the --ignore anchor and matched the way stow
// matches --ignore patterns, against every node on the path (the walk
// checks each ancestor). Package ignore files and stow's built-in floor
// are tree state, deliberately outside this computation.
func candidateIgnored(patterns []config.IgnorePattern, source string) (bool, error) {
	source = filepath.ToSlash(source)
	if source == ".dstow" || strings.HasPrefix(source, ".dstow/") {
		return true, nil // M8: unsilencable, in no config
	}

	var globs []config.IgnorePattern
	var compat []string
	for _, p := range patterns {
		if p.Language == config.LangGlob {
			globs = append(globs, p)
		} else {
			compat = append(compat, p.Pattern)
		}
	}

	chain, err := ignore.Compile(globs)
	if err != nil {
		return false, err
	}
	nodes := pathNodes(source)
	for i, node := range nodes {
		isDir := i < len(nodes)-1
		if chain.Match(node, isDir) {
			return true, nil
		}
	}
	for _, pat := range compat {
		re, cerr := stow.CompilePattern("ignore", stow.IgnoreAnchor, pat)
		if cerr != nil {
			return false, cerr
		}
		for _, node := range nodes {
			if re.MatchString(node) {
				return true, nil
			}
		}
	}
	return false, nil
}

// pathNodes lists every node on a "/"-joined relative path, ancestors
// first: "a/b/c" → a, a/b, a/b/c — the nodes a stow walk would consult.
func pathNodes(rel string) []string {
	parts := strings.Split(rel, "/")
	nodes := make([]string, 0, len(parts))
	for i := range parts {
		nodes = append(nodes, strings.Join(parts[:i+1], "/"))
	}
	return nodes
}
