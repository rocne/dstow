package cli

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/ui"
)

func boolPtr(b bool) *bool { return &b }

// interactivePrinter builds a printer whose TTY probes report interactive, with
// the given stdin, so the prompter's polarity display and read path can be
// exercised (A6 injectable seam).
func interactivePrinter(stdin, stderr *bytes.Buffer, interactive bool) *ui.Printer {
	return ui.New(ui.Options{
		Stdin:     stdin,
		Stdout:    &bytes.Buffer{},
		Stderr:    stderr,
		Mode:      ui.ColorNever,
		StdinTTY:  boolPtr(interactive),
		StderrTTY: boolPtr(interactive),
	})
}

// TestPromptPolarity asserts O12: destructive/bulk questions show [y/N] and
// benign-continue questions show [Y/n], and the default drives an empty answer.
func TestPromptPolarity(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		defaultYes bool
		wantSuffix string
		wantAnswer bool
	}{
		{"destructive default no, empty", "\n", false, "[y/N]", false},
		{"destructive explicit yes", "y\n", false, "[y/N]", true},
		{"benign default yes, empty", "\n", true, "[Y/n]", true},
		{"benign explicit no", "n\n", true, "[Y/n]", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			pr := interactivePrinter(&bytes.Buffer{}, &stderr, true)
			p := &prompter{printer: pr, reader: bufio.NewReader(strings.NewReader(tc.input))}
			got, err := p.Confirm("proceed?", tc.defaultYes)
			if err != nil {
				t.Fatalf("Confirm errored: %v", err)
			}
			if got != tc.wantAnswer {
				t.Errorf("answer = %v, want %v", got, tc.wantAnswer)
			}
			if !strings.Contains(stderr.String(), tc.wantSuffix) {
				t.Errorf("prompt %q missing polarity %q", stderr.String(), tc.wantSuffix)
			}
			// Prompts are commentary: they never touch stdout (O1) — guaranteed by
			// writing through the Err() face only.
		})
	}
}

// TestPromptNonInteractiveRefuses asserts the §1.2 non-interactive stance: no
// terminal → a *nonInteractiveError, not an answer.
func TestPromptNonInteractiveRefuses(t *testing.T) {
	pr := interactivePrinter(&bytes.Buffer{}, &bytes.Buffer{}, false)
	p := &prompter{printer: pr, reader: bufio.NewReader(strings.NewReader(""))}
	ok, err := p.Confirm("stow everything?", false)
	if ok {
		t.Errorf("non-interactive prompt must not answer yes")
	}
	var nie *nonInteractiveError
	if !errors.As(err, &nie) {
		t.Errorf("want *nonInteractiveError, got %v", err)
	}
}

// TestPromptAssumeYes asserts -y pre-answers a stated-intent confirmation
// without reading, in any context (D2/D9 — for the commands whose prompter
// honors it).
func TestPromptAssumeYes(t *testing.T) {
	pr := interactivePrinter(&bytes.Buffer{}, &bytes.Buffer{}, false) // even non-interactive
	p := &prompter{printer: pr, reader: bufio.NewReader(strings.NewReader("")), assumeYes: true}
	ok, err := p.Confirm("remove the orphan?", false)
	if err != nil || !ok {
		t.Errorf("assumeYes should answer yes without error, got ok=%v err=%v", ok, err)
	}
}
