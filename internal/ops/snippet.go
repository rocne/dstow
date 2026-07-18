package ops

import _ "embed"

// rcSnippet is the §9.1 B1 shell-rc bootstrap, embedded from a real file (B2:
// every snippet emission is a go:embed document — diffable, shellcheck-able in
// CI, never a Go string literal).
//
//go:embed snippets/rc.sh
var rcSnippet string

// SnippetResult is one canned snippet as data (§2.4 snippet). ops returns the
// text; cli writes it to stdout — ops never touches a stream (A4).
type SnippetResult struct {
	// Text is the exact bytes to emit, verbatim and unmodified.
	Text string
}

// SnippetRC returns the shell-rc bootstrap snippet (§9.1): the PATH idiom plus
// the install-iff-absent guard. It reads nothing and cannot fail — the text is
// compiled in — so it takes no request and returns no error.
func (a *App) SnippetRC() SnippetResult {
	return SnippetResult{Text: rcSnippet}
}
