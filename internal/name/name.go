// Package name is dstow's pure naming grammar: parse and format fully
// qualified names (FQNs), percent-encode and -decode coordinate segments,
// resolve segment-boundary suffix matches, force package-kind with a leading
// "::", and classify an operand as a path or a name expression.
//
// The grammar is scheme:coordinate::package (DESIGN.md §1). A repo drops the
// "::package" tail. ":" separates only the scheme; "::" separates only the
// package; every reserved byte percent-encodes (§1.2) so every path is
// spellable. The package is pure (A7): zero I/O, zero dependencies, stdlib
// only, and nothing OS-dependent — just string and byte functions.
package name

import "fmt"

// ParseError is the package's typed error. Every error returned by this
// package is a *ParseError. Reason is complete prose: what rule the input
// violates and, where the spec names one, the remedy.
type ParseError struct {
	Input  string
	Reason string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("dstow/name: cannot parse %q: %s", e.Input, e.Reason)
}

// rewrap re-homes a *ParseError (typically from Decode of a sub-segment) onto
// the full input string the caller was parsing, preserving the Reason.
func rewrap(err error, input string) *ParseError {
	if pe, ok := err.(*ParseError); ok {
		return &ParseError{Input: input, Reason: pe.Reason}
	}
	return &ParseError{Input: input, Reason: err.Error()}
}

const upperHex = "0123456789ABCDEF"

// isReserved reports whether byte c must percent-encode per §1.2: the scheme
// separator ":", the escape "%", the reserved suffix "@", control bytes
// 0x00–0x1F, and 0x7F. "/" is structural — it never appears inside a decoded
// segment and is never encoded here.
func isReserved(c byte) bool {
	return c == ':' || c == '%' || c == '@' || c < 0x20 || c == 0x7F
}

// Encode percent-encodes one decoded segment into canonical form. It is
// byte-oriented over the UTF-8 string: each reserved byte becomes "%XX" with
// uppercase hex; every other byte (including UTF-8 multibyte sequences) passes
// through untouched.
func Encode(segment string) string {
	// Fast path: nothing to encode.
	needs := false
	for i := 0; i < len(segment); i++ {
		if isReserved(segment[i]) {
			needs = true
			break
		}
	}
	if !needs {
		return segment
	}
	var b []byte
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		if isReserved(c) {
			b = append(b, '%', upperHex[c>>4], upperHex[c&0x0F])
			continue
		}
		b = append(b, c)
	}
	return string(b)
}

// Decode reverses Encode. It accepts "%XX" with hex of either case (and
// accepts sequences that did not need encoding, e.g. "%41"), and errors on a
// "%" not followed by two hex digits.
func Decode(segment string) (string, error) {
	// Fast path: no escapes present.
	if !containsByte(segment, '%') {
		return segment, nil
	}
	var b []byte
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		if c != '%' {
			b = append(b, c)
			continue
		}
		if i+2 >= len(segment) {
			return "", &ParseError{
				Input:  segment,
				Reason: "a '%' escape must be followed by two hex digits (e.g. %3A); write a literal '%' as %25",
			}
		}
		hi, ok1 := unhex(segment[i+1])
		lo, ok2 := unhex(segment[i+2])
		if !ok1 || !ok2 {
			return "", &ParseError{
				Input:  segment,
				Reason: "a '%' escape must be followed by two hex digits (e.g. %3A); write a literal '%' as %25",
			}
		}
		b = append(b, hi<<4|lo)
		i += 2
	}
	return string(b), nil
}

// IsPathOperand reports whether s is a path operand per §1.3: it starts with
// "/", "~/", "./", or "../". Everything else is a name expression. Exactly
// those four prefixes — no more, no less (so ".", "..", "~", and ".bashrc"
// are name expressions).
func IsPathOperand(s string) bool {
	switch {
	case len(s) >= 1 && s[0] == '/':
		return true
	case len(s) >= 2 && s[0] == '~' && s[1] == '/':
		return true
	case len(s) >= 2 && s[0] == '.' && s[1] == '/':
		return true
	case len(s) >= 3 && s[0] == '.' && s[1] == '.' && s[2] == '/':
		return true
	}
	return false
}

func unhex(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func containsByte(s string, b byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return true
		}
	}
	return false
}
