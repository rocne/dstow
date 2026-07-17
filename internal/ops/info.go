package ops

import (
	"sort"

	"github.com/rocne/dstow/internal/config"
	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// InfoRequest parameterizes an info run (§2.4). Name is the scope operand —
// empty is the global scope. Fields selects named fields (-f, repeatable);
// empty means every field of the scope. Recurse (-r) visits the scope's whole
// containment subtree, per-scope attributed. --json is cli's rendering choice.
type InfoRequest struct {
	Name    string
	Fields  []string
	Recurse bool
}

// FieldGroup separates a scope's two field families (§2.4): the inherent facts
// of the thing as it exists (permanently read-only) and the configured values
// resolved through the config chain.
type FieldGroup int

const (
	GroupInherent FieldGroup = iota
	GroupConfigured
)

// FieldStatus is one field's per-scope outcome, the datum cli maps to §2.4's
// exit codes: Set → 0, Unset → 1 (applicable but unset/empty), Unknown → 2
// (no such field anywhere), Illegal → 2 (a real field, wrong scope). Under
// -r, cli silently skips Illegal fields instead (§2.4).
type FieldStatus int

const (
	FieldSet FieldStatus = iota
	FieldUnset
	FieldUnknown
	FieldIllegal
)

// Field is one field's value for one scope (§2.4). Value is nil unless Set; it
// is a string, a bool, or a []string so cli/json render the native type.
// Suggestion names the nearest applicable field for an Unknown/Illegal ask.
type Field struct {
	Name       string
	Group      FieldGroup
	Status     FieldStatus
	Value      any
	Suggestion string
}

// ScopeKind names an info scope (§2.4: global installation, a repo, a package).
type ScopeKind int

const (
	ScopeGlobal ScopeKind = iota
	ScopeRepo
	ScopePackage
)

// InfoScope is one scope's fields, per-scope attributed (§2.4). FQN is zero
// for the global scope.
type InfoScope struct {
	FQN    name.FQN
	Kind   ScopeKind
	Fields []Field
}

// InfoResult is an info run as data (A4): one scope, or many under -r.
type InfoResult struct {
	Scopes   []InfoScope
	Warnings []Warning
}

// Info reads one scope's fields from config and metadata, never by inspecting
// targets (§2.4 — that is status's job). A named operand selects a repo or
// package scope; no name is the global scope. Under -r the scope's containment
// subtree is visited in turn. Run-level refusals (ambiguity, not-found) return
// as error.
func (a *App) Info(req InfoRequest) (*InfoResult, error) {
	ctxs := a.loadRepoCtxs()
	ents, byRepo := entities(ctxs)

	res := &InfoResult{}
	collect := func(sd *scopeData) {
		res.Warnings = append(res.Warnings, sd.warns...)
		res.Scopes = append(res.Scopes, InfoScope{
			FQN: sd.fqn, Kind: sd.kind, Fields: selectFields(sd, req.Fields),
		})
	}

	if req.Name == "" {
		collect(a.globalScope())
		if req.Recurse {
			for _, c := range sortedCtxs(ctxs) {
				collect(a.repoScope(c))
				for _, p := range c.packages {
					collect(a.packageScope(c, p))
				}
			}
		}
		return res, nil
	}

	ent, rc, err := resolveOne(req.Name, ents, byRepo)
	if err != nil {
		return nil, err
	}
	if ent.FQN.IsPackage() {
		collect(a.packageScope(rc, ent.FQN.Package))
		return res, nil
	}
	// A repo scope; -r adds its packages.
	collect(a.repoScope(rc))
	if req.Recurse {
		for _, p := range rc.packages {
			collect(a.packageScope(rc, p))
		}
	}
	return res, nil
}

// scopeData is one scope's computed legal fields, keyed and ordered, plus the
// warnings gathering the values raised.
type scopeData struct {
	kind   ScopeKind
	fqn    name.FQN
	fields map[string]Field
	order  []string // catalog order of this scope's legal tokens
	warns  []Warning
}

func (sd *scopeData) put(token string, group FieldGroup, status FieldStatus, value any) {
	sd.fields[token] = Field{Name: token, Group: group, Status: status, Value: value}
	sd.order = append(sd.order, token)
}

// setOrUnset stores a scalar field, calling it Unset when the value is empty.
func (sd *scopeData) setOrUnset(token string, group FieldGroup, s string) {
	if s == "" {
		sd.put(token, group, FieldUnset, nil)
		return
	}
	sd.put(token, group, FieldSet, s)
}

// globalScope builds the global installation's fields (§2.4): version and the
// known system paths (ruled), then the global-level effective config chain.
func (a *App) globalScope() *scopeData {
	sd := &scopeData{kind: ScopeGlobal, fields: map[string]Field{}}
	sd.setOrUnset("version", GroupInherent, a.Version)
	sd.put("managed-repos-dir", GroupInherent, FieldSet, repo.ManagedReposRoot())
	sd.put("global-config-dir", GroupInherent, FieldSet, config.GlobalConfigDir())
	sd.setOrUnset("ledger-path", GroupInherent, a.LedgerPath)
	sd.put("metadata-dir", GroupInherent, FieldSet, config.GlobalConfigDir())
	a.putConfigured(sd, config.Effective{Global: a.Global}, false)
	return sd
}

// repoScope builds a repo's fields (§2.4): its inherent facts, then the
// global+repo effective chain.
func (a *App) repoScope(c *repoCtx) *scopeData {
	sd := &scopeData{kind: ScopeRepo, fqn: c.r.FQN, fields: map[string]Field{}}
	sd.warns = append(sd.warns, c.warns...)
	sd.put("source", GroupInherent, FieldSet, c.r.FQN.String())
	sd.put("scheme", GroupInherent, FieldSet, c.r.FQN.Scheme)
	sd.setOrUnset("managed-path", GroupInherent, c.r.Root)
	sd.put("qualified-name", GroupInherent, FieldSet, c.r.FQN.String())
	a.putConfigured(sd, config.Effective{Global: a.Global, Repo: c.level}, true)
	return sd
}

// packageScope builds a package's fields (§2.4): its inherent facts (including
// its owning repo), then the full global+repo+package effective chain.
func (a *App) packageScope(c *repoCtx, pkg string) *scopeData {
	fqn := name.FQN{Scheme: c.r.FQN.Scheme, Coordinate: c.r.FQN.Coordinate, Package: pkg}
	sd := &scopeData{kind: ScopePackage, fqn: fqn, fields: map[string]Field{}}
	lvl, pw, perr := config.LoadPackageLevel(c.pkgRoot(pkg))
	sd.warns = append(sd.warns, warnConfig(pw)...)
	if perr != nil {
		sd.warns = append(sd.warns, Warning{
			Source: c.pkgRoot(pkg),
			Detail: "package " + fqn.String() + ": config load failed: " + perr.Error(),
		})
	}
	sd.put("repo", GroupInherent, FieldSet, c.r.FQN.String())
	sd.put("source", GroupInherent, FieldSet, c.r.FQN.String())
	sd.put("scheme", GroupInherent, FieldSet, c.r.FQN.Scheme)
	sd.setOrUnset("managed-path", GroupInherent, c.pkgRoot(pkg))
	sd.put("qualified-name", GroupInherent, FieldSet, fqn.String())
	a.putConfigured(sd, config.Effective{Global: a.Global, Repo: c.level, Package: lvl}, true)
	return sd
}

// putConfigured stores the configured-group fields common to every scope from
// an effective chain: target, dot-translation, fold, ignores — plus
// exclude-from-bulk where the scope has a bulk-exclusion knob (repo/package).
func (a *App) putConfigured(sd *scopeData, eff config.Effective, withExclude bool) {
	target, terr := eff.Target()
	if terr != nil {
		sd.put("target", GroupConfigured, FieldUnset, nil)
		sd.warns = append(sd.warns, Warning{Source: "target", Detail: terr.Error()})
	} else {
		sd.setOrUnset("target", GroupConfigured, target)
	}
	sd.put("translate", GroupConfigured, FieldSet, eff.TranslateDotPrefixes())
	sd.put("fold", GroupConfigured, FieldSet, eff.FoldTrees())

	ig := eff.Ignores()
	pats := make([]string, len(ig))
	for i, p := range ig {
		pats[i] = p.Pattern
	}
	if len(pats) == 0 {
		sd.put("ignores", GroupConfigured, FieldUnset, []string{})
	} else {
		sd.put("ignores", GroupConfigured, FieldSet, pats)
	}

	if withExclude {
		sd.put("exclude-from-bulk", GroupConfigured, FieldSet, eff.ExcludeFromBulk())
	}
}

// knownFields is every field token across all scopes — the universe an ask is
// checked against to tell Unknown (no such field) from Illegal (wrong scope).
var knownFields = []string{
	"version", "managed-repos-dir", "global-config-dir", "ledger-path",
	"metadata-dir", "repo", "source", "scheme", "managed-path",
	"qualified-name", "target", "translate", "fold", "ignores",
	"exclude-from-bulk",
}

// selectFields projects a scope's computed fields onto the request: no
// requested fields yields the whole catalog in group order; named fields are
// resolved one by one, an unknown or illegal field carrying a nearest-field
// suggestion (§2.4).
func selectFields(sd *scopeData, requested []string) []Field {
	if len(requested) == 0 {
		out := make([]Field, 0, len(sd.order))
		for _, tok := range sd.order {
			out = append(out, sd.fields[tok])
		}
		return out
	}
	out := make([]Field, 0, len(requested))
	for _, tok := range requested {
		if f, ok := sd.fields[tok]; ok {
			out = append(out, f)
			continue
		}
		if containsStr(knownFields, tok) {
			out = append(out, Field{
				Name: tok, Status: FieldIllegal, Suggestion: nearestField(tok, sd.order),
			})
			continue
		}
		out = append(out, Field{
			Name: tok, Status: FieldUnknown, Suggestion: nearestField(tok, knownFields),
		})
	}
	return out
}

// nearestField returns the candidate token closest to tok by edit distance
// (lexical tie-break), for an Unknown/Illegal suggestion. Empty candidates
// yields "".
func nearestField(tok string, candidates []string) string {
	best, bestD := "", -1
	sorted := append([]string(nil), candidates...)
	sort.Strings(sorted)
	for _, c := range sorted {
		d := editDistance(tok, c)
		if bestD == -1 || d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

// editDistance is the Levenshtein distance between a and b.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
