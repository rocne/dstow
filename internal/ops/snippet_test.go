package ops_test

import (
	"testing"

	"github.com/rocne/dstow/internal/ops"
)

// wantRC is the §9.1 B1 canonical text, transcribed from DESIGN.md (the spec,
// not the implementation): the exact bytes dstow snippet rc must emit. The
// test is a spec-vs-implementation check — the embedded snippets/rc.sh and
// this literal are two independent sources that must agree byte for byte.
const wantRC = `# dstow bootstrap — https://github.com/rocne/dstow
# Ensure the install dir is on PATH, then install dstow only if missing.
# POSIX "is dir in PATH" idiom (builtin, fork-free):
#   https://unix.stackexchange.com/q/32210
case ":$PATH:" in
  *":$HOME/.local/bin:"*) ;;                 # already on PATH
  *) PATH="$HOME/.local/bin:$PATH" ;;
esac

if ! command -v dstow >/dev/null 2>&1; then
  curl -fsSL https://raw.githubusercontent.com/rocne/dstow/main/install.sh | sh
fi
`

func TestSnippetRC(t *testing.T) {
	app := &ops.App{}
	got := app.SnippetRC().Text
	if got != wantRC {
		t.Errorf("SnippetRC() text does not match canonical B1 bytes.\n got: %q\nwant: %q", got, wantRC)
	}
}
