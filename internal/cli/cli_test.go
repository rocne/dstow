package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run drives Run with buffered streams (never TTYs), returning stdout, stderr,
// and the exit code — the black-box surface the ticket's tests assert against.
func run(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	full := append([]string{"dstow"}, args...)
	code = Run(full, "v1.2.3", strings.NewReader(""), &out, &errb)
	return out.String(), errb.String(), code
}

// isolateXDG points config/state/data at a fresh temp HOME so tests never touch
// the developer's real dstow files.
func isolateXDG(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("DSTOW_PATH", "")
	t.Setenv("DSTOW_COLORS", "")
	t.Setenv("NO_COLOR", "1") // deterministic, uncolored output for assertions
}

// TestTopLevelHelpVerbatim asserts `dstow --help` matches DESIGN.md §2.3's
// canonical block exactly (A2, §2.3). The expected text is extracted from
// docs/DESIGN.md itself, so the test is anchored to the design source, never to
// the const it guards (no contrived-green).
func TestTopLevelHelpVerbatim(t *testing.T) {
	isolateXDG(t)
	want := designBlock(t, "### 2.3 Top-level help (canonical)")

	out, _, code := run(t, "--help")
	if code != 0 {
		t.Fatalf("--help exit = %d, want 0 (help is the requested data, A2)", code)
	}
	if out != want {
		t.Errorf("top-level help does not match DESIGN.md §2.3 verbatim.\n--- got ---\n%q\n--- want ---\n%q", out, want)
	}

	// The bare invocation prints the same help on stdout, exit 0 (§2.1).
	bareOut, _, bareCode := run(t)
	if bareCode != 0 || bareOut != want {
		t.Errorf("bare dstow: code=%d, out matches want=%v", bareCode, bareOut == want)
	}
}

// designBlock reads docs/DESIGN.md and returns the first fenced code block that
// follows the given heading, newline-terminated (matching how cli emits help).
func designBlock(t *testing.T, heading string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "DESIGN.md"))
	if err != nil {
		t.Fatalf("read DESIGN.md: %v", err)
	}
	text := string(data)
	hi := strings.Index(text, heading)
	if hi < 0 {
		t.Fatalf("heading %q not found in DESIGN.md", heading)
	}
	rest := text[hi:]
	open := strings.Index(rest, "```")
	if open < 0 {
		t.Fatalf("no code fence after %q", heading)
	}
	// Skip the opening fence line (```\n or ```lang\n).
	afterOpen := rest[open+3:]
	nl := strings.IndexByte(afterOpen, '\n')
	body := afterOpen[nl+1:]
	close := strings.Index(body, "```")
	if close < 0 {
		t.Fatalf("unterminated code fence after %q", heading)
	}
	return body[:close]
}

// TestVersion prints the injected version to stdout, exit 0.
func TestVersion(t *testing.T) {
	isolateXDG(t)
	out, _, code := run(t, "version")
	if code != 0 || strings.TrimSpace(out) != "v1.2.3" {
		t.Errorf("version: out=%q code=%d, want %q / 0", out, code, "v1.2.3")
	}
}

// TestVersionFlag asserts `dstow --version` prints the version on line 1 with
// the version as its first semver-shaped token, exit 0 — the D30 contract
// (release-ci#15) the release dry-run asserts via assert-version-contract.sh,
// and what the canonical installer's ensure-check parses (D28/D30). The output
// matches the version subcommand exactly: one source of truth, two spellings.
func TestVersionFlag(t *testing.T) {
	isolateXDG(t)
	out, _, code := run(t, "--version")
	if code != 0 || strings.TrimSpace(out) != "v1.2.3" {
		t.Errorf("--version: out=%q code=%d, want %q / 0", out, code, "v1.2.3")
	}
}

// TestUsageErrorsExit2 asserts cobra flag/arg/unknown-command failures and a
// bad --color value all map to exit 2 (A3), with the message on stderr, stdout
// clean (O1).
func TestUsageErrorsExit2(t *testing.T) {
	isolateXDG(t)
	cases := []struct {
		name string
		args []string
	}{
		{"unknown flag", []string{"stow", "--bogus"}},
		{"unknown command", []string{"nope"}},
		{"bad color value", []string{"--color", "purple", "list"}},
		{"repos and packages", []string{"list", "--repos", "--packages"}},
		{"too many args", []string{"info", "a", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, errs, code := run(t, tc.args...)
			if code != 2 {
				t.Errorf("exit = %d, want 2; stderr=%q", code, errs)
			}
			if out != "" {
				t.Errorf("stdout not clean on a usage error: %q", out)
			}
			if !strings.Contains(errs, "error:") {
				t.Errorf("stderr missing error: line: %q", errs)
			}
		})
	}
}

// TestNameGroup exercises the hidden name encode/decode round trip on stdout.
func TestNameGroup(t *testing.T) {
	isolateXDG(t)
	enc, _, code := run(t, "name", "encode", "a:b@c")
	if code != 0 || strings.TrimSpace(enc) != "a%3Ab%40c" {
		t.Errorf("name encode: got %q code %d", enc, code)
	}
	dec, _, code := run(t, "name", "decode", "a%3Ab%40c")
	if code != 0 || strings.TrimSpace(dec) != "a:b@c" {
		t.Errorf("name decode: got %q code %d", dec, code)
	}
	// The name group is hidden: it must not appear in top-level help.
	help, _, _ := run(t, "--help")
	if strings.Contains(help, "\n  name ") || strings.Contains(help, "name        ") {
		t.Errorf("hidden name group leaked into top-level help")
	}
}

// TestSnippetRC emits the bootstrap snippet on stdout, byte-for-byte with the
// ops-owned text (B1/B2), nothing on stderr.
func TestSnippetRC(t *testing.T) {
	isolateXDG(t)
	out, errs, code := run(t, "snippet", "rc")
	if code != 0 {
		t.Fatalf("snippet rc exit = %d", code)
	}
	if errs != "" {
		t.Errorf("snippet rc wrote to stderr: %q", errs)
	}
	if !strings.HasPrefix(out, "#!/usr/bin/env sh") {
		t.Errorf("snippet rc stdout unexpected: %q", out)
	}
}

// TestListJSONEmptyRegistry asserts the empty-registry listing is a well-formed
// JSON object on stdout (O10), exit 0.
func TestListJSONEmptyRegistry(t *testing.T) {
	isolateXDG(t)
	out, _, code := run(t, "list", "--json")
	if code != 0 {
		t.Fatalf("list --json exit = %d", code)
	}
	if !strings.Contains(out, `"repos"`) {
		t.Errorf("list --json missing repos key: %q", out)
	}
}
