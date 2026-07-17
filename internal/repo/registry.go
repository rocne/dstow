package repo

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Registry is the parsed repo registry (C3): the persistent record of
// registered sources. It is config, not state — dstow-written, never
// reconstructible from disk — so it lives with intent. The registry package
// owns reading and writing it; config only knows where the file is and hands
// the path in.
type Registry struct {
	Sources []Source
}

// registryFile is the on-disk schema. Entries decode into primitives first so
// both C9 forms are accepted: the canonical shorthand string array
// (repos = ["github:rocne/dotfiles"]) and the documented growth form of tables
// with a source key ([[repos]] / source = "…").
type registryFile struct {
	Repos []toml.Primitive `toml:"repos"`
}

// LoadRegistry reads the registry at path (C3). A missing file is an empty
// registry, never an error. Malformed TOML is an error — there is no partial
// file to salvage. An entry that does not parse as a source is a per-entry
// warn-and-skip (C18 posture), as is an unknown top-level key; the rest of the
// file still loads.
func LoadRegistry(path string) (Registry, []Warning, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Registry{}, nil, nil
	}
	if err != nil {
		return Registry{}, nil, err
	}

	var rf registryFile
	md, derr := toml.Decode(string(data), &rf)
	if derr != nil {
		return Registry{}, nil, fmt.Errorf("%s is not valid TOML: %w", path, derr)
	}

	var warns []Warning

	// Unknown top-level keys warn, never refuse (C18).
	for _, key := range md.Undecoded() {
		full := key.String()
		if full == "repos" || strings.HasPrefix(full, "repos.") {
			continue
		}
		warns = append(warns, Warning{
			Source: path,
			Detail: fmt.Sprintf("unknown key %q — the registry recognizes only \"repos\"; the key is ignored", full),
		})
	}

	var reg Registry
	for i, prim := range rf.Repos {
		raw, perr := decodeEntry(md, prim)
		if perr != nil {
			warns = append(warns, Warning{
				Source: path,
				Detail: fmt.Sprintf("registry entry #%d is malformed: %v (the entry is skipped, the rest still load)", i+1, perr),
			})
			continue
		}
		src, serr := ParseSource(raw)
		if serr != nil {
			warns = append(warns, Warning{
				Source: path,
				Detail: fmt.Sprintf("registry entry %q does not parse as a source: %v (the entry is skipped, the rest still load)", raw, serr),
				Fix:    "fix or remove the entry; register a repo with: dstow repo add <source>",
			})
			continue
		}
		reg.Sources = append(reg.Sources, src)
	}

	return reg, warns, nil
}

// decodeEntry reads one repos entry as either a bare source string (the
// canonical shorthand) or a table with a non-empty source key (the growth
// form).
func decodeEntry(md toml.MetaData, prim toml.Primitive) (string, error) {
	var s string
	if err := md.PrimitiveDecode(prim, &s); err == nil {
		return s, nil
	}
	var tbl struct {
		Source string `toml:"source"`
	}
	if err := md.PrimitiveDecode(prim, &tbl); err == nil {
		if tbl.Source == "" {
			return "", fmt.Errorf("a table entry needs a non-empty \"source\" key")
		}
		return tbl.Source, nil
	}
	return "", fmt.Errorf("an entry must be a source string or a table with a \"source\" key")
}

// Save writes the registry to path (C3). It always emits the canonical
// shorthand string-array form, sorted, and writes with the ledger's exact
// discipline: a temp file in the same directory, fsync, rename, and a directory
// fsync (mirrored in writeAtomic). The parent directory is created if needed.
func (r Registry) Save(path string) error {
	strs := make([]string, 0, len(r.Sources))
	for _, s := range r.Sources {
		strs = append(strs, s.String())
	}
	sort.Strings(strs)

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(map[string][]string{"repos": strs}); err != nil {
		return err
	}
	return writeAtomic(path, buf.Bytes())
}

// writeAtomic commits data to path with the ledger's write discipline (§6.2,
// mirrored per A9): MkdirAll the parent → temp file in the same directory →
// fsync the file → rename over path → fsync the directory. Readers and crashes
// see the old file or the new, never a torn one.
func writeAtomic(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "repos-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmpName, path); err != nil {
		return err
	}
	return fsyncDir(dir)
}

// fsyncDir flushes a directory entry so the rename survives a crash.
func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		return err
	}
	return d.Close()
}
