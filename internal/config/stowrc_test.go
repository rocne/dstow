package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/config"
)

// C19/C20: ~/.stowrc slots to the global level; mappable options land on
// their native knobs.
func TestGlobalStowrcMapsKnobs(t *testing.T) {
	_, home := setupEnv(t)
	dst := t.TempDir()
	writeFile(t, filepath.Join(home, ".stowrc"),
		"--target='"+dst+"'\n--dotfiles\n--ignore='\\.git'\n")
	g, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	eff := config.Effective{Global: g}

	target, err := eff.Target()
	if err != nil {
		t.Fatalf("Target(): %v", err)
	}
	if target != dst {
		t.Errorf("Target() = %q, want the rc-mapped %q", target, dst)
	}
	if !eff.TranslateDotPrefixes() {
		t.Error("--dotfiles did not map to translate_dot_prefixes=true")
	}

	ignores := eff.Ignores()
	if len(ignores) != 1 {
		t.Fatalf("Ignores() = %v, want the one rc entry", ignores)
	}
	if ignores[0].Language != config.LangStowRegex {
		t.Error("rc --ignore entry is not LangStowRegex (C17: compat carriers speak stow regex)")
	}
	if ignores[0].Pattern != `\.git` {
		t.Errorf("rc ignore pattern = %q, want `\\.git`", ignores[0].Pattern)
	}
	// A migrated rc runs loudly but is not an error; no conflict here, so the
	// only acceptable warnings are none at all.
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
}

// C19: --no-folding maps to fold_trees=false; flag absence maps to nothing.
func TestGlobalStowrcNoFolding(t *testing.T) {
	_, home := setupEnv(t)
	writeFile(t, filepath.Join(home, ".stowrc"), "--no-folding\n")
	g, _, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if (config.Effective{Global: g}).FoldTrees() {
		t.Error("FoldTrees() = true, want the rc-mapped false")
	}
}

// C22 supplement mode: both carriers set the knob with differing values →
// native wins, loud warning naming files, values, winner, with a fix.
func TestSupplementConflictNativeWins(t *testing.T) {
	globalDir, home := setupEnv(t)
	nativeDst, rcDst := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(globalDir, "config.toml"), "target = '"+nativeDst+"'\n")
	writeFile(t, filepath.Join(home, ".stowrc"), "--target='"+rcDst+"'\n")
	g, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	target, err := config.Effective{Global: g}.Target()
	if err != nil {
		t.Fatalf("Target(): %v", err)
	}
	if target != nativeDst {
		t.Errorf("Target() = %q, want the native value %q (native wins)", target, nativeDst)
	}
	w := findWarning(warns, "target", nativeDst, rcDst)
	if w == nil {
		t.Fatalf("no conflict warning naming knob and both values; warnings: %v", warns)
	}
	if w.Fix == "" || !strings.Contains(w.Fix, ".stowrc") {
		t.Errorf("conflict warning fix %q does not suggest removal from the rc", w.Fix)
	}
}

// C22: equal values are silent — a true duplicate is not a conflict.
func TestSupplementEqualValuesSilent(t *testing.T) {
	globalDir, home := setupEnv(t)
	dst := t.TempDir()
	writeFile(t, filepath.Join(globalDir, "config.toml"), "target = '"+dst+"'\n")
	writeFile(t, filepath.Join(home, ".stowrc"), "--target='"+dst+"'\n")
	_, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if w := findWarning(warns, "target"); w != nil {
		t.Errorf("equal values drew a conflict warning: %v", *w)
	}
}

// C19: --dir in ~/.stowrc is a session-repo contribution, announced, with
// fix: suggesting repo add.
func TestGlobalStowrcDirContributesSessionRepo(t *testing.T) {
	_, home := setupEnv(t)
	repoDir := t.TempDir()
	writeFile(t, filepath.Join(home, ".stowrc"), "--dir='"+repoDir+"'\n")
	g, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if got := g.SessionRepoDir(); got != repoDir {
		t.Errorf("SessionRepoDir() = %q, want %q", got, repoDir)
	}
	w := findWarning(warns, repoDir)
	if w == nil {
		t.Fatalf("the session-repo contribution was not announced; warnings: %v", warns)
	}
	if !strings.Contains(w.Fix, "repo add") {
		t.Errorf("announcement fix %q does not suggest repo add", w.Fix)
	}
}

// C19: unmappables warn-and-ignore per option, naming why + the native
// remedy. The file always runs; degradation is loud, never rejection.
func TestUnmappableOptionsWarnPerOption(t *testing.T) {
	_, home := setupEnv(t)
	writeFile(t, filepath.Join(home, ".stowrc"),
		"--adopt\n--simulate\n--verbose=3\n--override=man\n-D\nsomepkg\n")
	g, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: want loud degradation, not rejection; got %v", err)
	}
	if g == nil {
		t.Fatal("LoadGlobal returned no level")
	}
	for _, opt := range []string{"adopt", "simulate", "verbose", "override"} {
		if findWarning(warns, opt) == nil {
			t.Errorf("no warning for unmappable --%s; warnings: %v", opt, warns)
		}
	}
	// The -D verb and its package name are rc content stow itself discards.
	if findWarning(warns, "somepkg") == nil {
		t.Errorf("no warning for the rc verb/package request; warnings: %v", warns)
	}
}

// An unknown rc option is degradation, not rejection (C19).
func TestUnknownRcOptionWarns(t *testing.T) {
	_, home := setupEnv(t)
	writeFile(t, filepath.Join(home, ".stowrc"), "--bogus-flag\n")
	_, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: want warning, got error %v", err)
	}
	if findWarning(warns, "bogus") == nil {
		t.Errorf("no warning for the unknown rc option; warnings: %v", warns)
	}
}

// C21: a non-RE2 compat --ignore pattern refuses, scoped to the level the
// pattern governs, naming the file.
func TestNonRE2CompatIgnoreRefuses(t *testing.T) {
	_, home := setupEnv(t)
	writeFile(t, filepath.Join(home, ".stowrc"), "--ignore=(unclosed\n")
	_, _, err := config.LoadGlobal()
	if err == nil {
		t.Fatal("LoadGlobal with a non-RE2 --ignore: want scoped refusal, got nil")
	}
	if msg := err.Error(); !strings.Contains(msg, ".stowrc") {
		t.Errorf("refusal %q does not name the file", msg)
	}
}

// gostow's rc pipeline dies on an undefined variable in --target, as stow
// does; dstow surfaces that as a level error naming the file.
func TestRcUndefinedVariableIsAnError(t *testing.T) {
	_, home := setupEnv(t)
	writeFile(t, filepath.Join(home, ".stowrc"), "--target=$DSTOW_TEST_UNSET_VAR/x\n")
	_, _, err := config.LoadGlobal()
	if err == nil {
		t.Fatal("LoadGlobal with an undefined rc variable: want error, got nil")
	}
}

// §3.6 routing: a native config whose content is flag-lines routes to the
// compat parser with a loud announcement naming the native equivalent.
func TestRenamedRcSniffsToCompat(t *testing.T) {
	globalDir, _ := setupEnv(t)
	dst := t.TempDir()
	// Leading comment and blank lines are not significant tokens.
	writeFile(t, filepath.Join(globalDir, "config.toml"),
		"# migrated from stow\n\n--target='"+dst+"'\n--dotfiles\n")
	g, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if findWarning(warns, "config.toml") == nil {
		t.Errorf("the compat routing was not announced; warnings: %v", warns)
	}
	target, err := config.Effective{Global: g}.Target()
	if err != nil {
		t.Fatalf("Target(): %v", err)
	}
	if target != dst {
		t.Errorf("Target() = %q, want the routed rc value %q", target, dst)
	}
}

// A TOML file that merely opens with a comment is not rc-shaped.
func TestNativeTOMLWithLeadingCommentStaysNative(t *testing.T) {
	globalDir, _ := setupEnv(t)
	writeFile(t, filepath.Join(globalDir, "config.toml"),
		"# dstow global config\nfold_trees = true\n")
	g, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
	if !(config.Effective{Global: g}).FoldTrees() {
		t.Error("fold_trees from the natively-parsed file did not apply")
	}
}
