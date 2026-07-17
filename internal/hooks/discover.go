package hooks

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Warning is a discovery diagnostic as data (A4), mirroring config.Warning:
// hooks returns warnings, the caller decides when and how to print them.
// Source is the offending entry's path; Detail is complete prose; Fix, when
// set, is the O2 remedy line the caller renders after it.
type Warning struct {
	Source string
	Detail string
	Fix    string
}

// Set maps each of the eight (Phase, Action) pairs that is present and
// executable to the absolute path of its hook file (M6). A pair with no
// present, executable file is simply absent from the map — its firing is a
// silent no-op.
type Set map[Hook]string

// Discover scans ONE hooks directory, non-recursively (M6). It returns the Set
// of present executable hooks plus any M6/M7 warnings the directory's contents
// draw. An absent directory is an empty Set with no warnings and no error
// (hooks are optional); any other ReadDir failure is returned as the error.
//
// Executability is tested with os.Stat, not Lstat (mode&0111 != 0): a symlink
// to an executable is a hook.
func Discover(dir string) (Set, []Warning, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return Set{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	set := make(Set)
	var warns []Warning
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(dir, name)

		// A <hook>.d entry — file OR directory — is reserved (M7): drop-in
		// execution is committed for a future version but inert here.
		if base, ok := reservedDotD(name); ok {
			warns = append(warns, Warning{
				Source: path,
				Detail: fmt.Sprintf("%q is reserved: the %s.d drop-in directory is committed for a future version but is not yet meaningful, so it is inert and nothing in it runs", name, base),
			})
			continue
		}

		// Any other subdirectory is inert helper territory — never fired,
		// never warned (lib/ is the documented convention) (M6).
		if entry.IsDir() {
			continue
		}

		if hook, ok := validHooks[name]; ok {
			// A valid hook name: executable ⇒ it is a hook; otherwise warn,
			// naming the exact chmod remedy (M6).
			if info, statErr := os.Stat(path); statErr == nil && info.Mode()&0o111 != 0 {
				set[hook] = path
				continue
			}
			warns = append(warns, Warning{
				Source: path,
				Detail: fmt.Sprintf("%q has a valid hook name but is not executable, so it will never run", name),
				Fix:    "chmod +x " + path,
			})
			continue
		}

		// Any other file directly in the directory is a non-hook (M6): warn,
		// with a did-you-mean when it is close to one of the eight names.
		w := Warning{
			Source: path,
			Detail: fmt.Sprintf("%q is not one of the eight recognized hook names ({pre,post}-{stow,unstow,restow,adopt}); it is ignored", name),
		}
		if suggestion, ok := didYouMean(name); ok {
			w.Detail = fmt.Sprintf("%q is not a recognized hook name; did you mean %q? It is ignored", name, suggestion)
			w.Fix = fmt.Sprintf("mv %s %s", path, filepath.Join(dir, suggestion))
		}
		warns = append(warns, w)
	}
	return set, warns, nil
}

// reservedDotD reports whether name is exactly "<hook>.d" for one of the eight
// hook names (M7), returning the hook base it reserves.
func reservedDotD(name string) (base string, ok bool) {
	trimmed, cut := strings.CutSuffix(name, ".d")
	if !cut {
		return "", false
	}
	if _, valid := validHooks[trimmed]; !valid {
		return "", false
	}
	return trimmed, true
}

// didYouMean suggests the closest hook name to a non-hook file (M6). Closeness
// is equality after normalization — lowercase and drop every non-alphanumeric
// byte, which collapses '_' and '-' alike — so the documented examples
// pre_stow and prestow both normalize to the same form as pre-stow and suggest
// it. No normalized match ⇒ no suggestion (the caller then warns without a Fix).
func didYouMean(name string) (suggestion string, ok bool) {
	n := normalize(name)
	if n == "" {
		return "", false
	}
	for _, p := range allPhases {
		for _, a := range allActions {
			h := Hook{Phase: p, Action: a}
			if normalize(h.FileName()) == n {
				return h.FileName(), true
			}
		}
	}
	return "", false
}

// normalize lowercases s and keeps only ASCII alphanumerics, collapsing every
// separator (M6): pre_stow, prestow, and pre-stow all become "prestow".
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
