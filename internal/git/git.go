// Package git is dstow's version-control seam: system git behind a port
// (DESIGN.md A17). dstow shells out to the user's own git rather than
// embedding a git implementation, because credential-helper and ssh-config
// fidelity is exactly what dotfiles users depend on (go-git was rejected for
// this reason). The Port interface states git in repo's terms — clone, fetch,
// fast-forward apply, ahead/behind, and the unsaved-work guard — and has two
// implementations: Exec drives the real binary in production, Fake is an
// in-memory double for other packages' tests.
//
// The package returns data and typed errors only (A4): it never writes to
// stdout or stderr, and every failure it names is a typed error whose message
// carries the remedy. A missing git binary surfaces as *NotInstalledError only
// when a Port method actually runs (A17: git is needed at remote-scheme
// operations, never at construction).
package git

import (
	"fmt"
	"strings"
)

// Port is the git seam in repo's terms (A17). Production wiring uses Exec (the
// system git binary); tests use Fake.
type Port interface {
	// Clone copies the repository at url into dir (§5.2 remote add).
	Clone(url, dir string) error
	// Fetch downloads upstream changes into dir's object store and updates its
	// remote-tracking refs. It touches no working-tree file (REQUIREMENTS
	// §6.1: the fetch phase alters no working tree).
	Fetch(dir string) error
	// FFApply fast-forwards dir's checked-out branch to its upstream and
	// reports the old and new revisions (§6.2). It is fast-forward only: on
	// divergence it refuses with a *DivergedError and moves nothing.
	FFApply(dir string) (old, new string, err error)
	// AheadBehind reports how far dir's branch is ahead of and behind its
	// upstream, as of the last fetch (§6.1). Divergence (both > 0) is data,
	// not an error.
	AheadBehind(dir string) (ahead, behind int, err error)
	// HasLocalWork reports whether dir holds work that its source does not —
	// an uncommitted working-tree change or unpushed commits (REQUIREMENTS
	// §5.3 unsaved-work guard, mapped to git's dirty/unpushed). The string is
	// complete prose naming exactly what would be lost; it is meaningful only
	// when the bool is true.
	HasLocalWork(dir string) (bool, string, error)
}

// NotInstalledError is the clean §1.4 refusal when git is required but its
// executable is not on PATH (A17). It surfaces only when a Port method runs —
// a machine with no remote repos never sees it — and its message names the
// remedy: install git.
type NotInstalledError struct {
	Binary string // the executable name that was looked up (normally "git")
}

func (e *NotInstalledError) Error() string {
	return fmt.Sprintf("git is required for remote repo operations, but the %q executable was not found on PATH; install git and retry", e.Binary)
}

// DivergedError is FFApply's refusal when the clone has diverged from its
// upstream (§6.2): dstow upgrades by fast-forward only and negotiates no
// stash, merge, or rebase. Stderr carries git's own explanation.
type DivergedError struct {
	Dir    string
	Stderr string
}

func (e *DivergedError) Error() string {
	msg := fmt.Sprintf("cannot fast-forward %s: the clone has diverged from its upstream, and dstow upgrades by fast-forward only (no stash, merge, or rebase); resolve the divergence in the clone yourself, or remove and re-add the repo", e.Dir)
	if s := strings.TrimSpace(e.Stderr); s != "" {
		msg += "\ngit said: " + s
	}
	return msg
}

// CommandError is the typed wrapper for any other git invocation that exits
// nonzero: it names the arguments and includes git's stderr so the caller can
// render a §1.4 message. The underlying exec error is available via Unwrap.
type CommandError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *CommandError) Error() string {
	msg := fmt.Sprintf("git %s failed", strings.Join(e.Args, " "))
	if s := strings.TrimSpace(e.Stderr); s != "" {
		msg += ": " + s
	} else if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

func (e *CommandError) Unwrap() error { return e.Err }
