// Package config is dstow's four-level configuration chain (DESIGN.md §3 +
// A8): the legality matrix over the one key vocabulary, use-time path
// expansion, warnings-as-data for unknown and misplaced keys, stow
// compatibility (stowrc discovery, slotting, option mapping, and supplement
// diffing via gostow's public stowrc package), content-sniff routing for a
// renamed rc, DSTOW_PATH parsing, and the metadata-location accessor.
//
// The package returns data, never output (A4): every diagnostic comes back
// as a Warning value or a typed error, and the caller decides how to render
// it. Loading is per level — LoadGlobal, LoadRepoLevel, LoadPackageLevel —
// so a broken level scopes exactly to what that level governs (C8, C21);
// Effective composes loaded levels into one per-package view, nearest level
// winning per knob except the additive ignore chain (REQUIREMENTS §4.1).
package config

import "fmt"

// Level identifies one level of the chain (§3.1). The built-in floor is a
// level too: it is where the out-of-the-box defaults live (REQUIREMENTS
// §4.4).
type Level int

const (
	LevelBuiltin Level = iota
	LevelGlobal
	LevelRepo
	LevelPackage
)

func (l Level) String() string {
	switch l {
	case LevelBuiltin:
		return "built-in"
	case LevelGlobal:
		return "global"
	case LevelRepo:
		return "repo"
	case LevelPackage:
		return "package"
	}
	return fmt.Sprintf("Level(%d)", int(l))
}

// Warning is a diagnostic as data (A4): loaders return warnings, the caller
// decides when and how to print them. Config warnings are surprise-class
// announcements (§3.5) — they survive --quiet. Detail is complete prose;
// Fix, when set, is the remedy line (O2's fix severity) the caller renders
// after it.
type Warning struct {
	Source string // where it came from: a file path, "DSTOW_PATH", …
	Detail string // complete prose: what is wrong and that the rest applies
	Fix    string // optional remedy, e.g. the native spelling to migrate to
}

// Language is the pattern language of an ignore carrier (C17): native
// carriers speak gitignore-glob, compat carriers speak stow regex; never
// mixed in one file.
type Language int

const (
	LangGlob      Language = iota // native gitignore-glob (C16)
	LangStowRegex                 // compat stow regex (.stowrc --ignore)
)

func (l Language) String() string {
	switch l {
	case LangGlob:
		return "gitignore-glob"
	case LangStowRegex:
		return "stow-regex"
	}
	return fmt.Sprintf("Language(%d)", int(l))
}

// IgnorePattern is one entry of the additive ignore chain (§3.4): a level
// adds to, never silences, inherited ignores, so entries carry provenance
// instead of overriding each other.
type IgnorePattern struct {
	Pattern  string
	Language Language
	Level    Level
	Source   string // the file that declared it
}

// PatternError refuses an ignore pattern: a native refused-and-reserved form
// (C16: leading '!' or '//') or a non-RE2 compat pattern (C21). The refusal
// scopes to the level the pattern governs — the loader for that level
// returns it, and the run continues past whatever that level covered.
type PatternError struct {
	File    string
	Pattern string // empty when the pattern is only known inside Reason
	Reason  string
}

func (e *PatternError) Error() string {
	if e.Pattern == "" {
		return fmt.Sprintf("in %s: %s", e.File, e.Reason)
	}
	return fmt.Sprintf("ignore pattern %q in %s: %s", e.Pattern, e.File, e.Reason)
}

// ExpandError is a C8 failure: a path-valued key whose use-time expansion
// hit an unset variable, or whose expanded result is not absolute. It names
// variable-or-result + file + key, and scopes per package — only effective
// views that select the offending value error.
type ExpandError struct {
	File   string // provenance; "built-in default" for the floor
	Key    string // the key as spelled where it was set ("target", "--target option", …)
	Value  string
	Reason string
}

func (e *ExpandError) Error() string {
	return fmt.Sprintf("config key %s = %q (from %s): %s", e.Key, e.Value, e.File, e.Reason)
}
