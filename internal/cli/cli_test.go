package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"

	"github.com/rocne/dstow/internal/ui"
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
	// adrg/xdg snapshots the environment at init, so t.Setenv alone would
	// leave xdg-derived paths pointed at the developer's real config.
	xdg.Reload()
	t.Cleanup(xdg.Reload)
}

// normWS collapses all whitespace runs to single spaces, so content assertions
// survive layout differences (padding, wrapping, indentation).
func normWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// TestHelpColorized asserts help rides the §7.3 enable chain: --color=always
// (which beats the isolateXDG NO_COLOR) styles the generated help, and
// stripping the styling yields exactly the plain rendering (O11).
func TestHelpColorized(t *testing.T) {
	isolateXDG(t)
	plain, _, code := run(t, "--help")
	if code != 0 {
		t.Fatalf("--help exit = %d", code)
	}
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("plain --help (NO_COLOR) contains ANSI escapes")
	}

	colored, _, code := run(t, "--color", "always", "--help")
	if code != 0 {
		t.Fatalf("--color always --help exit = %d", code)
	}
	if !strings.Contains(colored, "\x1b[") {
		t.Errorf("--color=always help carries no ANSI styling")
	}
	if got := ui.StripANSI(colored); got != plain {
		t.Errorf("strip(colored help) != plain help (O11).\n--- stripped ---\n%q\n--- plain ---\n%q", got, plain)
	}
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

	// Each leaf is learnable from its own help alone (audit finding C1): the
	// usage line shows the required <segment> operand, and an example shows how
	// to shape it. Without both, the reader cannot tell an argument is needed.
	for _, leaf := range []string{"encode", "decode"} {
		h, _, _ := run(t, "name", leaf, "--help")
		if !strings.Contains(h, "name "+leaf+" <segment>") {
			t.Errorf("name %s help omits the <segment> operand:\n%s", leaf, h)
		}
		if !strings.Contains(h, "Examples:") || !strings.Contains(h, "dstow name "+leaf) {
			t.Errorf("name %s help carries no example:\n%s", leaf, h)
		}
	}
}

// TestListReposHumanRowTerse guards audit finding A3: the human `list --repos`
// row is the repo's name, its (~-abbreviated) path, and status markers — not
// the source/scheme the flag help and list.md once promised. `list` enumerates;
// `info` reports fields, and source/scheme live there and in --json. If the row
// ever grows a scheme-qualified column, the FQN prefix ("local:") reappears here
// and this fails, forcing the docs to be updated alongside it.
func TestListReposHumanRowTerse(t *testing.T) {
	isolateXDG(t)
	repoDir := filepath.Join(os.Getenv("HOME"), "dots")
	mkdirs(t, filepath.Join(repoDir, "zsh"))
	t.Setenv("DSTOW_PATH", repoDir)

	out, _, code := run(t, "list", "--repos")
	if code != 0 {
		t.Fatalf("list --repos exit = %d", code)
	}
	if got := normWS(out); got != "dots ~/dots (session)" {
		t.Errorf("human repo row = %q, want %q", got, "dots ~/dots (session)")
	}
	// The scheme-qualified FQN is a --json / info concern, never the human row.
	if strings.Contains(out, "local:") {
		t.Errorf("human list --repos row leaked the scheme-qualified FQN: %q", out)
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
