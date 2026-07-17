// Package hooks is dstow's hook engine (DESIGN.md §5 + A11): discovery of the
// eight per-event executables in one hooks directory, the DSTOW_HOOK_*
// environment contract each hook runs under, direct exec of a hook, and the
// per-invocation sequencer that fires the nested/LIFO lifecycle
// (REQUIREMENTS §9.1) once per scope.
//
// The package returns data, never output (A4): diagnostics come back as
// Warning values and failures as typed errors; the caller (ui) decides how to
// render them, and only ui touches process streams. The package never
// references os.Stdout/os.Stderr — the Runner's streams are injected — and it
// reads os.Getenv/os.Environ only at the point of use (A2): once when building
// a hook's inherited environment, and once in InHook.
//
// hooks owns ordering and once-per-invocation firing (A11); ops owns the
// iteration loop. The no-op contract (§9.1.5) is the CALLER's: ops invokes the
// Invocation methods only for scopes that actually change something, and the
// once-per-invocation firing then makes repeated bulk runs quiet and
// idempotent. hooks does not second-guess the caller — a hook file that simply
// does not exist is a silent no-op (hooks are optional).
package hooks

import (
	"fmt"
	"os"
)

// Action is one of the four lifecycle actions (§5 H2). Restow is its own pair,
// not a stow+unstow composition: M6 pins "Restow fires only the restow pair",
// so the caller passes ActionRestow and the restow hooks fire, nothing else.
type Action int

const (
	ActionStow Action = iota
	ActionUnstow
	ActionRestow
	ActionAdopt
)

// String yields the DSTOW_HOOK_ACTION spelling (H2).
func (a Action) String() string {
	switch a {
	case ActionStow:
		return "stow"
	case ActionUnstow:
		return "unstow"
	case ActionRestow:
		return "restow"
	case ActionAdopt:
		return "adopt"
	}
	return fmt.Sprintf("Action(%d)", int(a))
}

// Phase distinguishes the pre and post firing around an action (§9.1.1).
type Phase int

const (
	PhasePre Phase = iota
	PhasePost
)

// String yields the DSTOW_HOOK_PHASE spelling (H2).
func (p Phase) String() string {
	switch p {
	case PhasePre:
		return "pre"
	case PhasePost:
		return "post"
	}
	return fmt.Sprintf("Phase(%d)", int(p))
}

// Level identifies which of the three hook scopes is firing (§9.1.2).
type Level int

const (
	LevelPackage Level = iota
	LevelRepo
	LevelGlobal
)

// String yields the DSTOW_HOOK_LEVEL spelling (H2).
func (l Level) String() string {
	switch l {
	case LevelPackage:
		return "package"
	case LevelRepo:
		return "repo"
	case LevelGlobal:
		return "global"
	}
	return fmt.Sprintf("Level(%d)", int(l))
}

// Hook identifies one of the eight lifecycle hooks: a (Phase, Action) pair
// (M6). It is the key of a Set.
type Hook struct {
	Phase  Phase
	Action Action
}

// FileName is the on-disk hook file name, Phase.String()+"-"+Action.String()
// (M6) — one of {pre,post}-{stow,unstow,restow,adopt}.
func (h Hook) FileName() string {
	return h.Phase.String() + "-" + h.Action.String()
}

// allPhases and allActions enumerate the eight hooks; discovery and did-you-mean
// iterate them in this fixed order, so results are deterministic.
var (
	allPhases  = []Phase{PhasePre, PhasePost}
	allActions = []Action{ActionStow, ActionUnstow, ActionRestow, ActionAdopt}
)

// validHooks maps each of the eight canonical file names to its Hook.
var validHooks = func() map[string]Hook {
	m := make(map[string]Hook, len(allPhases)*len(allActions))
	for _, p := range allPhases {
		for _, a := range allActions {
			h := Hook{Phase: p, Action: a}
			m[h.FileName()] = h
		}
	}
	return m
}()

// InHook reports whether this process is running inside a dstow hook — the
// entire H7 surface in this package. Detection is DSTOW_HOOK_ACTION present in
// the environment (H7), read at the point of use (A2). The actual write
// refusals live in the write commands (later tickets); this predicate only
// answers the question.
func InHook() bool {
	return os.Getenv(EnvAction) != ""
}
