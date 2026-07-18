package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// Dynamic completion (A20): complete package/repo names and schemes through the
// same repo resolver, best-effort-silent — any error yields no completions and
// never a diagnostic — with config loaded quietly and nothing that fires hooks
// or touches the network. Package enumeration is a filesystem read (repo
// ReadDir), which qualifies.

// completeNames completes package and repo name expressions.
func (e *env) completeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeEntities(toComplete, false), cobra.ShellCompDirectiveNoFileComp
}

// completeRepos completes repo name expressions only (for repo remove / update
// / upgrade, which act on repos).
func (e *env) completeRepos(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeEntities(toComplete, true), cobra.ShellCompDirectiveNoFileComp
}

// completeEntities builds the completion candidates: the shortest-unique
// display spelling of every repo (and, unless reposOnly, every package), plus
// the two scheme prefixes, filtered by the typed prefix. It recovers from any
// panic and returns nothing on trouble — completion must never be noisy (A20).
func completeEntities(toComplete string, reposOnly bool) (out []string) {
	defer func() {
		if r := recover(); r != nil {
			out = nil
		}
	}()

	global, _, _ := config.LoadGlobal()
	reg, _, _ := repo.LoadRegistry(config.RegistryFile())
	sessionDirs, _, _ := config.ParseDSTOWPath(os.Getenv("DSTOW_PATH"))
	if global != nil && global.SessionRepoDir() != "" {
		sessionDirs = append(sessionDirs, global.SessionRepoDir())
	}
	repos := repo.BuildSet(reg.Sources, sessionDirs)

	var fqns []name.FQN
	isRepo := map[string]bool{}
	for _, r := range repos {
		fqns = append(fqns, r.FQN)
		isRepo[r.FQN.String()] = true
		if reposOnly {
			continue
		}
		lvl, _, _ := config.LoadRepoLevel(r.Root)
		pkgs, _, perr := r.Packages(lvl.PackagesDir())
		if perr != nil {
			continue
		}
		for _, p := range pkgs {
			fqns = append(fqns, name.FQN{Scheme: r.FQN.Scheme, Coordinate: r.FQN.Coordinate, Package: p})
		}
	}

	display := name.ShortestUnique(fqns)
	candidates := make([]string, 0, len(display)+2)
	for i, f := range fqns {
		if reposOnly && isRepo[f.String()] && f.IsPackage() {
			continue
		}
		candidates = append(candidates, display[i])
	}
	// Schemes are valid name-expression prefixes (§1.1).
	candidates = append(candidates, "github:", "local:")

	seen := map[string]bool{}
	for _, c := range candidates {
		if seen[c] || !strings.HasPrefix(c, toComplete) {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}
