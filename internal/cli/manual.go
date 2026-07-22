package cli

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow"
)

// This file builds the hidden `manual` group: the tracked docs/ tree mirrored
// as commands, so dstow is fully learnable from the command line alone with no
// external documentation. The tree is generated from the embedded FS at
// startup — adding a topic is a file drop, never a code change.
//
// §2.1's shape rule (group or leaf) is carved out here by ruling: these nodes
// are not actions, they are a mirror of a folder, so a bare node prints its
// markdown rather than its help.

// manualDir is the embedded tree's root path inside dstow.Manual, and
// manualIndex is the file that is both a directory's table of contents and its
// content — running any node prints its index.md, which lists its children.
const (
	manualDir   = "docs"
	manualIndex = "index.md"
	manualExt   = ".md"
)

// newManualCmd builds the manual group over the embedded docs/ tree. Only this
// node is hidden: cobra filters completion candidates through
// IsAvailableCommand, which is false for a hidden command, so `dstow <TAB>`
// omits manual while `dstow manual <TAB>` completes its children normally.
//
// The error is repo-shaped — a malformed docs/ tree (a directory with no
// index.md, a name collision, a non-markdown file) — and cannot arise in a
// built binary, because the same builder runs over the same embedded FS in the
// unit suite, which is the gate. Callers wire the command only on success, the
// posture ui.BundledThemes takes with its own embedded assets.
func (e *env) newManualCmd() (*cobra.Command, error) {
	emit := func(content string) { e.pr().Out().Printf("%s", content) }
	cmd, err := buildManual(dstow.Manual, manualDir, "manual", emit)
	if err != nil {
		return nil, err
	}
	cmd.Hidden = true
	return cmd, nil
}

// buildManual mirrors dir into a command named use, recursively: directories
// become nodes with children, markdown files become leaves, and every node
// prints its own file verbatim through emit. It is a pure function of the
// filesystem it is handed, which is what makes the structural tests possible
// over a synthetic tree.
func buildManual(fsys fs.FS, dir, use string, emit func(string)) (*cobra.Command, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("manual: read %s: %w", dir, err)
	}

	// A directory's own content is its index.md — the one file that is both
	// table of contents and content. Its absence is an error rather than an
	// inferred listing: a node that lists its children without saying anything
	// about them is a worse answer than a loud failure.
	node, err := manualLeaf(fsys, path.Join(dir, manualIndex), use, emit)
	if err != nil {
		return nil, err
	}

	// Directory names are checked against the .md basenames beside them: a
	// format/ and a format.md would give one topic two sources, and the single
	// source of truth is the property this tree is built on.
	dirs := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dirs[entry.Name()] = true
		}
	}

	// fs.ReadDir returns entries sorted by filename, so children are added in
	// alphabetical order — the ordering cobra would apply itself if dstow had
	// not disabled command sorting for §2.3's curated root listing. Reading
	// order within a topic is index.md's job, not the tree's.
	for _, entry := range entries {
		name := entry.Name()
		child := path.Join(dir, name)

		if entry.IsDir() {
			sub, err := buildManual(fsys, child, name, emit)
			if err != nil {
				return nil, err
			}
			node.AddCommand(sub)
			continue
		}

		if name == manualIndex {
			continue
		}
		if !strings.HasSuffix(name, manualExt) {
			return nil, fmt.Errorf("manual: %s is not markdown (the tree is markdown only)", child)
		}
		stem := strings.TrimSuffix(name, manualExt)
		if dirs[stem] {
			return nil, fmt.Errorf("manual: %s collides with the directory %s (a topic has one source)", child, path.Join(dir, stem))
		}
		leaf, err := manualLeaf(fsys, child, stem, emit)
		if err != nil {
			return nil, err
		}
		node.AddCommand(leaf)
	}
	return node, nil
}

// manualLeaf builds one node over one markdown file. Filenames are command
// spellings exactly — identity mapping, no transform — so the file a command
// came from is always readable off the command line.
func manualLeaf(fsys fs.FS, file, use string, emit func(string)) (*cobra.Command, error) {
	content, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, fmt.Errorf("manual: read %s: %w", file, err)
	}
	text := string(content)
	return &cobra.Command{
		Use:   use,
		Short: manualShort(text),
		Args:  cobra.NoArgs,
		// The node prints its markdown raw on stdout: no styling, no transform,
		// no rendering pass. "The command prints the file" is the property, and
		// the output is the requested data (A2), so it goes to stdout at any
		// verbosity.
		RunE: func(cmd *cobra.Command, args []string) error {
			emit(text)
			return nil
		},
	}, nil
}

// manualShort is a node's one-line description: the file's first H1, which is
// what `dstow manual <TAB>` shows beside each candidate. A file with no H1
// yields an empty Short — a content defect the structural suite catches rather
// than a build failure, since the tree's shape is still sound without it.
func manualShort(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "# "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}
