package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/ui"
)

// renderWarnings prints diagnostics-as-data (A4) to stderr: one warning: line
// per warning (surviving --quiet, O7), followed by a fix: line where the
// warning names a remedy. cli is where warnings become output — every package
// returns them as values.
func (e *env) renderWarnings(ws []ops.Warning) {
	pr := e.pr()
	for _, w := range ws {
		pr.Warningf("%s", w.Detail)
		if w.Fix != "" {
			pr.Fixf("%s", w.Fix)
		}
	}
}

// stateRole maps a package state to its rendering role (O3: only ui names
// colors; callers name roles).
func stateRole(s ops.PackageState) ui.Role {
	switch s {
	case ops.StateStowed:
		return ui.RoleStowed
	case ops.StatePartiallyStowed:
		return ui.RolePartiallyStowed
	case ops.StateOccupied:
		return ui.RoleOccupied
	case ops.StateDamaged:
		return ui.RoleDamaged
	default:
		return ui.RoleNotStowed
	}
}

// linkStateRole maps a per-link state to its role.
func linkStateRole(s ops.LinkState) ui.Role {
	switch s {
	case ops.LinkStowed:
		return ui.RoleStowed
	case ops.LinkOccupied:
		return ui.RoleOccupied
	case ops.LinkDamaged:
		return ui.RoleDamaged
	default:
		return ui.RoleNotStowed
	}
}

// classRole maps a check class to its role. Unobservable is not a state or
// check class (a read-only #45 row), so it renders muted.
func classRole(c ops.Class) ui.Role {
	switch c {
	case ops.ClassBroken:
		return ui.RoleBroken
	case ops.ClassOrphaned:
		return ui.RoleOrphaned
	case ops.ClassContradicted:
		return ui.RoleContradicted
	default:
		return ui.RoleMuted
	}
}

// homeAbbrev replaces a leading home-directory prefix with "~" (§1.5: local
// coordinates display with ~ abbreviation). It is pure presentation, applied
// only to displayed paths, never to data dstow stores or re-parses.
func homeAbbrev(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	prefix := home + string(filepath.Separator)
	if strings.HasPrefix(path, prefix) {
		return "~" + string(filepath.Separator) + path[len(prefix):]
	}
	return path
}
