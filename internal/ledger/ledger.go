// Package ledger is dstow's current-state index of the symlinks it believes
// exist (DESIGN.md §6 + A10; ADR 0001). It is one JSON document per machine
// at $XDG_STATE_HOME/dstow/ledger.json — never a journal, never a history:
// entries are the links dstow currently believes exist, pruned wherever disk
// contradicts them (disk is always the truth).
//
// The package splits cleanly along §6.3's read/write line. Load (§6.2/§6.3)
// is a lock-free snapshot that never creates or writes a file. Update
// (§6.2/§6.3, A10) holds an exclusive flock for the whole operation, prunes
// contradicted entries in scope, runs the caller's mutation, and commits with
// one atomic temp-file+fsync+rename. Contradicted is the single owner of the
// disk-disagrees test so report (reads) and prune (writes) can never disagree.
//
// The package returns data and typed errors only (A4): it never writes to
// stdout/stderr and renders no prose for humans beyond its error strings,
// which carry the §6.5 remedies. RecordedAt is the caller's to stamp — entry
// creation belongs to the deploy verbs — so the package itself stamps nothing.
package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
)

// Version is the schema version this dstow writes and understands (§6.5). It
// is bumped only on an incompatible change; v1 is the first schema.
const Version = 1

// Entry is one ledgered link (§6.1). link is target-relative and source
// package-relative — exactly how stow.Expected keys and values its results —
// so ledger, Expected, and Owner compose directly.
type Entry struct {
	Link        string    `json:"link"`        // target-relative, exactly how stow.Expected keys results
	Package     string    `json:"package"`     // canonical percent-encoded FQN
	Source      string    `json:"source"`      // package-relative
	Destination string    `json:"destination"` // literal symlink text as written — the damaged evidence
	RecordedAt  time.Time `json:"recorded_at"` // RFC 3339 UTC: when the entry entered the ledger
}

// Ledger is the whole document (§6.1). Targets maps an absolute target root to
// its entries; the grouping is load-bearing — it is rebuild's replace boundary.
type Ledger struct {
	Version int                `json:"version"`
	Targets map[string][]Entry `json:"targets"` // absolute target root → entries; the grouping is rebuild's replace boundary
}

// Path is the one ledger document per machine: $XDG_STATE_HOME/dstow/ledger.json
// (§6.1). Machine state lives in JSON in its XDG state lane, via adrg/xdg.
func Path() string {
	return filepath.Join(xdg.StateHome, "dstow", "ledger.json")
}

// LockPath is the sibling ledger.lock of the given ledger path (§6.2): writers
// take an exclusive advisory flock on it; the ledger document itself is never
// locked.
func LockPath(path string) string {
	return filepath.Join(filepath.Dir(path), "ledger.lock")
}

// CorruptError refuses an unparseable or invalid-version ledger (§6.5):
// corruption must never degrade into amnesia, so Load names the path and
// points at the remedy rather than starting empty.
type CorruptError struct {
	Path string
	Err  error
}

func (e *CorruptError) Error() string {
	return fmt.Sprintf("dstow ledger at %s is corrupt: %v; run dstow rebuild to reconstruct it from disk, or restore the file from a backup", e.Path, e.Err)
}

func (e *CorruptError) Unwrap() error { return e.Err }

// NewerVersionError refuses a ledger written by a newer dstow (§6.5): never
// guess, never rewrite the file down — the remedy is to upgrade dstow.
type NewerVersionError struct {
	Path        string
	FileVersion int
}

func (e *NewerVersionError) Error() string {
	return fmt.Sprintf("dstow ledger at %s was written by a newer dstow (schema version %d; this dstow understands version %d); upgrade dstow — it will not guess or rewrite the file down", e.Path, e.FileVersion, Version)
}

// Load reads a lock-free snapshot of the ledger (§6.2/§6.3): it never creates
// or writes a file. A missing file is an empty ledger, never an error (§6.5).
// Unparseable JSON or a version below 1 is a *CorruptError; a version above
// this dstow's is a *NewerVersionError.
func Load(path string) (Ledger, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		// Missing file = empty ledger; reads never write, so nothing is
		// created here (§6.5).
		return Ledger{Version: Version, Targets: map[string][]Entry{}}, nil
	}
	if err != nil {
		return Ledger{}, err
	}

	var l Ledger
	if err := json.Unmarshal(data, &l); err != nil {
		return Ledger{}, &CorruptError{Path: path, Err: err}
	}
	switch {
	case l.Version < 1:
		return Ledger{}, &CorruptError{Path: path, Err: fmt.Errorf("schema version %d is not valid (the first schema is version %d)", l.Version, Version)}
	case l.Version > Version:
		return Ledger{}, &NewerVersionError{Path: path, FileVersion: l.Version}
	}

	// Forward-migration seam (§6.5): older-but-valid schemas migrate forward
	// in memory here. Today only v1 exists, so this passes it through — no
	// migrations are invented ahead of a real schema change.
	switch l.Version {
	case Version:
		// current schema; nothing to migrate.
	}

	if l.Targets == nil {
		l.Targets = map[string][]Entry{}
	}
	return l, nil
}

// Contradicted reports whether disk disagrees with the entry under the given
// target root (CONTEXT.md "Contradicted entry"): the recorded path is gone,
// holds a non-link, or holds a different link text than recorded. Evidence is
// complete prose when contradicted, naming what disk shows against what the
// entry records. This is the one owner of the test — check (ops) consults it
// too, so report and prune can never disagree.
func (e Entry) Contradicted(targetRoot string) (bool, string) {
	linkPath := filepath.Join(targetRoot, e.Link)

	info, err := os.Lstat(linkPath)
	if errors.Is(err, fs.ErrNotExist) {
		return true, fmt.Sprintf("the ledger records a link at %s pointing to %q, but nothing exists at that path", linkPath, e.Destination)
	}
	if err != nil {
		// Disk cannot be observed here (e.g. a permission error on the path).
		// Disk-is-truth means a contradiction is only ever claimed against an
		// actual sighting, so an unobservable path is not reported as
		// contradicted. (DESIGN.md enumerates only gone / non-link /
		// different-text as contradictions.)
		return false, ""
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		return true, fmt.Sprintf("the ledger records a link at %s pointing to %q, but that path holds a %s, not a symlink", linkPath, e.Destination, kindOf(info))
	}

	actual, err := os.Readlink(linkPath)
	if err != nil {
		return false, ""
	}
	if actual != e.Destination {
		return true, fmt.Sprintf("the ledger records a link at %s pointing to %q, but that path is a symlink to %q", linkPath, e.Destination, actual)
	}
	return false, ""
}

// kindOf names what disk holds at a non-symlink path, for Contradicted's
// evidence prose.
func kindOf(info fs.FileInfo) string {
	switch {
	case info.IsDir():
		return "directory"
	case info.Mode().IsRegular():
		return "regular file"
	default:
		return "non-symlink file"
	}
}
