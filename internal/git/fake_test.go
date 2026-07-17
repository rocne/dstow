package git_test

import (
	"errors"
	"testing"

	"github.com/rocne/dstow/internal/git"
)

// The Fake returns scripted results and records every call, so other packages'
// tests can drive repo's git seam without a real repository (A17).
func TestFakeScriptsResultsAndRecordsCalls(t *testing.T) {
	sentinel := errors.New("boom")
	f := &git.Fake{
		CloneErr:        sentinel,
		FFApplyOld:      "old0",
		FFApplyNew:      "new1",
		Ahead:           2,
		Behind:          3,
		LocalWork:       true,
		LocalWorkReason: "unpushed",
	}

	if err := f.Clone("url", "dir"); !errors.Is(err, sentinel) {
		t.Errorf("Clone err = %v, want %v", err, sentinel)
	}
	if err := f.Fetch("d"); err != nil {
		t.Errorf("Fetch err = %v, want nil", err)
	}
	old, newRev, err := f.FFApply("d")
	if old != "old0" || newRev != "new1" || err != nil {
		t.Errorf("FFApply = %q, %q, %v; want old0, new1, nil", old, newRev, err)
	}
	ahead, behind, err := f.AheadBehind("d")
	if ahead != 2 || behind != 3 || err != nil {
		t.Errorf("AheadBehind = %d, %d, %v; want 2, 3, nil", ahead, behind, err)
	}
	work, reason, err := f.HasLocalWork("d")
	if !work || reason != "unpushed" || err != nil {
		t.Errorf("HasLocalWork = %v, %q, %v; want true, unpushed, nil", work, reason, err)
	}

	if len(f.CloneCalls) != 1 || f.CloneCalls[0] != (git.ClonePair{URL: "url", Dir: "dir"}) {
		t.Errorf("CloneCalls = %+v", f.CloneCalls)
	}
	if len(f.FetchCalls) != 1 || f.FetchCalls[0] != "d" {
		t.Errorf("FetchCalls = %+v", f.FetchCalls)
	}
	if len(f.FFApplyCalls) != 1 || len(f.AheadBehindCalls) != 1 || len(f.LocalWorkCalls) != 1 {
		t.Errorf("call recording incomplete: %+v", f)
	}
}
