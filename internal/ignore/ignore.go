// Package ignore is dstow's native ignore matcher (DESIGN.md A15 + §3.4):
// the additive ignore chain's gitignore-glob entries, compiled once and
// matched per package against package-root-relative paths. C16 semantics
// come from go-git's gitignore implementation — no slash means basename at
// any depth, a slash anchors to the package root, a trailing slash is
// directory-only, and ** is supported.
//
// Languages stay in their lanes (A15): this package speaks gitignore-glob
// only. Compat stow-regex entries ride gostow's own Options.Ignore — engine
// routes them there — and are refused here. The two refused-and-reserved
// native forms (C16: leading '!' and leading '//') are refused at config
// parse and must never reach the matcher; Compile refuses them again as an
// invariant guard.
//
// The engine's always-on .dstow auto-ignore (M8) is not a chain entry — it
// rides the engine's IgnoreFunc seam, in no config, unsilencable.
package ignore

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/rocne/dstow/internal/config"
)

// Chain is a compiled native ignore chain: every gitignore-glob entry of one
// package's effective, additive chain (§3.4). Any entry matching ignores the
// path — a level adds to, never silences, inherited ignores, so there is no
// precedence to resolve. The zero-entry chain matches nothing.
type Chain struct {
	matcher gitignore.Matcher
}

// Compile builds a Chain from the effective chain's entries. Every entry
// must speak gitignore-glob (A15) and be free of the refused-and-reserved
// forms (C16) — config's loaders guarantee both, so a violation here is a
// wiring bug, refused with the same *config.PatternError shape the loaders
// use. Blank patterns are inert, as in gitignore itself.
func Compile(entries []config.IgnorePattern) (*Chain, error) {
	patterns := make([]gitignore.Pattern, 0, len(entries))
	for _, e := range entries {
		if e.Language != config.LangGlob {
			return nil, fmt.Errorf(
				"ignore pattern %q from %s speaks %s: only gitignore-glob entries reach the native matcher (A15 — compat chains ride the engine's own ignore option)",
				e.Pattern, e.Source, e.Language)
		}
		switch {
		case strings.HasPrefix(e.Pattern, "!"):
			return nil, &config.PatternError{
				File:    e.Source,
				Pattern: e.Pattern,
				Reason:  "a leading '!' (negation) is refused and must never reach the matcher (C16)",
			}
		case strings.HasPrefix(e.Pattern, "//"):
			return nil, &config.PatternError{
				File:    e.Source,
				Pattern: e.Pattern,
				Reason:  "a leading '//' is reserved and must never reach the matcher (C16)",
			}
		}
		if strings.TrimSpace(e.Pattern) == "" {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(e.Pattern, nil))
	}
	return &Chain{matcher: gitignore.NewMatcher(patterns)}, nil
}

// Match reports whether the chain ignores rel — a package-root-relative,
// "/"-joined path, exactly as the engine's IgnoreFunc seam presents it
// (A16). isDir is the node's lstat kind, which trailing-slash patterns
// require.
func (c *Chain) Match(rel string, isDir bool) bool {
	return c.matcher.Match(strings.Split(rel, "/"), isDir)
}
