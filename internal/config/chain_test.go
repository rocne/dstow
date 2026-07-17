package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/config"
)

// loadChain loads all three levels of a repo/package fixture, failing the
// test on any level error.
func loadChain(t *testing.T, repoRoot, pkgRoot string) (config.Effective, []config.Warning) {
	t.Helper()
	var all []config.Warning
	g, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	all = append(all, warns...)
	r, warns, err := config.LoadRepoLevel(repoRoot)
	if err != nil {
		t.Fatalf("LoadRepoLevel: %v", err)
	}
	all = append(all, warns...)
	p, warns, err := config.LoadPackageLevel(pkgRoot)
	if err != nil {
		t.Fatalf("LoadPackageLevel: %v", err)
	}
	all = append(all, warns...)
	return config.Effective{Global: g, Repo: r, Package: p}, all
}

// REQUIREMENTS §4.1: nearer level wins per knob — package over repo over
// global — except ignores, which compose additively across all levels.
func TestChainNearestWins(t *testing.T) {
	setupEnv(t)
	repoRoot := t.TempDir()
	pkgRoot := filepath.Join(repoRoot, "zsh")
	gDst, rDst, pDst := t.TempDir(), t.TempDir(), t.TempDir()

	globalDir := config.GlobalConfigDir()
	writeFile(t, filepath.Join(globalDir, "config.toml"),
		"target = '"+gDst+"'\ntranslate_dot_prefixes = false\nignore = ['g.log']\n")
	writeFile(t, filepath.Join(repoRoot, ".dstow", "config.toml"),
		"target = '"+rDst+"'\nexclude_from_bulk = true\nignore = ['r.log']\n")
	writeFile(t, filepath.Join(pkgRoot, ".dstow", "config.toml"),
		"target = '"+pDst+"'\ntranslate_dot_prefixes = true\nexclude_from_bulk = false\nignore = ['p.log']\n")

	eff, warns := loadChain(t, repoRoot, pkgRoot)
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}

	target, err := eff.Target()
	if err != nil {
		t.Fatalf("Target(): %v", err)
	}
	if target != pDst {
		t.Errorf("Target() = %q, want the package level's %q", target, pDst)
	}
	if !eff.TranslateDotPrefixes() {
		t.Error("package-level translate_dot_prefixes=true did not win over global false")
	}
	// An explicit package-level false overrides the repo's true — nearer
	// wins even when nearer is the default value.
	if eff.ExcludeFromBulk() {
		t.Error("package-level exclude_from_bulk=false did not win over repo true")
	}

	ignores := eff.Ignores()
	if len(ignores) != 3 {
		t.Fatalf("Ignores() = %v, want the additive three", ignores)
	}
	wantLevels := []config.Level{config.LevelGlobal, config.LevelRepo, config.LevelPackage}
	wantPatterns := []string{"g.log", "r.log", "p.log"}
	for i := range ignores {
		if ignores[i].Level != wantLevels[i] || ignores[i].Pattern != wantPatterns[i] {
			t.Errorf("Ignores()[%d] = %+v, want %q at %v", i, ignores[i], wantPatterns[i], wantLevels[i])
		}
	}
}

// Repo level falls through when the package level is silent (§4.1: the repo
// level acts as defaults for all packages in that repo).
func TestChainRepoDefaultsApply(t *testing.T) {
	setupEnv(t)
	repoRoot := t.TempDir()
	pkgRoot := filepath.Join(repoRoot, "zsh")
	rDst := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, ".dstow", "config.toml"),
		"target = '"+rDst+"'\nexclude_from_bulk = true\n")
	writeFile(t, filepath.Join(pkgRoot, "dot-zshrc"), "")

	eff, _ := loadChain(t, repoRoot, pkgRoot)
	target, err := eff.Target()
	if err != nil {
		t.Fatalf("Target(): %v", err)
	}
	if target != rDst {
		t.Errorf("Target() = %q, want the repo level's %q", target, rDst)
	}
	if !eff.ExcludeFromBulk() {
		t.Error("repo-level exclude_from_bulk=true did not apply to the package")
	}
}

// M3: packages_dir is repo level only, a repo-root-relative path.
func TestPackagesDir(t *testing.T) {
	setupEnv(t)
	repoRoot := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, ".dstow", "config.toml"), "packages_dir = 'packages'\n")
	r, warns, err := config.LoadRepoLevel(repoRoot)
	if err != nil {
		t.Fatalf("LoadRepoLevel: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
	if got := r.PackagesDir(); got != "packages" {
		t.Errorf("PackagesDir() = %q, want the raw repo-root-relative value", got)
	}
}

// The legality matrix at the repo level: fold_trees and theme are
// global-only; packages_dir at the package level is repo-only.
func TestMisplacedKeysAcrossLevels(t *testing.T) {
	setupEnv(t)
	repoRoot := t.TempDir()
	pkgRoot := filepath.Join(repoRoot, "zsh")
	writeFile(t, filepath.Join(repoRoot, ".dstow", "config.toml"),
		"fold_trees = true\ntheme = 'nord'\n")
	writeFile(t, filepath.Join(pkgRoot, ".dstow", "config.toml"),
		"packages_dir = 'nested'\n")

	eff, warns := loadChain(t, repoRoot, pkgRoot)
	for _, key := range []string{"fold_trees", "theme", "packages_dir"} {
		if findWarning(warns, key) == nil {
			t.Errorf("no misplaced-key warning for %q; warnings: %v", key, warns)
		}
	}
	// The misplaced fold_trees is ignored: folding stays at its built-in off.
	if eff.FoldTrees() {
		t.Error("misplaced repo-level fold_trees took effect")
	}
}

// REQUIREMENTS §3.3: fold flags in a migrated repo-level .stowrc ARE honored
// (the compat exception to fold-is-global-only); contradiction handling
// across repos composes in ops.
func TestRepoStowrcNoFoldingHonored(t *testing.T) {
	globalDir, _ := setupEnv(t)
	writeFile(t, filepath.Join(globalDir, "config.toml"), "fold_trees = true\n")
	repoRoot := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, ".stowrc"), "--no-folding\n")

	eff, _ := loadChain(t, repoRoot, filepath.Join(repoRoot, "zsh"))
	if eff.FoldTrees() {
		t.Error("FoldTrees() = true, want the repo rc's no-folding to win for this repo")
	}
	value, file, set := eff.Repo.CompatFoldTrees()
	if !set || value {
		t.Errorf("CompatFoldTrees() = %v, %q, %v; want false, rc path, true", value, file, set)
	}
	if !strings.Contains(file, ".stowrc") {
		t.Errorf("CompatFoldTrees() file = %q, want the rc path", file)
	}
}

// C19: --dir at the repo level is warn-and-ignore.
func TestRepoStowrcDirWarnsAndIgnores(t *testing.T) {
	setupEnv(t)
	repoRoot := t.TempDir()
	other := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, ".stowrc"), "--dir='"+other+"'\n")
	_, warns, err := config.LoadRepoLevel(repoRoot)
	if err != nil {
		t.Fatalf("LoadRepoLevel: %v", err)
	}
	if findWarning(warns, "--dir") == nil {
		t.Errorf("no warning for repo-level --dir; warnings: %v", warns)
	}
}

// C20: a repo's .stowrc slots to the repo level and supplements the native
// repo config per C22 — native wins on conflict.
func TestRepoSupplementConflict(t *testing.T) {
	setupEnv(t)
	repoRoot := t.TempDir()
	nativeDst, rcDst := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(repoRoot, ".dstow", "config.toml"), "target = '"+nativeDst+"'\n")
	writeFile(t, filepath.Join(repoRoot, ".stowrc"), "--target='"+rcDst+"'\n")

	eff, warns := loadChain(t, repoRoot, filepath.Join(repoRoot, "zsh"))
	target, err := eff.Target()
	if err != nil {
		t.Fatalf("Target(): %v", err)
	}
	if target != nativeDst {
		t.Errorf("Target() = %q, want the native %q (native wins)", target, nativeDst)
	}
	if findWarning(warns, "target") == nil {
		t.Errorf("no conflict warning; warnings: %v", warns)
	}
}

// C8: expansion failures scope per-package — a bad package-level target
// surfaces at use time, naming variable + file + key, and only this
// package's effective view errors.
func TestPackageLevelExpansionFailureScopes(t *testing.T) {
	setupEnv(t)
	repoRoot := t.TempDir()
	pkgRoot := filepath.Join(repoRoot, "zsh")
	writeFile(t, filepath.Join(pkgRoot, ".dstow", "config.toml"),
		"target = '$DSTOW_TEST_UNSET_VAR/x'\n")

	eff, _ := loadChain(t, repoRoot, pkgRoot)
	_, err := eff.Target()
	if err == nil {
		t.Fatal("Target(): want the use-time expansion error, got nil")
	}
	for _, want := range []string{"DSTOW_TEST_UNSET_VAR", "config.toml", "target"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}

	// A sibling view without the package level stays healthy: the failure
	// scoped to the package that declared the bad value.
	if _, err := (config.Effective{Global: eff.Global, Repo: eff.Repo}).Target(); err != nil {
		t.Errorf("sibling effective view errored too: %v", err)
	}
}

// M5: a repo or package .dstow top level is reserved territory; unknown
// entries warn, claimed entries (config.toml, hooks/) do not.
func TestMetadataDirUnknownEntryWarns(t *testing.T) {
	setupEnv(t)
	repoRoot := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, ".dstow", "config.toml"), "")
	writeFile(t, filepath.Join(repoRoot, ".dstow", "notes.md"), "hi")
	writeFile(t, filepath.Join(repoRoot, ".dstow", "hooks", "pre-stow"), "#!/bin/sh\n")

	_, warns, err := config.LoadRepoLevel(repoRoot)
	if err != nil {
		t.Fatalf("LoadRepoLevel: %v", err)
	}
	if findWarning(warns, "notes.md") == nil {
		t.Errorf("no warning for the unknown .dstow entry; warnings: %v", warns)
	}
	if w := findWarning(warns, "hooks"); w != nil {
		t.Errorf("claimed entry hooks/ drew a warning: %v", *w)
	}
}

// Levels absent on disk are simply empty: a bare repo and package still
// yield the full built-in floor.
func TestBareTreeLoadsEmpty(t *testing.T) {
	setupEnv(t)
	repoRoot := t.TempDir()
	eff, warns := loadChain(t, repoRoot, filepath.Join(repoRoot, "zsh"))
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
	if eff.Repo == nil || eff.Package == nil {
		t.Fatal("absent files should still yield level values")
	}
	if !eff.TranslateDotPrefixes() || eff.FoldTrees() || eff.ExcludeFromBulk() {
		t.Error("built-in floor did not survive empty levels")
	}
}
