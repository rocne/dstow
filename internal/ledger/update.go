package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Scope is a writer's pruning scope (§6.3): an entry is covered when All is
// set (the ledger-wide broom, clean §6.3), when its package is named, or when
// its absolute link path is named. Contradicted entries in scope are pruned;
// unrelated damage evidence survives.
type Scope struct {
	Packages []string // canonical FQNs: prune entries of these packages
	Paths    []string // absolute link paths (target root joined with entry link): prune entries at these paths
	All      bool     // the ledger-wide broom (clean §6.3)
}

// covers reports whether the scope covers entry e under target root r (§6.3):
// e is in scope iff All, or e.Package is named, or filepath.Join(r, e.Link) is
// a named path.
func (s Scope) covers(root string, e Entry) bool {
	if s.All {
		return true
	}
	for _, p := range s.Packages {
		if e.Package == p {
			return true
		}
	}
	if len(s.Paths) > 0 {
		linkPath := filepath.Join(root, e.Link)
		for _, p := range s.Paths {
			if linkPath == p {
				return true
			}
		}
	}
	return false
}

// Pruned records one entry removed by scoped pruning: the target root it lived
// under, the entry itself, and the complete evidence prose (what disk shows
// against what the entry recorded). The caller reports each prune loudly
// (§6.4).
type Pruned struct {
	TargetRoot string
	Entry      Entry
	Evidence   string // complete prose: what disk shows vs what the entry records
}

// LockedError is fail-fast lock contention (§6.2): another dstow operation
// holds the ledger lock, and dstow refuses to wait (dpkg/pacman precedent).
type LockedError struct {
	LockPath string
}

func (e *LockedError) Error() string {
	return fmt.Sprintf("another dstow operation holds the ledger lock (%s); dstow fails fast rather than wait — retry once the other operation finishes", e.LockPath)
}

// Update runs a write transaction against the ledger (§6.2/§6.3, A10). It
// takes an exclusive non-blocking flock on the sibling lock for the whole
// operation (contention → *LockedError, immediately), then under the lock:
// takes a fresh Load (never a stale snapshot), prunes contradicted entries the
// scope covers (reporting each in the returned slice — the entry is pruned,
// disk is never touched), runs fn on the pruned document, drops any empty
// target group, and commits with one atomic write. fn returning an error
// aborts the transaction with no write.
func Update(path string, scope Scope, fn func(*Ledger) error) (pruned []Pruned, err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	lockPath := LockPath(path)
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	// Release the flock and close the descriptor on every path (§6.2).
	defer func() {
		_ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
		if cerr := lockFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if lerr := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); lerr != nil {
		if errors.Is(lerr, unix.EWOULDBLOCK) {
			return nil, &LockedError{LockPath: lockPath}
		}
		return nil, lerr
	}

	// Fresh Load under the lock — never a stale caller-held snapshot (§6.3).
	l, err := Load(path)
	if err != nil {
		return nil, err
	}

	// Scoped pruning (§6.3): remove every contradicted entry the scope covers,
	// reporting each with its evidence. Disk is never touched here.
	for root, entries := range l.Targets {
		kept := entries[:0:0]
		for _, e := range entries {
			if scope.covers(root, e) {
				if bad, evidence := e.Contradicted(root); bad {
					pruned = append(pruned, Pruned{TargetRoot: root, Entry: e, Evidence: evidence})
					continue
				}
			}
			kept = append(kept, e)
		}
		l.Targets[root] = kept
	}

	if err := fn(&l); err != nil {
		// fn aborts the transaction with no write (§6.2).
		return nil, err
	}

	// An index holds what exists: drop target groups left empty by pruning or
	// by fn (§6.2/§6.3).
	for root, entries := range l.Targets {
		if len(entries) == 0 {
			delete(l.Targets, root)
		}
	}

	if err := writeAtomic(path, l); err != nil {
		return nil, err
	}
	return pruned, nil
}

// writeAtomic commits the document with the §6.2 discipline: marshal
// pretty-printed (a write-side courtesy, §6.1) → temp file in the same
// directory → fsync the file → rename over the ledger path → fsync the
// directory. Readers and crashes see the old file or the new, never a torn
// one.
func writeAtomic(path string, l Ledger) (err error) {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "ledger-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Clean up the temp file on any failure before the rename succeeds.
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmpName, path); err != nil {
		return err
	}
	return fsyncDir(dir)
}

// fsyncDir flushes a directory entry so the rename survives a crash.
func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		return err
	}
	return d.Close()
}
