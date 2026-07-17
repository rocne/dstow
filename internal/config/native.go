package config

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// nativeFile is the whole native key vocabulary (§3.2–3.3). Every level
// decodes into the same schema; the legality matrix (C7) then says which of
// the set keys mean anything here — placement decides the level, the matrix
// decides the meaning (C1).
type nativeFile struct {
	Target               *string                   `toml:"target"`
	TranslateDotPrefixes *bool                     `toml:"translate_dot_prefixes"`
	Ignore               []string                  `toml:"ignore"`
	ExcludeFromBulk      *bool                     `toml:"exclude_from_bulk"`
	PackagesDir          *string                   `toml:"packages_dir"`
	FoldTrees            *bool                     `toml:"fold_trees"`
	Theme                *string                   `toml:"theme"`
	Color                map[string]toml.Primitive `toml:"color"`
}

// legality is the C7 matrix, as amended. A key absent here is unknown; a
// key present is legal exactly at the listed levels. The registry is
// deliberately not a key: config.toml never declares repos (C3).
var legality = map[string][]Level{
	"target":                 {LevelPackage, LevelRepo, LevelGlobal},
	"translate_dot_prefixes": {LevelPackage, LevelRepo, LevelGlobal},
	"ignore":                 {LevelPackage, LevelRepo, LevelGlobal},
	"exclude_from_bulk":      {LevelPackage, LevelRepo},
	"packages_dir":           {LevelRepo},
	"fold_trees":             {LevelGlobal},
	"theme":                  {LevelGlobal},
	"color":                  {LevelGlobal},
}

func legalAt(key string, level Level) bool {
	for _, l := range legality[key] {
		if l == level {
			return true
		}
	}
	return false
}

// levelFileHint names the config file of a level for misplaced-key remedies
// (§3.5: the warning names the legal level and its file). Only the global
// level has one knowable absolute path; repo and package files are named by
// their pattern.
func levelFileHint(l Level) string {
	switch l {
	case LevelGlobal:
		return GlobalConfigFile()
	case LevelRepo:
		return "<repo>/" + metadataDirName + "/" + configFileName
	case LevelPackage:
		return "<package>/" + metadataDirName + "/" + configFileName
	}
	return ""
}

// parseNative decodes one native config file for one level: TOML decode
// (a syntax or type error aborts — there is no partial file to salvage),
// unknown keys warned with did-you-mean, misplaced keys warned naming the
// legal level and its file and then ignored (§3.5), refused-and-reserved
// ignore forms rejected (C16).
func parseNative(path string, content []byte, level Level) (*carrier, []Warning, error) {
	var nf nativeFile
	md, err := toml.Decode(string(content), &nf)
	if err != nil {
		return nil, nil, fmt.Errorf("%s is not valid dstow config: %w", path, err)
	}

	var warns []Warning

	// The [color] table decodes per entry so one wrongly-typed value warns
	// and skips (C18 posture) instead of failing the file. Slot-name
	// vocabulary is ui's to judge (§3.3) — config only carries the strings.
	var colorTable map[string]string
	if nf.Color != nil {
		colorTable = map[string]string{}
		keys := make([]string, 0, len(nf.Color))
		for k := range nf.Color {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			var v string
			if err := md.PrimitiveDecode(nf.Color[k], &v); err != nil {
				warns = append(warns, Warning{
					Source: path,
					Detail: fmt.Sprintf("color key %q must be a string in git's color grammar (this entry is skipped, the rest still applies)", k),
				})
				continue
			}
			colorTable[k] = v
		}
	}

	// Unknown keys: warn, never refuse (C18), with did-you-mean.
	for _, key := range md.Undecoded() {
		full := key.String()
		if strings.HasPrefix(full, "color.") {
			continue // handled (and possibly warned) by the color loop above
		}
		if full == "repos" {
			warns = append(warns, Warning{
				Source: path,
				Detail: "key \"repos\" is not config: the repo registry is dstow-written (repos.toml), never declared in config.toml (the key is ignored)",
				Fix:    "register repos with: dstow repo add <source>",
			})
			continue
		}
		detail := fmt.Sprintf("unknown key %q (the key is ignored, the rest still applies)", full)
		if suggestion := didYouMean(full); suggestion != "" {
			detail = fmt.Sprintf("unknown key %q — did you mean %q? (the key is ignored, the rest still applies)", full, suggestion)
		}
		warns = append(warns, Warning{Source: path, Detail: detail})
	}

	// Misplaced keys: legal elsewhere per the matrix → warn naming the legal
	// level and its file, then ignore the key (§3.5).
	misplaced := func(key string) bool {
		if legalAt(key, level) {
			return false
		}
		var hints []string
		for _, l := range legality[key] {
			hints = append(hints, fmt.Sprintf("the %s level (%s)", l, levelFileHint(l)))
		}
		warns = append(warns, Warning{
			Source: path,
			Detail: fmt.Sprintf("key %q means nothing at the %s level; it belongs at %s (the key is ignored here)",
				key, level, strings.Join(hints, " or ")),
		})
		return true
	}

	c := &carrier{file: path}
	if nf.Target != nil && !misplaced("target") {
		c.target = &strKnob{value: *nf.Target, file: path, key: "target"}
	}
	if nf.TranslateDotPrefixes != nil && !misplaced("translate_dot_prefixes") {
		c.translate = &boolKnob{value: *nf.TranslateDotPrefixes, file: path, key: "translate_dot_prefixes"}
	}
	if nf.ExcludeFromBulk != nil && !misplaced("exclude_from_bulk") {
		c.excludeFromBulk = &boolKnob{value: *nf.ExcludeFromBulk, file: path, key: "exclude_from_bulk"}
	}
	if nf.PackagesDir != nil && !misplaced("packages_dir") {
		c.packagesDir = *nf.PackagesDir
	}
	if nf.FoldTrees != nil && !misplaced("fold_trees") {
		c.fold = &boolKnob{value: *nf.FoldTrees, file: path, key: "fold_trees"}
	}
	if nf.Theme != nil && !misplaced("theme") {
		c.theme = &strKnob{value: *nf.Theme, file: path, key: "theme"}
	}
	if colorTable != nil && !misplaced("color") {
		c.color = colorTable
	}

	if len(nf.Ignore) > 0 && !misplaced("ignore") {
		var patternErrs []error
		for _, p := range nf.Ignore {
			switch {
			case strings.HasPrefix(p, "!"):
				patternErrs = append(patternErrs, &PatternError{
					File:    path,
					Pattern: p,
					Reason: "a leading '!' (negation) is refused: ignores compose additively — a level adds to, never silences, " +
						"inherited ignores (C16). Remove the pattern; to re-include something, narrow the broader pattern instead",
				})
			case strings.HasPrefix(p, "//"):
				patternErrs = append(patternErrs, &PatternError{
					File:    path,
					Pattern: p,
					Reason: "a leading '//' is reserved for a possible future regex marker and has no meaning in v1 (C16); " +
						"write the pattern as a plain gitignore-glob",
				})
			default:
				c.ignores = append(c.ignores, IgnorePattern{
					Pattern:  p,
					Language: LangGlob,
					Level:    level,
					Source:   path,
				})
			}
		}
		if len(patternErrs) > 0 {
			return nil, warns, errors.Join(patternErrs...)
		}
	}

	return c, warns, nil
}

// didYouMean suggests the nearest key of the vocabulary, or "" when nothing
// is close enough to be a plausible typo.
func didYouMean(key string) string {
	best, bestDist := "", 3 // suggest only within edit distance 2
	for known := range legality {
		if d := editDistance(key, known); d < bestDist {
			best, bestDist = known, d
		}
	}
	return best
}

// editDistance is plain Levenshtein — the vocabulary is eight short keys,
// so the simple O(len²) form is the right amount of machinery.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, min(cur[j-1]+1, prev[j-1]+cost))
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
