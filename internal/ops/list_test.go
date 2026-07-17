package ops_test

import (
	"path/filepath"
	"testing"

	"github.com/rocne/dstow/internal/ops"
)

// TestListReposShowsSourceSchemeExclusion: bare list enumerates the repos —
// the global scope's content — carrying source, scheme, and bulk-exclusion
// (§2.4), reading only configuration.
func TestListReposShowsSourceSchemeExclusion(t *testing.T) {
	e := newEnv(t)
	e.addRepo("alpha")
	e.addRepo("beta")
	// beta excludes itself from bulk via the repo-level knob.
	e.writeFile(filepath.Join(e.base, "beta", ".dstow", "config.toml"),
		"target = \""+e.target+"\"\nexclude_from_bulk = true\n")

	res, err := e.app.List(ops.ListRequest{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if res.Kind != ops.KindRepos {
		t.Fatalf("kind = %v, want KindRepos", res.Kind)
	}
	if len(res.Repos) != 2 {
		t.Fatalf("got %d repos, want 2: %+v", len(res.Repos), res.Repos)
	}
	byDisplay := map[string]ops.RepoListing{}
	for _, r := range res.Repos {
		byDisplay[r.Display] = r
	}
	alpha, ok := byDisplay["alpha"]
	if !ok {
		t.Fatalf("alpha not listed by shortest-unique name: %+v", res.Repos)
	}
	if alpha.Scheme != "local" {
		t.Errorf("alpha scheme = %q, want local", alpha.Scheme)
	}
	if alpha.Source == "" {
		t.Error("alpha source should name where it came from")
	}
	if alpha.ExcludedBulk {
		t.Error("alpha is not bulk-excluded")
	}
	if beta, ok := byDisplay["beta"]; !ok || !beta.ExcludedBulk {
		t.Errorf("beta should be listed and bulk-excluded: %+v", byDisplay)
	}
}

// TestListRepoLandsOnItsPackages: naming a repo lists its packages,
// repo-attributed (§2.4).
func TestListRepoLandsOnItsPackages(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")
	e.writeFile(filepath.Join(root, "git", "dot-gitconfig"), "g\n")

	res, err := e.app.List(ops.ListRequest{Name: "dots"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if res.Kind != ops.KindPackages {
		t.Fatalf("kind = %v, want KindPackages", res.Kind)
	}
	got := map[string]bool{}
	for _, p := range res.Packages {
		got[p.Display] = true
		if p.Repo.String() != pkgFQN(root, "").Repo().String() {
			t.Errorf("package %q not attributed to its repo: %v", p.Display, p.Repo)
		}
	}
	if !got["git"] || !got["zsh"] {
		t.Errorf("packages = %v, want git and zsh (unqualified, unique within one repo)", got)
	}
}

// TestListPackagesQualifiesSharedNames: --packages spans every repo; entries
// sharing a bare name are shown with qualified names (REQUIREMENTS §7.1, O9).
func TestListPackagesQualifiesSharedNames(t *testing.T) {
	e := newEnv(t)
	ra := e.addRepo("alpha")
	rb := e.addRepo("beta")
	e.writeFile(filepath.Join(ra, "zsh", "dot-zshrc"), "z\n")
	e.writeFile(filepath.Join(rb, "zsh", "dot-zshrc"), "z\n")
	e.writeFile(filepath.Join(ra, "git", "dot-gitconfig"), "g\n")

	res, err := e.app.List(ops.ListRequest{PackagesOnly: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if res.Kind != ops.KindPackages {
		t.Fatalf("kind = %v, want KindPackages", res.Kind)
	}
	displays := map[string]bool{}
	for _, p := range res.Packages {
		displays[p.Display] = true
	}
	// The lone git package stays bare; the shared zsh climbs to qualified.
	if !displays["git"] {
		t.Errorf("git should stay bare: %v", displays)
	}
	if displays["zsh"] {
		t.Errorf("shared zsh must be qualified, not bare: %v", displays)
	}
	if !displays["alpha::zsh"] || !displays["beta::zsh"] {
		t.Errorf("shared zsh should qualify to alpha::zsh / beta::zsh: %v", displays)
	}
}

// TestListPackagePathsAreRawRelative: naming a package lists its paths,
// relative to the package directory, raw — a plain walk, no translation and no
// ignore application (§2.4 ruling).
func TestListPackagePathsAreRawRelative(t *testing.T) {
	e := newEnv(t)
	root := e.addRepo("dots")
	e.writeFile(filepath.Join(root, "zsh", "dot-zshrc"), "z\n")
	e.writeFile(filepath.Join(root, "zsh", "conf", "dot-aliases"), "a\n")
	// The .dstow metadata dir is dstow's own bookkeeping, not package content.
	e.writeFile(filepath.Join(root, "zsh", ".dstow", "config.toml"), "\n")

	res, err := e.app.List(ops.ListRequest{Name: "zsh"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if res.Kind != ops.KindPaths {
		t.Fatalf("kind = %v, want KindPaths", res.Kind)
	}
	got := map[string]bool{}
	for _, p := range res.Paths {
		got[p.Path] = true
	}
	// Raw (untranslated) package-relative paths — no dot- rewrite.
	if !got["dot-zshrc"] || !got["conf/dot-aliases"] {
		t.Errorf("paths = %v, want raw dot-zshrc and conf/dot-aliases", got)
	}
	if got[".zshrc"] {
		t.Errorf("paths must be raw, not dot-translated: %v", got)
	}
	if got[".dstow/config.toml"] {
		t.Errorf("the .dstow metadata dir is not package content: %v", got)
	}
}
