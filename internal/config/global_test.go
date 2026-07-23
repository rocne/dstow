package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/ledger"
)

// findWarning returns the first warning whose Detail contains every needle.
func findWarning(warns []config.Warning, needles ...string) *config.Warning {
	for i := range warns {
		ok := true
		for _, n := range needles {
			if !strings.Contains(warns[i].Detail, n) {
				ok = false
				break
			}
		}
		if ok {
			return &warns[i]
		}
	}
	return nil
}

// The built-in floor (REQUIREMENTS §4.4 / C7): target $HOME, dot-translation
// on, folding off, bulk exclusion off, no ignores from the chain.
func TestEffectiveBuiltinFloor(t *testing.T) {
	_, home := setupEnv(t)
	g, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal with no files: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
	eff := config.Effective{Global: g}
	target, err := eff.Target()
	if err != nil {
		t.Fatalf("Target(): %v", err)
	}
	if target != home {
		t.Errorf("Target() = %q, want the home dir %q", target, home)
	}
	if !eff.TranslateDotPrefixes() {
		t.Error("TranslateDotPrefixes() = false, want the built-in default true")
	}
	if eff.FoldTrees() {
		t.Error("FoldTrees() = true, want the built-in default false")
	}
	if eff.ExcludeFromBulk() {
		t.Error("ExcludeFromBulk() = true, want the built-in default false")
	}
	if got := eff.Ignores(); len(got) != 0 {
		t.Errorf("Ignores() = %v, want none", got)
	}
}

func TestGlobalNativeKnobs(t *testing.T) {
	globalDir, home := setupEnv(t)
	writeFile(t, filepath.Join(globalDir, "config.toml"), `
target = '~/deploy'
translate_dot_prefixes = false
fold_trees = true
ignore = ['*.log', 'scratch/']
theme = 'catppuccin-mocha'

[color]
stowed = 'green'
`)
	g, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
	eff := config.Effective{Global: g}

	target, err := eff.Target()
	if err != nil {
		t.Fatalf("Target(): %v", err)
	}
	if want := filepath.Join(home, "deploy"); target != want {
		t.Errorf("Target() = %q, want tilde-expanded %q", target, want)
	}
	if eff.TranslateDotPrefixes() {
		t.Error("TranslateDotPrefixes() = true, want the configured false")
	}
	if !eff.FoldTrees() {
		t.Error("FoldTrees() = false, want the configured true")
	}

	ignores := eff.Ignores()
	if len(ignores) != 2 {
		t.Fatalf("Ignores() = %v, want two entries", ignores)
	}
	for i, want := range []string{"*.log", "scratch/"} {
		if ignores[i].Pattern != want {
			t.Errorf("Ignores()[%d].Pattern = %q, want %q", i, ignores[i].Pattern, want)
		}
		if ignores[i].Language != config.LangGlob {
			t.Errorf("Ignores()[%d].Language = %v, want LangGlob (C17: native carriers speak glob)", i, ignores[i].Language)
		}
		if ignores[i].Level != config.LevelGlobal {
			t.Errorf("Ignores()[%d].Level = %v, want LevelGlobal", i, ignores[i].Level)
		}
	}

	theme, err := g.Theme()
	if err != nil {
		t.Fatalf("Theme(): %v", err)
	}
	if theme != "catppuccin-mocha" {
		t.Errorf("Theme() = %q, want the bare name verbatim", theme)
	}
	if got := g.ColorTable()["stowed"]; got != "green" {
		t.Errorf("ColorTable()[stowed] = %q, want green", got)
	}
}

// C8: $VAR and ${VAR} expand at use time; the result must be absolute.
func TestTargetEnvExpansion(t *testing.T) {
	globalDir, _ := setupEnv(t)
	base := t.TempDir()
	t.Setenv("DSTOW_TEST_BASE", base)
	writeFile(t, filepath.Join(globalDir, "config.toml"), `target = '${DSTOW_TEST_BASE}/dst'`)
	g, _, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	target, err := config.Effective{Global: g}.Target()
	if err != nil {
		t.Fatalf("Target(): %v", err)
	}
	if want := filepath.Join(base, "dst"); target != want {
		t.Errorf("Target() = %q, want %q", target, want)
	}
}

// C8: an unset variable is a loud error naming variable + file + key.
func TestTargetUnsetVariableError(t *testing.T) {
	globalDir, _ := setupEnv(t)
	writeFile(t, filepath.Join(globalDir, "config.toml"), `target = '$DSTOW_TEST_UNSET_VAR/x'`)
	g, _, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	_, err = config.Effective{Global: g}.Target()
	if err == nil {
		t.Fatal("Target() with an unset variable: want error, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"DSTOW_TEST_UNSET_VAR", "config.toml", "target"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not name %q (C8: variable + file + key)", msg, want)
		}
	}
}

// C8: the expanded result must be absolute.
func TestTargetMustBeAbsolute(t *testing.T) {
	globalDir, _ := setupEnv(t)
	writeFile(t, filepath.Join(globalDir, "config.toml"), `target = 'relative/dir'`)
	g, _, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if _, err := (config.Effective{Global: g}).Target(); err == nil {
		t.Fatal("Target() with a relative result: want error, got nil")
	}
}

// §3.5 / C18: unknown key → warning naming file + key, with did-you-mean;
// never a refusal.
func TestUnknownKeyWarnsWithDidYouMean(t *testing.T) {
	globalDir, _ := setupEnv(t)
	writeFile(t, filepath.Join(globalDir, "config.toml"), `tagret = '/x'`)
	_, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: want warning not refusal, got error %v", err)
	}
	w := findWarning(warns, "tagret", "target")
	if w == nil {
		t.Fatalf("no warning naming the unknown key and suggesting target; warnings: %v", warns)
	}
	if !strings.Contains(w.Source, "config.toml") {
		t.Errorf("warning source %q does not name the file", w.Source)
	}
}

// C3: config.toml never declares repos — the registry is dstow-written.
func TestReposKeyPointsAtRegistry(t *testing.T) {
	globalDir, _ := setupEnv(t)
	writeFile(t, filepath.Join(globalDir, "config.toml"), `repos = ['github:rocne/dotfiles']`)
	_, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if w := findWarning(warns, "repos"); w == nil || !strings.Contains(w.Detail+w.Fix, "repo add") {
		t.Errorf("want a warning pointing at dstow repo add for a repos key; warnings: %v", warns)
	}
}

// §3.5: a key legal elsewhere per the matrix warns naming the legal level
// and its file, and is ignored.
func TestMisplacedKeyAtGlobal(t *testing.T) {
	globalDir, _ := setupEnv(t)
	writeFile(t, filepath.Join(globalDir, "config.toml"), `exclude_from_bulk = true`)
	g, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	w := findWarning(warns, "exclude_from_bulk")
	if w == nil {
		t.Fatalf("no warning for the misplaced key; warnings: %v", warns)
	}
	if !strings.Contains(w.Detail, "repo") && !strings.Contains(w.Detail, "package") {
		t.Errorf("warning %q does not name the legal level", w.Detail)
	}
	// The key is ignored: bulk exclusion stays at its built-in false.
	if (config.Effective{Global: g}).ExcludeFromBulk() {
		t.Error("misplaced exclude_from_bulk took effect at the global level")
	}
}

// C16: leading '!' and leading '//' are refused-and-reserved forms.
func TestNativeIgnoreRefusedForms(t *testing.T) {
	for _, pattern := range []string{"!keep.txt", "//^regex$"} {
		t.Run(pattern, func(t *testing.T) {
			globalDir, _ := setupEnv(t)
			writeFile(t, filepath.Join(globalDir, "config.toml"), "ignore = ['"+pattern+"']\n")
			_, _, err := config.LoadGlobal()
			if err == nil {
				t.Fatalf("LoadGlobal with ignore pattern %q: want refusal, got nil", pattern)
			}
			if !strings.Contains(err.Error(), pattern) {
				t.Errorf("refusal %q does not name the pattern", err)
			}
		})
	}
}

// A TOML syntax error is a level error — there is no partial file to salvage.
func TestMalformedTOMLIsAnError(t *testing.T) {
	globalDir, _ := setupEnv(t)
	writeFile(t, filepath.Join(globalDir, "config.toml"), "target = [broken\n")
	if _, _, err := config.LoadGlobal(); err == nil {
		t.Fatal("LoadGlobal on malformed TOML: want error, got nil")
	}
}

// C18 posture for the [color] table: a wrongly-typed value warns and is
// skipped; the rest of the file still applies.
func TestColorTableBadValueWarnsAndSkips(t *testing.T) {
	globalDir, _ := setupEnv(t)
	writeFile(t, filepath.Join(globalDir, "config.toml"), `
[color]
stowed = 3
error = 'red'
`)
	g, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if findWarning(warns, "stowed") == nil {
		t.Errorf("no warning for the non-string color value; warnings: %v", warns)
	}
	table := g.ColorTable()
	if _, present := table["stowed"]; present {
		t.Error("non-string color value was not skipped")
	}
	if table["error"] != "red" {
		t.Error("the rest of the [color] table did not apply")
	}
}

// C13: the theme key is name-or-path per the operand rule; the path form
// follows C8 (expansion, absolute).
func TestThemePathForm(t *testing.T) {
	globalDir, home := setupEnv(t)
	writeFile(t, filepath.Join(globalDir, "config.toml"), `theme = '~/mytheme.toml'`)
	g, _, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	theme, err := g.Theme()
	if err != nil {
		t.Fatalf("Theme(): %v", err)
	}
	if want := filepath.Join(home, "mytheme.toml"); theme != want {
		t.Errorf("Theme() = %q, want expanded path %q", theme, want)
	}
}

// M5: the global config dir's top level is reserved territory — unknown
// entries draw a C18-style warning, never a refusal.
func TestGlobalDirUnknownEntryWarns(t *testing.T) {
	globalDir, _ := setupEnv(t)
	writeFile(t, filepath.Join(globalDir, "stray.txt"), "boo")
	if err := os.MkdirAll(filepath.Join(globalDir, "themes"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if findWarning(warns, "stray.txt") == nil {
		t.Errorf("no warning for the unknown reserved-territory entry; warnings: %v", warns)
	}
	if w := findWarning(warns, "themes"); w != nil {
		t.Errorf("claimed entry themes/ drew a warning: %v", *w)
	}
}

// M5 / #181: on the macOS default layout the config dir and the ledger's state
// dir are the same directory (adrg/xdg maps both $XDG_CONFIG_HOME and
// $XDG_STATE_HOME to ~/Library/Application Support). dstow's own ledger.json
// and ledger.lock then sit inside the config dir, and the scan must NOT flag
// state dstow wrote itself — while a genuinely stray entry still warns.
func TestGlobalDirColocatedLedgerFilesNotFlagged(t *testing.T) {
	globalDir, _ := setupEnv(t)
	// Point the state lane at the same base as the config lane, reproducing the
	// collision on any platform.
	t.Setenv("XDG_STATE_HOME", filepath.Dir(globalDir))
	xdg.Reload()
	if config.GlobalConfigDir() != ledger.Dir() {
		t.Fatalf("precondition: config dir %q and ledger dir %q should coincide",
			config.GlobalConfigDir(), ledger.Dir())
	}

	writeFile(t, filepath.Join(globalDir, "ledger.json"), "{}")
	writeFile(t, filepath.Join(globalDir, "ledger.lock"), "")
	writeFile(t, filepath.Join(globalDir, "stray.txt"), "boo")

	_, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if w := findWarning(warns, "ledger.json"); w != nil {
		t.Errorf("dstow's own ledger.json drew an M5 warning on the colocated layout: %v", *w)
	}
	if w := findWarning(warns, "ledger.lock"); w != nil {
		t.Errorf("dstow's own ledger.lock drew an M5 warning on the colocated layout: %v", *w)
	}
	if findWarning(warns, "stray.txt") == nil {
		t.Errorf("a genuinely stray entry no longer warns; warnings: %v", warns)
	}
}

// M5 / #181: the ledger allow-list is collision-conditional. Where the config
// and state lanes differ (stock Linux), a ledger.json dropped in the config dir
// is genuinely stray and MUST still warn — the scan does not go blind.
func TestGlobalDirStrayLedgerFileWarnsWhenLanesDiffer(t *testing.T) {
	globalDir, _ := setupEnv(t)
	// A distinct state lane, guaranteed different on every platform.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	xdg.Reload()
	if config.GlobalConfigDir() == ledger.Dir() {
		t.Fatalf("precondition: config dir and ledger dir must differ, both were %q",
			config.GlobalConfigDir())
	}

	writeFile(t, filepath.Join(globalDir, "ledger.json"), "{}")

	_, warns, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if findWarning(warns, "ledger.json") == nil {
		t.Errorf("a stray ledger.json in the config dir should still warn when the lanes differ; warnings: %v", warns)
	}
}
