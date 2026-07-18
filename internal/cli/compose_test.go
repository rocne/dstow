package cli

import (
	"testing"

	"github.com/rocne/dstow/internal/ui"
)

// TestParseColorMode asserts the --color <when> resolution (§7.2/O6): the value
// is required and one of the three words; anything else is an error.
func TestParseColorMode(t *testing.T) {
	cases := []struct {
		in      string
		want    ui.ColorMode
		wantErr bool
	}{
		{"auto", ui.ColorAuto, false},
		{"always", ui.ColorAlways, false},
		{"never", ui.ColorNever, false},
		{"", ui.ColorAuto, true},
		{"purple", ui.ColorAuto, true},
		{"Auto", ui.ColorAuto, true}, // case-sensitive, per the pinned words
	}
	for _, tc := range cases {
		got, err := parseColorMode(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("parseColorMode(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
		}
		if err == nil && got != tc.want {
			t.Errorf("parseColorMode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestParseColorFormat asserts the colors --format resolution (env default,
// toml, error otherwise) — a format flag never changes the concept.
func TestParseColorFormat(t *testing.T) {
	if f, err := parseColorFormat(""); err != nil || f != 0 {
		t.Errorf("empty format should default to env, got %v err %v", f, err)
	}
	if _, err := parseColorFormat("toml"); err != nil {
		t.Errorf("toml is a valid format: %v", err)
	}
	if _, err := parseColorFormat("yaml"); err == nil {
		t.Errorf("an unknown format must error")
	}
}
