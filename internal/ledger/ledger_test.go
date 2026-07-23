package ledger_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"golang.org/x/sys/unix"

	"github.com/rocne/dstow/internal/ledger"
)

// recordedAt is a fixed RFC 3339 UTC instant used across the schema tests so
// the JSON rendering is deterministic.
func recordedAt(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, "2026-07-15T09:12:03Z")
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// sampleLedger mirrors DESIGN.md §6.1's example document.
func sampleLedger(t *testing.T) ledger.Ledger {
	t.Helper()
	return ledger.Ledger{
		Version: ledger.Version,
		Targets: map[string][]ledger.Entry{
			"/home/rocne": {{
				Link:        ".config/nvim",
				Package:     "github:rocne/dotfiles::nvim",
				Source:      "dot-config/nvim",
				Destination: "../repos/github/rocne/dotfiles/nvim/dot-config/nvim",
				RecordedAt:  recordedAt(t),
			}},
		},
	}
}

func ledgerPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "state", "dstow", "ledger.json")
}

// C6.1: the on-disk schema uses exactly the pinned field names and renders
// recorded_at as RFC 3339 UTC; a write→read round-trip is lossless.
func TestSchemaFieldNamesAndRoundTrip(t *testing.T) {
	l := sampleLedger(t)
	raw, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(raw)
	for _, field := range []string{
		`"version"`, `"targets"`, `"link"`, `"package"`,
		`"source"`, `"destination"`, `"recorded_at"`,
	} {
		if !strings.Contains(got, field) {
			t.Errorf("marshalled ledger missing field %s: %s", field, got)
		}
	}
	if !strings.Contains(got, `"2026-07-15T09:12:03Z"`) {
		t.Errorf("recorded_at not rendered as RFC 3339 UTC: %s", got)
	}

	// Round-trip through a real file via Update (write) then Load (read).
	path := ledgerPath(t)
	if _, err := ledger.Update(path, ledger.Scope{}, func(doc *ledger.Ledger) error {
		*doc = l
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	back, err := ledger.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if back.Version != l.Version {
		t.Errorf("round-trip version = %d, want %d", back.Version, l.Version)
	}
	want := l.Targets["/home/rocne"][0]
	got0 := back.Targets["/home/rocne"][0]
	if !got0.RecordedAt.Equal(want.RecordedAt) {
		t.Errorf("round-trip recorded_at = %v, want %v", got0.RecordedAt, want.RecordedAt)
	}
	got0.RecordedAt, want.RecordedAt = time.Time{}, time.Time{}
	if got0 != want {
		t.Errorf("round-trip entry = %+v, want %+v", got0, want)
	}
}

// §6.5: a missing file is an empty ledger, no error — and, because reads never
// write (§6.3), nothing is created on disk.
func TestLoadMissingFileIsEmptyAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "state", "dstow")
	path := filepath.Join(stateDir, "ledger.json")

	l, err := ledger.Load(path)
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if l.Version != ledger.Version {
		t.Errorf("version = %d, want %d", l.Version, ledger.Version)
	}
	if l.Targets == nil {
		t.Error("Targets is nil, want non-nil empty map")
	}
	if len(l.Targets) != 0 {
		t.Errorf("Targets = %v, want empty", l.Targets)
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Errorf("Load created %s (reads must never write): stat err = %v", stateDir, err)
	}
}

// §6.5: unparseable JSON refuses loudly, names the path, and points at the
// rebuild remedy — corruption must never degrade into amnesia.
func TestLoadGarbageIsCorruptError(t *testing.T) {
	path := ledgerPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("this is not json {{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ledger.Load(path)
	var ce *ledger.CorruptError
	if !errors.As(err, &ce) {
		t.Fatalf("Load garbage: got %T (%v), want *CorruptError", err, err)
	}
	if ce.Path != path {
		t.Errorf("CorruptError.Path = %q, want %q", ce.Path, path)
	}
	if !strings.Contains(err.Error(), "rebuild") {
		t.Errorf("corrupt error missing rebuild remedy: %q", err.Error())
	}
}

// §6.5: a newer schema version refuses loudly — never guess, never rewrite
// down; the remedy is to upgrade dstow.
func TestLoadNewerVersionRefuses(t *testing.T) {
	path := ledgerPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version": 99, "targets": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ledger.Load(path)
	var nve *ledger.NewerVersionError
	if !errors.As(err, &nve) {
		t.Fatalf("Load newer: got %T (%v), want *NewerVersionError", err, err)
	}
	if nve.FileVersion != 99 {
		t.Errorf("FileVersion = %d, want 99", nve.FileVersion)
	}
	msg := err.Error()
	if !strings.Contains(msg, "newer") || !strings.Contains(msg, "upgrade") {
		t.Errorf("newer-version error missing 'newer'/'upgrade': %q", msg)
	}
}

// §6.5: version 0 (or missing) is not a valid schema — refuse as corrupt
// rather than silently treating it as current.
func TestLoadVersionZeroIsCorrupt(t *testing.T) {
	for _, body := range []string{`{"version": 0, "targets": {}}`, `{"targets": {}}`} {
		path := ledgerPath(t)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := ledger.Load(path)
		var ce *ledger.CorruptError
		if !errors.As(err, &ce) {
			t.Fatalf("Load %q: got %T (%v), want *CorruptError", body, err, err)
		}
	}
}

// §6.1/§6.2: Update commits an atomic, pretty-printed write, creating the
// parent directory, and leaves no temp file behind.
func TestUpdateHappyPathWritesPrettyAndCreatesDir(t *testing.T) {
	path := ledgerPath(t)
	entry := sampleLedger(t).Targets["/home/rocne"][0]

	if _, err := ledger.Update(path, ledger.Scope{}, func(doc *ledger.Ledger) error {
		doc.Targets["/home/rocne"] = append(doc.Targets["/home/rocne"], entry)
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after Update: %v", err)
	}
	if !strings.Contains(string(raw), "\n  ") {
		t.Errorf("ledger is not pretty-printed (no indented newline): %s", raw)
	}
	var l ledger.Ledger
	if err := json.Unmarshal(raw, &l); err != nil {
		t.Fatalf("written ledger does not parse: %v", err)
	}
	if len(l.Targets["/home/rocne"]) != 1 {
		t.Errorf("entry not persisted: %+v", l.Targets)
	}

	// No leftover temp files in the directory.
	names, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if strings.HasSuffix(n.Name(), ".tmp") {
			t.Errorf("leftover temp file: %s", n.Name())
		}
	}
}

// §6.2: contention fails fast with *LockedError, and the ledger is left
// unchanged.
func TestUpdateLockContentionFailsFast(t *testing.T) {
	path := ledgerPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	// Hold the flock ourselves via the same syscall the writer uses.
	held, err := os.OpenFile(ledger.LockPath(path), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Close() }()
	if err := unix.Flock(int(held.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatalf("test could not take the lock: %v", err)
	}
	defer func() { _ = unix.Flock(int(held.Fd()), unix.LOCK_UN) }()

	_, err = ledger.Update(path, ledger.Scope{}, func(doc *ledger.Ledger) error {
		doc.Targets["/x"] = []ledger.Entry{{Link: "a"}}
		return nil
	})
	var le *ledger.LockedError
	if !errors.As(err, &le) {
		t.Fatalf("Update under contention: got %T (%v), want *LockedError", err, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("ledger file was written despite contention: %v", err)
	}
}

// §6.2: fn returning an error aborts with no write, and the lock is released so
// a subsequent Update succeeds.
func TestUpdateFnErrorAbortsAndReleasesLock(t *testing.T) {
	path := ledgerPath(t)
	// Seed a known-good document.
	if _, err := ledger.Update(path, ledger.Scope{}, func(doc *ledger.Ledger) error {
		doc.Targets["/home/rocne"] = []ledger.Entry{{Link: "seed", Package: "p"}}
		return nil
	}); err != nil {
		t.Fatalf("seed Update: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	sentinel := errors.New("boom")
	if _, err := ledger.Update(path, ledger.Scope{}, func(doc *ledger.Ledger) error {
		doc.Targets["/home/rocne"] = append(doc.Targets["/home/rocne"], ledger.Entry{Link: "extra"})
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("Update fn error: got %v, want sentinel", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("ledger changed despite fn error:\nbefore %s\nafter  %s", before, after)
	}

	// Lock was released: a second Update must succeed.
	if _, err := ledger.Update(path, ledger.Scope{}, func(doc *ledger.Ledger) error {
		return nil
	}); err != nil {
		t.Fatalf("second Update after fn error (lock leak?): %v", err)
	}
}

// TestPathHonorsXDGStateHome exercises Path() against an explicit
// XDG_STATE_HOME, the paths_test.go pattern (t.Setenv + xdg.Reload); no other
// test touches the real state home.
func TestPathHonorsXDGStateHome(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	xdg.Reload()
	t.Cleanup(xdg.Reload)

	want := filepath.Join(state, "dstow", "ledger.json")
	if got := ledger.Path(); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
	if got, want := ledger.LockPath(ledger.Path()), filepath.Join(state, "dstow", "ledger.lock"); got != want {
		t.Errorf("LockPath() = %q, want %q", got, want)
	}
}

// TestDirIsPathParent pins Dir() as the directory holding the ledger document
// and its lock — the value config's M5 scan compares against its config dir.
func TestDirIsPathParent(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	xdg.Reload()
	t.Cleanup(xdg.Reload)

	if got, want := ledger.Dir(), filepath.Join(state, "dstow"); got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
	if got := filepath.Dir(ledger.Path()); got != ledger.Dir() {
		t.Errorf("filepath.Dir(Path()) = %q, want Dir() = %q", got, ledger.Dir())
	}
}

// TestReservedNamesAreTheLedgerBasenames pins the names config borrows for its
// M5 scan to the actual basenames Path()/LockPath() write, so the two owners
// can never drift.
func TestReservedNamesAreTheLedgerBasenames(t *testing.T) {
	names := ledger.ReservedNames()
	want := map[string]bool{
		filepath.Base(ledger.Path()):                  false,
		filepath.Base(ledger.LockPath(ledger.Path())): false,
	}
	if len(names) != len(want) {
		t.Fatalf("ReservedNames() = %v, want the %d ledger basenames", names, len(want))
	}
	for _, n := range names {
		if _, ok := want[n]; !ok {
			t.Errorf("ReservedNames() contains %q, not a ledger basename", n)
		}
		want[n] = true
	}
	for n, seen := range want {
		if !seen {
			t.Errorf("ReservedNames() omits the ledger basename %q", n)
		}
	}
}
