package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

// metadataDirName is the one spelling of the metadata directory (M1). It
// never appears outside this file: every consumer goes through the
// accessors, so observation and deployment can never disagree about where
// metadata lives.
const metadataDirName = ".dstow"

// configFileName is the one name at every level (C1): the level is
// determined by placement alone.
const configFileName = "config.toml"

// MetadataDir returns the metadata directory of a repo or package root —
// the same rule at both levels (M1). This is the one accessor hiding the
// metadata location (A8).
func MetadataDir(scopeRoot string) string {
	return filepath.Join(scopeRoot, metadataDirName)
}

// GlobalConfigDir returns the global level's metadata directory,
// $XDG_CONFIG_HOME/dstow (§3.1, XDG paths via adrg/xdg).
func GlobalConfigDir() string {
	return filepath.Join(xdg.ConfigHome, "dstow")
}

// GlobalConfigFile returns the global config file path (§3.1).
func GlobalConfigFile() string {
	return filepath.Join(GlobalConfigDir(), configFileName)
}

// RegistryFile returns the repo registry path (C3). The registry is config,
// not state — dstow-written, never declared in config.toml — and the repo
// package owns reading and writing it (A9); config only knows where it is.
func RegistryFile() string {
	return filepath.Join(GlobalConfigDir(), "repos.toml")
}

// UserThemesDir returns the user theme presets directory (§3.1); ui's theme
// loader consumes it (C4).
func UserThemesDir() string {
	return filepath.Join(GlobalConfigDir(), "themes")
}

// ParseDSTOWPath parses a DSTOW_PATH value (C23): separator-joined absolute
// local directory paths, PATH convention — the platform's list separator,
// colon on Unix. No qualified sources, no dstow-side expansion. Empty
// entries warn-and-skip; relative entries are refused loudly, every bad
// entry named with its remedy (§1.4). The caller reads the environment at
// point of use (A2) and hands the raw value in; existence of the
// directories is the repo set's concern, not the grammar's.
func ParseDSTOWPath(raw string) ([]string, []Warning, error) {
	if raw == "" {
		return nil, nil, nil
	}
	var (
		paths []string
		warns []Warning
		bad   []string
	)
	for _, entry := range strings.Split(raw, string(os.PathListSeparator)) {
		if entry == "" {
			warns = append(warns, Warning{
				Source: "DSTOW_PATH",
				Detail: "empty entry (skipped; the rest of DSTOW_PATH still applies)",
			})
			continue
		}
		if !filepath.IsAbs(entry) {
			bad = append(bad, entry)
			continue
		}
		paths = append(paths, entry)
	}
	if len(bad) > 0 {
		return nil, warns, fmt.Errorf(
			"DSTOW_PATH takes absolute local directory paths only (no expansion is applied); not absolute: %q. "+
				"Use an absolute path; for a remote or qualified source, clone it and register it with dstow repo add",
			bad)
	}
	return paths, warns, nil
}

// scanReserved applies the reserved-territory posture (M5) to one metadata
// directory: dstow claims the top level; unknown entries draw a C18-style
// warning, never a refusal. An absent or unreadable directory is silent —
// this is a courtesy scan, not a gate.
func scanReserved(dir string, claimed ...string) []Warning {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var warns []Warning
	for _, e := range entries {
		known := false
		for _, c := range claimed {
			if e.Name() == c {
				known = true
				break
			}
		}
		if known {
			continue
		}
		warns = append(warns, Warning{
			Source: dir,
			Detail: fmt.Sprintf("unexpected entry %q — this directory's top level is dstow's reserved territory (M5); the entry is ignored", e.Name()),
		})
	}
	return warns
}
