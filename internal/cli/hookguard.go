package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow/internal/hooks"
)

// annotationWrite marks a command as a write command for H7 (DESIGN §5): a
// non-empty value means the command refuses when dstow runs from inside a
// hook. The marking is a cobra annotation rather than a list of names held
// somewhere else, so a command declares its own kind beside its definition and
// the guard has exactly one place to look.
const annotationWrite = "dstow:write"

// writeCommand is the annotations map every H7 write command carries. The
// refusing set is enumerated in DESIGN §5: stow, unstow, restow, adopt, clean,
// rebuild, and the four repo verbs.
func writeCommand() map[string]string {
	return map[string]string{annotationWrite: "yes"}
}

// hookWriteError is the H7 refusal: a write command invoked from inside a
// hook. It maps to exit 3 (refusal/environment, A3) — dstow understood the
// command and declined the environment it was asked to run in.
type hookWriteError struct {
	// path is the full command path (`dstow repo add`), so the message names
	// what was refused rather than making the reader infer it.
	path string
}

func (e *hookWriteError) Error() string {
	return fmt.Sprintf(
		"refusing to run '%s' from inside a dstow hook: %s is set, and write commands "+
			"refuse while a hook is running so a deploy can never re-enter itself",
		e.path, hooks.EnvAction)
}

// refuseInHook is the H7 guard, called once for every command from the root's
// PersistentPreRunE (the one place every command passes through). Reads —
// everything not annotated — are unaffected. The check is deliberately
// flag-blind: H7 enumerates verbs, not effects, so `stow --dry-run` refuses
// with the rest (ruled at #139, in the retractable direction — relaxing a
// refusal later is additive).
func refuseInHook(cmd *cobra.Command) error {
	if cmd.Annotations[annotationWrite] == "" || !hooks.InHook() {
		return nil
	}
	return &hookWriteError{path: cmd.CommandPath()}
}
