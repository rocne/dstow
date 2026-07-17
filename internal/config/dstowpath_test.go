package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/config"
)

// C23: DSTOW_PATH is colon-separated absolute local directory paths, PATH
// convention — no qualified sources, no dstow-side expansion; relative
// entries refused loudly; empty entries warn-and-skip. "PATH convention"
// binds the separator to the platform's (os.PathListSeparator): colon on
// Unix per C23's letter, semicolon on Windows where colon would collide
// with drive letters.
const pathListSep = os.PathListSeparator

func TestParseDSTOWPathUnset(t *testing.T) {
	paths, warns, err := config.ParseDSTOWPath("")
	if err != nil || len(paths) != 0 || len(warns) != 0 {
		t.Errorf("ParseDSTOWPath(\"\") = %v, %v, %v; want empty, no warnings, nil", paths, warns, err)
	}
}

func TestParseDSTOWPathAbsoluteEntries(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	paths, warns, err := config.ParseDSTOWPath(a + string(pathListSep) + b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
	if len(paths) != 2 || paths[0] != a || paths[1] != b {
		t.Errorf("paths = %v, want [%q %q]", paths, a, b)
	}
}

func TestParseDSTOWPathEmptyEntriesWarnAndSkip(t *testing.T) {
	a := t.TempDir()
	paths, warns, err := config.ParseDSTOWPath(string(pathListSep) + a + string(pathListSep))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 || paths[0] != a {
		t.Errorf("paths = %v, want [%q]", paths, a)
	}
	if len(warns) == 0 {
		t.Fatal("want a warning per empty entry, got none")
	}
	for _, w := range warns {
		if w.Source != "DSTOW_PATH" {
			t.Errorf("warning source = %q, want DSTOW_PATH", w.Source)
		}
	}
}

func TestParseDSTOWPathRelativeEntryRefusedLoudly(t *testing.T) {
	_, _, err := config.ParseDSTOWPath("relative/dir")
	if err == nil {
		t.Fatal("want refusal for a relative entry, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "relative/dir") {
		t.Errorf("refusal does not name the entry: %q", msg)
	}
	// Every refusal names its remedy (§1.4); remote sources need a clone.
	if !strings.Contains(msg, "repo add") {
		t.Errorf("refusal does not name the repo add remedy: %q", msg)
	}
}

func TestParseDSTOWPathNamesEveryBadEntry(t *testing.T) {
	_, _, err := config.ParseDSTOWPath("one" + string(pathListSep) + "two")
	if err == nil {
		t.Fatal("want refusal, got nil")
	}
	if msg := err.Error(); !strings.Contains(msg, "one") || !strings.Contains(msg, "two") {
		t.Errorf("refusal should name every bad entry: %q", msg)
	}
}
