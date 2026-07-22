package cli

import (
	"github.com/spf13/cobra"

	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/ui"
)

// newCheckCmd builds the check leaf (§2.4): a read, --json only. Its report is
// the requested data (stdout); findings present means exit 1.
func (e *env) newCheckCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "check",
		GroupID: groupMaintain,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.runCheck(asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Machine-readable report")
	return cmd
}

func (e *env) runCheck(asJSON bool) error {
	app, warns, err := e.load(false)
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	rep, err := app.Check()
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	e.renderWarnings(warns)
	e.renderWarnings(rep.Warnings)
	if len(rep.Findings) > 0 {
		e.status = exitNegative // check found findings (A3 exit 1)
	}
	if asJSON {
		return e.writeJSON(checkJSON(rep))
	}
	e.renderCheck(rep)
	return nil
}

// renderCheck prints the classification report to stdout.
func (e *env) renderCheck(rep *ops.CheckReport) {
	out := e.pr().Out()
	if len(rep.Findings) == 0 {
		e.pr().Notef("check: every ledgered link is healthy")
		return
	}
	for _, f := range rep.Findings {
		out.Println(out.Style(classRole(f.Class), f.Class.String()) + "  " + f.Entry.Link + "  " + out.Style(ui.RoleMuted, f.Evidence))
	}
}

// newCleanCmd builds the clean leaf (§2.4): --force. --yes answers the orphan
// prompt (stated intent), so clean's prompter honors it; --force skips the
// prompt in any context (CleanRequest.Force).
func (e *env) newCleanCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:     "clean",
		GroupID: groupMaintain,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.runClean(force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Remove orphans without confirmation in any context")
	return cmd
}

func (e *env) runClean(force bool) error {
	app, warns, err := e.load(true) // clean honors -y for the orphan confirm
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	res, err := app.Clean(ops.CleanRequest{Force: force})
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	e.renderWarnings(warns)
	e.renderClean(res)
	if res.Failed() {
		e.status = exitNegative
	}
	return nil
}

// renderClean prints a clean run to stderr (a write command's output is
// commentary; §6.4's per-finding report with its outcome).
func (e *env) renderClean(res *ops.CleanResult) {
	pr := e.pr()
	for _, w := range res.Warnings {
		pr.Warningf("%s", w.Detail)
		if w.Fix != "" {
			pr.Fixf("%s", w.Fix)
		}
	}
	acted := 0
	for _, f := range res.Findings {
		switch f.Outcome {
		case ops.OutcomeRemoved:
			pr.Announcef("removed %s (%s): %s", f.Entry.Link, f.Class, f.Evidence)
			acted++
		case ops.OutcomePruned:
			pr.Announcef("pruned entry %s: %s", f.Entry.Link, f.Evidence)
			acted++
		case ops.OutcomeDeclined:
			pr.Notef("kept %s (%s): declined", f.Entry.Link, f.Class)
		case ops.OutcomeUntouched:
			pr.Notef("left %s (%s): %s", f.Entry.Link, f.Class, f.Evidence)
		case ops.OutcomeFailed:
			pr.Errorf("could not clean %s: %v", f.Entry.Link, f.Err)
		}
	}
	if acted == 0 && len(res.Findings) == 0 {
		pr.Notef("clean: nothing to do")
	}
}

// newRebuildCmd builds the rebuild leaf (§2.4): no flags, the only full tree
// walk. Its output is commentary (stderr).
func (e *env) newRebuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rebuild",
		GroupID: groupMaintain,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.runRebuild()
		},
	}
	return cmd
}

func (e *env) runRebuild() error {
	app, warns, err := e.load(false)
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	res, err := app.Rebuild()
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	e.renderWarnings(warns)
	e.renderWarnings(res.Warnings)
	pr := e.pr()
	total := 0
	for root, n := range res.Counts {
		pr.Announcef("rebuilt %s: %d link(s)", homeAbbrev(root), n)
		total += n
	}
	pr.Notef("rebuild: recorded %d link(s) across %d target(s)", total, len(res.Counts))
	return nil
}
