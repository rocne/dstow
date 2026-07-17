package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

// C8: path-valued keys expand ~ and $VAR/${VAR} — stow's grammar — at use
// time, per invocation. The grammar mirrors gostow's stowrc expansion
// (backslash escapes, braced and bare variables, leading tilde with an
// optional user), but the failure shape is dstow's: an unset variable is a
// loud ExpandError naming variable + file + key, never stow's die bytes.
var (
	reEnvBraced = regexp.MustCompile(`\$\{(\w+)\}`)
	reEnvBare   = regexp.MustCompile(`\$(\w+)`)
	reTilde     = regexp.MustCompile(`^~([^/]*)`)
)

// expandPath applies C8 expansion to one path-valued setting. file and key
// are provenance for the error message.
func expandPath(value, file, key string) (string, error) {
	expanded := value
	for _, re := range []*regexp.Regexp{reEnvBraced, reEnvBare} {
		var unset string
		var b strings.Builder
		last := 0
		for _, loc := range re.FindAllStringSubmatchIndex(expanded, -1) {
			start, end := loc[0], loc[1]
			// A backslash before the '$' escapes the substitution.
			if start > 0 && expanded[start-1] == '\\' {
				continue
			}
			name := expanded[loc[2]:loc[3]]
			val, ok := os.LookupEnv(name)
			if !ok && unset == "" {
				unset = name
			}
			b.WriteString(expanded[last:start])
			b.WriteString(val)
			last = end
		}
		b.WriteString(expanded[last:])
		if unset != "" {
			return "", &ExpandError{
				File:  file,
				Key:   key,
				Value: value,
				Reason: fmt.Sprintf("references the undefined environment variable $%s; set the variable, or fix the value in %s",
					unset, file),
			}
		}
		expanded = b.String()
	}
	expanded = strings.ReplaceAll(expanded, `\$`, "$")
	expanded = expandTilde(expanded)
	return strings.ReplaceAll(expanded, `\~`, "~"), nil
}

// expandTilde resolves a leading ~ or ~user. An unknown user leaves the
// token alone — the absoluteness check then rejects it with full
// provenance, rather than guessing a directory the user never named.
func expandTilde(path string) string {
	return reTilde.ReplaceAllStringFunc(path, func(m string) string {
		name := m[1:]
		if name == "" {
			if home, ok := os.LookupEnv("HOME"); ok && home != "" {
				return home
			}
			if home, err := os.UserHomeDir(); err == nil {
				return home
			}
			return m
		}
		if u, err := user.Lookup(name); err == nil {
			return u.HomeDir
		}
		return m
	})
}

// expandAbsolute is expandPath plus C8's closing rule: the expanded result
// must be absolute.
func expandAbsolute(value, file, key string) (string, error) {
	expanded, err := expandPath(value, file, key)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(expanded) {
		return "", &ExpandError{
			File:  file,
			Key:   key,
			Value: value,
			Reason: fmt.Sprintf("expands to %q, which is not an absolute path; path-valued keys must expand to absolute paths (C8)",
				expanded),
		}
	}
	return expanded, nil
}
