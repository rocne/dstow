package repo

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rocne/dstow/internal/name"
)

// Repo is one member of the repo set (REQUIREMENTS §2.3): its FQN, the
// directory that holds its packages, and whether it is a managed clone and/or a
// session repo. The set is unordered — these flags carry no priority.
type Repo struct {
	FQN     name.FQN
	Root    string // the directory whose visible subdirectories are packages
	Managed bool   // a clone dstow owns under the managed directory
	Session bool   // contributed by DSTOW_PATH for this shell only
}

// BuildSet assembles the repo set from registered sources and session
// directories (REQUIREMENTS §2.3, unordered). A github source becomes a managed
// repo rooted at its clone directory; a local source and each session directory
// become local repos rooted at their path (session repos additionally marked).
// Identity is the exact FQN: a repo both registered and present in DSTOW_PATH is
// one repo — the first wins, as they are identical by construction. There is no
// other deduplication and no ordering semantics.
func BuildSet(registered []Source, sessionDirs []string) []Repo {
	var repos []Repo
	seen := map[string]bool{}
	add := func(r Repo) {
		key := r.FQN.String()
		if seen[key] {
			return
		}
		seen[key] = true
		repos = append(repos, r)
	}

	for _, s := range registered {
		switch s.Scheme {
		case "github":
			add(Repo{
				FQN:     name.FQN{Scheme: "github", Coordinate: s.Coordinate},
				Root:    s.CloneDir(),
				Managed: true,
			})
		case "local":
			add(Repo{
				FQN:  name.FQN{Scheme: "local", Coordinate: s.Coordinate},
				Root: coordPath(s.Coordinate),
			})
		}
	}

	for _, dir := range sessionDirs {
		add(Repo{
			FQN:     name.FQN{Scheme: "local", Coordinate: pathSegments(dir)},
			Root:    dir,
			Session: true,
		})
	}

	return repos
}

// Packages enumerates the repo's packages (M2/M3). packagesDir is the repo-level
// config value the caller passes in (config owns reading config). In root mode
// (""), the visible directories directly under Root are the packages; hidden
// (dot-prefixed) directories are skipped silently and non-directories are
// ignored. In scoped mode, the entries live under Root/packagesDir; every
// visible directory is definitionally a package, hidden directories are skipped
// LOUDLY (one Warning each, M2), and non-directories are ignored silently. A
// missing Root or missing packages directory is an error naming the path.
// Symlinks are transparent (ruled 2026-07-17 on #41): a symlink to a
// directory is a package like any other directory; a broken symlink is
// skipped loudly in scoped mode, silently in root mode. Output is sorted.
func (r Repo) Packages(packagesDir string) ([]string, []Warning, error) {
	scoped := packagesDir != ""
	dir := r.Root
	if scoped {
		dir = filepath.Join(r.Root, packagesDir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if scoped {
			return nil, nil, fmt.Errorf("repo %s: cannot read packages directory %s: %w", r.FQN, dir, err)
		}
		return nil, nil, fmt.Errorf("repo %s: cannot read repo root %s: %w", r.FQN, dir, err)
	}

	var (
		pkgs  []string
		warns []Warning
	)
	for _, entry := range entries {
		entryName := entry.Name()
		isDir := entry.IsDir()
		if !isDir && entry.Type()&fs.ModeSymlink != 0 {
			// Symlinks are transparent (ruled 2026-07-17 on #41): a symlink to
			// a directory is a visible directory and enumerates as a package,
			// silently — transparency means it is no surprise. A symlink to a
			// file is transparently a file (ignored silently, both modes). A
			// broken symlink is skipped: loudly in scoped mode, where every
			// visible entry is definitionally a package, so one that cannot
			// resolve deserves the announcement; silently in root mode.
			info, serr := os.Stat(filepath.Join(dir, entryName))
			switch {
			case serr == nil && info.IsDir():
				isDir = true
			case serr == nil:
				continue
			default:
				if scoped && !strings.HasPrefix(entryName, ".") {
					warns = append(warns, Warning{
						Source: dir,
						Detail: fmt.Sprintf("symlink %q does not resolve and was skipped; with a packages directory set, every visible directory is a package, and this entry cannot be read as one", entryName),
					})
				}
				continue
			}
		}
		if !isDir {
			continue // non-directories are ignored in both modes
		}
		if strings.HasPrefix(entryName, ".") {
			if scoped {
				// M2: with packages_dir set, every visible directory is a
				// package, so a hidden entry here is skipped loudly.
				warns = append(warns, Warning{
					Source: dir,
					Detail: fmt.Sprintf("hidden directory %q is not a package and was skipped; with a packages directory set, every visible directory is a package (M2)", entryName),
				})
			}
			continue // root mode skips hidden directories silently
		}
		pkgs = append(pkgs, entryName)
	}
	sort.Strings(pkgs)
	return pkgs, warns, nil
}
