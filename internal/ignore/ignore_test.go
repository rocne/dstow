package ignore_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/ignore"
)

// glob builds a native chain entry the way config's loaders do (C16/C17).
func glob(pattern string) config.IgnorePattern {
	return config.IgnorePattern{
		Pattern:  pattern,
		Language: config.LangGlob,
		Level:    config.LevelPackage,
		Source:   "pkg/.dstow/config.toml",
	}
}

func compile(t *testing.T, patterns ...string) *ignore.Chain {
	t.Helper()
	entries := make([]config.IgnorePattern, len(patterns))
	for i, p := range patterns {
		entries[i] = glob(p)
	}
	c, err := ignore.Compile(entries)
	if err != nil {
		t.Fatalf("Compile(%q) = %v, want nil error", patterns, err)
	}
	return c
}

// --- §3.4 / C16: gitignore-glob semantics ------------------------------------

func TestMatchSemantics(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		rel      string
		isDir    bool
		want     bool
	}{
		// No slash = basename at any depth (C16).
		{"basename at root", []string{"*.log"}, "debug.log", false, true},
		{"basename at depth", []string{"*.log"}, "a/b/debug.log", false, true},
		{"bare name matches dir at depth", []string{"build"}, "x/build", true, true},
		{"bare name matches file at depth", []string{"build"}, "x/build", false, true},
		{"bare name is whole segment", []string{"build"}, "x/rebuild", false, false},

		// Slash = anchored to the package root (C16).
		{"leading slash anchors", []string{"/vendor"}, "vendor", true, true},
		{"leading slash does not float", []string{"/vendor"}, "x/vendor", true, false},
		{"middle slash anchors", []string{"docs/notes.md"}, "docs/notes.md", false, true},
		{"middle slash does not float", []string{"docs/notes.md"}, "x/docs/notes.md", false, false},

		// Trailing slash = directory-only (C16).
		{"dir-only matches a directory", []string{"cache/"}, "cache", true, true},
		{"dir-only rejects a file", []string{"cache/"}, "cache", false, false},
		{"dir-only floats like a bare name", []string{"cache/"}, "a/cache", true, true},

		// ** is supported (C16).
		{"doublestar bridges depth", []string{"a/**/z"}, "a/b/c/z", false, true},
		{"doublestar leading", []string{"**/tmp"}, "x/y/tmp", true, true},
		{"doublestar trailing", []string{"gen/**"}, "gen/a/b", false, true},

		// Single * never crosses a separator.
		{"star stays in segment", []string{"a/*/z"}, "a/b/c/z", false, false},

		// Non-matches.
		{"unrelated path", []string{"*.log"}, "a/b/file.txt", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := compile(t, tt.patterns...)
			if got := c.Match(tt.rel, tt.isDir); got != tt.want {
				t.Errorf("Match(%q, isDir=%v) with %q = %v, want %v",
					tt.rel, tt.isDir, tt.patterns, got, tt.want)
			}
		})
	}
}

// --- §3.4: the chain is additive ----------------------------------------------

func TestChainIsAdditive(t *testing.T) {
	// Entries from different levels compose: any entry matching ignores the
	// path (a level adds to, never silences, inherited ignores).
	entries := []config.IgnorePattern{
		{Pattern: "*.bak", Language: config.LangGlob, Level: config.LevelGlobal, Source: "global config.toml"},
		{Pattern: "/secrets", Language: config.LangGlob, Level: config.LevelRepo, Source: "repo config.toml"},
		{Pattern: "scratch/", Language: config.LangGlob, Level: config.LevelPackage, Source: "pkg config.toml"},
	}
	c, err := ignore.Compile(entries)
	if err != nil {
		t.Fatalf("Compile = %v, want nil", err)
	}
	for _, probe := range []struct {
		rel   string
		isDir bool
	}{
		{"deep/old.bak", false},
		{"secrets", true},
		{"a/scratch", true},
	} {
		if !c.Match(probe.rel, probe.isDir) {
			t.Errorf("Match(%q, isDir=%v) = false, want true (additive chain)", probe.rel, probe.isDir)
		}
	}
}

func TestEmptyChainMatchesNothing(t *testing.T) {
	c, err := ignore.Compile(nil)
	if err != nil {
		t.Fatalf("Compile(nil) = %v, want nil", err)
	}
	if c.Match("anything", false) || c.Match(".hidden", true) {
		t.Error("empty chain matched a path; want no matches")
	}
}

func TestBlankPatternsAreInert(t *testing.T) {
	// gitignore semantics: a blank line means nothing (C16 adopts the
	// language wholesale).
	c := compile(t, "", "   ", "*.log")
	if c.Match("file.txt", false) {
		t.Error("blank pattern matched; want inert")
	}
	if !c.Match("x.log", false) {
		t.Error("real pattern alongside blanks did not match")
	}
}

// --- C16: refused-and-reserved forms never reach the matcher -------------------

func TestRefusedForms(t *testing.T) {
	for _, pattern := range []string{"!important", "//^re$"} {
		t.Run(pattern, func(t *testing.T) {
			_, err := ignore.Compile([]config.IgnorePattern{glob(pattern)})
			if err == nil {
				t.Fatalf("Compile(%q) = nil error, want refusal (C16)", pattern)
			}
			var perr *config.PatternError
			if !errors.As(err, &perr) {
				t.Fatalf("Compile(%q) error = %T, want *config.PatternError", pattern, err)
			}
			if perr.Pattern != pattern {
				t.Errorf("PatternError.Pattern = %q, want %q", perr.Pattern, pattern)
			}
			if perr.File != "pkg/.dstow/config.toml" {
				t.Errorf("PatternError.File = %q, want the declaring source", perr.File)
			}
		})
	}
}

// --- A15: languages stay in their lanes ----------------------------------------

func TestCompatLanguageRefused(t *testing.T) {
	entries := []config.IgnorePattern{{
		Pattern:  `\.git`,
		Language: config.LangStowRegex,
		Level:    config.LevelRepo,
		Source:   "repo/.stowrc",
	}}
	_, err := ignore.Compile(entries)
	if err == nil {
		t.Fatal("Compile(stow-regex entry) = nil error, want lane refusal (A15)")
	}
	if !strings.Contains(err.Error(), "stow-regex") {
		t.Errorf("lane refusal %q does not name the offending language", err)
	}
}
