package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rocne/dstow/internal/git"
	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/ui"
)

// finish maps a non-nil error out of cobra's Execute to an exit code and
// renders it (A3, the one place exit codes live). An error with the command
// body never entered is a cobra flag/arg/unknown-command failure — a usage
// error (exit 2); everything else is a typed domain error cli renders through
// the printer and classifies.
func (e *env) finish(err error) int {
	if !e.entered {
		return e.renderUsageError(err)
	}
	return e.renderError(err)
}

// renderUsageError renders a cobra usage failure (bad flag, wrong arg count,
// unknown command, invalid --color) as a §1.4 line and returns exit 2.
func (e *env) renderUsageError(err error) int {
	pr := e.pr()
	pr.Errorf("%s", err.Error())
	pr.Fixf("run 'dstow --help' or 'dstow <command> --help' for usage")
	return exitUsage
}

// renderError renders a typed domain error through the printer and returns its
// exit code (A3). The error: line is the full §1.4 message; a fix: line adds
// the machine-stable runnable remedy where the error's type names one (O2).
func (e *env) renderError(err error) int {
	pr := e.pr()
	pr.Errorf("%s", err.Error())
	if fix := fixFor(err); fix != "" {
		pr.Fixf("%s", fix)
	}
	return classifyExit(err)
}

// Exit codes (A3), named once.
const (
	exitOK       = 0 // success
	exitNegative = 1 // negative answer: a package failed, a field unset/empty, findings present
	exitUsage    = 2 // usage error
	exitRefusal  = 3 // refusal / environment
)

// usageError is a cli-side malformed-invocation error a RunE raises when cobra
// cannot (an operand-shape mismatch the Args validator does not catch). It maps
// to exit 2, the same as cobra's own flag/arg failures.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// bulkRefusalError is the non-interactive bulk gate (D2/D9): a deploy verb with
// no names and no --all cannot ask "everything?" without a terminal. It maps to
// exit 3 (refusal) and its fix names --all.
type bulkRefusalError struct{ verb string }

func (e *bulkRefusalError) Error() string {
	return fmt.Sprintf(
		"refusing to %s every package without confirmation in a non-interactive session; name packages or pass --all to act on everything",
		e.verb)
}

// classifyExit is the A3 exit-code map over the typed domain errors, decided by
// errors.As so wrapping never defeats it. The refusal/environment shapes map to
// 3; everything else — a named thing that resolves to nothing, engine failures,
// I/O, generic git command failures — is a general negative outcome (1). Exit 2
// is reserved for malformed invocation (cobra's flag/arg/unknown-command
// failures, and cli's own usageError), never for a well-formed name that simply
// is not there (ruled on #47: the not-found family → 1).
func classifyExit(err error) int {
	var usage *usageError
	if errors.As(err, &usage) {
		return exitUsage
	}
	var bulk *bulkRefusalError
	if errors.As(err, &bulk) {
		return exitRefusal
	}
	// Refusal / environment (exit 3).
	var (
		corrupt      *ledger.CorruptError
		newer        *ledger.NewerVersionError
		locked       *ledger.LockedError
		notInstalled *git.NotInstalledError
		diverged     *git.DivergedError
		ambiguous    *ops.AmbiguousNameError
		srcAmbig     *ops.SourceAmbiguousError
		srcDeclined  *ops.SourceDeclinedError
		srcUnres     *ops.SourceUnresolvableError
		renameReq    *ops.RenameRequestedError
		stillStowed  *ops.StillStowedError
		unsavedWork  *ops.UnsavedWorkError
		foldConflict *ops.FoldConflictError
		nonInter     *nonInteractiveError
		inHook       *hookWriteError
	)
	switch {
	case errors.As(err, &corrupt),
		errors.As(err, &newer),
		errors.As(err, &locked),
		errors.As(err, &notInstalled),
		errors.As(err, &diverged),
		errors.As(err, &ambiguous),
		errors.As(err, &srcAmbig),
		errors.As(err, &srcDeclined),
		errors.As(err, &srcUnres),
		errors.As(err, &renameReq),
		errors.As(err, &stillStowed),
		errors.As(err, &unsavedWork),
		errors.As(err, &foldConflict),
		errors.As(err, &nonInter),
		errors.As(err, &inHook):
		return exitRefusal
	}

	// A named thing that resolves to nothing (a package/repo not found; a theme
	// not found) is a negative answer, not a usage error — it falls through to
	// exitNegative with every other domain outcome (ruled on #47).
	return exitNegative
}

// fixFor derives the runnable/pointed remedy for a typed error from its fields
// (never by parsing its message), for the fix: line. It returns "" where the
// error carries no distinct remedy beyond its own prose.
func fixFor(err error) string {
	var corrupt *ledger.CorruptError
	if errors.As(err, &corrupt) {
		return "dstow rebuild   # reconstruct the ledger from disk (or restore it from a backup)"
	}
	var newer *ledger.NewerVersionError
	if errors.As(err, &newer) {
		return "upgrade dstow to a build that understands ledger schema " + fmt.Sprint(newer.FileVersion)
	}
	var locked *ledger.LockedError
	if errors.As(err, &locked) {
		return "wait for the other dstow operation to finish, then retry"
	}
	var notInstalled *git.NotInstalledError
	if errors.As(err, &notInstalled) {
		return "install git and retry"
	}
	var diverged *git.DivergedError
	if errors.As(err, &diverged) {
		return "resolve the divergence in " + diverged.Dir + " yourself, or remove and re-add the repo"
	}
	var ambiguous *ops.AmbiguousNameError
	if errors.As(err, &ambiguous) {
		return "re-run naming one of: " + fqnList(ambiguous.Matches)
	}
	var srcAmbig *ops.SourceAmbiguousError
	if errors.As(err, &srcAmbig) {
		return "dstow repo add " + srcAmbig.Github.String() + "   (or " + srcAmbig.Local.String() + ")"
	}
	var srcUnres *ops.SourceUnresolvableError
	if errors.As(err, &srcUnres) {
		return "dstow repo add github:" + srcUnres.Input + "   (or local:/absolute/path)"
	}
	var stillStowed *ops.StillStowedError
	if errors.As(err, &stillStowed) {
		return "dstow repo remove " + stillStowed.FQN.String() + " --unstow   (or --force to keep the links)"
	}
	var unsavedWork *ops.UnsavedWorkError
	if errors.As(err, &unsavedWork) {
		return "push or discard the work in " + unsavedWork.Dir + ", or re-run with --force"
	}
	var notFound *ops.NotFoundError
	if errors.As(err, &notFound) {
		return "dstow list   # shows every registered repo and package"
	}
	var nonInter *nonInteractiveError
	if errors.As(err, &nonInter) {
		return "rerun in an interactive terminal, or pass --all / --force / --unstow (or a qualified name)"
	}
	var bulk *bulkRefusalError
	if errors.As(err, &bulk) {
		return "dstow " + bulk.verb + " --all   # act on every package without asking"
	}
	var inHook *hookWriteError
	if errors.As(err, &inHook) {
		return "run this command outside the hook; reads (dstow list, info, status, check) work from inside one"
	}
	return ""
}

// fqnList joins FQNs for a remedy line.
func fqnList(fqns []name.FQN) string {
	ss := make([]string, len(fqns))
	for i, f := range fqns {
		ss[i] = f.String()
	}
	return strings.Join(ss, ", ")
}

// pr returns the printer, building a plain fallback if PersistentPreRunE never
// ran (e.g. a bad --color value aborted it) so early errors still render.
func (e *env) pr() *ui.Printer {
	if e.printer == nil {
		e.printer = ui.New(ui.Options{
			Stdin:  e.stdin,
			Stdout: e.stdout,
			Stderr: e.stderr,
			Mode:   ui.ColorAuto,
		})
	}
	return e.printer
}
