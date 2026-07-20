package cli

import (
	"encoding/json"
	"strings"

	"github.com/rocne/dstow/internal/ops"
)

// This file owns the per-command --json views (O10: the JSON view "arrives with
// its consumer" — cli). Every shape uses lower_snake keys, carries full FQNs
// (never the O9 short display), and spells state/class strings verbatim from
// CONTEXT.md via the ops String() methods (including the space in "partially
// stowed"). The exact field set of each shape is cli's to design within these
// O10 conventions; DESIGN pins the conventions, not the schema.

// writeJSON marshals a view to stdout, pretty-printed and newline-terminated
// (JSON is data → stdout, O1).
func (e *env) writeJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	out := e.pr().Out()
	out.Printf("%s\n", string(data))
	return nil
}

// snakeField maps an info field token (kebab-case, e.g. "managed-path") to its
// lower_snake JSON key (C5: config/JSON/slots are snake_case; CLI flags keep
// kebab).
func snakeField(token string) string { return strings.ReplaceAll(token, "-", "_") }

// --- list ---

type jsonRepoRow struct {
	FQN              string `json:"fqn"`
	Source           string `json:"source"`
	Scheme           string `json:"scheme"`
	Root             string `json:"root"`
	ExcludedFromBulk bool   `json:"excluded_from_bulk"`
	Managed          bool   `json:"managed"`
	Session          bool   `json:"session"`
}

type jsonPackageRow struct {
	FQN  string `json:"fqn"`
	Repo string `json:"repo"`
}

func listJSON(res *ops.ListResult) any {
	switch res.Kind {
	case ops.KindRepos:
		rows := make([]jsonRepoRow, 0, len(res.Repos))
		for _, r := range res.Repos {
			rows = append(rows, jsonRepoRow{
				FQN: r.FQN.String(), Source: r.Source, Scheme: r.Scheme, Root: r.Root,
				ExcludedFromBulk: r.ExcludedBulk, Managed: r.Managed, Session: r.Session,
			})
		}
		return struct {
			Repos []jsonRepoRow `json:"repos"`
		}{rows}
	case ops.KindPackages:
		rows := make([]jsonPackageRow, 0, len(res.Packages))
		for _, p := range res.Packages {
			rows = append(rows, jsonPackageRow{FQN: p.FQN.String(), Repo: p.Repo.String()})
		}
		out := struct {
			Scope    string           `json:"scope,omitempty"`
			Packages []jsonPackageRow `json:"packages"`
		}{Packages: rows}
		if res.Scope.Scheme != "" {
			out.Scope = res.Scope.String()
		}
		return out
	default: // KindPaths
		paths := make([]string, 0, len(res.Paths))
		for _, p := range res.Paths {
			paths = append(paths, p.Path)
		}
		return struct {
			Package string   `json:"package"`
			Paths   []string `json:"paths"`
		}{Package: res.Scope.String(), Paths: paths}
	}
}

// --- info ---

// infoScopeJSON is one scope as a flat object keyed by field name (O10), built
// over the Set and Unset fields (Unknown/Illegal fields are request errors, not
// data, and never appear). Under --recurse the caller wraps these in an array
// and injects qualified_name for per-scope attribution.
func infoScopeJSON(scope ops.InfoScope, attribute bool) map[string]any {
	obj := map[string]any{}
	for _, f := range scope.Fields {
		switch f.Status {
		case ops.FieldSet:
			obj[snakeField(f.Name)] = f.Value
		case ops.FieldUnset:
			// null for an unset scalar; the empty slice keeps its [] shape.
			obj[snakeField(f.Name)] = f.Value
		}
	}
	if attribute {
		// Per-scope attribution under --recurse: always name the scope, even when
		// the requested field is not qualified-name (§2.4 "per-scope attributed").
		obj["qualified_name"] = scope.FQN.String()
	}
	return obj
}

func infoJSON(res *ops.InfoResult, recurse bool) any {
	if recurse {
		out := make([]map[string]any, 0, len(res.Scopes))
		for _, s := range res.Scopes {
			out = append(out, infoScopeJSON(s, true))
		}
		return out
	}
	if len(res.Scopes) == 0 {
		return map[string]any{}
	}
	return infoScopeJSON(res.Scopes[0], false)
}

// --- status ---

type jsonLinkStatus struct {
	Link   string `json:"link"`
	Source string `json:"source"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

type jsonPackageStatus struct {
	FQN     string           `json:"fqn"`
	State   string           `json:"state"`
	Drifted bool             `json:"drifted"`
	Links   []jsonLinkStatus `json:"links"`
}

type jsonRepoSync struct {
	FQN    string `json:"fqn"`
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
	Known  bool   `json:"known"`
	Error  string `json:"error,omitempty"`
}

type jsonCandidate struct {
	FQN       string `json:"fqn"`
	Source    string `json:"source"`
	Neighbors int    `json:"neighbors"`
}

type jsonPathStatus struct {
	Path       string          `json:"path"`
	Exists     bool            `json:"exists"`
	IsSymlink  bool            `json:"is_symlink"`
	LinkDest   string          `json:"link_dest,omitempty"`
	Kind       string          `json:"kind"`
	Owner      string          `json:"owner,omitempty"`
	OwnerKnown bool            `json:"owner_known"`
	Candidates []jsonCandidate `json:"candidates"`
}

func statusJSON(res *ops.StatusResult) any {
	if res.Path != nil {
		p := res.Path
		cands := make([]jsonCandidate, 0, len(p.Candidates))
		for _, c := range p.Candidates {
			cands = append(cands, jsonCandidate{FQN: c.FQN.String(), Source: c.Source, Neighbors: c.Neighbors})
		}
		owner := ""
		if p.OwnerKnown {
			owner = p.Owner.String()
		}
		return struct {
			Path jsonPathStatus `json:"path"`
		}{jsonPathStatus{
			Path: p.Path, Exists: p.Exists, IsSymlink: p.IsSymlink, LinkDest: p.LinkDest,
			Kind: p.Kind, Owner: owner, OwnerKnown: p.OwnerKnown, Candidates: cands,
		}}
	}

	pkgs := make([]jsonPackageStatus, 0, len(res.Packages))
	for _, p := range res.Packages {
		links := make([]jsonLinkStatus, 0, len(p.Links))
		for _, l := range p.Links {
			links = append(links, jsonLinkStatus{Link: l.Link, Source: l.Source, State: l.State.String(), Detail: l.Detail})
		}
		pkgs = append(pkgs, jsonPackageStatus{
			FQN: p.FQN.String(), State: p.State.String(), Drifted: p.Drifted, Links: links,
		})
	}
	repos := make([]jsonRepoSync, 0, len(res.Repos))
	for _, r := range res.Repos {
		row := jsonRepoSync{FQN: r.FQN.String(), Ahead: r.Ahead, Behind: r.Behind, Known: r.Known}
		if r.Err != nil {
			row.Error = r.Err.Error()
		}
		repos = append(repos, row)
	}
	return struct {
		Packages []jsonPackageStatus `json:"packages"`
		Repos    []jsonRepoSync      `json:"repos"`
	}{pkgs, repos}
}

// --- check ---

type jsonFinding struct {
	Class       string `json:"class"`
	TargetRoot  string `json:"target_root"`
	Link        string `json:"link"`
	Package     string `json:"package"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Evidence    string `json:"evidence"`
}

// --- theme slots ---

type jsonSlot struct {
	Slot        string   `json:"slot"`
	Description string   `json:"description"`
	Consumers   []string `json:"consumers"`
}

func slotsJSON(res *ops.ThemeSlotsResult) any {
	rows := make([]jsonSlot, 0, len(res.Rows))
	for _, r := range res.Rows {
		cons := r.Consumers
		if cons == nil {
			cons = []string{}
		}
		rows = append(rows, jsonSlot{Slot: r.Slot, Description: r.Description, Consumers: cons})
	}
	return rows
}

func checkJSON(rep *ops.CheckReport) any {
	fs := make([]jsonFinding, 0, len(rep.Findings))
	for _, f := range rep.Findings {
		fs = append(fs, jsonFinding{
			Class: f.Class.String(), TargetRoot: f.TargetRoot, Link: f.Entry.Link,
			Package: f.Entry.Package, Source: f.Entry.Source, Destination: f.Entry.Destination,
			Evidence: f.Evidence,
		})
	}
	return struct {
		Findings []jsonFinding `json:"findings"`
	}{fs}
}
