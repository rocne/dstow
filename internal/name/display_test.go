package name

import (
	"strings"
	"testing"
)

// pkg builds a package FQN from a github-style scheme:owner/name::package.
func gh(owner, repo, pkg string) FQN {
	return FQN{Scheme: "github", Coordinate: []string{owner, repo}, Package: pkg}
}

func ghRepo(owner, repo string) FQN {
	return FQN{Scheme: "github", Coordinate: []string{owner, repo}}
}

func TestShortestUnique_LoneNameIsBare(t *testing.T) {
	// A package with no same-named neighbor shows its bare name (O9 default).
	got := ShortestUnique([]FQN{gh("rocne", "dotfiles", "zsh")})
	if got[0] != "zsh" {
		t.Fatalf("lone package: got %q, want %q", got[0], "zsh")
	}
}

func TestShortestUnique_SharedNameQualifies(t *testing.T) {
	// Two packages sharing a bare name climb to the shortest distinguishing
	// coordinate tail (REQUIREMENTS §7.1: same-named entries shown qualified).
	fqns := []FQN{
		gh("rocne", "dotfiles", "zsh"),
		gh("rocne", "work", "zsh"),
	}
	got := ShortestUnique(fqns)
	want := []string{"dotfiles::zsh", "work::zsh"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("shared name [%d]: got %q, want %q (all: %v)", i, got[i], want[i], got)
		}
	}
	// Each spelling must resolve back to exactly its own FQN.
	for i, s := range got {
		assertResolvesTo(t, s, fqns, i)
	}
}

func TestShortestUnique_SharedTailClimbsFurther(t *testing.T) {
	// Same package name AND same last coordinate segment: the tail must grow
	// past the collision.
	fqns := []FQN{
		gh("alice", "dotfiles", "zsh"),
		gh("bob", "dotfiles", "zsh"),
	}
	got := ShortestUnique(fqns)
	want := []string{"alice/dotfiles::zsh", "bob/dotfiles::zsh"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("shared tail [%d]: got %q, want %q (all: %v)", i, got[i], want[i], got)
		}
	}
	for i, s := range got {
		assertResolvesTo(t, s, fqns, i)
	}
}

func TestShortestUnique_FullFQNWhenCoordinatesTie(t *testing.T) {
	// Identical coordinates but different schemes cannot be told apart by any
	// suffix — only the full FQN carries the scheme (§1.1: a scheme attaches
	// to the full coordinate).
	local := FQN{Scheme: "local", Coordinate: []string{"", "home", "x", "dotfiles"}, Package: "zsh"}
	remote := FQN{Scheme: "github", Coordinate: []string{"", "home", "x", "dotfiles"}, Package: "zsh"}
	fqns := []FQN{local, remote}
	got := ShortestUnique(fqns)
	if got[0] != local.String() || got[1] != remote.String() {
		t.Fatalf("scheme tie: got %v, want full FQNs %q / %q", got, local.String(), remote.String())
	}
}

func TestShortestUnique_RepoEntities(t *testing.T) {
	// Repo entities climb coordinate tails, no package tail.
	fqns := []FQN{ghRepo("rocne", "dotfiles"), ghRepo("rocne", "work")}
	got := ShortestUnique(fqns)
	want := []string{"dotfiles", "work"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("repo [%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestShortestUnique_CrossKindBareCollision(t *testing.T) {
	// A repo whose last coordinate segment equals a package's bare name: the
	// bare form is ambiguous across kinds (§1.1), so the package must qualify.
	pkg := gh("rocne", "dotfiles", "zsh")
	repo := ghRepo("acme", "zsh")
	fqns := []FQN{pkg, repo}
	got := ShortestUnique(fqns)
	// The package cannot be a bare "zsh" (the repo tail "zsh" also matches it
	// under the cross-kind rule); it climbs to a qualified form.
	if got[0] == "zsh" {
		t.Fatalf("package should not keep the ambiguous bare name: got %v", got)
	}
	assertResolvesTo(t, got[0], fqns, 0)
	assertResolvesTo(t, got[1], fqns, 1)
}

// assertResolvesTo parses spelling as an expression and asserts it matches
// exactly the FQN at index want in set.
func assertResolvesTo(t *testing.T, spelling string, set []FQN, want int) {
	t.Helper()
	expr, err := ParseExpr(spelling)
	if err != nil {
		t.Fatalf("spelling %q does not parse: %v", spelling, err)
	}
	var matched []int
	for i, f := range set {
		if expr.Matches(f) {
			matched = append(matched, i)
		}
	}
	if len(matched) != 1 || matched[0] != want {
		t.Fatalf("spelling %q resolves to %v, want exactly [%d]", spelling, matched, want)
	}
}

func TestShortestUnique_ParallelAndCanonical(t *testing.T) {
	// The result is parallel to input and every spelling round-trips through
	// the grammar (canonical encoding).
	fqns := []FQN{gh("rocne", "dot files", "z:sh")}
	got := ShortestUnique(fqns)
	if len(got) != 1 {
		t.Fatalf("result length %d, want 1", len(got))
	}
	if strings.Contains(got[0], " ") {
		t.Fatalf("spelling not canonical: %q", got[0])
	}
	assertResolvesTo(t, got[0], fqns, 0)
}
