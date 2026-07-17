// Package ops is dstow's application core (A13): the verbs as deep modules
// composing config, repo, engine, ledger, and hooks into structured results.
// This file carries the composed environment and the shared result
// vocabulary; the deploy verbs live in deploy.go and adopt in adopt.go.
//
// ops returns data, never output (A4): every diagnostic is a value — cli
// renders them through the printer. The one exception to "no side channels"
// is hook execution, whose streams are the caller-injected hooks.Runner;
// ops itself never touches a process stream.
package ops

import (
	"time"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/hooks"
	"github.com/rocne/dstow/internal/repo"
)

// App is the composed environment a verb runs against. The caller (cli)
// loads the global pieces once — global config, the registry, DSTOW_PATH —
// and hands them in; ops loads the per-repo and per-package levels itself,
// per invocation, at use time.
type App struct {
	Global     *config.GlobalLevel // loaded global level; nil when absent
	Repos      []repo.Repo         // the built repo set (unordered)
	LedgerPath string              // production: ledger.Path()
	GlobalDir  string              // global config dir; hooks' GlobalScope.Dir
	Hooks      hooks.Runner        // injected hook streams (H6)
	Prompt     Prompter            // confirmation seam; cli owns O12 rendering
	Now        func() time.Time    // entry RecordedAt clock; nil means time.Now
}

// now returns the injected clock or the wall clock.
func (a *App) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// Prompter answers confirmation prompts of stated intent (D2). ops asks in
// complete prose; the implementation owns rendering, polarity display, and
// the non-interactive stance — a non-interactive implementation returns an
// error naming the unambiguous form rather than answering (§1.2).
type Prompter interface {
	// Confirm asks a yes/no question. defaultYes selects the O12 polarity:
	// destructive/bulk questions default No, benign-continue questions
	// default Yes.
	Confirm(question string, defaultYes bool) (bool, error)
}

// Warning is one diagnostic as data (A4), the same shape config and repo
// speak: Source is where it arose, Detail is complete prose, Fix an
// optional remedy.
type Warning struct {
	Source string
	Detail string
	Fix    string
}

// warnConfig converts config warnings into ops warnings.
func warnConfig(ws []config.Warning) []Warning {
	out := make([]Warning, 0, len(ws))
	for _, w := range ws {
		out = append(out, Warning{Source: w.Source, Detail: w.Detail, Fix: w.Fix})
	}
	return out
}

// warnRepo converts repo warnings into ops warnings.
func warnRepo(ws []repo.Warning) []Warning {
	out := make([]Warning, 0, len(ws))
	for _, w := range ws {
		out = append(out, Warning{Source: w.Source, Detail: w.Detail, Fix: w.Fix})
	}
	return out
}
