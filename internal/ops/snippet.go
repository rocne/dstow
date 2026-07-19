package ops

import dstow "github.com/rocne/dstow"

// SnippetResult is one canned snippet as data (§2.4 snippet). ops returns the
// text; cli writes it to stdout — ops never touches a stream (A4).
type SnippetResult struct {
	// Text is the exact bytes to emit, verbatim and unmodified.
	Text string
}

// SnippetRC returns the shell-rc bootstrap snippet (§9.1): the vendored
// snippet.sh, embedded at the repo root (B1 as amended per release-ci D26 —
// one file, one owner) and emitted verbatim (B2). It reads nothing and cannot
// fail — the text is compiled in — so it takes no request and returns no
// error.
func (a *App) SnippetRC() SnippetResult {
	return SnippetResult{Text: dstow.RCSnippet}
}
