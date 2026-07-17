package hooks

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dstow/internal/name"
)

// The twelve DSTOW_HOOK_* variable names (H2). Each is a named constant so no
// other file spells a variable name as a literal.
const (
	EnvLevel      = "DSTOW_HOOK_LEVEL"
	EnvAction     = "DSTOW_HOOK_ACTION"
	EnvPhase      = "DSTOW_HOOK_PHASE"
	EnvFQN        = "DSTOW_HOOK_FQN"
	EnvScheme     = "DSTOW_HOOK_SCHEME"
	EnvCoordinate = "DSTOW_HOOK_COORDINATE"
	EnvPackage    = "DSTOW_HOOK_PACKAGE"
	EnvPackageDir = "DSTOW_HOOK_PACKAGE_DIR"
	EnvTarget     = "DSTOW_HOOK_TARGET"
	EnvRepoFQN    = "DSTOW_HOOK_REPO_FQN"
	EnvRepoDir    = "DSTOW_HOOK_REPO_DIR"
	EnvPackages   = "DSTOW_HOOK_PACKAGES"
)

// managedEnv is the set of variable names hooks controls (H2). The inherited
// environment is stripped of these before the per-level set is layered on, so
// absent-not-empty holds regardless of what the parent process carried: a
// variable either appears with a value or is not set at all.
var managedEnv = map[string]bool{
	EnvLevel: true, EnvAction: true, EnvPhase: true, EnvFQN: true,
	EnvScheme: true, EnvCoordinate: true, EnvPackage: true, EnvPackageDir: true,
	EnvTarget: true, EnvRepoFQN: true, EnvRepoDir: true, EnvPackages: true,
}

// PackageScope carries a package-level firing's context (H2). Every field is
// caller-supplied; hooks derives the env values from it. Dir is the package
// directory and also the hook cwd (H5). The repo's FQN is not a field: a
// package FQN's repo is FQN.Repo() by construction, so carrying it separately
// could only ever disagree.
type PackageScope struct {
	FQN     name.FQN // the package FQN
	Dir     string   // absolute package dir; also the hook cwd (H5)
	Target  string   // effective target root, absolute
	RepoDir string   // absolute repo dir
}

// RepoScope carries a repo-level firing's context (H2). Dir is the repo
// directory and also the hook cwd (H5); Packages are all packages acting under
// this repo this invocation, for DSTOW_HOOK_PACKAGES.
type RepoScope struct {
	FQN      name.FQN
	Dir      string     // absolute repo dir; also the hook cwd (H5)
	Packages []name.FQN // all packages acting under this repo
}

// GlobalScope carries a global-level firing's context (H2). Dir is the global
// config dir ($XDG_CONFIG_HOME/dstow in production, supplied by the caller for
// testability); the hooks directory is Dir/hooks and the hook cwd is Dir (H5).
// Packages are all packages acting this invocation, for DSTOW_HOOK_PACKAGES.
type GlobalScope struct {
	Dir      string     // global config dir; hooks dir = Dir/hooks; also the hook cwd (H5)
	Packages []name.FQN // all packages acting this invocation
}

// hooksDir is the global level's hooks directory, Dir/hooks (M6) — not a
// .dstow subtree; the global config dir is dstow's namespace directly.
func (g GlobalScope) hooksDir() string {
	return filepath.Join(g.Dir, "hooks")
}

// kv is one KEY=VALUE pair, kept ordered so a composed environment is
// deterministic.
type kv struct{ key, value string }

// packageVars is the package level's DSTOW_HOOK_* set (H2): the scope's own
// FQN encoded, its decomposed segments decoded, the package/target/repo
// context. No DSTOW_HOOK_PACKAGES at this level.
func packageVars(action Action, phase Phase, s PackageScope) []kv {
	return []kv{
		{EnvLevel, LevelPackage.String()},
		{EnvAction, action.String()},
		{EnvPhase, phase.String()},
		{EnvFQN, s.FQN.String()},
		{EnvScheme, s.FQN.Scheme},
		{EnvCoordinate, strings.Join(s.FQN.Coordinate, "/")},
		{EnvPackage, s.FQN.Package},
		{EnvPackageDir, s.Dir},
		{EnvTarget, s.Target},
		{EnvRepoFQN, s.FQN.Repo().String()},
		{EnvRepoDir, s.RepoDir},
	}
}

// repoVars is the repo level's DSTOW_HOOK_* set (H2): the repo's own FQN
// encoded (DSTOW_HOOK_FQN and DSTOW_HOOK_REPO_FQN are the same value here), its
// decomposed segments decoded, and DSTOW_HOOK_PACKAGES. No package/target
// variables at this level.
func repoVars(action Action, phase Phase, fqn name.FQN, dir string, packages []name.FQN) []kv {
	return []kv{
		{EnvLevel, LevelRepo.String()},
		{EnvAction, action.String()},
		{EnvPhase, phase.String()},
		{EnvFQN, fqn.String()},
		{EnvScheme, fqn.Scheme},
		{EnvCoordinate, strings.Join(fqn.Coordinate, "/")},
		{EnvRepoFQN, fqn.String()},
		{EnvRepoDir, dir},
		{EnvPackages, joinPackages(packages)},
	}
}

// globalVars is the global level's DSTOW_HOOK_* set (H2): only
// LEVEL/ACTION/PHASE plus DSTOW_HOOK_PACKAGES.
func globalVars(action Action, phase Phase, packages []name.FQN) []kv {
	return []kv{
		{EnvLevel, LevelGlobal.String()},
		{EnvAction, action.String()},
		{EnvPhase, phase.String()},
		{EnvPackages, joinPackages(packages)},
	}
}

// joinPackages renders DSTOW_HOOK_PACKAGES (H4): one canonical encoded FQN per
// line, newline-separated, no trailing newline. Canonical encoding of control
// bytes (including a literal newline in a name) makes one-per-line airtight.
func joinPackages(packages []name.FQN) string {
	lines := make([]string, len(packages))
	for i, p := range packages {
		lines[i] = p.String()
	}
	return strings.Join(lines, "\n")
}

// environ composes a hook's process environment (H2): the inherited
// environment (read at the point of use, A2) stripped of every managed
// DSTOW_HOOK_* name, then the per-level set layered on. Stripping enforces
// absent-not-empty against a possibly-polluted parent environment.
func environ(vars []kv) []string {
	inherited := os.Environ()
	out := make([]string, 0, len(inherited)+len(vars))
	for _, e := range inherited {
		key := e
		if i := strings.IndexByte(e, '='); i >= 0 {
			key = e[:i]
		}
		if managedEnv[key] {
			continue
		}
		out = append(out, e)
	}
	for _, v := range vars {
		out = append(out, v.key+"="+v.value)
	}
	return out
}
