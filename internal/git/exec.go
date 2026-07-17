package git

import (
	"bytes"
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

// Exec is the production Port: it drives the system git binary (A17). The
// Binary field is an internal seam — empty means the real "git" on PATH; a
// test may point it at a nonexistent name to exercise the *NotInstalledError
// path. Presence is checked per call (LookPath is cheap), so the error
// surfaces at the operation, never at construction.
type Exec struct {
	Binary string // executable name; "" means "git"
}

// compile-time assertion that Exec is a Port.
var _ Port = Exec{}

// binary returns the executable name to run.
func (e Exec) binary() string {
	if e.Binary == "" {
		return "git"
	}
	return e.Binary
}

// run invokes git with args, capturing stdout and stderr. A missing binary is
// a *NotInstalledError (checked here, per call, so it surfaces only when an
// operation actually runs); a nonzero exit is a *CommandError carrying git's
// stderr.
func (e Exec) run(args ...string) (stdout string, err error) {
	bin := e.binary()
	if _, lerr := exec.LookPath(bin); lerr != nil {
		return "", &NotInstalledError{Binary: bin}
	}
	cmd := exec.Command(bin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if rerr := cmd.Run(); rerr != nil {
		return out.String(), &CommandError{Args: args, Stderr: errb.String(), Err: rerr}
	}
	return out.String(), nil
}

// Clone runs `git clone url dir`.
func (e Exec) Clone(url, dir string) error {
	_, err := e.run("clone", url, dir)
	return err
}

// Fetch runs `git -C dir fetch` — download only, no working-tree changes.
func (e Exec) Fetch(dir string) error {
	_, err := e.run("-C", dir, "fetch")
	return err
}

// FFApply fast-forwards dir to its upstream and reports old→new HEAD (§6.2).
// It records HEAD, runs `git -C dir merge --ff-only @{upstream}` (whose refusal
// on divergence becomes a *DivergedError with git's stderr), then records HEAD
// again.
func (e Exec) FFApply(dir string) (old, new string, err error) {
	old, err = e.revParse(dir, "HEAD")
	if err != nil {
		return "", "", err
	}
	if _, merr := e.run("-C", dir, "merge", "--ff-only", "@{upstream}"); merr != nil {
		var ce *CommandError
		if errors.As(merr, &ce) {
			// Claim divergence only on the evidence for it — local and
			// upstream commits both present (§1.5: no unbacked claims). Any
			// other merge failure (no upstream, an overwrite refusal) stays a
			// *CommandError with git's own stderr.
			if ahead, behind, aerr := e.AheadBehind(dir); aerr == nil && ahead > 0 && behind > 0 {
				return "", "", &DivergedError{Dir: dir, Stderr: ce.Stderr}
			}
		}
		return "", "", merr
	}
	new, err = e.revParse(dir, "HEAD")
	if err != nil {
		return "", "", err
	}
	return old, new, nil
}

// AheadBehind reports ahead/behind versus the upstream (§6.1) via
// `git -C dir rev-list --left-right --count HEAD...@{upstream}`: the left count
// is commits on HEAD not upstream (ahead), the right is commits on upstream not
// HEAD (behind). Divergence (both > 0) is returned as data, never an error.
func (e Exec) AheadBehind(dir string) (ahead, behind int, err error) {
	out, err := e.run("-C", dir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) != 2 {
		return 0, 0, &CommandError{
			Args:   []string{"rev-list", "--left-right", "--count", "HEAD...@{upstream}"},
			Stderr: "expected two counts from rev-list, got: " + strings.TrimSpace(out),
		}
	}
	ahead, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, &CommandError{Args: []string{"rev-list", "--left-right", "--count"}, Stderr: "unparseable ahead count: " + fields[0], Err: err}
	}
	behind, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, &CommandError{Args: []string{"rev-list", "--left-right", "--count"}, Stderr: "unparseable behind count: " + fields[1], Err: err}
	}
	return ahead, behind, nil
}

// HasLocalWork implements the unsaved-work guard (REQUIREMENTS §5.3): a
// non-empty `git -C dir status --porcelain` means uncommitted local changes;
// otherwise ahead > 0 versus the upstream means commits not pushed to the
// source. The prose names exactly what would be lost.
func (e Exec) HasLocalWork(dir string) (bool, string, error) {
	out, err := e.run("-C", dir, "status", "--porcelain")
	if err != nil {
		return false, "", err
	}
	if strings.TrimSpace(out) != "" {
		return true, "the clone at " + dir + " holds local changes that are not committed; they exist only here and are not present at the source", nil
	}
	ahead, _, err := e.AheadBehind(dir)
	if err != nil {
		return false, "", err
	}
	if ahead > 0 {
		return true, "the clone at " + dir + " holds " + strconv.Itoa(ahead) + " commit(s) not pushed to its source; they exist only here", nil
	}
	return false, "", nil
}

// revParse resolves a revision to its full hash.
func (e Exec) revParse(dir, rev string) (string, error) {
	out, err := e.run("-C", dir, "rev-parse", rev)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
