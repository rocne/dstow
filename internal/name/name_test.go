package name_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/rocne/dstow/internal/name"
)

// --- §1.2 Encoding -----------------------------------------------------------

func TestEncode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// §1.2 encoding table, verbatim rows.
		{"colon is scheme separator", ":", "%3A"},
		{"percent is the escape itself", "%", "%25"},
		{"at is the reserved suffix", "@", "%40"},
		{"newline is a control char", "\n", "%0A"},
		// Other control bytes and 0x7F.
		{"nul", "\x00", "%00"},
		{"unit separator 0x1F", "\x1f", "%1F"},
		{"del 0x7F", "\x7f", "%7F"},
		// Pass-through: unreserved bytes, including "/" (structural, never
		// appears inside a decoded segment) and UTF-8 multibyte sequences.
		{"plain word", "zsh", "zsh"},
		{"owner name", "rocne/dotfiles", "rocne/dotfiles"},
		{"utf8 multibyte untouched", "café", "café"},
		{"mixed reserved and plain", "a:b@c%d", "a%3Ab%40c%25d"},
		{"empty stays empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := name.Encode(tt.in); got != tt.want {
				t.Errorf("Encode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"colon", "%3A", ":"},
		{"percent", "%25", "%"},
		{"at", "%40", "@"},
		{"newline", "%0A", "\n"},
		{"lowercase hex accepted", "%3a", ":"},
		{"mixed case hex accepted", "%0a", "\n"},
		// A sequence that did not need encoding is still accepted.
		{"over-encoded uppercase A", "%41", "A"},
		{"plain pass-through", "rocne/dotfiles", "rocne/dotfiles"},
		{"utf8 untouched", "café", "café"},
		{"mixed", "a%3Ab%40c%25d", "a:b@c%d"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := name.Decode(tt.in)
			if err != nil {
				t.Fatalf("Decode(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("Decode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"lone percent", "%"},
		{"percent then one hex", "%A"},
		{"percent then one digit", "%1"},
		{"percent then non-hex", "%GG"},
		{"trailing percent", "abc%"},
		{"percent one hex then non-hex", "%1Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := name.Decode(tt.in)
			if err == nil {
				t.Fatalf("Decode(%q) = nil error, want *ParseError", tt.in)
			}
			var pe *name.ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("Decode(%q) error type = %T, want *name.ParseError", tt.in, err)
			}
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	segments := []string{
		"zsh",
		"rocne",
		"a:b",
		"x@y",
		"50%",
		"a\nb",
		"\x00\x1f\x7f",
		"café",
		"",
		"has:all%the@reserved\n",
	}
	for _, seg := range segments {
		enc := name.Encode(seg)
		dec, err := name.Decode(enc)
		if err != nil {
			t.Fatalf("Decode(Encode(%q)) error: %v", seg, err)
		}
		if dec != seg {
			t.Errorf("round trip %q -> Encode %q -> Decode %q", seg, enc, dec)
		}
	}
}

// --- §1.3 Names vs. paths ----------------------------------------------------

func TestIsPathOperand(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		// Path operands: exactly the four prefixes.
		{"/x", true},
		{"~/x", true},
		{"./x", true},
		{"../x", true},
		{"/", true},
		// §1.3: the first character decides via exactly "/", "~/", "./", "../".
		// Bare ".", "..", and "~" are therefore NAME expressions, as is a
		// dotfile like ".bashrc" and a "~x" home-ish token.
		{".", false},
		{"..", false},
		{"~", false},
		{".bashrc", false},
		{"~x", false},
		{"zsh", false},
		{"dots::zsh", false},
		{"github:rocne/dotfiles", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := name.IsPathOperand(tt.in); got != tt.want {
				t.Errorf("IsPathOperand(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// --- FQN parse, format, round trip ------------------------------------------

func TestParseFQN(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want name.FQN
	}{
		{
			"package fqn",
			"github:rocne/dotfiles::zsh",
			name.FQN{Scheme: "github", Coordinate: []string{"rocne", "dotfiles"}, Package: "zsh"},
		},
		{
			"repo fqn drops the package tail",
			"github:rocne/dotfiles",
			name.FQN{Scheme: "github", Coordinate: []string{"rocne", "dotfiles"}, Package: ""},
		},
		{
			"local absolute-path coordinate keeps a leading empty segment",
			"local:/home/rocne/dots::zsh",
			name.FQN{Scheme: "local", Coordinate: []string{"", "home", "rocne", "dots"}, Package: "zsh"},
		},
		{
			"local absolute-path repo",
			"local:/home/x",
			name.FQN{Scheme: "local", Coordinate: []string{"", "home", "x"}, Package: ""},
		},
		{
			"encoded reserved characters decode in fields",
			"local:a%3Ab/c::x%40y",
			name.FQN{Scheme: "local", Coordinate: []string{"a:b", "c"}, Package: "x@y"},
		},
		{
			"single-segment repo coordinate",
			"github:dotfiles",
			name.FQN{Scheme: "github", Coordinate: []string{"dotfiles"}, Package: ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := name.ParseFQN(tt.in)
			if err != nil {
				t.Fatalf("ParseFQN(%q) unexpected error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseFQN(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseFQNErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty input", ""},
		{"no scheme", "nocolon"},
		{"empty scheme", ":coord"},
		{"scheme contains slash", "sch/eme:coord"},
		{"empty coordinate", "local:"},
		{"raw colon in coordinate needs %3A", "local:a:b"},
		{"raw at rejected outright", "github:owner@repo::zsh"},
		{"multiple package separators", "github:a::b::c"},
		{"triple colon", "github:a:::b"},
		{"leading :: is not an fqn", "::zsh"},
		{"empty package", "github:rocne/dotfiles::"},
		{"trailing slash makes an empty segment", "local:/"},
		{"double slash makes an empty segment", "local://x"},
		{"empty middle segment", "local:a//b"},
		{"trailing empty segment", "github:x/"},
		{"bad percent escape in coordinate", "local:a%b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := name.ParseFQN(tt.in)
			if err == nil {
				t.Fatalf("ParseFQN(%q) = nil error, want *ParseError", tt.in)
			}
			var pe *name.ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("ParseFQN(%q) error type = %T, want *name.ParseError", tt.in, err)
			}
		})
	}
}

func TestFQNString(t *testing.T) {
	tests := []struct {
		name string
		in   name.FQN
		want string
	}{
		{
			"package fqn",
			name.FQN{Scheme: "github", Coordinate: []string{"rocne", "dotfiles"}, Package: "zsh"},
			"github:rocne/dotfiles::zsh",
		},
		{
			"repo fqn",
			name.FQN{Scheme: "github", Coordinate: []string{"rocne", "dotfiles"}},
			"github:rocne/dotfiles",
		},
		{
			"absolute-path coordinate re-emits the leading slash",
			name.FQN{Scheme: "local", Coordinate: []string{"", "home", "x"}},
			"local:/home/x",
		},
		{
			"reserved bytes re-encode canonically",
			name.FQN{Scheme: "local", Coordinate: []string{"a:b"}, Package: "x@y"},
			"local:a%3Ab::x%40y",
		},
		{
			"at encodes to %40",
			name.FQN{Scheme: "s", Coordinate: []string{"@"}, Package: "p"},
			"s:%40::p",
		},
		{
			"percent encodes to %25",
			name.FQN{Scheme: "s", Coordinate: []string{"%"}},
			"s:%25",
		},
		{
			"newline encodes to %0A",
			name.FQN{Scheme: "s", Coordinate: []string{"\n"}},
			"s:%0A",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.String(); got != tt.want {
				t.Errorf("(%+v).String() = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// Property: ParseFQN(f.String()) round-trips to f across the corpus.
func TestFQNStringRoundTrip(t *testing.T) {
	corpus := []name.FQN{
		{Scheme: "github", Coordinate: []string{"rocne", "dotfiles"}, Package: "zsh"},
		{Scheme: "github", Coordinate: []string{"rocne", "dotfiles"}},
		{Scheme: "local", Coordinate: []string{"", "home", "rocne", "dots"}, Package: "zsh"},
		{Scheme: "local", Coordinate: []string{"", "home", "x"}},
		{Scheme: "local", Coordinate: []string{"a:b", "c"}, Package: "x@y"},
		{Scheme: "s", Coordinate: []string{"@", "%", "\n"}, Package: "50%"},
		{Scheme: "local", Coordinate: []string{"café", "naïve"}, Package: "zsh"},
		{Scheme: "s", Coordinate: []string{"\x00\x1f\x7f"}},
	}
	for _, f := range corpus {
		s := f.String()
		got, err := name.ParseFQN(s)
		if err != nil {
			t.Fatalf("ParseFQN(%q) from %+v: unexpected error %v", s, f, err)
		}
		if !reflect.DeepEqual(got, f) {
			t.Errorf("round trip %+v -> String %q -> ParseFQN %+v", f, s, got)
		}
	}
}

func TestFQNIsPackageAndRepo(t *testing.T) {
	pkg := name.FQN{Scheme: "github", Coordinate: []string{"rocne", "dotfiles"}, Package: "zsh"}
	repo := name.FQN{Scheme: "github", Coordinate: []string{"rocne", "dotfiles"}}

	if !pkg.IsPackage() {
		t.Errorf("pkg.IsPackage() = false, want true")
	}
	if repo.IsPackage() {
		t.Errorf("repo.IsPackage() = true, want false")
	}
	if got := pkg.Repo(); !reflect.DeepEqual(got, repo) {
		t.Errorf("pkg.Repo() = %+v, want %+v", got, repo)
	}
	if got := repo.Repo(); !reflect.DeepEqual(got, repo) {
		t.Errorf("repo.Repo() = %+v, want %+v", got, repo)
	}
}
