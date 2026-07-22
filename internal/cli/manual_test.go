package cli

import (
	"io/fs"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow"
)

// The manual tree is asserted structurally, never against prose: the ticket's
// property is that the mechanism is correct independently of content, so these
// tests assert the shape of the mapping — every file reaches exactly one
// command and every command came from exactly one file — and never a heading,
// a position, or a body string.

// TestManualMirrorsEmbeddedTree walks the real embedded docs/ and asserts the
// generated tree is a faithful mirror of it: one command per markdown file,
// one file per command, no empty nodes.
func TestManualMirrorsEmbeddedTree(t *testing.T) {
	emitted := ""
	root, err := buildManual(dstow.Manual, manualDir, "manual", func(s string) { emitted = s })
	if err != nil {
		t.Fatalf("build manual over the embedded docs/: %v", err)
	}

	// Every markdown file in the tree, as the command path it must be reachable
	// at: docs/index.md is `manual`, docs/foo.md is `manual foo`, and
	// docs/foo/index.md is `manual foo`.
	wantPaths := map[string]bool{}
	err = fs.WalkDir(dstow.Manual, manualDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, manualExt) {
			t.Errorf("non-markdown file in the manual tree: %s", p)
			return nil
		}
		rel := strings.TrimPrefix(p, manualDir+"/")
		rel = strings.TrimSuffix(rel, manualExt)
		rel = strings.TrimSuffix(strings.TrimSuffix(rel, "index"), "/")
		wantPaths[strings.TrimSpace("manual "+strings.ReplaceAll(rel, "/", " "))] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded docs/: %v", err)
	}

	gotPaths := map[string]bool{}
	walkCommands(root, "", func(cmd *cobra.Command, cmdPath string) {
		if gotPaths[cmdPath] {
			t.Errorf("two commands share the path %q", cmdPath)
		}
		gotPaths[cmdPath] = true
		if cmd.Short == "" {
			t.Errorf("%q has no Short — its file carries no H1, so completion describes it as nothing", cmdPath)
		}
		if cmd.RunE == nil {
			t.Errorf("%q is an empty node: running it prints nothing", cmdPath)
		}
	})

	for p := range wantPaths {
		if !gotPaths[p] {
			t.Errorf("embedded file has no command: %q", p)
		}
	}
	for p := range gotPaths {
		if !wantPaths[p] {
			t.Errorf("command has no embedded file: %q", p)
		}
	}

	// Every directory in the tree carries the index.md that is its content.
	err = fs.WalkDir(dstow.Manual, manualDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if _, err := fs.Stat(dstow.Manual, path.Join(p, manualIndex)); err != nil {
			t.Errorf("directory %s has no %s", p, manualIndex)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded docs/: %v", err)
	}

	// Running a node prints its file verbatim — no styling, no transform.
	if err := root.RunE(root, nil); err != nil {
		t.Fatalf("run manual: %v", err)
	}
	index, err := fs.ReadFile(dstow.Manual, path.Join(manualDir, manualIndex))
	if err != nil {
		t.Fatalf("read embedded index: %v", err)
	}
	if emitted != string(index) {
		t.Errorf("manual printed %q, want its index.md verbatim %q", emitted, string(index))
	}
}

// TestManualTreeShape drives the builder over a synthetic tree richer than the
// one that ships today: the mechanism must be correct at depth, which is the
// whole point of shipping it before any content exists.
func TestManualTreeShape(t *testing.T) {
	fsys := fstest.MapFS{
		"docs/index.md":               {Data: []byte("# dstow manual\n")},
		"docs/naming.md":              {Data: []byte("# Naming\n\nbody\n")},
		"docs/commands/index.md":      {Data: []byte("# Commands\n")},
		"docs/commands/stow.md":       {Data: []byte("# stow\n")},
		"docs/commands/repo/index.md": {Data: []byte("# repo\n")},
		"docs/commands/repo/add.md":   {Data: []byte("# repo add\n")},
	}
	var emitted string
	root, err := buildManual(fsys, "docs", "manual", func(s string) { emitted = s })
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	want := map[string]string{
		"manual":                   "dstow manual",
		"manual commands":          "Commands",
		"manual commands repo":     "repo",
		"manual commands repo add": "repo add",
		"manual commands stow":     "stow",
		"manual naming":            "Naming",
	}
	got := map[string]string{}
	walkCommands(root, "", func(cmd *cobra.Command, cmdPath string) { got[cmdPath] = cmd.Short })
	if len(got) != len(want) {
		t.Errorf("tree has %d nodes, want %d: %v", len(got), len(want), got)
	}
	for p, short := range want {
		if got[p] != short {
			t.Errorf("%q Short = %q, want %q (the file's first H1)", p, got[p], short)
		}
	}

	// Children are alphabetical: dstow disables cobra's command sorting for
	// §2.3's curated root listing, so the order the walk adds them in is the
	// order help and completion show.
	commands := root.Commands()
	names := make([]string, 0, len(commands))
	for _, c := range commands {
		names = append(names, c.Name())
	}
	if strings.Join(names, " ") != "commands naming" {
		t.Errorf("children in order %v, want alphabetical [commands naming]", names)
	}

	// A leaf takes no operands, and a node prints its own file.
	repoAdd, _, err := root.Find([]string{"commands", "repo", "add"})
	if err != nil {
		t.Fatalf("find manual commands repo add: %v", err)
	}
	if err := repoAdd.Args(repoAdd, []string{"stray"}); err == nil {
		t.Error("a manual node accepted an operand; nodes take none")
	}
	if err := repoAdd.RunE(repoAdd, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if emitted != "# repo add\n" {
		t.Errorf("printed %q, want the file verbatim", emitted)
	}
}

// TestManualTreeErrors asserts the three malformed-tree cases are errors
// rather than a quietly wrong surface. Each is a repo defect this suite exists
// to catch before it can ship.
func TestManualTreeErrors(t *testing.T) {
	cases := map[string]fstest.MapFS{
		"directory without index.md": {
			"docs/index.md":         {Data: []byte("# dstow manual\n")},
			"docs/commands/stow.md": {Data: []byte("# stow\n")},
		},
		"file colliding with a directory": {
			"docs/index.md":          {Data: []byte("# dstow manual\n")},
			"docs/commands.md":       {Data: []byte("# Commands\n")},
			"docs/commands/index.md": {Data: []byte("# Commands\n")},
		},
		"non-markdown file": {
			"docs/index.md":    {Data: []byte("# dstow manual\n")},
			"docs/diagram.png": {Data: []byte("\x89PNG")},
		},
		"no index.md at the root": {
			"docs/naming.md": {Data: []byte("# Naming\n")},
		},
	}
	for name, fsys := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := buildManual(fsys, "docs", "manual", func(string) {}); err == nil {
				t.Errorf("built a tree from a %s without error", name)
			}
		})
	}
}

// TestManualHiddenEntryPoint asserts the discovery contract: the group is
// hidden so `dstow <TAB>` does not offer it, its children are not, so
// `dstow manual <TAB>` completes them, and root help carries the one footer
// line that is the entire discovery affordance.
func TestManualHiddenEntryPoint(t *testing.T) {
	isolateXDG(t)

	e := &env{}
	manual, err := e.newManualCmd()
	if err != nil {
		t.Fatalf("build manual: %v", err)
	}
	if !manual.Hidden {
		t.Error("the manual group is not hidden; dstow <TAB> would offer it")
	}
	walkCommands(manual, "", func(cmd *cobra.Command, cmdPath string) {
		if cmd != manual && cmd.Hidden {
			t.Errorf("%q is hidden; cobra filters hidden children out of completion", cmdPath)
		}
	})

	// The footer line is load-bearing: it is the only path from root help to
	// the manual, so its absence would leave the tree unreachable in practice.
	out, _, code := run(t, "--help")
	if code != 0 {
		t.Fatalf("--help exit = %d", code)
	}
	if !strings.Contains(normWS(out), "Run 'dstow manual' for the full documentation.") {
		t.Error("root help carries no pointer at the manual")
	}

	// Completion is the other half: cobra filters candidates through
	// IsAvailableCommand, false for a hidden command.
	rootComp, _, _ := run(t, cobra.ShellCompRequestCmd, "")
	for _, line := range strings.Split(rootComp, "\n") {
		if strings.HasPrefix(line, "manual") {
			t.Errorf("dstow <TAB> offers the manual: %q", line)
		}
	}

	// The manual itself prints its index rather than its help (the §2.1
	// carve-out): running a node is how the tree is navigated.
	body, stderr, code := run(t, "manual")
	if code != 0 {
		t.Fatalf("dstow manual exit = %d", code)
	}
	if stderr != "" {
		t.Errorf("dstow manual wrote to stderr: %q", stderr)
	}
	index, err := fs.ReadFile(dstow.Manual, path.Join(manualDir, manualIndex))
	if err != nil {
		t.Fatalf("read embedded index: %v", err)
	}
	if body != string(index) {
		t.Errorf("dstow manual printed %q, want docs/index.md verbatim", body)
	}
	if strings.Contains(body, "Usage:") {
		t.Error("dstow manual printed help; a bare node prints its index.md (§2.1 carve-out)")
	}
}

// walkCommands visits cmd and every descendant, passing each one's full command
// path ("manual commands repo add").
func walkCommands(cmd *cobra.Command, prefix string, visit func(*cobra.Command, string)) {
	cmdPath := strings.TrimSpace(prefix + " " + cmd.Name())
	visit(cmd, cmdPath)
	for _, child := range cmd.Commands() {
		walkCommands(child, cmdPath, visit)
	}
}
