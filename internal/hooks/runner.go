package hooks

import (
	"fmt"
	"io"
	"os/exec"
)

// Runner carries the injected streams a hook runs with (A4: only ui touches
// the process streams; hooks receives them). Both hook output streams land on
// Stderr (H6) — nothing a hook prints is dstow's answer to a question, so hook
// output is commentary definitionally — and Stdin passes through to the hook.
type Runner struct {
	Stdin  io.Reader // passed through to the hook (H6)
	Stderr io.Writer // both hook streams land here (H6)
}

// run directly execs the hook at path (M6: shebang decides the interpreter),
// with cwd set to the scope's own directory (H5) and the composed environment.
// A non-zero exit or an exec failure comes back as a non-nil error, which the
// caller wraps in a HookError.
func (r Runner) run(dir, path string, env []string) error {
	cmd := exec.Command(path)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = r.Stdin
	cmd.Stdout = r.Stderr // H6: hook stdout is commentary, onto dstow's stderr
	cmd.Stderr = r.Stderr
	return cmd.Run()
}

// HookError reports that one firing failed — a non-zero exit, an exec error,
// or the scope's hooks directory being unreadable at discovery. hooks only
// classifies: it names the Level and Phase off which the caller (ops, ticket
// #44) applies REQUIREMENTS §9.1.4 blocking (a failed package-pre blocks that
// package; a failed repo/global-pre blocks everything under it; a failed post
// marks its scope failed but completed work stays).
type HookError struct {
	Level  Level
	Action Action
	Phase  Phase
	Path   string // the hook file that failed, or the hooks dir when discovery failed
	Err    error
}

func (e *HookError) Error() string {
	return fmt.Sprintf("%s %s-%s hook %s failed: %v", e.Level, e.Phase, e.Action, e.Path, e.Err)
}

func (e *HookError) Unwrap() error { return e.Err }
