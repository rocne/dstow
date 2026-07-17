package repo

import (
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"

	"github.com/rocne/dstow/internal/name"
)

// ManagedReposRoot is the root of the managed directory where remote-sourced
// repos are cloned (A19): $XDG_DATA_HOME/dstow/repos. It is data, not state and
// not cache — links point into it — so it lives in the XDG data lane.
func ManagedReposRoot() string {
	return filepath.Join(xdg.DataHome, "dstow", "repos")
}

// CloneDir is the managed clone directory of a source (A19):
// <managed>/repos/<scheme>/<encoded-segment>/… . Every segment is
// percent-encoded via the name grammar, so the on-disk path is filesystem-safe
// by construction (github: owner/name yields the documented A19 shape).
func (s Source) CloneDir() string {
	parts := make([]string, 0, len(s.Coordinate)+2)
	parts = append(parts, ManagedReposRoot(), s.Scheme)
	for _, seg := range s.Coordinate {
		parts = append(parts, name.Encode(seg))
	}
	return filepath.Join(parts...)
}

// coordPath joins decoded coordinate segments back into a filesystem path. A
// leading empty segment (the name grammar's absolute-path coordinate marker)
// makes the join absolute, e.g. ["", "home", "x"] -> "/home/x".
func coordPath(coord []string) string {
	return strings.Join(coord, "/")
}

// pathSegments splits a filesystem path into name-grammar coordinate segments,
// the inverse of coordPath: an absolute path keeps its leading empty segment so
// the round-trip is lossless, e.g. "/home/x" -> ["", "home", "x"].
func pathSegments(p string) []string {
	return strings.Split(p, "/")
}
