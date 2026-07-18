package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow/internal/engine"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/ui"
)

// verbFor maps a leaf name to its engine verb.
func verbFor(cmdName string) engine.Verb {
	switch cmdName {
	case "unstow":
		return engine.VerbUnstow
	case "restow":
		return engine.VerbRestow
	default:
		return engine.VerbStow
	}
}

// newStowCmd builds stow, unstow, or restow — identical in shape (§2.4): name
// operands or --all, -n/--dry-run, and (for stow/restow only, since --adopt
// pre-accepts stow's occupied refusal, D15) --adopt.
func (e *env) newStowCmd(cmdName, help string) *cobra.Command {
	var (
		all    bool
		adopt  bool
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:                   cmdName,
		Short:                 firstLine(help),
		DisableFlagsInUseLine: true,
		ValidArgsFunction:     e.completeNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.runDeploy(verbFor(cmdName), cmdName, args, all, adopt, dryRun)
		},
	}
	staticHelp(cmd, help)
	cmd.Flags().BoolVar(&all, "all", false, "Every package of every registered repo, without asking")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Show the plan; change nothing")
	if cmdName != "unstow" {
		cmd.Flags().BoolVar(&adopt, "adopt", false, "Adopt a real file at an expected path instead of refusing")
	}
	return cmd
}

// runDeploy composes and renders a stow/unstow/restow run. The bulk gate is
// cli's, upstream of ops (D2/D9): with no names and no --all, an interactive
// session asks once; a non-interactive one refuses (pass --all). -y/--yes never
// answers the bulk prompt, so the deploy prompter is built with honorYes false.
func (e *env) runDeploy(verb engine.Verb, cmdName string, names []string, all, adopt, dryRun bool) error {
	app, warns, err := e.load(false)
	if err != nil {
		e.renderWarnings(warns)
		return err
	}

	if len(names) == 0 && !all {
		if !e.printer.Interactive() {
			e.renderWarnings(warns)
			return &bulkRefusalError{verb: cmdName}
		}
		ok, perr := app.Prompt.Confirm(fmt.Sprintf("%s every package of every registered repo?", cmdName), false)
		if perr != nil {
			e.renderWarnings(warns)
			return perr
		}
		if !ok {
			e.renderWarnings(warns)
			e.printer.Notef("nothing to %s", cmdName)
			return nil
		}
	}

	res, err := app.Deploy(ops.DeployRequest{Verb: verb, Names: names, Adopt: adopt, DryRun: dryRun})
	if err != nil {
		e.renderWarnings(warns)
		return err
	}

	e.renderWarnings(warns)
	e.renderDeploy(cmdName, res)
	if res.Failed() {
		e.status = exitNegative
	}
	return nil
}

// renderDeploy prints a deploy run to stderr (O1: a deploy is commentary, not a
// query — its data is the exit code). Run-level announcements first, then the
// per-package run-lines (O8: verb name result, failure detail indented), then
// warnings, then a summary; a dry-run reads identically over the plan.
func (e *env) renderDeploy(verb string, res *ops.DeployResult) {
	pr := e.pr()

	for _, note := range res.Notes {
		pr.Announcef("%s", note)
	}

	var ok, bad int
	for i := range res.Packages {
		p := &res.Packages[i]
		e.renderPackageResult(verb, p, res.DryRun)
		if p.Status == ops.StatusSucceeded {
			ok++
		} else {
			bad++
		}
	}

	for _, w := range res.Warnings {
		pr.Warningf("%s", w.Detail)
		if w.Fix != "" {
			pr.Fixf("%s", w.Fix)
		}
	}
	for _, rerr := range res.RunErrors {
		pr.Errorf("%s", rerr.Error())
	}

	for _, p := range res.Pruned {
		pr.Announcef("pruned a stale ledger entry: %s", p.Evidence)
	}

	suffix := ""
	if res.DryRun {
		suffix = " (dry run — nothing changed)"
	}
	pr.Notef("%s: %d succeeded, %d failed%s", verb, ok, bad, suffix)
}

// renderPackageResult prints one package's run-line and its indented detail.
func (e *env) renderPackageResult(verb string, p *ops.PackageResult, dryRun bool) {
	pr := e.pr()
	label := deployName(p)

	slot := ui.SlotStowed
	word := p.Status.String()
	switch p.Status {
	case ops.StatusFailed, ops.StatusBlocked:
		slot = ui.SlotError
	case ops.StatusNotFound:
		slot = ui.SlotWarning
	}
	if dryRun && p.Status == ops.StatusSucceeded {
		word = "planned"
	}
	pr.Err().Println(fmt.Sprintf("%s %s %s", verb, pr.Err().Style(ui.SlotName, label), pr.Err().Style(slot, word)))

	for _, note := range p.Notes {
		pr.Err().Println("    " + note)
	}
	for _, act := range p.Actions {
		pr.Err().Println("    " + actionLine(act))
	}
	if p.Err != nil {
		pr.Err().Println("    " + p.Err.Error())
	}
	for _, w := range p.Warnings {
		pr.Warningf("%s", w.Detail)
		if w.Fix != "" {
			pr.Fixf("%s", w.Fix)
		}
	}
}

// deployName is the display label for a package result: its FQN, or the raw
// operand when nothing resolved.
func deployName(p *ops.PackageResult) string {
	if p.FQN.Scheme == "" {
		return p.Operand
	}
	return p.FQN.String()
}

// actionLine renders one planned/executed action.
func actionLine(a engine.Action) string {
	switch a.Kind {
	case engine.LinkCreated:
		return "+ " + a.Path
	case engine.LinkRemoved:
		return "- " + a.Path
	case engine.FileMoved:
		return "~ " + a.Path + " (adopted)"
	default:
		return "  " + a.Path
	}
}

// newAdoptCmd builds the adopt leaf (§2.4): a file (path operand) and an
// optional package, or --occupied over a named package; -n/--dry-run and
// --force. The differing-content confirmation is a data-loss guard, so its
// prompter never honors -y (--force is the override).
func (e *env) newAdoptCmd() *cobra.Command {
	var (
		occupied bool
		dryRun   bool
		force    bool
	)
	cmd := &cobra.Command{
		Use:               "adopt",
		Short:             firstLine(adoptHelp),
		ValidArgsFunction: e.completeNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.runAdopt(args, occupied, dryRun, force)
		},
	}
	staticHelp(cmd, adoptHelp)
	cmd.Flags().BoolVar(&occupied, "occupied", false, "All occupied paths of the named package")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Show the plan; change nothing")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite differing package content without asking")
	return cmd
}

// runAdopt routes the two adopt shapes. With a path operand and no package,
// dstow lists the ranked candidates named as remedies; an interactive picker is
// a v1 reserved doorway (§10 "Interactive selection"), so v1 never prompts a
// choice. With a package it runs the adoption.
func (e *env) runAdopt(args []string, occupied, dryRun, force bool) error {
	app, warns, err := e.load(false)
	if err != nil {
		e.renderWarnings(warns)
		return err
	}

	req := ops.AdoptRequest{Occupied: occupied, DryRun: dryRun, Force: force}
	switch {
	case occupied:
		if len(args) != 1 {
			return &usageError{fmt.Errorf("adopt --occupied takes exactly one package name")}
		}
		req.Package = args[0]
	case len(args) == 1:
		// A lone operand: a file to adopt (no package → show candidates) or, if
		// it is a name expression, an error (adopt needs a file). Only a path
		// operand is a file; classify by the operand rule (§1.3).
		if name.IsPathOperand(args[0]) {
			return e.renderCandidates(app, args[0], warns)
		}
		return &usageError{fmt.Errorf("adopt needs a file to import; %q is a name, not a path (a path starts with / ~/ ./ or ../)", args[0])}
	case len(args) == 2:
		req.File, req.Package = args[0], args[1]
	default:
		return &usageError{fmt.Errorf("adopt takes <file> [<package>], or --occupied <package>")}
	}

	res, err := app.Adopt(req)
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	e.renderWarnings(warns)
	e.renderAdopt(res)
	if res.Failed() {
		e.status = exitNegative
	}
	return nil
}

// renderCandidates prints the ranked adoption candidates for a path with no
// package named (§2.4). An interactive pick here is deferred to the §10
// "Interactive selection" doorway; v1 lists the candidates as remedies (a
// fix: line names the next command), in every context.
func (e *env) renderCandidates(app *ops.App, file string, warns []ops.Warning) error {
	cands, cwarns, err := app.AdoptCandidates(file)
	e.renderWarnings(warns)
	e.renderWarnings(cwarns)
	if err != nil {
		return err
	}
	pr := e.pr()
	if len(cands) == 0 {
		pr.Notef("no package could adopt %s", homeAbbrev(file))
		return nil
	}
	pr.Announcef("packages that could adopt %s (ranked):", homeAbbrev(file))
	for _, c := range cands {
		pr.Err().Println("    " + pr.Err().Style(ui.SlotName, c.FQN.String()) + "  -> " + c.Source)
	}
	pr.Fixf("dstow adopt %s <package>   # name one of the above", homeAbbrev(file))
	return nil
}

// renderAdopt prints an adopt run to stderr.
func (e *env) renderAdopt(res *ops.AdoptResult) {
	pr := e.pr()
	for _, note := range res.Notes {
		pr.Announcef("%s", note)
	}
	verb := "adopt"
	if res.DryRun {
		verb = "would adopt"
	}
	for _, m := range res.Moves {
		pr.Err().Println(fmt.Sprintf("%s %s -> %s::%s", verb, homeAbbrev(m.File), pr.Err().Style(ui.SlotName, res.FQN.String()), m.Source))
	}
	for _, s := range res.Skipped {
		pr.Notef("skipped %s: %s", homeAbbrev(s.File), s.Reason)
	}
	for _, w := range res.Warnings {
		pr.Warningf("%s", w.Detail)
		if w.Fix != "" {
			pr.Fixf("%s", w.Fix)
		}
	}
	for _, err := range res.Errs {
		pr.Errorf("%s", err.Error())
	}
	for _, p := range res.Pruned {
		pr.Announcef("pruned a stale ledger entry: %s", p.Evidence)
	}
}
