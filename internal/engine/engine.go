// Package engine is dstow's one seam onto gostow (DESIGN.md A14–A16): the
// deployment verbs as per-package operations, and the two introspections
// (Expected, Owner) the ledger and maintenance verbs compose with. gostow's
// types stop here — Conflict, Task, and the fatal class are mapped into
// dstow's typed results in this package and nowhere else, so no other dstow
// package imports gostow's engine.
//
// Standing option law (A14): NoGlobalIgnoreFile is always set (the caller
// owns all configuration; a stray ~/.stow-global-ignore must not silently
// change behavior — stow's built-in default ignores still apply beneath the
// per-package .stow-local-ignore compat file); FixQuirks is always on for
// engine operations; the engine log is discarded — dstow reports from the
// Result, never engine prose. Apply is per-package by construction
// (REQUIREMENTS §3.2 independence: ops loops packages, each succeeding or
// failing alone), while within one package stow's all-or-nothing planning
// holds.
//
// Ignore composition at the seam (A15+A16+M8): compat stow-regex chain
// entries ride gostow's own Options.Ignore; native gitignore-glob entries
// are matched dstow-side by internal/ignore behind Options.IgnoreFunc; and
// the always-on .dstow auto-ignore — package-root-anchored, in no config,
// unsilencable — is composed into the same closure. Apply and Expected build
// options identically, so observation and deployment can never disagree.
package engine

import (
	"fmt"
	"io"

	"github.com/rocne/gostow/stow"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/ignore"
)

// Verb is a deployment verb (CONTEXT.md: stow / unstow / restow — GNU
// stow's own vocabulary). Restow is one atomic unstow+stow plan.
type Verb int

const (
	VerbStow Verb = iota
	VerbUnstow
	VerbRestow
)

func (v Verb) String() string {
	switch v {
	case VerbStow:
		return "stow"
	case VerbUnstow:
		return "unstow"
	case VerbRestow:
		return "restow"
	}
	return fmt.Sprintf("Verb(%d)", int(v))
}

// Op parameterizes one per-package engine operation. Everything here is an
// effective config value or an operand — the engine takes decisions, it
// never reads config.
type Op struct {
	// Dir is the stow directory: the repo root, or its packages directory
	// when the repo sets one. Must exist.
	Dir string
	// Target is where links land. Must exist — auto-creating and announcing
	// a missing target (§3.1) is the caller's job, before the engine runs.
	Target string
	// Package is the package's directory name within Dir.
	Package string
	// Fold enables tree folding (§3.3: off is dstow's default; the value
	// here is the effective global knob).
	Fold bool
	// TranslateDotPrefixes rewrites dot- prefixed names on deployment
	// (§3.4: on is dstow's default).
	TranslateDotPrefixes bool
	// Adopt imports an occupying real file into the package instead of
	// conflicting (stow only).
	Adopt bool
	// Simulate plans and reports without touching the filesystem.
	Simulate bool
	// Ignores is the package's effective additive ignore chain (§3.4), both
	// languages mixed as config.Effective returns it; the engine routes each
	// entry to its lane (A15).
	Ignores []config.IgnorePattern
}

// ActionKind classifies one executed (or, under Simulate, planned)
// filesystem mutation.
type ActionKind int

const (
	LinkCreated ActionKind = iota
	LinkRemoved
	DirCreated
	DirRemoved
	FileMoved
)

func (k ActionKind) String() string {
	switch k {
	case LinkCreated:
		return "link"
	case LinkRemoved:
		return "unlink"
	case DirCreated:
		return "mkdir"
	case DirRemoved:
		return "rmdir"
	case FileMoved:
		return "move"
	}
	return fmt.Sprintf("ActionKind(%d)", int(k))
}

// Action is one filesystem mutation, in plan order. Path is target-relative
// — exactly how the ledger's `link` field and stow's own output name paths.
// Dest carries the literal symlink text for LinkCreated (the ledger's
// `destination` field) and the move destination for FileMoved; it is empty
// otherwise.
type Action struct {
	Kind ActionKind
	Path string
	Dest string
}

// Result reports what an Apply executed (or, under Simulate, would
// execute). An unchanged path yields no action: stow cancels opposing
// remove/create pairs, so a restow of an intact package legitimately
// reports nothing.
type Result struct {
	Actions []Action
}

// ConflictKind classifies what occupies a path the plan needed
// (structurally — gostow's classification, renamed into dstow's types).
type ConflictKind int

const (
	// ConflictExistingFile is a real file (or other non-link, non-directory
	// node) where a link belongs — the adoptable case (§3.5 names adopt as
	// the remedy).
	ConflictExistingFile ConflictKind = iota + 1
	// ConflictDirMismatch is a directory over a non-directory, or the
	// reverse.
	ConflictDirMismatch
	// ConflictForeignLink is an existing symlink owned by no package.
	ConflictForeignLink
	// ConflictOtherPackage is an existing symlink owned by a different
	// package.
	ConflictOtherPackage
	// ConflictSourceAbsolute is a package node that is itself an absolute
	// symlink, which the engine cannot represent as a relative link.
	ConflictSourceAbsolute
)

func (k ConflictKind) String() string {
	switch k {
	case ConflictExistingFile:
		return "existing-file"
	case ConflictDirMismatch:
		return "dir-mismatch"
	case ConflictForeignLink:
		return "foreign-link"
	case ConflictOtherPackage:
		return "other-package"
	case ConflictSourceAbsolute:
		return "source-absolute"
	}
	return fmt.Sprintf("ConflictKind(%d)", int(k))
}

// Conflict is one reason the engine refused to touch the filesystem. Path
// is target-relative; Message is stow's prose, for display; Verb is the
// phase that raised it (informative under VerbRestow, where either phase
// can conflict).
type Conflict struct {
	Verb    Verb
	Path    string
	Kind    ConflictKind
	Message string
}

// ConflictError reports that planning found conflicts and nothing was
// written — stow's all-or-nothing planning, scoped to the one package this
// Apply covered (§3.2).
type ConflictError struct {
	Package   string
	Conflicts []Conflict
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("package %s: %d conflict(s), nothing deployed", e.Package, len(e.Conflicts))
}

// OpError is the engine's fatal class for one per-package operation: a
// missing package, an unreadable tree, a bad directory. The run continues
// past it — per-package independence is the caller's loop; this error
// scopes the failure to its package.
type OpError struct {
	Package string
	Err     error
}

func (e *OpError) Error() string {
	return fmt.Sprintf("package %s: %v", e.Package, e.Err)
}

func (e *OpError) Unwrap() error { return e.Err }

// Apply runs one deployment verb for one package. Conflicts return a
// *ConflictError (typed, nothing written); every other engine failure
// returns an *OpError. Both scope to op.Package.
func Apply(verb Verb, op Op) (*Result, error) {
	opts, err := options(op)
	if err != nil {
		return nil, err
	}
	var action stow.Action
	switch verb {
	case VerbStow:
		action = stow.ActionStow
	case VerbUnstow:
		action = stow.ActionUnstow
	case VerbRestow:
		action = stow.ActionRestow
	default:
		return nil, &OpError{Package: op.Package, Err: fmt.Errorf("unknown verb %d", int(verb))}
	}

	res, err := stow.Apply(opts, stow.Request{Action: action, Packages: []string{op.Package}})
	if err != nil {
		if cerr, ok := err.(*stow.ConflictError); ok {
			return nil, conflictError(op.Package, cerr.Conflicts)
		}
		return nil, &OpError{Package: op.Package, Err: err}
	}

	out := &Result{}
	for _, t := range res.Tasks {
		a, ok := mapTask(t)
		if !ok {
			return nil, &OpError{Package: op.Package, Err: fmt.Errorf("engine planned an unmappable task: %v %v %s", t.Action, t.Type, t.Path)}
		}
		out.Actions = append(out.Actions, a)
	}
	return out, nil
}

// Expected computes the links deploying op now would produce, against an
// empty target: target-relative link → package-relative source, exactly the
// ledger's link/source fields (§6.1). It builds options identically to
// Apply — same ignore closure, same translation — so check, status, and
// deploy can never disagree about what a package means.
func Expected(op Op) (map[string]string, error) {
	opts, err := options(op)
	if err != nil {
		return nil, err
	}
	out, err := stow.Expected(opts, op.Package)
	if err != nil {
		return nil, &OpError{Package: op.Package, Err: err}
	}
	return out, nil
}

// Owner reports which package in dir owns the symlink at path, resolving
// the link's destination by the engine's own pinned rules — never
// re-derived. owned is false for a link pointing anywhere but into dir.
func Owner(dir, path string) (pkg string, owned bool, err error) {
	return stow.Owner(dir, path)
}

// options builds the one gostow Options both Apply and Expected use,
// composing the three additive ignore layers at the seam.
func options(op Op) (stow.Options, error) {
	var globs []config.IgnorePattern
	var compat []string
	for _, p := range op.Ignores {
		switch p.Language {
		case config.LangGlob:
			globs = append(globs, p)
		case config.LangStowRegex:
			compat = append(compat, p.Pattern)
		default:
			return stow.Options{}, &OpError{Package: op.Package, Err: fmt.Errorf("ignore pattern %q from %s has unknown language %v", p.Pattern, p.Source, p.Language)}
		}
	}
	chain, err := ignore.Compile(globs)
	if err != nil {
		return stow.Options{}, err
	}
	return stow.Options{
		Dir:      op.Dir,
		Target:   op.Target,
		Fold:     op.Fold,
		Dotfiles: op.TranslateDotPrefixes,
		Adopt:    op.Adopt,
		Simulate: op.Simulate,
		Ignore:   compat,
		IgnoreFunc: func(rel string, isDir bool) bool {
			// M8: .dstow anchored at the package root only — a deeper
			// .dstow is content. Always on, in no config, unsilencable.
			if rel == ".dstow" {
				return true
			}
			return chain.Match(rel, isDir)
		},
		NoGlobalIgnoreFile: true,
		FixQuirks:          true,
		Log:                io.Discard,
	}, nil
}

func conflictError(pkg string, conflicts []stow.Conflict) *ConflictError {
	out := &ConflictError{Package: pkg}
	for _, c := range conflicts {
		out.Conflicts = append(out.Conflicts, Conflict{
			Verb:    mapVerb(c.Action),
			Path:    c.Path,
			Kind:    mapConflictKind(c.Kind),
			Message: c.Message,
		})
	}
	return out
}

func mapVerb(a stow.Action) Verb {
	switch a {
	case stow.ActionUnstow:
		return VerbUnstow
	case stow.ActionRestow:
		return VerbRestow
	default:
		return VerbStow
	}
}

func mapConflictKind(k stow.ConflictKind) ConflictKind {
	switch k {
	case stow.ConflictExistingFile:
		return ConflictExistingFile
	case stow.ConflictDirMismatch:
		return ConflictDirMismatch
	case stow.ConflictForeignLink:
		return ConflictForeignLink
	case stow.ConflictOtherPackage:
		return ConflictOtherPackage
	case stow.ConflictSourceAbsolute:
		return ConflictSourceAbsolute
	}
	// The engine promises one of the named kinds at every conflict site; the
	// zero value would mean a gostow version this mapping does not know.
	return ConflictKind(0)
}

// mapTask renames one gostow task into dstow's action vocabulary. The
// combinations the planner produces are enumerable; anything else reports
// unmappable so a new gostow behavior surfaces loudly instead of silently
// dropping a mutation the ledger should have seen.
func mapTask(t stow.Task) (Action, bool) {
	switch {
	case t.Action == stow.TaskCreate && t.Type == stow.TypeLink:
		return Action{Kind: LinkCreated, Path: t.Path, Dest: t.Source}, true
	case t.Action == stow.TaskRemove && t.Type == stow.TypeLink:
		return Action{Kind: LinkRemoved, Path: t.Path}, true
	case t.Action == stow.TaskCreate && t.Type == stow.TypeDir:
		return Action{Kind: DirCreated, Path: t.Path}, true
	case t.Action == stow.TaskRemove && t.Type == stow.TypeDir:
		return Action{Kind: DirRemoved, Path: t.Path}, true
	case t.Action == stow.TaskMove && t.Type == stow.TypeFile:
		return Action{Kind: FileMoved, Path: t.Path, Dest: t.Dest}, true
	}
	return Action{}, false
}
