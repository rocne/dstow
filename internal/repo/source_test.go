package repo_test

import (
	"strings"
	"testing"

	"github.com/rocne/dstow/internal/repo"
)

func TestParseSourceGithubHappyPath(t *testing.T) {
	s, err := repo.ParseSource("github:rocne/dotfiles")
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	if s.Scheme != "github" {
		t.Errorf("scheme = %q, want github", s.Scheme)
	}
	if got, want := strings.Join(s.Coordinate, "/"), "rocne/dotfiles"; got != want {
		t.Errorf("coordinate = %q, want %q", got, want)
	}
	if s.String() != "github:rocne/dotfiles" {
		t.Errorf("String() = %q, want github:rocne/dotfiles", s.String())
	}
}

// A percent-encoded segment decodes on parse and re-encodes losslessly on
// String (§1.2).
func TestParseSourcePercentEncodedRoundTrip(t *testing.T) {
	const in = "github:rocne/dot%3Afiles"
	s, err := repo.ParseSource(in)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	if s.Coordinate[1] != "dot:files" {
		t.Errorf("decoded segment = %q, want dot:files", s.Coordinate[1])
	}
	if s.String() != in {
		t.Errorf("String() = %q, want %q", s.String(), in)
	}
}

func TestParseSourceLocalAbsolute(t *testing.T) {
	s, err := repo.ParseSource("local:/home/rocne/dotfiles")
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	if s.Scheme != "local" {
		t.Errorf("scheme = %q, want local", s.Scheme)
	}
	if s.String() != "local:/home/rocne/dotfiles" {
		t.Errorf("String() = %q, want local:/home/rocne/dotfiles", s.String())
	}
}

func TestParseSourceErrors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSubs []string // substrings the error must name (the remedy/shape)
	}{
		{"github wrong arity too few", "github:rocne", []string{"github:owner/name", "two"}},
		{"github wrong arity too many", "github:rocne/dotfiles/extra", []string{"github:owner/name", "two"}},
		{"unknown scheme", "gitlab:o/r", []string{"unknown scheme", "github", "local"}},
		{"relative local", "local:dotfiles", []string{"absolute", "canonicalizes"}},
		{"relative local nested", "local:sub/dir", []string{"absolute", "canonicalizes"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.ParseSource(tt.input)
			if err == nil {
				t.Fatalf("ParseSource(%q) = nil error", tt.input)
			}
			msg := err.Error()
			for _, sub := range tt.wantSubs {
				if !strings.Contains(msg, sub) {
					t.Errorf("error %q does not name %q", msg, sub)
				}
			}
		})
	}
}

func TestClassifySourceInput(t *testing.T) {
	tests := []struct {
		input string
		want  repo.Classification
	}{
		{"/abs", repo.PathForm},
		{"~/x", repo.PathForm},
		{"./x", repo.PathForm},
		{"../x", repo.PathForm},
		{"https://github.com/o/r.git", repo.URLForm},
		{"git@github.com:o/r.git", repo.URLForm},
		{"github:o/r", repo.QualifiedForm},
		{"local:/home/x", repo.QualifiedForm},
		{"o/r", repo.BareForm},
		{"zsh", repo.BareForm},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := repo.ClassifySourceInput(tt.input); got != tt.want {
				t.Errorf("ClassifySourceInput(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
