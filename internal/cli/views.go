package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/ui"
)

// newListCmd builds the list leaf (§2.4): a scope operand plus --repos /
// --packages / --json. cli owns the flag mutual-exclusion (ops takes both).
func (e *env) newListCmd() *cobra.Command {
	var (
		reposOnly    bool
		packagesOnly bool
		asJSON       bool
	)
	cmd := &cobra.Command{
		Use:               "list [<name>]",
		Short:             shorts["list"],
		Long:              listLong,
		GroupID:           groupInspect,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: e.completeNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if reposOnly && packagesOnly {
				return &usageError{fmt.Errorf("--repos and --packages are mutually exclusive")}
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return e.runList(ops.ListRequest{Name: name, ReposOnly: reposOnly, PackagesOnly: packagesOnly}, asJSON)
		},
	}
	cmd.Flags().BoolVar(&reposOnly, "repos", false, "Repos only (source, scheme, bulk-exclusion)")
	cmd.Flags().BoolVar(&packagesOnly, "packages", false, "Packages only (repo-attributed; same-named entries shown with qualified names)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Machine-readable listing")
	return cmd
}

func (e *env) runList(req ops.ListRequest, asJSON bool) error {
	app, warns, err := e.load(false)
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	res, err := app.List(req)
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	e.renderWarnings(warns)
	e.renderWarnings(res.Warnings)
	if asJSON {
		return e.writeJSON(listJSON(res))
	}
	e.renderList(res)
	return nil
}

// renderList prints a listing to stdout (O1: a listing is the requested data).
func (e *env) renderList(res *ops.ListResult) {
	out := e.pr().Out()
	switch res.Kind {
	case ops.KindRepos:
		for _, r := range res.Repos {
			markers := repoMarkers(r)
			out.Println(out.Style(ui.RoleName, r.Display) + "  " + out.Style(ui.RoleMuted, homeAbbrev(r.Root)) + markers)
		}
	case ops.KindPackages:
		for _, p := range res.Packages {
			out.Println(out.Style(ui.RoleName, p.Display))
		}
	default: // KindPaths
		for _, p := range res.Paths {
			out.Println(p.Path)
		}
	}
}

// repoMarkers renders a repo row's flag markers (no priority; §2.3).
func repoMarkers(r ops.RepoListing) string {
	var m []string
	if r.Managed {
		m = append(m, "managed")
	}
	if r.Session {
		m = append(m, "session")
	}
	if r.ExcludedBulk {
		m = append(m, "excluded from bulk")
	}
	if len(m) == 0 {
		return ""
	}
	return "  (" + strings.Join(m, ", ") + ")"
}

// newInfoCmd builds the info leaf (§2.4): a scope operand, -f/--field
// (repeatable), -r/--recurse, --json.
func (e *env) newInfoCmd() *cobra.Command {
	var (
		fields  []string
		recurse bool
		asJSON  bool
	)
	cmd := &cobra.Command{
		Use:               "info [<name>]",
		Short:             shorts["info"],
		Long:              infoLong,
		Example:           infoExample,
		GroupID:           groupInspect,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: e.completeNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return e.runInfo(ops.InfoRequest{Name: name, Fields: fields, Recurse: recurse}, asJSON)
		},
	}
	cmd.Flags().StringArrayVarP(&fields, "field", "f", nil, "Only the named field(s); repeatable. One field prints its bare value; several print labeled lines")
	cmd.Flags().BoolVarP(&recurse, "recurse", "r", false, "Visit every applicable scope in turn, per-scope attributed; scopes a named field does not apply to are silently skipped")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Machine-readable: a flat object keyed by field name (an array of scope objects under --recurse)")
	return cmd
}

func (e *env) runInfo(req ops.InfoRequest, asJSON bool) error {
	app, warns, err := e.load(false)
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	res, err := app.Info(req)
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	e.renderWarnings(warns)
	e.renderWarnings(res.Warnings)

	// Exit status (§2.4): computed over the requested fields across scopes. With
	// no -f, the whole catalog is a plain dump (exit 0).
	e.status = e.infoExitStatus(req, res)

	if asJSON {
		return e.writeJSON(infoJSON(res, req.Recurse))
	}
	e.renderInfo(req, res)
	return nil
}

// infoExitStatus applies §2.4's exit rules over the requested fields, and
// renders the unknown/illegal diagnostics (naming a suggestion) as it goes.
// Precedence: unknown/illegal (2) beats unset (1) beats set (0). Under -r,
// illegal fields are silently skipped.
func (e *env) infoExitStatus(req ops.InfoRequest, res *ops.InfoResult) int {
	if len(req.Fields) == 0 {
		return exitOK
	}
	pr := e.pr()
	worst := exitOK
	reported := map[string]bool{}
	for _, scope := range res.Scopes {
		for _, f := range scope.Fields {
			switch f.Status {
			case ops.FieldUnknown:
				if !reported[f.Name] {
					reported[f.Name] = true
					pr.Errorf("unknown field %q", f.Name)
					if f.Suggestion != "" {
						pr.Fixf("did you mean %q?", f.Suggestion)
					}
				}
				worst = maxInt(worst, exitUsage)
			case ops.FieldIllegal:
				if req.Recurse {
					continue // silently skipped under -r (§2.4)
				}
				if !reported[f.Name] {
					reported[f.Name] = true
					pr.Errorf("field %q is not valid for this scope", f.Name)
					if f.Suggestion != "" {
						pr.Fixf("did you mean %q?", f.Suggestion)
					}
				}
				worst = maxInt(worst, exitUsage)
			case ops.FieldUnset:
				worst = maxInt(worst, exitNegative)
			}
		}
	}
	return worst
}

// renderInfo prints one or many scopes to stdout. A single requested field
// prints its bare value; several print labeled lines; the full view prints the
// inherent group then the configured group, per scope (§2.4).
func (e *env) renderInfo(req ops.InfoRequest, res *ops.InfoResult) {
	out := e.pr().Out()
	singleField := len(req.Fields) == 1 && !req.Recurse
	multiScope := len(res.Scopes) > 1

	for _, scope := range res.Scopes {
		if multiScope {
			out.Println(out.Style(ui.RoleHeading, scopeHeading(scope)))
		}
		printable := printableFields(scope)
		if singleField && len(printable) == 1 {
			out.Println(formatFieldValue(printable[0], true))
			continue
		}
		for _, f := range printable {
			out.Println("  " + out.Style(ui.RoleMuted, f.Name+":") + " " + formatFieldValue(f, false))
		}
	}
}

// printableFields keeps only the fields that carry a value to print (Set and
// Unset). Unknown and Illegal fields are request errors — already reported by
// infoExitStatus — and never print a value line.
func printableFields(scope ops.InfoScope) []ops.Field {
	out := make([]ops.Field, 0, len(scope.Fields))
	for _, f := range scope.Fields {
		if f.Status == ops.FieldSet || f.Status == ops.FieldUnset {
			out = append(out, f)
		}
	}
	return out
}

// scopeHeading names a scope for the multi-scope human view.
func scopeHeading(scope ops.InfoScope) string {
	if scope.Kind == ops.ScopeGlobal {
		return "global"
	}
	return scope.FQN.String()
}

// formatFieldValue renders one field's value. bare omits the label; unset shows
// (unset) for a scalar and [] for an empty list (§2.4).
func formatFieldValue(f ops.Field, bare bool) string {
	if f.Status == ops.FieldUnset {
		if _, isList := f.Value.([]string); isList {
			return "[]"
		}
		return "(unset)"
	}
	switch v := f.Value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []string:
		if bare {
			return strings.Join(v, "\n")
		}
		return "[" + strings.Join(v, ", ") + "]"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// newStatusCmd builds the status leaf (§2.4): name operands or a single path,
// plus --json.
func (e *env) newStatusCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:               "status [<name>... | <path>]",
		Short:             shorts["status"],
		Long:              statusLong,
		Example:           statusExample,
		GroupID:           groupInspect,
		ValidArgsFunction: e.completeNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e.runStatus(args, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Machine-readable status")
	return cmd
}

func (e *env) runStatus(args []string, asJSON bool) error {
	app, warns, err := e.load(false)
	if err != nil {
		e.renderWarnings(warns)
		return err
	}

	// The operand rule (§1.3): a single path operand selects the per-path view;
	// everything else is name expressions.
	req := ops.StatusRequest{}
	if len(args) == 1 && name.IsPathOperand(args[0]) {
		req.Path = args[0]
	} else {
		req.Names = args
	}

	res, err := app.Status(req)
	if err != nil {
		e.renderWarnings(warns)
		return err
	}
	e.renderWarnings(warns)
	e.renderWarnings(res.Warnings)
	if asJSON {
		return e.writeJSON(statusJSON(res))
	}
	e.renderStatus(res)
	return nil
}

// renderStatus prints live status to stdout (a query result).
func (e *env) renderStatus(res *ops.StatusResult) {
	out := e.pr().Out()
	if res.Path != nil {
		e.renderPathStatus(res.Path)
		return
	}
	for _, p := range res.Packages {
		state := out.Style(stateRole(p.State), p.State.String())
		if p.Drifted {
			state += " " + out.Style(ui.RoleDrifted, "(drifted)")
		}
		out.Println(out.Style(ui.RoleName, p.FQN.String()) + "  " + state)
		for _, l := range p.Links {
			if l.State == ops.LinkStowed {
				continue // the clean case needs no detail line
			}
			out.Println("    " + out.Style(linkStateRole(l.State), l.State.String()) + "  " + l.Link)
		}
	}
	for _, r := range res.Repos {
		out.Println(out.Style(ui.RoleName, r.FQN.String()) + "  " + repoSyncLine(r))
	}
}

// renderPathStatus prints the per-path view (§2.4).
func (e *env) renderPathStatus(p *ops.PathStatus) {
	out := e.pr().Out()
	out.Println(out.Style(ui.RoleName, homeAbbrev(p.Path)) + "  " + p.Kind)
	if p.IsSymlink && p.LinkDest != "" {
		out.Println("  -> " + p.LinkDest)
	}
	if p.OwnerKnown {
		out.Println("  owner: " + out.Style(ui.RoleName, p.Owner.String()))
	}
	if len(p.Candidates) > 0 {
		out.Println("  adoption candidates (ranked):")
		for _, c := range p.Candidates {
			out.Println("    " + out.Style(ui.RoleName, c.FQN.String()) + " -> " + c.Source)
		}
	}
}

// repoSyncLine renders a remote repo's behind/ahead line (§7.2.1).
func repoSyncLine(r ops.RepoSync) string {
	if !r.Known {
		if r.Err != nil {
			return "sync unknown (" + r.Err.Error() + ")"
		}
		return "sync unknown"
	}
	return fmt.Sprintf("behind %d, ahead %d", r.Behind, r.Ahead)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
