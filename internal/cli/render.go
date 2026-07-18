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

// stateSlot maps a package state to its semantic slot (O3: only ui names
// colors; callers name slots).
func stateSlot(s ops.PackageState) ui.Slot {
	switch s {
	case ops.StateStowed:
		return ui.SlotStowed
	case ops.StatePartiallyStowed:
		return ui.SlotPartiallyStowed
	case ops.StateOccupied:
		return ui.SlotOccupied
	case ops.StateDamaged:
		return ui.SlotDamaged
	default:
		return ui.SlotNotStowed
	}
}

// linkStateSlot maps a per-link state to its slot.
func linkStateSlot(s ops.LinkState) ui.Slot {
	switch s {
	case ops.LinkStowed:
		return ui.SlotStowed
	case ops.LinkOccupied:
		return ui.SlotOccupied
	case ops.LinkDamaged:
		return ui.SlotDamaged
	default:
		return ui.SlotNotStowed
	}
}

// classSlot maps a check class to its slot. Unobservable has no palette entry
// (it is a read-only #45 row, not a state or check class), so it renders muted.
func classSlot(c ops.Class) ui.Slot {
	switch c {
	case ops.ClassBroken:
		return ui.SlotBroken
	case ops.ClassOrphaned:
		return ui.SlotOrphaned
	case ops.ClassContradicted:
		return ui.SlotContradicted
	default:
		return ui.SlotMuted
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
