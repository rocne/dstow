package git

// Fake is an in-memory Port for other packages' tests (A17). Every method's
// result is a scriptable field; every call is recorded so a test can assert
// what dstow asked git to do without touching a real repository. It satisfies
// Port on a pointer receiver (so calls can be recorded).
type Fake struct {
	// Scripted results.
	CloneErr        error
	FetchErr        error
	FFApplyOld      string
	FFApplyNew      string
	FFApplyErr      error
	Ahead           int
	Behind          int
	AheadBehindErr  error
	LocalWork       bool
	LocalWorkReason string
	LocalWorkErr    error

	// Recorded calls, in order.
	CloneCalls       []ClonePair
	FetchCalls       []string
	FFApplyCalls     []string
	AheadBehindCalls []string
	LocalWorkCalls   []string
}

// ClonePair records one Clone(url, dir) call.
type ClonePair struct {
	URL string
	Dir string
}

// Clone records the call and returns the scripted CloneErr.
func (f *Fake) Clone(url, dir string) error {
	f.CloneCalls = append(f.CloneCalls, ClonePair{URL: url, Dir: dir})
	return f.CloneErr
}

// Fetch records the call and returns the scripted FetchErr.
func (f *Fake) Fetch(dir string) error {
	f.FetchCalls = append(f.FetchCalls, dir)
	return f.FetchErr
}

// FFApply records the call and returns the scripted revisions or error.
func (f *Fake) FFApply(dir string) (old, new string, err error) {
	f.FFApplyCalls = append(f.FFApplyCalls, dir)
	return f.FFApplyOld, f.FFApplyNew, f.FFApplyErr
}

// AheadBehind records the call and returns the scripted counts or error.
func (f *Fake) AheadBehind(dir string) (ahead, behind int, err error) {
	f.AheadBehindCalls = append(f.AheadBehindCalls, dir)
	return f.Ahead, f.Behind, f.AheadBehindErr
}

// HasLocalWork records the call and returns the scripted verdict, prose, and
// error.
func (f *Fake) HasLocalWork(dir string) (bool, string, error) {
	f.LocalWorkCalls = append(f.LocalWorkCalls, dir)
	return f.LocalWork, f.LocalWorkReason, f.LocalWorkErr
}

// compile-time assertion that *Fake is a Port.
var _ Port = (*Fake)(nil)
