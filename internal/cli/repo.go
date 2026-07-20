package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/ui"
)

// newRepoCmd builds the repo group (§2.4). A bare group prints its help (§2.1).
func (e *env) newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repo",
		Short:   shorts["repo"],
		Long:    repoLong,
		GroupID: groupGroups,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		e.newRepoAddCmd(),
		e.newRepoRemoveCmd(),
		e.newRepoSyncCmd("update"),
		e.newRepoSyncCmd("upgrade"),
	)
	return cmd
}

// newRepoAddCmd builds repo add (§2.4): <source>, --stow. The §1.2 confirm flow
// (github interpretation, encoding continue-or-rename) is stated intent, so the
// prompter honors -y.
func (e *env) newRepoAddCmd() *cobra.Command {
	var stow bool
	cmd := &cobra.Command{
		Use:     "add <source>",
		Short:   "Register a repo from a source (path, URL, github:owner/name)",
		Long:    repoAddLong,
		Example: repoAddExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.runRepoAdd(args[0], stow)
		},
	}
	cmd.Flags().BoolVar(&stow, "stow", false, "After adding, stow this repo's packages (exclusions apply)")
	return cmd
}

func (e *env) runRepoAdd(source string, stow bool) error {
	app, warns, err := e.load(true) // add's confirms are stated intent → honor -y
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	res, err := app.RepoAdd(ops.RepoAddRequest{Source: source, Stow: stow})
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	e.renderWarnings(warns)
	e.renderWarnings(res.Warnings)

	pr := e.pr()
	for _, note := range res.Notes {
		pr.Announcef("%s", note)
	}
	if res.Cloned {
		pr.Announcef("cloned %s", res.FQN)
	}
	if !res.AlreadyPresent {
		pr.Announcef("registered %s", res.FQN)
	}
	if len(res.Packages) > 0 {
		pr.Announcef("packages: %s", strings.Join(res.Packages, ", "))
	}
	if len(res.Shadowed) > 0 {
		pr.Warningf("these package names now need qualification (shared with another repo): %s", strings.Join(res.Shadowed, ", "))
	}
	if res.Deploy != nil {
		e.renderDeploy("stow", res.Deploy)
		if res.Deploy.Failed() {
			e.status = exitNegative
		}
	}
	return nil
}

// newRepoRemoveCmd builds repo remove (§2.4): <repo>, --unstow, --force. The
// still-stowed and unsaved-work guards are bypassed only by --unstow/--force,
// never by -y, so the prompter does not honor -y.
func (e *env) newRepoRemoveCmd() *cobra.Command {
	var (
		unstow bool
		force  bool
	)
	cmd := &cobra.Command{
		Use:               "remove <repo>",
		Short:             "Unregister a repo (deletes managed clones only)",
		Long:              repoRemoveLong,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: e.completeRepos,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.runRepoRemove(args[0], unstow, force)
		},
	}
	cmd.Flags().BoolVar(&unstow, "unstow", false, "Unstow the repo's packages first, without prompting")
	cmd.Flags().BoolVar(&force, "force", false, "Override both guards (unsaved work will be lost)")
	return cmd
}

func (e *env) runRepoRemove(repoName string, unstow, force bool) error {
	app, warns, err := e.load(false) // guards: -y never bypasses them
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	res, err := app.RepoRemove(ops.RepoRemoveRequest{Repo: repoName, Unstow: unstow, Force: force})
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	e.renderWarnings(warns)
	e.renderWarnings(res.Warnings)
	if res.Unstowed != nil {
		e.renderDeploy("unstow", res.Unstowed)
	}
	pr := e.pr()
	for _, note := range res.Notes {
		pr.Announcef("%s", note)
	}
	return nil
}

// newRepoSyncCmd builds repo update or repo upgrade (§2.4): optional repo
// operands; empty means every remote repo. A per-repo failure is exit 1.
func (e *env) newRepoSyncCmd(cmdName string) *cobra.Command {
	short := "Download remote repo changes; touch nothing on disk"
	if cmdName == "upgrade" {
		short = "Fast-forward clean clones to what update downloaded"
	}
	cmd := &cobra.Command{
		Use:               cmdName + " [<repo>...]",
		Short:             short,
		Long:              repoSyncLong,
		Example:           repoSyncExample,
		ValidArgsFunction: e.completeRepos,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.runRepoSync(cmdName, args)
		},
	}
	return cmd
}

func (e *env) runRepoSync(cmdName string, names []string) error {
	app, warns, err := e.load(false)
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	req := ops.RepoSyncRequest{Names: names}
	var res *ops.RepoSyncResult
	if cmdName == "upgrade" {
		res, err = app.RepoUpgrade(req)
	} else {
		res, err = app.RepoUpdate(req)
	}
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	e.renderWarnings(warns)
	e.renderWarnings(res.Warnings)
	e.renderRepoSync(cmdName, res)
	if res.Failed() {
		e.status = exitNegative
	}
	return nil
}

// renderRepoSync prints an update/upgrade run to stderr (commentary).
func (e *env) renderRepoSync(cmdName string, res *ops.RepoSyncResult) {
	pr := e.pr()
	for _, r := range res.Repos {
		name := pr.Err().Style(ui.RoleName, r.FQN.String())
		switch {
		case r.Err != nil:
			pr.Errorf("%s: %v", r.FQN, r.Err)
		case r.Skipped:
			pr.Notef("%s: %s", r.FQN, r.Note)
		case cmdName == "update":
			pr.Announcef("fetched %s", name)
		case r.Changed:
			pr.Announcef("upgraded %s: %s -> %s", name, r.Old, r.New)
		default:
			pr.Notef("%s: %s", r.FQN, r.Note)
		}
	}
}
