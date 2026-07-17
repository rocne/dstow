package repo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/repo"
)

// Save→Load round-trips the canonical form.
func TestRegistryRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos.toml")
	reg := repo.Registry{Sources: []repo.Source{
		mustSource(t, "github:rocne/dotfiles"),
		mustSource(t, "local:/srv/dots"),
	}}
	if err := reg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, warns, err := repo.LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %+v", warns)
	}
	var strs []string
	for _, s := range got.Sources {
		strs = append(strs, s.String())
	}
	// Sorted canonical form.
	want := []string{"github:rocne/dotfiles", "local:/srv/dots"}
	if strings.Join(strs, ",") != strings.Join(want, ",") {
		t.Errorf("round-trip sources = %v, want %v", strs, want)
	}
}

// Save emits the sorted shorthand string-array form (assert raw TOML).
func TestRegistrySaveSortedShorthand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos.toml")
	reg := repo.Registry{Sources: []repo.Source{
		mustSource(t, "local:/srv/dots"),
		mustSource(t, "github:rocne/dotfiles"),
	}}
	if err := reg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `repos = ["github:rocne/dotfiles", "local:/srv/dots"]` + "\n"
	if string(raw) != want {
		t.Errorf("raw registry = %q, want %q", raw, want)
	}
}

// The documented growth form (tables with a source key) parses.
func TestLoadRegistryGrowthForm(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos.toml")
	content := "" +
		"[[repos]]\n" +
		"source = \"github:rocne/dotfiles\"\n\n" +
		"[[repos]]\n" +
		"source = \"local:/srv/dots\"\n"
	writeRegistry(t, path, content)

	reg, warns, err := repo.LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %+v", warns)
	}
	if len(reg.Sources) != 2 {
		t.Fatalf("growth-form sources = %d, want 2", len(reg.Sources))
	}
	if reg.Sources[0].String() != "github:rocne/dotfiles" || reg.Sources[1].String() != "local:/srv/dots" {
		t.Errorf("growth-form sources = %v", reg.Sources)
	}
}

// A bad entry warn-and-skips while the rest of the file loads (C18 posture).
func TestLoadRegistryBadEntryWarnAndSkips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos.toml")
	content := `repos = ["github:rocne/dotfiles", "gitlab:o/r", "github:only-one-seg"]` + "\n"
	writeRegistry(t, path, content)

	reg, warns, err := repo.LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(reg.Sources) != 1 || reg.Sources[0].String() != "github:rocne/dotfiles" {
		t.Errorf("loaded sources = %v; want only the valid one", reg.Sources)
	}
	if len(warns) != 2 {
		t.Errorf("want 2 warn-and-skip warnings, got %d: %+v", len(warns), warns)
	}
}

// An unknown top-level key warns (C18).
func TestLoadRegistryUnknownKeyWarns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos.toml")
	content := `repos = ["github:rocne/dotfiles"]` + "\n" + `nonsense = 42` + "\n"
	writeRegistry(t, path, content)

	reg, warns, err := repo.LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(reg.Sources) != 1 {
		t.Errorf("sources = %v, want the one valid source", reg.Sources)
	}
	if len(warns) != 1 || !strings.Contains(warns[0].Detail, "nonsense") {
		t.Errorf("want one warning naming the unknown key; got %+v", warns)
	}
}

// A missing file is an empty registry, never an error.
func TestLoadRegistryMissingFileEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.toml")
	reg, warns, err := repo.LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry on missing file = %v, want nil", err)
	}
	if len(reg.Sources) != 0 || len(warns) != 0 {
		t.Errorf("missing file gave sources=%v warns=%v; want empty", reg.Sources, warns)
	}
}

// Malformed TOML is an error (no partial file to salvage).
func TestLoadRegistryMalformedTOMLErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos.toml")
	writeRegistry(t, path, "repos = [unterminated\n")
	if _, _, err := repo.LoadRegistry(path); err == nil {
		t.Fatal("LoadRegistry on malformed TOML returned nil error")
	}
}

// The atomic write leaves no temp files behind.
func TestRegistrySaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.toml")
	reg := repo.Registry{Sources: []repo.Source{mustSource(t, "github:rocne/dotfiles")}}
	if err := reg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

// Save creates the parent directory when it does not exist (MkdirAll).
func TestRegistrySaveCreatesParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "repos.toml")
	reg := repo.Registry{Sources: []repo.Source{mustSource(t, "github:rocne/dotfiles")}}
	if err := reg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("registry not written under created parent: %v", err)
	}
}

func writeRegistry(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
