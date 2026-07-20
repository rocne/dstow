package ui

import (
	"bytes"
	"strings"
	"testing"
)

func ptr(b bool) *bool { return &b }

// clearColorEnv sets the four enable-chain env vars to a neutral baseline:
// NO_COLOR="" (present-but-empty falls through), CLICOLOR_FORCE="" (not
// forcing), CLICOLOR="" (not "0"), TERM="" (not "dumb"). Subtests then set only
// the rung under test. t.Setenv restores originals and forbids t.Parallel.
func clearColorEnv(t *testing.T) {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("CLICOLOR", "")
	t.Setenv("TERM", "")
}

// --- §7.3 enable chain: --color > NO_COLOR > CLICOLOR_FORCE > CLICOLOR > TTY --

func TestEnableChain(t *testing.T) {
	tests := []struct {
		name  string
		mode  ColorMode
		isTTY bool
		// env rungs (empty string = neutral unless the *Set flag is used)
		noColor       string
		clicolorForce string
		clicolor      string
		term          string
		want          bool
	}{
		// --color always/never beat everything downstream.
		{name: "always beats NO_COLOR", mode: ColorAlways, isTTY: false, noColor: "1", want: true},
		{name: "never beats CLICOLOR_FORCE", mode: ColorNever, isTTY: true, clicolorForce: "1", want: false},
		// NO_COLOR (present, non-empty) beats CLICOLOR_FORCE.
		{name: "NO_COLOR beats CLICOLOR_FORCE", mode: ColorAuto, isTTY: true, noColor: "1", clicolorForce: "1", want: false},
		// NO_COLOR present but empty falls through to the per-face TTY test.
		{name: "empty NO_COLOR falls through", mode: ColorAuto, isTTY: true, noColor: "", want: true},
		// CLICOLOR_FORCE (present, non-empty, != "0") beats CLICOLOR=0 and non-TTY.
		{name: "CLICOLOR_FORCE beats CLICOLOR=0", mode: ColorAuto, isTTY: false, clicolorForce: "1", clicolor: "0", want: true},
		// CLICOLOR_FORCE="0" is not a force; falls through.
		{name: "CLICOLOR_FORCE=0 is not force", mode: ColorAuto, isTTY: false, clicolorForce: "0", want: false},
		// CLICOLOR=0 beats a TTY.
		{name: "CLICOLOR=0 beats TTY", mode: ColorAuto, isTTY: true, clicolor: "0", want: false},
		// Per-face TTY test at the bottom of the chain.
		{name: "TTY on", mode: ColorAuto, isTTY: true, want: true},
		{name: "TTY off", mode: ColorAuto, isTTY: false, want: false},
		// TERM=dumb kills a TTY-on.
		{name: "TERM=dumb kills TTY", mode: ColorAuto, isTTY: true, term: "dumb", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearColorEnv(t)
			t.Setenv("NO_COLOR", tt.noColor)
			t.Setenv("CLICOLOR_FORCE", tt.clicolorForce)
			t.Setenv("CLICOLOR", tt.clicolor)
			t.Setenv("TERM", tt.term)
			if got := enabledFor(tt.mode, tt.isTTY); got != tt.want {
				t.Errorf("enabledFor(%v, tty=%v) = %v, want %v", tt.mode, tt.isTTY, got, tt.want)
			}
		})
	}
}

// Per-face independence under auto: stdout non-TTY stays off while stderr TTY
// turns on, from one New call.
func TestPerFaceEnablement(t *testing.T) {
	clearColorEnv(t)
	p := New(Options{
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		Mode:      ColorAuto,
		StdoutTTY: ptr(false), StderrTTY: ptr(true),
	})
	if p.Out().colorOn {
		t.Error("stdout non-TTY should be color-off")
	}
	if !p.Err().colorOn {
		t.Error("stderr TTY should be color-on")
	}
}

// --- O11 / Face.Style --------------------------------------------------------

func TestFaceStyleOnOff(t *testing.T) {
	clearColorEnv(t)
	on := New(Options{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Mode: ColorAlways})
	off := New(Options{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Mode: ColorNever})

	const text = "payload"
	styled := on.Out().Style(RoleStowed, text)
	if styled == text {
		t.Error("color-on face should style the text")
	}
	if StripANSI(styled) != text {
		t.Errorf("StripANSI(styled) = %q, want %q", StripANSI(styled), text)
	}
	if plain := off.Out().Style(RoleStowed, text); plain != text {
		t.Errorf("color-off face should return plain text, got %q", plain)
	}
}

// A role whose slot has no theme entry renders plain (no crash, no styling).
func TestFaceStyleUnknownSlotPlain(t *testing.T) {
	clearColorEnv(t)
	p := New(Options{
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Mode: ColorAlways,
		Theme: Theme{}, // empty theme: no slots
	})
	if got := p.Out().Style(RoleStowed, "x"); got != "x" {
		t.Errorf("unknown slot should render plain, got %q", got)
	}
}

// --- O2 / O7 severity lines --------------------------------------------------

func newBuffered(t *testing.T, mode ColorMode, quiet bool) (*Printer, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	clearColorEnv(t)
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	p := New(Options{Stdout: out, Stderr: errb, Mode: mode, Quiet: quiet})
	return p, out, errb
}

// Prefix text and padding are exact and align to len("warning:")+1 columns.
func TestSeverityPrefixPadding(t *testing.T) {
	tests := []struct {
		name string
		call func(p *Printer)
		want string
	}{
		{"note", func(p *Printer) { p.Notef("m") }, "note:    m\n"},
		{"announce uses note prefix", func(p *Printer) { p.Announcef("m") }, "note:    m\n"},
		{"warning", func(p *Printer) { p.Warningf("m") }, "warning: m\n"},
		{"error", func(p *Printer) { p.Errorf("m") }, "error:   m\n"},
		{"fix", func(p *Printer) { p.Fixf("m") }, "fix:     m\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _, errb := newBuffered(t, ColorNever, false)
			tt.call(p)
			if got := errb.String(); got != tt.want {
				t.Errorf("line = %q, want %q", got, tt.want)
			}
		})
	}
}

// Every severity line goes to stderr; stdout stays untouched.
func TestSeverityAllOnStderr(t *testing.T) {
	p, out, errb := newBuffered(t, ColorNever, false)
	p.Notef("a")
	p.Announcef("b")
	p.Warningf("c")
	p.Errorf("d")
	p.Fixf("e")
	if out.Len() != 0 {
		t.Errorf("stdout should be untouched, got %q", out.String())
	}
	if errb.Len() == 0 {
		t.Error("commentary should be on stderr")
	}
}

// Quiet drops Notef only; announcements, warnings, errors, fixes survive (O7).
func TestSeverityQuietMatrix(t *testing.T) {
	p, _, errb := newBuffered(t, ColorNever, true)
	p.Notef("dropped")
	if errb.Len() != 0 {
		t.Errorf("Notef must be dropped under quiet, got %q", errb.String())
	}
	errb.Reset()

	p.Announcef("kept")
	p.Warningf("kept")
	p.Errorf("kept")
	p.Fixf("kept")
	got := errb.String()
	for _, want := range []string{"note:", "warning:", "error:", "fix:"} {
		if !strings.Contains(got, want) {
			t.Errorf("under quiet, %q should survive; output = %q", want, got)
		}
	}
}

// The prefix is styled, the message is plain, and stripping recovers the plain
// line (O11) — verified against a color-on printer.
func TestSeverityStyledPrefixPlainMessage(t *testing.T) {
	p, _, errb := newBuffered(t, ColorAlways, false)
	p.Warningf("disk full")
	line := errb.String()
	if !strings.Contains(line, "\x1b[") {
		t.Error("prefix should be colored under color-always")
	}
	if StripANSI(line) != "warning: disk full\n" {
		t.Errorf("StripANSI(line) = %q, want %q", StripANSI(line), "warning: disk full\n")
	}
	// The message text must not be wrapped in color codes: nothing sits between
	// the last reset and the message.
	if !strings.Contains(line, "disk full\n") {
		t.Errorf("message should be plain, got %q", line)
	}
}

// fix: renders via info1 — bold brightblue in the defaults — never through the
// success family: a fix appears when nothing succeeded (O2).
func TestFixStyledPerPalette(t *testing.T) {
	p, _, errb := newBuffered(t, ColorAlways, false)
	p.Fixf("run this")
	line := errb.String()
	if !strings.Contains(line, "\x1b[1;94m") {
		t.Errorf("fix prefix should be bold brightblue (SGR 1;94, §7.2), got %q", line)
	}
	if strings.Contains(line, "\x1b[32m") || strings.Contains(line, "\x1b[1;32m") {
		t.Error("fix prefix must not render through the success family (green)")
	}
}

// Face.Printf / Println write raw and are never filtered by quiet.
func TestFacePrintfUnfilteredByQuiet(t *testing.T) {
	p, out, _ := newBuffered(t, ColorNever, true)
	p.Out().Printf("data %d\n", 42)
	p.Out().Println("more")
	if out.String() != "data 42\nmore\n" {
		t.Errorf("stdout = %q, want unfiltered data", out.String())
	}
}

// --- A6 Interactive: stdin TTY && stderr TTY ---------------------------------

func TestInteractiveMatrix(t *testing.T) {
	tests := []struct {
		stdin, stderr, want bool
	}{
		{true, true, true},
		{true, false, false},
		{false, true, false},
		{false, false, false},
	}
	for _, tt := range tests {
		clearColorEnv(t)
		p := New(Options{
			Stdin: &bytes.Buffer{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
			StdinTTY: ptr(tt.stdin), StderrTTY: ptr(tt.stderr),
		})
		if got := p.Interactive(); got != tt.want {
			t.Errorf("Interactive(stdin=%v, stderr=%v) = %v, want %v", tt.stdin, tt.stderr, got, tt.want)
		}
	}
}

// A nil Theme resolves to the default palette.
func TestNewNilThemeUsesDefault(t *testing.T) {
	clearColorEnv(t)
	p := New(Options{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Mode: ColorAlways})
	styled := p.Out().Style(RoleStowed, "x")
	if StripANSI(styled) != "x" || styled == "x" {
		t.Errorf("nil theme should style via the default palette, got %q", styled)
	}
}
