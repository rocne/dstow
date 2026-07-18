package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dstow/internal/name"
	"github.com/rocne/dstow/internal/repo"
)

// resolvedSource is a raw add input canonicalized to a source (§5.2): the
// registry entry, plus — for a remote — the URL to clone from. The clone URL
// is honored as given for a URL form (ssh/credential fidelity, A17) and
// constructed https for the qualified/bare github forms (spelling is ops').
type resolvedSource struct {
	src      repo.Source
	cloneURL string // non-empty ⇒ remote; clone from here into src.CloneDir()
	remote   bool
}

// SourceAmbiguousError reports a bare owner/name that could mean either the
// github source or an existing local directory (§1.2 + §5.2.2): the §1.2
// explicit-choice case. ops returns it as data — cli renders the choice and
// re-invokes with one of the two qualified spellings — and non-interactively
// it renders as the hard refusal naming both.
type SourceAmbiguousError struct {
	Input  string
	Github repo.Source
	Local  repo.Source
}

func (e *SourceAmbiguousError) Error() string {
	return fmt.Sprintf(
		"%q is ambiguous: it could mean the github source %s or the local directory %s; qualify it — add %s or %s",
		e.Input, e.Github, e.Local, e.Github, e.Local)
}

// SourceDeclinedError reports that the interactive github interpretation of a
// bare source was declined (§1.2). Its message names the qualified spelling to
// use instead.
type SourceDeclinedError struct {
	Input          string
	Interpretation string
}

func (e *SourceDeclinedError) Error() string {
	return fmt.Sprintf(
		"declined the interpretation of %q as %s; qualify the source explicitly (e.g. %s, or local:/absolute/path)",
		e.Input, e.Interpretation, e.Interpretation)
}

// SourceUnresolvableError reports a bare input that is neither an owner/name
// github source nor an existing local directory (§1.4). Every refusal names
// its remedy.
type SourceUnresolvableError struct {
	Input string
}

func (e *SourceUnresolvableError) Error() string {
	return fmt.Sprintf(
		"%q is neither an existing local directory nor an owner/name github source; qualify it — github:owner/name or local:/absolute/path",
		e.Input)
}

// RenameRequestedError reports that the encoding continue-or-rename prompt was
// answered rename (§1.2 + §2.4 add): the add is cancelled so the user can
// rename first. It names the encoded form that would otherwise be used.
type RenameRequestedError struct {
	Source string // the canonical encoded source
}

func (e *RenameRequestedError) Error() string {
	return fmt.Sprintf("add cancelled: rename requested rather than continue with the encoded source %s", e.Source)
}

// resolveAddSource canonicalizes raw add input into a source (§5.2), consulting
// the Prompter for the bare-form §1.2 confirm-unless-unambiguous flow. It does
// the filesystem check the classifier forbids (existence of a local dir) here,
// where interpretation happens.
func (a *App) resolveAddSource(raw string) (resolvedSource, error) {
	switch repo.ClassifySourceInput(raw) {
	case repo.QualifiedForm:
		src, err := repo.ParseSource(raw)
		if err != nil {
			return resolvedSource{}, err
		}
		return fromSource(src)
	case repo.PathForm:
		abs, err := absLocalPath(raw)
		if err != nil {
			return resolvedSource{}, err
		}
		return resolvedSource{src: localSource(abs)}, nil
	case repo.URLForm:
		owner, name, ok := githubFromURL(raw)
		if !ok {
			return resolvedSource{}, fmt.Errorf("cannot read an owner/name from the URL %q; v1 clones github sources — use a github URL or github:owner/name", raw)
		}
		return resolvedSource{
			src:      repo.Source{Scheme: "github", Coordinate: []string{owner, name}},
			cloneURL: raw, // honor the given URL (ssh/credential fidelity)
			remote:   true,
		}, nil
	default: // repo.BareForm
		return a.resolveBareSource(raw)
	}
}

// resolveBareSource applies confirm-unless-unambiguous to a bare input (§1.2,
// §5.2.2): a matching local directory alongside an owner/name shape is the
// ambiguity (explicit choice); a lone owner/name confirms the github.com
// interpretation; a bare name matching a local directory is that directory;
// anything else is unresolvable.
func (a *App) resolveBareSource(raw string) (resolvedSource, error) {
	localExists := isLocalDir(raw)
	ghOK := looksLikeOwnerName(raw)

	switch {
	case ghOK && localExists:
		abs, err := absLocalPath(raw)
		if err != nil {
			return resolvedSource{}, err
		}
		return resolvedSource{}, &SourceAmbiguousError{
			Input:  raw,
			Github: repo.Source{Scheme: "github", Coordinate: ownerNameSegs(raw)},
			Local:  localSource(abs),
		}
	case ghOK:
		src := repo.Source{Scheme: "github", Coordinate: ownerNameSegs(raw)}
		ok, err := a.Prompt.Confirm(fmt.Sprintf(
			"interpret %q as the github source %s (host github.com assumed)?", raw, src), true)
		if err != nil {
			return resolvedSource{}, err // non-interactive: the §1.2 hard refusal
		}
		if !ok {
			return resolvedSource{}, &SourceDeclinedError{Input: raw, Interpretation: src.String()}
		}
		return resolvedSource{src: src, cloneURL: githubHTTPS(src), remote: true}, nil
	case localExists:
		abs, err := absLocalPath(raw)
		if err != nil {
			return resolvedSource{}, err
		}
		return resolvedSource{src: localSource(abs)}, nil
	default:
		return resolvedSource{}, &SourceUnresolvableError{Input: raw}
	}
}

// fromSource wraps a parsed qualified source with its remote/clone facts.
func fromSource(src repo.Source) (resolvedSource, error) {
	switch src.Scheme {
	case "github":
		return resolvedSource{src: src, cloneURL: githubHTTPS(src), remote: true}, nil
	case "local":
		return resolvedSource{src: src}, nil
	default:
		// ParseSource already refuses unknown schemes; this is unreachable.
		return resolvedSource{}, fmt.Errorf("unsupported source scheme %q", src.Scheme)
	}
}

// githubHTTPS is the constructed clone URL for a github source (pre-ruled: the
// https://github.com/owner/name.git spelling is ops').
func githubHTTPS(src repo.Source) string {
	owner, name := src.Coordinate[0], src.Coordinate[1]
	return fmt.Sprintf("https://github.com/%s/%s.git", owner, name)
}

// githubFromURL extracts owner/name from a github https or scp-like ssh URL.
func githubFromURL(raw string) (owner, name string, ok bool) {
	s := raw
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
		j := strings.Index(s, "/")
		if j < 0 {
			return "", "", false
		}
		s = s[j+1:] // drop the host
	} else if i := strings.Index(s, ":"); i >= 0 {
		s = s[i+1:] // scp-like user@host:path
	}
	s = strings.TrimSuffix(strings.Trim(s, "/"), ".git")
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	owner, name = parts[len(parts)-2], parts[len(parts)-1]
	if owner == "" || name == "" {
		return "", "", false
	}
	return owner, name, true
}

// ownerNameSegs splits a bare owner/name into its two coordinate segments.
func ownerNameSegs(raw string) []string {
	return strings.SplitN(raw, "/", 2)
}

// looksLikeOwnerName reports whether raw is a two-segment owner/name.
func looksLikeOwnerName(raw string) bool {
	parts := strings.Split(raw, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

// localSource builds a local source from an absolute path, the coordinate a
// leading empty segment marks as absolute (matching BuildSet's local repos).
func localSource(abs string) repo.Source {
	return repo.Source{Scheme: "local", Coordinate: strings.Split(filepath.ToSlash(abs), "/")}
}

// absLocalPath expands a leading ~ and makes the path absolute (add-time
// canonicalization, §5.2: ParseSource refuses a relative local source).
func absLocalPath(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return filepath.Abs(p)
}

// isLocalDir reports whether raw names an existing local directory.
func isLocalDir(raw string) bool {
	abs, err := absLocalPath(raw)
	if err != nil {
		return false
	}
	info, err := os.Stat(abs)
	return err == nil && info.IsDir()
}

// needsEncoding reports whether any coordinate segment percent-encodes (§1.2).
func needsEncoding(src repo.Source) bool {
	for _, seg := range src.Coordinate {
		if name.Encode(seg) != seg {
			return true
		}
	}
	return false
}
