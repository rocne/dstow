package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"

	"github.com/rocne/dstow/internal/config"
)

// setupEnv points HOME and XDG_CONFIG_HOME at fresh temp dirs so every test
// sees an isolated four-level chain. USERPROFILE rides along so Windows'
// os.UserHomeDir agrees with HOME. Returns the global config dir (which does
// not exist yet) and the home dir.
func setupEnv(t *testing.T) (globalDir, home string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	xdgRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgRoot)
	xdg.Reload()
	t.Cleanup(xdg.Reload) // recompute from the restored environment
	return filepath.Join(xdgRoot, "dstow"), home
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalConfigDirHonorsXDG(t *testing.T) {
	globalDir, _ := setupEnv(t)
	if got := config.GlobalConfigDir(); got != globalDir {
		t.Errorf("GlobalConfigDir() = %q, want %q", got, globalDir)
	}
}

func TestGlobalAccessorsComposeFromGlobalDir(t *testing.T) {
	globalDir, _ := setupEnv(t)
	if got, want := config.GlobalConfigFile(), filepath.Join(globalDir, "config.toml"); got != want {
		t.Errorf("GlobalConfigFile() = %q, want %q", got, want)
	}
	if got, want := config.RegistryFile(), filepath.Join(globalDir, "repos.toml"); got != want {
		t.Errorf("RegistryFile() = %q, want %q", got, want)
	}
	if got, want := config.UserThemesDir(), filepath.Join(globalDir, "themes"); got != want {
		t.Errorf("UserThemesDir() = %q, want %q", got, want)
	}
}

// MetadataDir is the one accessor hiding the metadata location (A8/M1): the
// same rule at repo and package roots.
func TestMetadataDir(t *testing.T) {
	if got, want := config.MetadataDir(filepath.Join("r", "pkg")), filepath.Join("r", "pkg", ".dstow"); got != want {
		t.Errorf("MetadataDir() = %q, want %q", got, want)
	}
}
