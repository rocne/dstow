package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rocne/dstow/internal/ledger"
	"github.com/rocne/dstow/internal/name"
)

// strKnob and boolKnob are one set value with provenance. expanded marks a
// value that already went through expansion (rc values arrive expanded by
// gostow's own pipeline); C8's absoluteness rule applies either way, at use
// time.
type strKnob struct {
	value    string
	file     string
	key      string
	expanded bool
}

type boolKnob struct {
	value bool
	file  string
	key   string
}

// carrier is one option source at one level: the native config file or a
// .stowrc (C22's "carrier"). A renamed rc routed by the sniff is the native
// slot with rc content — its position keeps native-slot precedence, its rc
// flag keeps compat pattern language.
type carrier struct {
	file            string
	rc              bool
	target          *strKnob
	translate       *boolKnob
	fold            *boolKnob
	ignores         []IgnorePattern
	excludeFromBulk *boolKnob
	packagesDir     string
	theme           *strKnob
	color           map[string]string
	sessionRepoDir  string
}

// GlobalLevel is the loaded global level (§3.1): the shared knobs plus the
// global-only territory — folding, theming, and the session-repo
// contribution a migrated ~/.stowrc --dir makes.
type GlobalLevel struct {
	target         *strKnob
	translate      *boolKnob
	fold           *boolKnob
	ignores        []IgnorePattern
	theme          *strKnob
	color          map[string]string
	sessionRepoDir string
}

// Theme returns the theme reference (C13): a bare name verbatim, or — for
// the path form per the operand rule — the C8-expanded absolute path.
// Empty when unset.
func (g *GlobalLevel) Theme() (string, error) {
	if g == nil || g.theme == nil {
		return "", nil
	}
	if !name.IsPathOperand(g.theme.value) {
		return g.theme.value, nil
	}
	if g.theme.expanded {
		return g.theme.value, nil
	}
	return expandAbsolute(g.theme.value, g.theme.file, g.theme.key)
}

// ColorTable returns the raw [color] table values (§3.3) for ui to parse —
// config carries the strings, ui owns the slot vocabulary and the value
// grammar. Nil when no table was set.
func (g *GlobalLevel) ColorTable() map[string]string {
	if g == nil {
		return nil
	}
	return g.color
}

// SessionRepoDir returns the session-repo contribution of a migrated
// ~/.stowrc --dir (C19), or "" when there is none.
func (g *GlobalLevel) SessionRepoDir() string {
	if g == nil {
		return ""
	}
	return g.sessionRepoDir
}

// RepoLevel is one repo's level: the shared knobs, the package-level knob
// set acting as defaults for the repo's packages (REQUIREMENTS §4.1), plus
// the repo-only packages_dir (M3) and the compat fold contribution a
// migrated repo .stowrc makes (REQUIREMENTS §3.3).
type RepoLevel struct {
	target          *strKnob
	translate       *boolKnob
	excludeFromBulk *boolKnob
	ignores         []IgnorePattern
	packagesDir     string
	compatFold      *boolKnob
}

// PackagesDir returns the raw repo-root-relative packages directory (M3) —
// deliberately outside C8's expand-to-absolute grammar — or "" when unset
// (packages at the repo root; stow compat binds the default).
func (r *RepoLevel) PackagesDir() string {
	if r == nil {
		return ""
	}
	return r.packagesDir
}

// CompatFoldTrees reports the fold contribution of a migrated repo-level
// .stowrc (REQUIREMENTS §3.3: honored, the compat exception to
// fold-is-global-only). ops composes the cross-repo contradiction guard
// from these per-repo values.
func (r *RepoLevel) CompatFoldTrees() (value bool, file string, set bool) {
	if r == nil || r.compatFold == nil {
		return false, "", false
	}
	return r.compatFold.value, r.compatFold.file, true
}

// PackageLevel is one package's level: the package knob set (§3.2).
type PackageLevel struct {
	target          *strKnob
	translate       *boolKnob
	excludeFromBulk *boolKnob
	ignores         []IgnorePattern
}

// loadCarrier reads and parses one option source. A missing file is an
// empty carrier (nil); any other read failure surfaces — a level the user
// wrote must never be silently skipped. rcShaped forces the compat parser
// (a real .stowrc); a native path routes through the sniff (§3.6).
func loadCarrier(path string, rcShaped bool, level Level) (*carrier, []Warning, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read config file %q: %w", path, err)
	}
	if rcShaped {
		return parseRC(path, content, level)
	}
	if sniffRC(content) {
		announcement := Warning{
			Source: path,
			Detail: fmt.Sprintf("%s contains stow rc flag lines, not TOML; it was parsed as a .stowrc (compat). The native format never bends to compat", path),
			Fix:    "translate the flags to native TOML keys, e.g. --target=DIR becomes target = 'DIR'",
		}
		c, warns, err := parseRC(path, content, level)
		return c, append([]Warning{announcement}, warns...), err
	}
	return parseNative(path, content, level)
}

// mergeCarriers applies supplement mode (C22) at one level: per knob, where
// both carriers set it, equal values are silent, differing values go loud
// — the native slot wins, the warning names level, files, knob, values and
// winner, and the fix suggests removing the option from the rc. Ignores
// never conflict: additive, per-carrier language.
func mergeCarriers(level Level, native, rc *carrier) (*carrier, []Warning) {
	if native == nil && rc == nil {
		return &carrier{}, nil
	}
	if rc == nil {
		return native, nil
	}
	if native == nil {
		return rc, nil
	}

	var warns []Warning
	conflict := func(knob, nativeVal, rcVal, rcOption string) {
		warns = append(warns, Warning{
			Source: rc.file,
			Detail: fmt.Sprintf("at the %s level, %s is set by both %s (%s) and %s (%s); the native value wins",
				level, knob, native.file, nativeVal, rc.file, rcVal),
			Fix: fmt.Sprintf("remove %s from %s", rcOption, rc.file),
		})
	}

	merged := *native
	if native.target == nil {
		merged.target = rc.target
	} else if rc.target != nil {
		// Compare expanded-to-expanded where possible: the rc side arrives
		// expanded, so a best-effort expansion of the native side keeps a
		// true duplicate ("~/x" vs its expansion) silent, as C22 intends.
		// Expansion failures fall back to the literal spelling — this only
		// decides warning-or-silence, never the winner.
		nativeCmp := native.target.value
		if expanded, err := expandPath(nativeCmp, native.target.file, native.target.key); err == nil {
			nativeCmp = expanded
		}
		if nativeCmp != rc.target.value {
			conflict("target", native.target.value, rc.target.value, "--target")
		}
	}
	if native.translate == nil {
		merged.translate = rc.translate
	} else if rc.translate != nil && native.translate.value != rc.translate.value {
		conflict("translate_dot_prefixes",
			strconv.FormatBool(native.translate.value), "--dotfiles ("+strconv.FormatBool(rc.translate.value)+")", "--dotfiles")
	}
	if native.fold == nil {
		merged.fold = rc.fold
	} else if rc.fold != nil && native.fold.value != rc.fold.value {
		conflict("fold_trees",
			strconv.FormatBool(native.fold.value), "--no-folding ("+strconv.FormatBool(rc.fold.value)+")", "--no-folding")
	}
	merged.ignores = append(append([]IgnorePattern{}, native.ignores...), rc.ignores...)
	if merged.sessionRepoDir == "" {
		merged.sessionRepoDir = rc.sessionRepoDir
	}
	return &merged, warns
}

// LoadGlobal loads the global level: $XDG_CONFIG_HOME/dstow/config.toml
// supplemented by ~/.stowrc (slotted global, C20), plus the reserved-
// territory scan of the global config dir (M5: claimed entries are
// config.toml, repos.toml, themes/, hooks/). When the config dir and the
// ledger's state dir resolve to the same directory (the macOS default), the
// ledger's own files are claimed too, so dstow never flags its own state.
func LoadGlobal() (*GlobalLevel, []Warning, error) {
	claimed := []string{configFileName, "repos.toml", "themes", "hooks"}
	// M5 across the macOS-default lane collision: when the global config dir
	// and the ledger's state dir are the same directory (xdg.ConfigHome ==
	// xdg.StateHome, as on stock macOS where both resolve to ~/Library/
	// Application Support), dstow's own ledger.json/ledger.lock sit inside the
	// config dir. Claim them so the scan never flags state dstow wrote itself.
	// Where the lanes differ (stock Linux), a stray ledger.json dropped in the
	// config dir still warns — the guard is collision-conditional by design.
	if GlobalConfigDir() == ledger.Dir() {
		claimed = append(claimed, ledger.ReservedNames()...)
	}
	warns := scanReserved(GlobalConfigDir(), claimed...)

	native, w, err := loadCarrier(GlobalConfigFile(), false, LevelGlobal)
	warns = append(warns, w...)
	if err != nil {
		return nil, warns, err
	}

	var rc *carrier
	if home, herr := os.UserHomeDir(); herr == nil {
		rc, w, err = loadCarrier(filepath.Join(home, ".stowrc"), true, LevelGlobal)
		warns = append(warns, w...)
		if err != nil {
			return nil, warns, err
		}
	}

	merged, w := mergeCarriers(LevelGlobal, native, rc)
	warns = append(warns, w...)
	return &GlobalLevel{
		target:         merged.target,
		translate:      merged.translate,
		fold:           merged.fold,
		ignores:        merged.ignores,
		theme:          merged.theme,
		color:          merged.color,
		sessionRepoDir: merged.sessionRepoDir,
	}, warns, nil
}

// LoadRepoLevel loads one repo's level: <repo>/.dstow/config.toml
// supplemented by <repo>/.stowrc — discovered via the repo, never cwd
// (C20) — plus the metadata dir's reserved-territory scan (M5: claimed
// entries are config.toml and hooks/).
func LoadRepoLevel(repoRoot string) (*RepoLevel, []Warning, error) {
	warns := scanReserved(MetadataDir(repoRoot), configFileName, "hooks")

	native, w, err := loadCarrier(filepath.Join(MetadataDir(repoRoot), configFileName), false, LevelRepo)
	warns = append(warns, w...)
	if err != nil {
		return nil, warns, err
	}
	rc, w, err := loadCarrier(filepath.Join(repoRoot, ".stowrc"), true, LevelRepo)
	warns = append(warns, w...)
	if err != nil {
		return nil, warns, err
	}

	merged, w := mergeCarriers(LevelRepo, native, rc)
	warns = append(warns, w...)
	return &RepoLevel{
		target:          merged.target,
		translate:       merged.translate,
		excludeFromBulk: merged.excludeFromBulk,
		ignores:         merged.ignores,
		packagesDir:     merged.packagesDir,
		// Fold at the repo level exists only as migrated stow content
		// (REQUIREMENTS §3.3) — native fold_trees was already warned off by
		// the legality matrix, so whatever fold survives here is compat.
		compatFold: merged.fold,
	}, warns, nil
}

// LoadPackageLevel loads one package's level: <package>/.dstow/config.toml
// (there is no package-level rc slot — stow has none) plus the metadata
// dir's reserved-territory scan.
func LoadPackageLevel(pkgRoot string) (*PackageLevel, []Warning, error) {
	warns := scanReserved(MetadataDir(pkgRoot), configFileName, "hooks")

	native, w, err := loadCarrier(filepath.Join(MetadataDir(pkgRoot), configFileName), false, LevelPackage)
	warns = append(warns, w...)
	if err != nil {
		return nil, warns, err
	}
	if native == nil {
		native = &carrier{}
	}
	return &PackageLevel{
		target:          native.target,
		translate:       native.translate,
		excludeFromBulk: native.excludeFromBulk,
		ignores:         native.ignores,
	}, warns, nil
}

// Effective is one package's view of the chain: nearest level wins per
// knob, ignores compose additively (REQUIREMENTS §4.1), and the built-in
// floor closes every fall-through (§4.4). Nil levels are simply absent —
// a package with no config is the common case, not an error.
type Effective struct {
	Global  *GlobalLevel
	Repo    *RepoLevel
	Package *PackageLevel
}

// Target resolves the effective target: package → repo → global → the
// built-in $HOME floor, expanded at use time per C8 (unset variable and
// non-absolute results error with full provenance, scoping to the views
// that select the offending value).
func (e Effective) Target() (string, error) {
	var knob *strKnob
	switch {
	case e.Package != nil && e.Package.target != nil:
		knob = e.Package.target
	case e.Repo != nil && e.Repo.target != nil:
		knob = e.Repo.target
	case e.Global != nil && e.Global.target != nil:
		knob = e.Global.target
	}
	if knob == nil {
		// The built-in floor is the home directory itself, resolved at use
		// time — the platform's notion of $HOME, absolute by construction.
		home, err := os.UserHomeDir()
		if err != nil {
			return "", &ExpandError{
				File:   "built-in default",
				Key:    "target",
				Value:  "$HOME",
				Reason: fmt.Sprintf("the home directory cannot be determined: %v; set a target in %s", err, GlobalConfigFile()),
			}
		}
		return home, nil
	}
	if knob.expanded {
		if !filepath.IsAbs(knob.value) {
			return "", &ExpandError{
				File:   knob.file,
				Key:    knob.key,
				Value:  knob.value,
				Reason: "is not an absolute path; targets must expand to absolute paths (C8)",
			}
		}
		return knob.value, nil
	}
	return expandAbsolute(knob.value, knob.file, knob.key)
}

// TranslateDotPrefixes resolves dot-translation: nearest level wins;
// the built-in default is on (REQUIREMENTS §3.4).
func (e Effective) TranslateDotPrefixes() bool {
	switch {
	case e.Package != nil && e.Package.translate != nil:
		return e.Package.translate.value
	case e.Repo != nil && e.Repo.translate != nil:
		return e.Repo.translate.value
	case e.Global != nil && e.Global.translate != nil:
		return e.Global.translate.value
	}
	return true
}

// FoldTrees resolves folding: a migrated repo rc's contribution is honored
// for this repo (REQUIREMENTS §3.3), else the global setting, else the
// built-in off (REQUIREMENTS §3.3: predictable link topology). The
// cross-repo contradiction guard composes in ops from CompatFoldTrees.
func (e Effective) FoldTrees() bool {
	if e.Repo != nil && e.Repo.compatFold != nil {
		return e.Repo.compatFold.value
	}
	if e.Global != nil && e.Global.fold != nil {
		return e.Global.fold.value
	}
	return false
}

// ExcludeFromBulk resolves bulk exclusion: package over repo (nearer wins,
// even when nearer says false); the built-in default is off. Explicit
// naming overriding bulk exclusion is the caller's law, not a config fact.
func (e Effective) ExcludeFromBulk() bool {
	switch {
	case e.Package != nil && e.Package.excludeFromBulk != nil:
		return e.Package.excludeFromBulk.value
	case e.Repo != nil && e.Repo.excludeFromBulk != nil:
		return e.Repo.excludeFromBulk.value
	}
	return false
}

// Ignores returns the additive ignore chain in level order — global, repo,
// package — each entry carrying its carrier language and provenance
// (§3.4). The engine's always-on metadata auto-ignore (M8) and stow's
// built-in ignores ride the engine seam, not this chain.
func (e Effective) Ignores() []IgnorePattern {
	var out []IgnorePattern
	if e.Global != nil {
		out = append(out, e.Global.ignores...)
	}
	if e.Repo != nil {
		out = append(out, e.Repo.ignores...)
	}
	if e.Package != nil {
		out = append(out, e.Package.ignores...)
	}
	return out
}
