package ops

import "sort"

// orderWork is the one ordering owner for multi-package execution (ruled
// 2026-07-17 on #44): canonical-FQN sort — repos by canonical repo FQN,
// packages within a repo by canonical FQN — regardless of operand spelling
// or order, so every spelling of a run behaves identically. The same sort
// is the adopt candidate tie-break.
//
// Deliberate doorway (same ruling): a future configuration knob may
// customize the order. Every caller sorts through this function only, so
// the knob slots in here without touching them; v1 assigns it no
// semantics.
func orderWork(ws []work) {
	sort.SliceStable(ws, func(i, j int) bool {
		ri, rj := ws[i].pkg.FQN.Repo().String(), ws[j].pkg.FQN.Repo().String()
		if ri != rj {
			return ri < rj
		}
		return ws[i].pkg.FQN.String() < ws[j].pkg.FQN.String()
	})
}
