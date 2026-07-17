package repo_test

import (
	"sort"
	"testing"

	"github.com/rocne/dstow/internal/repo"
)

// fixtureEntities builds a small, deterministic entity set over three repos:
//
//	github:rocne/dotfiles  packages: zsh, git
//	github:acme/tools      packages: zsh
//	github:rocne/tmux      (no packages) — its last coordinate segment "tmux"
//	                        collides with the package name below
//	github:rocne/box       packages: tmux
func fixtureEntities(t *testing.T) []repo.Entity {
	t.Helper()
	pkgs := map[string][]string{
		"github:rocne/dotfiles": {"zsh", "git"},
		"github:acme/tools":     {"zsh"},
		"github:rocne/tmux":     {},
		"github:rocne/box":      {"tmux"},
	}
	set := repo.BuildSet([]repo.Source{
		mustSource(t, "github:rocne/dotfiles"),
		mustSource(t, "github:acme/tools"),
		mustSource(t, "github:rocne/tmux"),
		mustSource(t, "github:rocne/box"),
	}, nil)

	ents, err := repo.Entities(set, func(r repo.Repo) ([]string, error) {
		return pkgs[r.FQN.String()], nil
	})
	if err != nil {
		t.Fatalf("Entities: %v", err)
	}
	return ents
}

func fqnStrings(ents []repo.Entity) []string {
	var out []string
	for _, e := range ents {
		out = append(out, e.FQN.String())
	}
	sort.Strings(out)
	return out
}

// A bare suffix resolves to the package (segment-boundary suffix match).
func TestResolveSuffixMatchesPackage(t *testing.T) {
	ents := fixtureEntities(t)
	got, err := repo.Resolve("git", ents)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got) != 1 || got[0].FQN.String() != "github:rocne/dotfiles::git" {
		t.Errorf("Resolve(git) = %v, want the dotfiles git package", fqnStrings(got))
	}
}

// A leading :: forces package-kind: ::zsh matches only packages named zsh.
func TestResolveKindForcing(t *testing.T) {
	ents := fixtureEntities(t)
	got, err := repo.Resolve("::zsh", ents)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []string{"github:acme/tools::zsh", "github:rocne/dotfiles::zsh"}
	if g := fqnStrings(got); !equal(g, want) {
		t.Errorf("Resolve(::zsh) = %v, want %v", g, want)
	}
}

// A cross-kind bare tie (a name matching both a repo and a package) returns
// BOTH — ambiguity is the caller's to resolve (§1.2).
func TestResolveCrossKindTieReturnsBoth(t *testing.T) {
	ents := fixtureEntities(t)
	got, err := repo.Resolve("tmux", ents)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []string{"github:rocne/box::tmux", "github:rocne/tmux"}
	if g := fqnStrings(got); !equal(g, want) {
		t.Errorf("Resolve(tmux) = %v, want both the repo and the package", g)
	}
}

// A scheme attaches only to the FULL coordinate: a partial coordinate under a
// scheme resolves to nothing.
func TestResolveSchemeRequiresFullCoordinate(t *testing.T) {
	ents := fixtureEntities(t)

	partial, err := repo.Resolve("github:dotfiles", ents)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(partial) != 0 {
		t.Errorf("Resolve(github:dotfiles) = %v, want empty (scheme needs the full coordinate)", fqnStrings(partial))
	}

	full, err := repo.Resolve("github:rocne/dotfiles", ents)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(full) != 1 || full[0].FQN.String() != "github:rocne/dotfiles" {
		t.Errorf("Resolve(github:rocne/dotfiles) = %v, want the repo", fqnStrings(full))
	}
}

// Multi-match is ambiguity data (non-error): the same package name in two repos
// returns two entities.
func TestResolveMultiMatchIsData(t *testing.T) {
	ents := fixtureEntities(t)
	got, err := repo.Resolve("zsh", ents)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []string{"github:acme/tools::zsh", "github:rocne/dotfiles::zsh"}
	if g := fqnStrings(got); !equal(g, want) {
		t.Errorf("Resolve(zsh) = %v, want %v", g, want)
	}
}

// A parse error surfaces (it is not swallowed as "no match").
func TestResolveParseErrorSurfaces(t *testing.T) {
	ents := fixtureEntities(t)
	if _, err := repo.Resolve("foo::", ents); err == nil {
		t.Error("Resolve on a malformed expression returned nil error")
	}
}

// A reserved @-suffix expression matches nothing — empty slice, not an error.
func TestResolveReservedAtSuffixMatchesNothing(t *testing.T) {
	ents := fixtureEntities(t)
	got, err := repo.Resolve("dotfiles@v1", ents)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Resolve(dotfiles@v1) = %v, want empty", fqnStrings(got))
	}
}

// Entities carries repo and package entities; a package entity's FQN has the
// package set and its Repo backreference is the owning repo.
func TestEntitiesBuildsRepoAndPackageEntities(t *testing.T) {
	set := repo.BuildSet([]repo.Source{mustSource(t, "github:rocne/dotfiles")}, nil)
	ents, err := repo.Entities(set, func(repo.Repo) ([]string, error) {
		return []string{"zsh"}, nil
	})
	if err != nil {
		t.Fatalf("Entities: %v", err)
	}
	if len(ents) != 2 {
		t.Fatalf("Entities len = %d, want 2 (one repo + one package)", len(ents))
	}
	var pkg *repo.Entity
	for i := range ents {
		if ents[i].FQN.IsPackage() {
			pkg = &ents[i]
		}
	}
	if pkg == nil {
		t.Fatal("no package entity produced")
	}
	if pkg.FQN.Package != "zsh" {
		t.Errorf("package entity FQN.Package = %q, want zsh", pkg.FQN.Package)
	}
	if pkg.Repo.FQN.String() != "github:rocne/dotfiles" {
		t.Errorf("package entity Repo = %q, want the owning repo", pkg.Repo.FQN.String())
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
