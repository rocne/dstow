package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/rocne/dstow/internal/ui"
)

// prompter is cli's ops.Prompter (O12): it renders confirmations on stderr
// through the printer, displays the polarity ([y/N] destructive/bulk default
// No, [Y/n] benign-continue default Yes), and gates on ui.Interactive(). A
// non-interactive prompt returns a *nonInteractiveError naming the unambiguous
// form rather than answering (§1.2), which the exit-code map sends to 3.
//
// assumeYes carries -y/--yes, but only for the commands whose prompts are
// confirmations of stated intent (clean, repo add). Guard prompts (adopt's
// differing-content, repo remove's still-stowed) construct the prompter with
// assumeYes false, so -y can never bypass a guard (§2.2, D2/D9).
type prompter struct {
	printer   *ui.Printer
	reader    *bufio.Reader
	assumeYes bool
}

// newPrompter builds the confirmation seam for one command. honorYes says
// whether this command's prompts may be pre-answered by -y/--yes; the actual
// assume-yes is that AND the flag being set.
func (e *env) newPrompter(honorYes bool) *prompter {
	return &prompter{
		printer:   e.printer,
		reader:    bufio.NewReader(e.stdin),
		assumeYes: honorYes && e.yes,
	}
}

// Confirm implements ops.Prompter.
func (p *prompter) Confirm(question string, defaultYes bool) (bool, error) {
	if p.assumeYes {
		// -y pre-answers this confirmation of stated intent (never reached for
		// guard or ambiguity prompts — those build the prompter with assumeYes
		// false or never route through Confirm at all).
		return true, nil
	}
	if !p.printer.Interactive() {
		return false, &nonInteractiveError{question: question}
	}

	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}
	// The prompt is commentary: stderr, styled with the note slot (O1/O2).
	err := p.printer.Err()
	err.Printf("%s %s ", err.Style(ui.RoleNote, question), suffix)

	line, rerr := p.reader.ReadString('\n')
	if rerr != nil && line == "" {
		// EOF with nothing typed: take the default rather than error — the stream
		// was interactive by the TTY probe, so this is a closed pipe edge.
		return defaultYes, nil
	}
	return parseYesNo(line, defaultYes), nil
}

// parseYesNo reads a yes/no answer: empty takes the default, a leading y/Y is
// yes, anything else is no.
func parseYesNo(line string, defaultYes bool) bool {
	s := strings.TrimSpace(strings.ToLower(line))
	if s == "" {
		return defaultYes
	}
	return strings.HasPrefix(s, "y")
}

// nonInteractiveError is the §1.2 refusal a prompt raises with no terminal to
// ask: it names the situation and points at the stated-intent flags that
// pre-answer it. It maps to exit 3 (refusal/environment).
type nonInteractiveError struct {
	question string
}

func (e *nonInteractiveError) Error() string {
	return fmt.Sprintf(
		"this operation needs a yes/no answer, but stdin or stderr is not a terminal: %q. "+
			"Rerun in an interactive terminal, or state the answer up front — pass the flag the message names "+
			"(--all, --force, --unstow) or a fully qualified name",
		strings.TrimSpace(e.question))
}
