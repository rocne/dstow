package hooks_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rocne/dstow/internal/name"
)

// writeExec writes an executable POSIX-sh script at path (creating parents),
// with a shebang and mode 0755. CI has no Windows (linux x2 + macOS), so real
// scripts with exec bits run.
func writeExec(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// writeFile writes a non-executable file at path (creating parents), mode 0644.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mkdir makes a directory (and parents) at path.
func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// symlink creates newname → oldname, making newname's parent first.
func symlink(oldname, newname string) error {
	if err := os.MkdirAll(filepath.Dir(newname), 0o755); err != nil {
		return err
	}
	return os.Symlink(oldname, newname)
}

// pkgFQN builds a package FQN with decoded fields, e.g.
// github:rocne/dotfiles::zsh.
func pkgFQN(scheme string, coord []string, pkg string) name.FQN {
	return name.FQN{Scheme: scheme, Coordinate: coord, Package: pkg}
}

// repoFQN builds a repo FQN (no ::package tail).
func repoFQN(scheme string, coord []string) name.FQN {
	return name.FQN{Scheme: scheme, Coordinate: coord}
}
