package config

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/rocne/gostow/stowrc"
)

// Stow compatibility (§3.6): rc parsing is consumed from gostow's public
// stowrc package, quirk-faithful (fixQuirks=false) — conformance is
// gostow's job, knob mapping is dstow's (C20). Discovery and slotting stay
// here: ~/.stowrc slots to the global level, <repo>/.stowrc to the repo
// level, discovered via the repo, never cwd.

// sniffRC reports whether a native-named config file is really stow rc
// flag-lines (§3.6): the first significant token starts with '-', which is
// impossible in top-level TOML. Blank lines and '#'-led lines are not
// significant — a TOML comment is insignificant by definition, and stow rc
// files open with '#' lines too (stow discards their tokens; gostow ledger
// PL-02), so neither language's opening comment decides the routing.
func sniffRC(content []byte) bool {
	for _, line := range bytes.Split(content, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		return line[0] == '-'
	}
	return false
}

// invalidIgnorePrefix is the parity-pinned opening of gostow's diagnostic
// for an uncompilable --ignore pattern (stow.CompilePattern's bytes). It is
// what separates C21's scoped refusal from C19's warn-and-ignore
// degradation among the parse diagnostics.
const invalidIgnorePrefix = `Invalid --ignore regex `

// parseRC parses one rc-shaped source (an actual .stowrc, or a renamed rc
// routed here by the sniff) and maps it onto dstow's knobs per C19. The
// file always runs; degradation is loud, never rejection — the two refusals
// are C21 (a non-RE2 --ignore pattern) and stow's own die shapes (an
// unreadable file, an undefined variable in --dir/--target), both scoped to
// the level this file governs.
func parseRC(path string, content []byte, level Level) (*carrier, []Warning, error) {
	res, err := stowrc.ParseReader(bytes.NewReader(content), path, false)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot use %s: %w — fix or remove the file, or translate it to native config (%s)",
			path, err, levelFileHint(level))
	}

	var (
		warns       []Warning
		patternErrs []error
	)
	warnf := func(fix, format string, args ...any) {
		warns = append(warns, Warning{Source: path, Detail: fmt.Sprintf(format, args...), Fix: fix})
	}

	// Parse diagnostics: stow collects them and keeps parsing. A bad
	// --ignore pattern is C21's refusal; everything else (unknown options,
	// missing arguments) is loud degradation per option (C19).
	for _, diag := range res.Errors {
		if strings.HasPrefix(diag, invalidIgnorePrefix) {
			patternErrs = append(patternErrs, &PatternError{
				File: path,
				Reason: diag + " — rewrite the pattern as an RE2-compatible regex, or move it to the native " +
					"ignore key, which speaks gitignore-glob (C16)",
			})
			continue
		}
		warnf("", "stow rc diagnostic: %s (the option is ignored, the rest of the file still applies)", diag)
	}
	if len(patternErrs) > 0 {
		return nil, warns, errors.Join(patternErrs...)
	}

	c := &carrier{file: path, rc: true}
	if res.Target != nil {
		// gostow already applied stow's own expansion, so the value arrives
		// expanded; C8's absoluteness rule still applies at use time.
		c.target = &strKnob{value: *res.Target, file: path, key: "--target option", expanded: true}
	}
	if res.Dotfiles {
		c.translate = &boolKnob{value: true, file: path, key: "--dotfiles option"}
	}
	if res.NoFolding {
		// Flag absence maps to nothing — dstow's default applies (C19).
		c.fold = &boolKnob{value: false, file: path, key: "--no-folding option"}
	}
	for _, p := range res.Ignore {
		c.ignores = append(c.ignores, IgnorePattern{
			Pattern:  p,
			Language: LangStowRegex,
			Level:    level,
			Source:   path,
		})
	}

	if res.Dir != nil {
		switch level {
		case LevelGlobal:
			// --dir in ~/.stowrc: a session-repo contribution, announced
			// (C19) — a surprise-class statement, never silence.
			c.sessionRepoDir = *res.Dir
			warnf("make it persistent with: dstow repo add "+*res.Dir,
				"--dir here contributes %s as a session repo for this invocation", *res.Dir)
		default:
			warnf("", "--dir means nothing at the %s level and is ignored; a repo's location is where the repo is", level)
		}
	}

	// Unmappables: warn-and-ignore per option, naming why + the native
	// remedy (C19).
	if res.Verbose != nil {
		warnf("", "--verbose is ignored: verbosity is a per-invocation choice, never config; use dstow's command-line flags")
	}
	if res.Simulate {
		warnf("", "--simulate is ignored: simulation is a per-invocation choice; use dstow's dry-run form on the command line")
	}
	if res.Adopt {
		warnf("", "--adopt is ignored: adopting is an explicit dstow command (dstow adopt), never ambient config")
	}
	if res.Help {
		warnf("", "--help is ignored: rc files configure, they never run commands")
	}
	if res.Version {
		warnf("", "--version is ignored: rc files configure, they never run commands")
	}
	if res.Compat {
		warnf("", "--compat is ignored: stow's compat mode has no dstow equivalent")
	}
	if res.FixQuirks {
		warnf("", "--gostow-fix is ignored: it is gostow's own extension; dstow reads rc files quirk-faithfully")
	}
	for _, p := range res.Override {
		warnf("", "--override=%s is ignored: stow's tie-breaking has no dstow equivalent — same-target collisions surface as per-package conflicts", p)
	}
	for _, p := range res.Defer {
		warnf("", "--defer=%s is ignored: stow's tie-breaking has no dstow equivalent — same-target collisions surface as per-package conflicts", p)
	}
	for _, req := range res.Requests {
		warnf("", "rc files configure, they never run verbs: %s of %s is ignored (stow itself discards verbs and package names in rc files)",
			req.Action, strconv.Quote(strings.Join(req.Packages, " ")))
	}
	if len(res.Leftover) > 0 {
		warnf("", "tokens after \"--\" are ignored: %s (stow itself discards them in rc files)",
			strconv.Quote(strings.Join(res.Leftover, " ")))
	}

	return c, warns, nil
}
