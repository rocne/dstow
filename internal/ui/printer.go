package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// ColorMode is the resolved --color <when> value (§7.2): auto (default),
// always, or never.
type ColorMode int

const (
	ColorAuto ColorMode = iota
	ColorAlways
	ColorNever
)

// Options constructs a Printer. Streams are injected; the environment is read
// via os.Getenv at point of use inside New (A2). The TTY overrides are the
// test seam (A6): nil means auto-detect (an *os.File probed via isatty;
// anything else is not a TTY).
type Options struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Mode           ColorMode
	Quiet          bool
	Theme          Theme // resolved; nil means DefaultPalette()

	StdinTTY, StdoutTTY, StderrTTY *bool
}

// Printer is dstow's rendering seam: the two faces (stdout is data, stderr is
// commentary — O1), severity lines, and the interactivity probe. It is the
// only thing that touches the streams (A4).
type Printer struct {
	out, err  *Face
	quiet     bool
	stdinTTY  bool
	stderrTTY bool
}

// Face is one stream's rendering surface. Each face styles against its OWN
// enablement (O1): stdout and stderr are enabled independently.
type Face struct {
	w       io.Writer
	colorOn bool
	theme   Theme
}

// New builds a Printer, computing per-stream color enablement (§7.3) strictly
// upstream of theming: theme config can never re-enable what the chain turned
// off. Enablement is per-instance — ui never reads or writes fatih's global
// NoColor (A5).
func New(o Options) *Printer {
	theme := o.Theme
	if theme == nil {
		theme = DefaultPalette()
	}

	stdinTTY := resolveTTY(o.StdinTTY, o.Stdin)
	stdoutTTY := resolveTTY(o.StdoutTTY, o.Stdout)
	stderrTTY := resolveTTY(o.StderrTTY, o.Stderr)

	return &Printer{
		out:       &Face{w: o.Stdout, colorOn: enabledFor(o.Mode, stdoutTTY), theme: theme},
		err:       &Face{w: o.Stderr, colorOn: enabledFor(o.Mode, stderrTTY), theme: theme},
		quiet:     o.Quiet,
		stdinTTY:  stdinTTY,
		stderrTTY: stderrTTY,
	}
}

// enabledFor computes color enablement for one stream via the §7.3 precedence
// chain: --color > NO_COLOR > CLICOLOR_FORCE > CLICOLOR > TTY+TERM=dumb. Env is
// read here, at point of use (A2).
func enabledFor(mode ColorMode, isTTY bool) bool {
	switch mode {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	}
	// ColorAuto.
	if v, ok := os.LookupEnv("NO_COLOR"); ok && v != "" {
		return false
	}
	if v, ok := os.LookupEnv("CLICOLOR_FORCE"); ok && v != "" && v != "0" {
		return true
	}
	if os.Getenv("CLICOLOR") == "0" {
		return false
	}
	return isTTY && os.Getenv("TERM") != "dumb"
}

// resolveTTY reports whether a stream is a TTY, honoring the injected override
// (A6). With no override, a stream is a TTY iff it is an *os.File that isatty
// reports as a terminal; anything else is not a TTY.
func resolveTTY(override *bool, stream any) bool {
	if override != nil {
		return *override
	}
	f, ok := stream.(*os.File)
	if !ok || f == nil {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// Interactive reports whether dstow may prompt: stdin TTY && stderr TTY (A6),
// since prompts live on stderr.
func (p *Printer) Interactive() bool {
	return p.stdinTTY && p.stderrTTY
}

// Out is the data face (stdout). Err is the commentary face (stderr) (O1).
func (p *Printer) Out() *Face { return p.out }
func (p *Printer) Err() *Face { return p.err }

// Printf and Println write data unstyled and unfiltered: quiet is a
// severity-line policy, not a stream policy — data output is never chatter.
func (f *Face) Printf(format string, a ...any) { fmt.Fprintf(f.w, format, a...) }
func (f *Face) Println(a ...any)               { fmt.Fprintln(f.w, a...) }

// Style returns text styled for slot iff this face's color is on; otherwise
// text unchanged. StripANSI(result) == text always (O11).
func (f *Face) Style(slot Slot, text string) string {
	if !f.colorOn {
		return text
	}
	st, ok := f.theme[slot]
	if !ok {
		return text
	}
	return st.render(text)
}

// StyleWith renders text in an explicit style iff this face's color is on —
// the seam for showing a theme's own colors rather than the printer's (theme
// show renders each slot in the theme under inspection, not the active one).
// StripANSI(result) == text always (O11).
func (f *Face) StyleWith(st Style, text string) string {
	if !f.colorOn {
		return text
	}
	return st.render(text)
}

// severityWidth is len("warning:") — the width every severity prefix is padded
// to so stacked commentary aligns (§7.2; the padding is provisional in the
// spec, implemented as specced).
const severityWidth = len("warning:")

// severity writes one commentary line to stderr: the slot-colored prefix (styled
// against the STDERR face's enablement), right-padded to severityWidth, one
// space, then the plain message. The colored region is only the prefix, so
// StripANSI(line) == the plain line (O11).
func (p *Printer) severity(slot Slot, label, format string, a ...any) {
	prefix := label + ":"
	pad := severityWidth - len(prefix)
	if pad < 0 {
		pad = 0
	}
	line := p.err.Style(slot, prefix) + strings.Repeat(" ", pad) + " " + fmt.Sprintf(format, a...)
	fmt.Fprintln(p.err.w, line)
}

// Notef is routine chatter: note:-prefixed and DROPPED under --quiet (O7).
func (p *Printer) Notef(format string, a ...any) {
	if p.quiet {
		return
	}
	p.severity(SlotNote, "note", format, a...)
}

// Announcef is a note:-prefixed announcement — surprise-class, so it SURVIVES
// --quiet (O7: "announcements always survive").
func (p *Printer) Announcef(format string, a ...any) {
	p.severity(SlotNote, "note", format, a...)
}

// Warningf survives --quiet.
func (p *Printer) Warningf(format string, a ...any) {
	p.severity(SlotWarning, "warning", format, a...)
}

// Errorf survives --quiet.
func (p *Printer) Errorf(format string, a ...any) {
	p.severity(SlotError, "error", format, a...)
}

// Fixf survives --quiet; fix: is blue, not green — a fix appears precisely when
// nothing succeeded (O2).
func (p *Printer) Fixf(format string, a ...any) {
	p.severity(SlotFix, "fix", format, a...)
}
