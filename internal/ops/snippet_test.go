package ops_test

import (
	"os"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/ops"
)

// TestSnippetRC asserts the emitted snippet is byte-identical to the vendored
// snippet.sh at the repo root — the canonical rc bootstrap (§9.1 B1 as amended
// per release-ci D26: one file, one owner, zero transcription drift). Reading
// the file at test time (rather than comparing two embeds) verifies the embed
// actually points at the vendored file and cannot go stale against it.
func TestSnippetRC(t *testing.T) {
	want, err := os.ReadFile("../../snippet.sh")
	if err != nil {
		t.Fatalf("reading vendored snippet.sh: %v", err)
	}
	got := (&ops.App{}).SnippetRC().Text
	if got != string(want) {
		t.Errorf("SnippetRC() text does not match the vendored snippet.sh.\n got: %q\nwant: %q", got, want)
	}
}

// TestSnippetRCContract asserts the §9.1 invariants the snippet must keep
// whatever its exact bytes: the PATH line bakes the contractual default
// install dir (B6) and precedes the install guard, and the guard
// short-circuits on presence before any network (present ⇒ offline).
func TestSnippetRCContract(t *testing.T) {
	text := (&ops.App{}).SnippetRC().Text

	pathIdx := strings.Index(text, `PATH="$HOME/.local/bin:$PATH"`)
	guardIdx := strings.Index(text, "! command -v")
	curlIdx := strings.Index(text, "curl -fsSL")

	if pathIdx < 0 {
		t.Fatal("snippet does not bake the contractual default install dir onto PATH")
	}
	if guardIdx < 0 {
		t.Fatal("snippet has no negated command -v presence guard")
	}
	if curlIdx < 0 {
		t.Fatal("snippet never fetches the installer")
	}
	if pathIdx >= guardIdx || guardIdx >= curlIdx {
		t.Errorf("snippet order must be PATH line (%d) < presence guard (%d) < fetch (%d)", pathIdx, guardIdx, curlIdx)
	}
}
