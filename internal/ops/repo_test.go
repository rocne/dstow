package ops_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/git"
	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/repo"
)

// --- test helpers for the repo verbs ---------------------------------------

// localSrc mirrors ops' localSource: the local source of an absolute dir.
func localSrc(dir string) repo.Source {
	return repo.Source{Scheme: "local", Coordinate: strings.Split(filepath.ToSlash(dir), "/")}
}

// githubSrc builds a github source from owner/name.
func githubSrc(owner, name string) repo.Source {
	return repo.Source{Scheme: "github", Coordinate: []string{owner, name}}
}

// seedRegistry writes the given sources to the (XDG-temp) registry file.
func seedRegistry(t *testing.T, sources ...repo.Source) {
	t.Helper()
	reg := repo.Registry{Sources: sources}
	if err := reg.Save(config.RegistryFile()); err != nil {
		t.Fatalf("seed registry: %v", err)
	}
}

// registrySources reads the registry back, canonical strings sorted.
func registrySources(t *testing.T) []string {
	t.Helper()
	reg, _, err := repo.LoadRegistry(config.RegistryFile())
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	var out []string
	for _, s := range reg.Sources {
		out = append(out, s.String())
	}
	return out
}

// managedRepo builds a managed (github) repo set-member and creates its clone
// dir so removal has something to delete.
func managedRepo(t *testing.T, owner, name string) repo.Repo {
	t.Helper()
	src := githubSrc(owner, name)
	r := repo.BuildSet([]repo.Source{src}, nil)[0]
	if err := os.MkdirAll(r.Root, 0o755); err != nil {
		t.Fatalf("mkdir clone dir: %v", err)
	}
	return r
}

// --- repo add --------------------------------------------------------------

func TestRepoAddQualifiedGithubClones(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	fake := &git.Fake{}
	e.app.Git = fake

	res, err := e.app.RepoAdd(ops.RepoAddRequest{Source: "github:rocne/dotfiles"})
	if err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	if !res.Managed || !res.Cloned {
		t.Errorf("expected a managed clone, got Managed=%v Cloned=%v", res.Managed, res.Cloned)
	}
	if len(fake.CloneCalls) != 1 {
		t.Fatalf("expected 1 clone call, got %d", len(fake.CloneCalls))
	}
	call := fake.CloneCalls[0]
	if call.URL != "https://github.com/rocne/dotfiles.git" {
		t.Errorf("clone URL = %q", call.URL)
	}
	if call.Dir != githubSrc("rocne", "dotfiles").CloneDir() {
		t.Errorf("clone dir = %q, want %q", call.Dir, githubSrc("rocne", "dotfiles").CloneDir())
	}
	if got := registrySources(t); len(got) != 1 || got[0] != "github:rocne/dotfiles" {
		t.Errorf("registry = %v, want [github:rocne/dotfiles]", got)
	}
}

func TestRepoAddLocalPathCanonicalizedInPlace(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	fake := &git.Fake{}
	e.app.Git = fake

	dir := filepath.Join(e.base, "mydots")
	if err := os.MkdirAll(filepath.Join(dir, "zsh"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.writeFile(filepath.Join(dir, "zsh", ".zshrc"), "# zsh\n")

	res, err := e.app.RepoAdd(ops.RepoAddRequest{Source: dir}) // absolute path → PathForm
	if err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	if res.Managed || res.Cloned {
		t.Errorf("local add should neither be managed nor cloned")
	}
	if len(fake.CloneCalls) != 0 {
		t.Errorf("local add must not clone")
	}
	if got := registrySources(t); len(got) != 1 || got[0] != localSrc(dir).String() {
		t.Errorf("registry = %v, want [%s]", got, localSrc(dir).String())
	}
	if len(res.Packages) != 1 || res.Packages[0] != "zsh" {
		t.Errorf("announced packages = %v, want [zsh]", res.Packages)
	}
}

func TestRepoAddBareConfirmsGithubInterpretation(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	e.app.Git = &git.Fake{}
	e.prompt.answers = []bool{true} // confirm the github interpretation

	res, err := e.app.RepoAdd(ops.RepoAddRequest{Source: "rocne/dotfiles"})
	if err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	if !res.Managed {
		t.Errorf("bare owner/name confirmed should resolve to a github source")
	}
	if len(e.prompt.asked) != 1 {
		t.Errorf("expected exactly one confirmation, got %d", len(e.prompt.asked))
	}
}

func TestRepoAddBareDeclinedRefuses(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	e.app.Git = &git.Fake{}
	e.prompt.answers = []bool{false} // decline

	_, err := e.app.RepoAdd(ops.RepoAddRequest{Source: "rocne/dotfiles"})
	var declined *ops.SourceDeclinedError
	if !errors.As(err, &declined) {
		t.Fatalf("expected *SourceDeclinedError, got %v", err)
	}
	if got := registrySources(t); len(got) != 0 {
		t.Errorf("nothing should be registered after a declined add, got %v", got)
	}
}

func TestRepoAddBareNonInteractiveHardRefusal(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	e.app.Git = &git.Fake{}
	// no scripted answers → the prompter errors (non-interactive stance)

	_, err := e.app.RepoAdd(ops.RepoAddRequest{Source: "rocne/dotfiles"})
	if err == nil {
		t.Fatal("expected a hard refusal non-interactively")
	}
}

func TestRepoAddBareLocalAndGithubIsAmbiguous(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	e.app.Git = &git.Fake{}

	// Create ./owner/name relative to a cwd we control, so the bare input
	// matches both a local directory and an owner/name github source.
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll(filepath.Join(dir, "owner", "name"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := e.app.RepoAdd(ops.RepoAddRequest{Source: "owner/name"})
	var amb *ops.SourceAmbiguousError
	if !errors.As(err, &amb) {
		t.Fatalf("expected *SourceAmbiguousError, got %v", err)
	}
}

func TestRepoAddEncodingConfirm(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	e.app.Git = &git.Fake{}

	weird := filepath.Join(e.base, "od:d") // ':' percent-encodes
	if err := os.MkdirAll(weird, 0o755); err != nil {
		t.Fatal(err)
	}

	// Answer no → rename requested, nothing registered.
	e.prompt.answers = []bool{false}
	_, err := e.app.RepoAdd(ops.RepoAddRequest{Source: weird})
	var rename *ops.RenameRequestedError
	if !errors.As(err, &rename) {
		t.Fatalf("expected *RenameRequestedError, got %v", err)
	}
	if got := registrySources(t); len(got) != 0 {
		t.Errorf("rename request must not register, got %v", got)
	}

	// Answer yes → proceeds with the encoded form.
	e.prompt.answers = []bool{true}
	res, err := e.app.RepoAdd(ops.RepoAddRequest{Source: weird})
	if err != nil {
		t.Fatalf("RepoAdd (confirmed): %v", err)
	}
	if !strings.Contains(res.Source.String(), "od%3Ad") {
		t.Errorf("expected the encoded segment in %s", res.Source.String())
	}
}

func TestRepoAddReAddIsNoOp(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	fake := &git.Fake{}
	e.app.Git = fake
	seedRegistry(t, githubSrc("rocne", "dotfiles"))

	res, err := e.app.RepoAdd(ops.RepoAddRequest{Source: "github:rocne/dotfiles"})
	if err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	if !res.AlreadyPresent {
		t.Errorf("re-adding a present repo should be an announced no-op")
	}
	if len(fake.CloneCalls) != 0 {
		t.Errorf("re-add must not clone again")
	}
	if got := registrySources(t); len(got) != 1 {
		t.Errorf("re-add must not duplicate the entry, got %v", got)
	}
}

func TestRepoAddWithStowStowsThePackages(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	e.app.Git = &git.Fake{}

	dir := filepath.Join(e.base, "dots")
	e.writeFile(filepath.Join(dir, ".dstow", "config.toml"), "target = \""+e.target+"\"\n")
	e.writeFile(filepath.Join(dir, "zsh", ".zshrc"), "# zsh\n")

	res, err := e.app.RepoAdd(ops.RepoAddRequest{Source: dir, Stow: true})
	if err != nil {
		t.Fatalf("RepoAdd --stow: %v", err)
	}
	if res.Deploy == nil {
		t.Fatal("--stow should compose a deploy run")
	}
	if res.Deploy.Failed() {
		t.Errorf("stow run failed: %+v", res.Deploy)
	}
	isLinkTo(t, filepath.Join(e.target, ".zshrc"), filepath.Join(dir, "zsh", ".zshrc"))
}

func TestRepoAddAnnouncesShadowedNames(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	e.app.Git = &git.Fake{}

	// An existing repo already has a package named "zsh".
	existing := e.addRepo("existing")
	e.writeFile(filepath.Join(existing, "zsh", ".zshrc"), "# a\n")

	// The new repo also has "zsh" (shadow) and a unique "vim".
	dir := filepath.Join(e.base, "newdots")
	e.writeFile(filepath.Join(dir, "zsh", ".zshrc"), "# b\n")
	e.writeFile(filepath.Join(dir, "vim", ".vimrc"), "\" b\n")

	res, err := e.app.RepoAdd(ops.RepoAddRequest{Source: dir})
	if err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	if len(res.Shadowed) != 1 || res.Shadowed[0] != "zsh" {
		t.Errorf("shadowed = %v, want [zsh] (the name now shared across repos)", res.Shadowed)
	}
}

// --- repo remove -----------------------------------------------------------

func TestRepoRemoveLocalForgetsButKeepsDir(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	dir := e.addRepo("dots") // in the set, target configured
	seedRegistry(t, localSrc(dir))

	res, err := e.app.RepoRemove(ops.RepoRemoveRequest{Repo: localSrc(dir).String()})
	if err != nil {
		t.Fatalf("RepoRemove: %v", err)
	}
	if res.Deleted {
		t.Errorf("a local repo's directory must never be deleted")
	}
	if _, serr := os.Stat(dir); serr != nil {
		t.Errorf("local repo dir should still exist: %v", serr)
	}
	if got := registrySources(t); len(got) != 0 {
		t.Errorf("local repo should be forgotten from the registry, got %v", got)
	}
}

func TestRepoRemoveStillStowedRefusesNonInteractively(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	dir := e.addRepo("dots")
	e.writeFile(filepath.Join(dir, "zsh", ".zshrc"), "# zsh\n")
	seedRegistry(t, localSrc(dir))

	// Stow so the ledger holds a link for the repo.
	if _, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}}); err != nil {
		t.Fatalf("stow: %v", err)
	}

	_, err := e.app.RepoRemove(ops.RepoRemoveRequest{Repo: localSrc(dir).String()})
	var stowed *ops.StillStowedError
	if !errors.As(err, &stowed) {
		t.Fatalf("expected *StillStowedError, got %v", err)
	}
	if got := registrySources(t); len(got) != 1 {
		t.Errorf("a refused removal must not touch the registry, got %v", got)
	}
}

func TestRepoRemoveStillStowedUnstowThenRemove(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	dir := e.addRepo("dots")
	e.writeFile(filepath.Join(dir, "zsh", ".zshrc"), "# zsh\n")
	seedRegistry(t, localSrc(dir))
	if _, err := e.app.Deploy(ops.DeployRequest{Verb: engine.VerbStow, Names: []string{"zsh"}}); err != nil {
		t.Fatalf("stow: %v", err)
	}

	res, err := e.app.RepoRemove(ops.RepoRemoveRequest{Repo: localSrc(dir).String(), Unstow: true})
	if err != nil {
		t.Fatalf("RepoRemove --unstow: %v", err)
	}
	if res.Unstowed == nil {
		t.Error("--unstow should compose an unstow run")
	}
	if _, serr := os.Lstat(filepath.Join(e.target, ".zshrc")); !os.IsNotExist(serr) {
		t.Errorf("the link should be gone after unstow-then-remove")
	}
	if got := registrySources(t); len(got) != 0 {
		t.Errorf("the repo should be forgotten, got %v", got)
	}
}

func TestRepoRemoveManagedUnsavedWorkGuard(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	r := managedRepo(t, "rocne", "dotfiles")
	e.app.Repos = []repo.Repo{r}
	seedRegistry(t, githubSrc("rocne", "dotfiles"))
	e.app.Git = &git.Fake{LocalWork: true, LocalWorkReason: "1 uncommitted change"}

	_, err := e.app.RepoRemove(ops.RepoRemoveRequest{Repo: "rocne/dotfiles"})
	var unsaved *ops.UnsavedWorkError
	if !errors.As(err, &unsaved) {
		t.Fatalf("expected *UnsavedWorkError, got %v", err)
	}
	if _, serr := os.Stat(r.Root); serr != nil {
		t.Errorf("a guarded managed clone must not be deleted: %v", serr)
	}

	// --force bypasses the guard and deletes the clone.
	res, ferr := e.app.RepoRemove(ops.RepoRemoveRequest{Repo: "rocne/dotfiles", Force: true})
	if ferr != nil {
		t.Fatalf("RepoRemove --force: %v", ferr)
	}
	if !res.Deleted {
		t.Errorf("--force should delete the managed clone")
	}
	if _, serr := os.Stat(r.Root); !os.IsNotExist(serr) {
		t.Errorf("clone dir should be gone after --force remove")
	}
}

// --- repo update / upgrade -------------------------------------------------

func TestRepoUpdateAllRemoteFetches(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	a := managedRepo(t, "rocne", "a")
	b := managedRepo(t, "rocne", "b")
	local := e.addRepo("localdots") // a local repo, must be skipped
	_ = local
	e.app.Repos = append(e.app.Repos, a, b)
	fake := &git.Fake{}
	e.app.Git = fake

	res, err := e.app.RepoUpdate(ops.RepoSyncRequest{})
	if err != nil {
		t.Fatalf("RepoUpdate: %v", err)
	}
	if len(fake.FetchCalls) != 2 {
		t.Errorf("expected 2 fetches (managed only), got %d", len(fake.FetchCalls))
	}
	if len(res.Repos) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(res.Repos))
	}
	for _, rep := range res.Repos {
		if !rep.Fetched || rep.Err != nil {
			t.Errorf("repo %s: Fetched=%v Err=%v", rep.FQN, rep.Fetched, rep.Err)
		}
	}
}

func TestRepoUpgradeFastForwardReportsOldNew(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	r := managedRepo(t, "rocne", "dotfiles")
	e.app.Repos = []repo.Repo{r}
	e.app.Git = &git.Fake{FFApplyOld: "aaa111", FFApplyNew: "bbb222"}

	res, err := e.app.RepoUpgrade(ops.RepoSyncRequest{})
	if err != nil {
		t.Fatalf("RepoUpgrade: %v", err)
	}
	if len(res.Repos) != 1 {
		t.Fatalf("expected 1 report, got %d", len(res.Repos))
	}
	rep := res.Repos[0]
	if rep.Old != "aaa111" || rep.New != "bbb222" || !rep.Changed {
		t.Errorf("upgrade report = %+v, want old→new aaa111→bbb222 changed", rep)
	}
}

func TestRepoUpgradeDivergedRefusesPerRepo(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	r := managedRepo(t, "rocne", "dotfiles")
	e.app.Repos = []repo.Repo{r}
	e.app.Git = &git.Fake{FFApplyErr: &git.DivergedError{Dir: r.Root, Stderr: "diverged"}}

	res, err := e.app.RepoUpgrade(ops.RepoSyncRequest{})
	if err != nil {
		t.Fatalf("RepoUpgrade returned a run-level error, want per-repo data: %v", err)
	}
	if !res.Failed() {
		t.Errorf("a diverged upgrade should mark the run failed")
	}
	var diverged *git.DivergedError
	if !errors.As(res.Repos[0].Err, &diverged) {
		t.Errorf("expected a *DivergedError in the repo report, got %v", res.Repos[0].Err)
	}
}

func TestRepoUpdateNotInstalledSurfacesNoPanic(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	r := managedRepo(t, "rocne", "dotfiles")
	e.app.Repos = []repo.Repo{r}
	e.app.Git = &git.Fake{FetchErr: &git.NotInstalledError{Binary: "git"}}

	res, err := e.app.RepoUpdate(ops.RepoSyncRequest{})
	if err != nil {
		t.Fatalf("RepoUpdate: %v", err)
	}
	var notInstalled *git.NotInstalledError
	if !errors.As(res.Repos[0].Err, &notInstalled) {
		t.Errorf("expected a *NotInstalledError in the repo report, got %v", res.Repos[0].Err)
	}
}

func TestRepoUpgradeNamedLocalSkipped(t *testing.T) {
	e := newEnv(t)
	setupXDG(t)
	dir := e.addRepo("dots")
	e.app.Git = &git.Fake{}

	res, err := e.app.RepoUpgrade(ops.RepoSyncRequest{Names: []string{localSrc(dir).String()}})
	if err != nil {
		t.Fatalf("RepoUpgrade: %v", err)
	}
	if len(res.Repos) != 1 || !res.Repos[0].Skipped {
		t.Errorf("a named local repo should be skipped, got %+v", res.Repos)
	}
}
