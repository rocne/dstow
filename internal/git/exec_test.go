package git_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rocne/dstow/internal/git"
)

func asDiverged(err error, target **git.DivergedError) bool {
	return errors.As(err, target)
}

func asNotInstalled(err error, target **git.NotInstalledError) bool {
	return errors.As(err, target)
}

// requireGit skips when git is absent; CI runners have it. Exec tests run the
// real binary against throwaway repos in t.TempDir.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found on PATH; skipping Exec integration tests")
	}
}

// runGit runs git in dir (or cwd when dir is "") and fatals on failure. Used
// only to build fixtures.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	var full []string
	if dir != "" {
		full = append(full, "-C", dir)
	}
	full = append(full, args...)
	cmd := exec.Command("git", full...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", full, err, out)
	}
}

// configUser sets a per-repo identity so runners without global git config can
// commit; branch/gpg noise is disabled per repo too.
func configUser(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "config", "user.name", "dstow test")
	runGit(t, dir, "config", "user.email", "test@dstow.invalid")
	runGit(t, dir, "config", "commit.gpgsign", "false")
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

// newOrigin creates a bare origin with one committed-and-pushed file on main,
// returning the origin path.
func newOrigin(t *testing.T) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", "-b", "main", origin)

	seed := t.TempDir()
	runGit(t, "", "clone", origin, seed)
	configUser(t, seed)
	writeFile(t, filepath.Join(seed, "file.txt"), "v1\n")
	runGit(t, seed, "add", ".")
	runGit(t, seed, "commit", "-m", "initial")
	runGit(t, seed, "push", "origin", "main")
	return origin
}

// cloneOf clones origin into a fresh temp dir with tracking set up and returns
// the clone path.
func cloneOf(t *testing.T, origin string) string {
	t.Helper()
	clone := t.TempDir()
	runGit(t, "", "clone", origin, clone)
	configUser(t, clone)
	return clone
}

// advanceOrigin pushes a new commit to origin's main, so clones fall behind.
func advanceOrigin(t *testing.T, origin, content string) {
	t.Helper()
	w := t.TempDir()
	runGit(t, "", "clone", origin, w)
	configUser(t, w)
	writeFile(t, filepath.Join(w, "file.txt"), content)
	runGit(t, w, "commit", "-am", "advance")
	runGit(t, w, "push", "origin", "main")
}

func TestExecClone(t *testing.T) {
	requireGit(t)
	origin := newOrigin(t)
	dst := filepath.Join(t.TempDir(), "clone")

	if err := (git.Exec{}).Clone(origin, dst); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "file.txt")); err != nil {
		t.Errorf("clone did not produce file.txt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); err != nil {
		t.Errorf("clone did not produce a .git directory: %v", err)
	}
}

func TestExecFetchTouchesNoWorkingTreeAndAheadBehindReportsBehind(t *testing.T) {
	requireGit(t)
	origin := newOrigin(t)
	clone := cloneOf(t, origin)
	advanceOrigin(t, origin, "v2\n")

	file := filepath.Join(clone, "file.txt")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	e := git.Exec{}
	if err := e.Fetch(clone); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("Fetch changed the working tree: %q -> %q", before, after)
	}

	ahead, behind, err := e.AheadBehind(clone)
	if err != nil {
		t.Fatalf("AheadBehind: %v", err)
	}
	if ahead != 0 || behind != 1 {
		t.Errorf("AheadBehind = %d, %d; want 0, 1", ahead, behind)
	}
}

func TestExecFFApplyFastForwards(t *testing.T) {
	requireGit(t)
	origin := newOrigin(t)
	clone := cloneOf(t, origin)
	advanceOrigin(t, origin, "v2\n")

	e := git.Exec{}
	if err := e.Fetch(clone); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	old, newRev, err := e.FFApply(clone)
	if err != nil {
		t.Fatalf("FFApply: %v", err)
	}
	if old == "" || newRev == "" || old == newRev {
		t.Errorf("FFApply old/new = %q/%q; want two distinct non-empty hashes", old, newRev)
	}

	// old must be the pre-merge HEAD; new must be the upstream tip.
	upstream := revParse(t, clone, "@{upstream}")
	if newRev != upstream {
		t.Errorf("FFApply new = %q; want upstream %q", newRev, upstream)
	}
	// The working tree is now the applied content.
	got, err := os.ReadFile(filepath.Join(clone, "file.txt"))
	if err != nil || string(got) != "v2\n" {
		t.Errorf("file after FFApply = %q, err %v; want v2", got, err)
	}
}

func TestExecFFApplyDivergedRefusesAndMovesNothing(t *testing.T) {
	requireGit(t)
	origin := newOrigin(t)
	clone := cloneOf(t, origin)

	// Local commit on the clone: now ahead by one.
	writeFile(t, filepath.Join(clone, "local.txt"), "local\n")
	runGit(t, clone, "add", ".")
	runGit(t, clone, "commit", "-m", "local work")
	// Origin advances differently: after fetch the clone is ahead 1, behind 1.
	advanceOrigin(t, origin, "v2\n")

	e := git.Exec{}
	if err := e.Fetch(clone); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	head := revParse(t, clone, "HEAD")

	old, newRev, err := e.FFApply(clone)
	var de *git.DivergedError
	if err == nil {
		t.Fatalf("FFApply on a diverged clone returned nil error (old=%q new=%q)", old, newRev)
	}
	if !asDiverged(err, &de) {
		t.Fatalf("FFApply error = %T (%v); want *git.DivergedError", err, err)
	}
	if revParse(t, clone, "HEAD") != head {
		t.Errorf("FFApply moved HEAD on a diverged clone")
	}
}

// A merge failure that is NOT divergence must not be claimed as one
// (REQUIREMENTS §1.5, no unbacked claims): a repo with no upstream fails
// FFApply with a plain *CommandError, never a *DivergedError.
func TestExecFFApplyNonDivergenceFailureStaysCommandError(t *testing.T) {
	requireGit(t)
	lone := t.TempDir()
	runGit(t, "", "init", "-b", "main", lone)
	configUser(t, lone)
	writeFile(t, filepath.Join(lone, "file.txt"), "v1\n")
	runGit(t, lone, "add", ".")
	runGit(t, lone, "commit", "-m", "initial")

	_, _, err := git.Exec{}.FFApply(lone)
	if err == nil {
		t.Fatal("FFApply with no upstream returned nil error")
	}
	var de *git.DivergedError
	if asDiverged(err, &de) {
		t.Fatalf("FFApply claimed divergence for a no-upstream failure: %v", err)
	}
	var ce *git.CommandError
	if !errors.As(err, &ce) {
		t.Fatalf("FFApply error = %T (%v); want *git.CommandError", err, err)
	}
}

func TestExecHasLocalWork(t *testing.T) {
	requireGit(t)
	origin := newOrigin(t)
	e := git.Exec{}

	// Clean clone: no local work.
	clean := cloneOf(t, origin)
	if work, prose, err := e.HasLocalWork(clean); err != nil || work || prose != "" {
		t.Errorf("HasLocalWork(clean) = %v, %q, %v; want false, \"\", nil", work, prose, err)
	}

	// Dirty clone: an uncommitted change.
	dirty := cloneOf(t, origin)
	writeFile(t, filepath.Join(dirty, "file.txt"), "changed\n")
	if work, prose, err := e.HasLocalWork(dirty); err != nil || !work || prose == "" {
		t.Errorf("HasLocalWork(dirty) = %v, %q, %v; want true, prose, nil", work, prose, err)
	}

	// Committed-but-unpushed clone: ahead of upstream.
	unpushed := cloneOf(t, origin)
	writeFile(t, filepath.Join(unpushed, "new.txt"), "new\n")
	runGit(t, unpushed, "add", ".")
	runGit(t, unpushed, "commit", "-m", "unpushed")
	if work, prose, err := e.HasLocalWork(unpushed); err != nil || !work || prose == "" {
		t.Errorf("HasLocalWork(unpushed) = %v, %q, %v; want true, prose, nil", work, prose, err)
	}
}

// TestExecNotInstalled exercises the *NotInstalledError path with an Exec
// pointed at a nonexistent binary; it needs no real git.
func TestExecNotInstalled(t *testing.T) {
	e := git.Exec{Binary: "dstow-nonexistent-git-xyz"}
	var ni *git.NotInstalledError

	if err := e.Fetch("anywhere"); !asNotInstalled(err, &ni) {
		t.Errorf("Fetch err = %T (%v); want *git.NotInstalledError", err, err)
	}
	if err := e.Clone("url", "dir"); !asNotInstalled(err, &ni) {
		t.Errorf("Clone err = %T (%v); want *git.NotInstalledError", err, err)
	}
	if _, _, err := e.FFApply("dir"); !asNotInstalled(err, &ni) {
		t.Errorf("FFApply err = %T (%v); want *git.NotInstalledError", err, err)
	}
	if _, _, err := e.AheadBehind("dir"); !asNotInstalled(err, &ni) {
		t.Errorf("AheadBehind err = %T (%v); want *git.NotInstalledError", err, err)
	}
	if _, _, err := e.HasLocalWork("dir"); !asNotInstalled(err, &ni) {
		t.Errorf("HasLocalWork err = %T (%v); want *git.NotInstalledError", err, err)
	}
}

func revParse(t *testing.T, dir, rev string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", rev).Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", rev, err)
	}
	return string(out[:len(out)-1]) // trim trailing newline
}
