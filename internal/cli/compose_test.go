package cli

import (
	"testing"

	"github.com/rocne/dstow/internal/ops"
	"github.com/rocne/dstow/internal/ui"
)

// TestParseColorMode asserts the --color <when> resolution (§7.2/O6): one of
// the three words, or absent ("" — the flag's declared default, which means
// auto); anything else is an error. Bare `--color` without a value is still
// refused, at the flag layer (pflag requires a value for a string flag).
func TestParseColorMode(t *testing.T) {
	cases := []struct {
		in      string
		want    ui.ColorMode
		wantErr bool
	}{
		{"auto", ui.ColorAuto, false},
		{"always", ui.ColorAlways, false},
		{"never", ui.ColorNever, false},
		{"", ui.ColorAuto, false},
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

// TestParseColorFormat asserts the theme show --format resolution (rendered
// default, env, toml, error otherwise) — a format flag never changes the
// concept.
func TestParseColorFormat(t *testing.T) {
	if f, err := parseColorFormat(""); err != nil || f != ops.ColorFormatRendered {
		t.Errorf("empty format should default to the rendered view, got %v err %v", f, err)
	}
	if f, err := parseColorFormat("env"); err != nil || f != ops.ColorFormatEnv {
		t.Errorf("env is a valid format, got %v err %v", f, err)
	}
	if f, err := parseColorFormat("toml"); err != nil || f != ops.ColorFormatTOML {
		t.Errorf("toml is a valid format, got %v err %v", f, err)
	}
	if _, err := parseColorFormat("yaml"); err == nil {
		t.Errorf("an unknown format must error")
	}
}
