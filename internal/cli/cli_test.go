package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

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
}

// TestTopLevelHelpContent asserts `dstow --help` carries DESIGN.md §2.3's
// canonical content (A2 as amended — issue #96): the command inventory with
// its wording, the global flag roster, and the closing prose. The expected
// content is extracted from docs/DESIGN.md itself, so the test is anchored to
// the design source, never to the consts it guards (no contrived-green).
// Layout is cobra's and is deliberately not asserted.
func TestTopLevelHelpContent(t *testing.T) {
	isolateXDG(t)
	block := designBlock(t, "### 2.3 Top-level help (canonical)", 0)

	out, _, code := run(t, "--help")
	if code != 0 {
		t.Fatalf("--help exit = %d, want 0 (help is the requested data, A2)", code)
	}
	assertHelpContent(t, block, out)

	// The bare invocation prints the same help on stdout, exit 0 (§2.1).
	bareOut, _, bareCode := run(t)
	if bareCode != 0 || bareOut != out {
		t.Errorf("bare dstow: code=%d, out matches --help=%v", bareCode, bareOut == out)
	}
}

// TestPerCommandHelpContent asserts each per-command help carries its §2.4
// canonical block's content, extracted from DESIGN.md.
func TestPerCommandHelpContent(t *testing.T) {
	isolateXDG(t)
	cases := []struct {
		heading string
		index   int
		args    []string
	}{
		{"#### stow / unstow / restow (leaves)", 0, []string{"stow"}},
		{"#### adopt (leaf)", 0, []string{"adopt"}},
		{"#### repo (group)", 0, []string{"repo"}},
		{"#### repo (group)", 1, []string{"repo", "add"}},
		{"#### repo (group)", 2, []string{"repo", "remove"}},
		// update and upgrade share §2.4's combined two-phase block.
		{"#### repo (group)", 3, []string{"repo", "update"}},
		{"#### repo (group)", 3, []string{"repo", "upgrade"}},
		{"#### list (leaf)", 0, []string{"list"}},
		{"#### info (leaf)", 0, []string{"info"}},
		{"#### status (leaf)", 0, []string{"status"}},
		{"#### check / clean / rebuild (leaves)", 0, []string{"check"}},
		{"#### check / clean / rebuild (leaves)", 1, []string{"clean"}},
		{"#### check / clean / rebuild (leaves)", 2, []string{"rebuild"}},
		{"#### snippet (group)", 0, []string{"snippet"}},
		{"#### theme (group)", 0, []string{"theme"}},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			block := designBlock(t, tc.heading, tc.index)
			out, _, code := run(t, append(tc.args, "--help")...)
			if code != 0 {
				t.Fatalf("%v --help exit = %d, want 0", tc.args, code)
			}
			assertHelpContent(t, block, out)
		})
	}
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

// designBlock reads docs/DESIGN.md and returns the index-th fenced code block
// between the given heading and the next heading of equal-or-higher level.
func designBlock(t *testing.T, heading string, index int) string {
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
	rest := text[hi+len(heading):]
	// Truncate at the next heading so index-th never crosses sections.
	if next := regexp.MustCompile(`(?m)^#{1,4} `).FindStringIndex(rest); next != nil {
		rest = rest[:next[0]]
	}
	for i := 0; ; i++ {
		open := strings.Index(rest, "```")
		if open < 0 {
			t.Fatalf("fence %d after %q not found", index, heading)
		}
		afterOpen := rest[open+3:]
		nl := strings.IndexByte(afterOpen, '\n')
		body := afterOpen[nl+1:]
		closing := strings.Index(body, "```")
		if closing < 0 {
			t.Fatalf("unterminated code fence after %q", heading)
		}
		if i == index {
			return body[:closing]
		}
		rest = body[closing+3:]
	}
}

// normWS collapses all whitespace runs to single spaces, so content assertions
// survive layout differences (padding, wrapping, indentation).
func normWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// assertHelpContent asserts every content atom of a canonical §2.3/§2.4 block
// appears in the generated help: prose lines, command-list entries (name +
// description), flag long names with their descriptions, example lines, and
// entry-shaped content sections (Environment:, Exit status:). Usage syntax
// lines are cobra's and are skipped.
func assertHelpContent(t *testing.T, block, got string) {
	t.Helper()
	normGot := normWS(got)
	contains := func(kind, atom string) {
		t.Helper()
		if atom == "" {
			return
		}
		if !strings.Contains(normGot, normWS(atom)) {
			t.Errorf("help is missing §2.3/§2.4 %s: %q", kind, atom)
		}
	}

	headingRe := regexp.MustCompile(`^[A-Za-z][A-Za-z ]{0,28}:$`)
	entryRe := regexp.MustCompile(`^  (\S+)(\s{2,})(.*)$`)
	flagStartRe := regexp.MustCompile(`^\s+-`)
	longFlagRe := regexp.MustCompile(`--[\w-]+`)
	flagSpecRe := regexp.MustCompile(`^\s+(-[A-Za-z], )?(--[\w-]+)( <\w+>)?\s{2,}(.*)$`)

	section := ""
	// pending accumulates a multi-line entry (command list or flag) until the
	// next entry or section flushes it.
	var pending []string
	var flush func()
	flushEntry := func(kind string) {
		if len(pending) > 0 {
			contains(kind, strings.Join(pending, " "))
			pending = nil
		}
	}
	flush = func() {}

	for _, ln := range strings.Split(block, "\n") {
		trimmed := strings.TrimRight(ln, " \t")
		if trimmed == "" {
			flush()
			continue
		}
		if headingRe.MatchString(trimmed) && !strings.HasPrefix(trimmed, " ") {
			flush()
			section = trimmed
			continue
		}
		switch section {
		case "Usage:":
			// Usage syntax lines belong to cobra's rendering.
		case "Flags:", "Global flags:":
			if flagStartRe.MatchString(trimmed) {
				flush()
				m := flagSpecRe.FindStringSubmatch(trimmed)
				if m == nil {
					t.Errorf("unparseable canonical flag line: %q", trimmed)
					continue
				}
				for _, lf := range longFlagRe.FindAllString(trimmed[:len(trimmed)-len(m[4])], -1) {
					contains("flag", lf)
				}
				pending = []string{m[4]}
				flush = func() { flushEntry("flag description") }
			} else {
				pending = append(pending, strings.TrimSpace(trimmed))
			}
		case "Examples:":
			contains("example", strings.TrimSpace(trimmed))
		default:
			// The §2.3 groups, group-command Commands:, Environment:, Exit
			// status: — and prose when no entry is open.
			if m := entryRe.FindStringSubmatch(trimmed); m != nil {
				flush()
				pending = []string{m[1] + " " + m[3]}
				flush = func() { flushEntry("entry") }
			} else if strings.HasPrefix(ln, "    ") && len(pending) > 0 {
				pending = append(pending, strings.TrimSpace(trimmed))
			} else {
				flush()
				contains("prose", trimmed)
			}
		}
	}
	flush()
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
