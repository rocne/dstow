package name_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/rocne/dstow/internal/name"
)

func TestParseExpr(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want name.Expr
	}{
		// §1.1 suffix chain: zsh -> dots::zsh -> rocne/dotfiles::zsh.
		{
			"bare name",
			"zsh",
			name.Expr{Segments: []string{"zsh"}},
		},
		{
			"repo-qualified package",
			"dots::zsh",
			name.Expr{Segments: []string{"dots"}, HasPackage: true, Package: "zsh"},
		},
		{
			"multi-segment repo-qualified package",
			"rocne/dotfiles::zsh",
			name.Expr{Segments: []string{"rocne", "dotfiles"}, HasPackage: true, Package: "zsh"},
		},
		{
			// A leading :: forces package-kind: empty Segments, HasPackage true.
			"kind-forcing leading ::",
			"::zsh",
			name.Expr{Segments: nil, HasPackage: true, Package: "zsh"},
		},
		{
			"scheme-qualified full name",
			"github:rocne/dotfiles::zsh",
			name.Expr{Scheme: "github", Segments: []string{"rocne", "dotfiles"}, HasPackage: true, Package: "zsh"},
		},
		{
			"scheme-qualified repo",
			"github:rocne/dotfiles",
			name.Expr{Scheme: "github", Segments: []string{"rocne", "dotfiles"}},
		},
		{
			"absolute-path coordinate expression",
			"local:/home/x",
			name.Expr{Scheme: "local", Segments: []string{"", "home", "x"}},
		},
		{
			"reserved @-suffix is parsed, opaque and decoded",
			"zsh@1.0",
			name.Expr{Segments: []string{"zsh"}, AtSuffix: "1.0"},
		},
		{
			"encoded reserved character decodes in a segment",
			"a%3Ab",
			name.Expr{Segments: []string{"a:b"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := name.ParseExpr(tt.in)
			if err != nil {
				t.Fatalf("ParseExpr(%q) unexpected error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseExpr(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseExprErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty input", ""},
		{"multiple package separators", "a::b::c"},
		{"empty package after ::", "::"},
		{"second @ needs %40", "zsh@a@b"},
		{"raw colon in coordinate needs %3A", "github:a:b"},
		{"empty scheme", ":x"},
		{"empty segment without scheme", "a//b"},
		{"leading empty segment without scheme", "/home"},
		{"suffix with no coordinate", "@foo"},
		{"bad percent escape", "a%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := name.ParseExpr(tt.in)
			if err == nil {
				t.Fatalf("ParseExpr(%q) = nil error, want *ParseError", tt.in)
			}
			var pe *name.ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("ParseExpr(%q) error type = %T, want *name.ParseError", tt.in, err)
			}
		})
	}
}

// --- §1.1 suffix resolution / matching --------------------------------------

func TestMatches(t *testing.T) {
	// Corpus of entities.
	pkgGitHub := name.FQN{Scheme: "github", Coordinate: []string{"rocne", "dotfiles"}, Package: "zsh"}
	repoGitHub := name.FQN{Scheme: "github", Coordinate: []string{"rocne", "dotfiles"}}
	pkgLocal := name.FQN{Scheme: "local", Coordinate: []string{"", "home", "rocne", "dots"}, Package: "zsh"}
	// A repo whose coordinate tail is "zsh" — used for the cross-kind pair.
	repoZshTail := name.FQN{Scheme: "local", Coordinate: []string{"rocne", "zsh"}}

	tests := []struct {
		name string
		expr string
		f    name.FQN
		want bool
	}{
		// Suffix chain resolves against the package entity.
		{"bare name matches package", "zsh", pkgGitHub, true},
		{"repo-qualified package matches", "dotfiles::zsh", pkgGitHub, true},
		{"one-segment tail + package matches", "dots::zsh", pkgLocal, true},
		{"multi-segment tail + package matches", "rocne/dotfiles::zsh", pkgGitHub, true},
		{"full scheme-qualified name matches", "github:rocne/dotfiles::zsh", pkgGitHub, true},

		// Kind-forcing: ::zsh matches the package but never a repo.
		{"kind-forcing matches package", "::zsh", pkgGitHub, true},
		{"kind-forcing never matches repo", "::zsh", repoGitHub, false},

		// Cross-kind bare name: a single segment matches BOTH a package (by
		// package name) AND a repo (by coordinate tail) — two assertions.
		{"single-segment bare matches package entity", "zsh", pkgGitHub, true},
		{"single-segment bare matches repo tail", "zsh", repoZshTail, true},
		// A multi-segment bare expression is repo-shaped: it matches a repo
		// tail but never a package entity.
		{"multi-segment bare matches repo tail", "rocne/dotfiles", repoGitHub, true},
		{"multi-segment bare never matches package", "rocne/dotfiles", pkgGitHub, false},

		// A scheme attaches only to the FULL coordinate — no partial suffix.
		{"scheme requires full coordinate", "github:dotfiles::zsh", pkgGitHub, false},
		{"scheme-qualified repo matches repo", "github:rocne/dotfiles", repoGitHub, true},
		{"scheme-qualified repo does not match package", "github:rocne/dotfiles", pkgGitHub, false},
		{"wrong scheme does not match", "gitlab:rocne/dotfiles::zsh", pkgGitHub, false},

		// Repo tails.
		{"repo tail matches repo", "dotfiles", repoGitHub, true},
		{"non-tail bare does not match repo", "zsh", repoGitHub, false},

		// Package-name mismatches.
		{"wrong package does not match", "dots::bash", pkgLocal, false},
		{"non-tail with package does not match", "xyz::zsh", pkgGitHub, false},
		{"bare name that is not the package does not match", "dotfiles", pkgGitHub, false},

		// Reserved @-suffix never matches anything in v1.
		{"at-suffix never matches", "zsh@1.0", pkgGitHub, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := name.ParseExpr(tt.expr)
			if err != nil {
				t.Fatalf("ParseExpr(%q) unexpected error: %v", tt.expr, err)
			}
			if got := e.Matches(tt.f); got != tt.want {
				t.Errorf("ParseExpr(%q).Matches(%q) = %v, want %v", tt.expr, tt.f.String(), got, tt.want)
			}
		})
	}
}
