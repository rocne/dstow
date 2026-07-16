package main

import (
	"runtime/debug"
	"testing"
)

// resolveVersion exists because two build paths produce dstow and only one of
// them sets the ldflag.
//
// `go install github.com/rocne/dstow/cmd/dstow@v0.1.0` passes no ldflags, so
// v0.1.0 installed that way would report "dev" — a lie, since the toolchain
// knows exactly which module version it fetched and has already stamped it
// into the build info.
func TestResolveVersion(t *testing.T) {
	buildInfo := func(v string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: v}}, true
		}
	}
	noBuildInfo := func() (*debug.BuildInfo, bool) { return nil, false }

	for _, tc := range []struct {
		name   string
		ldflag string
		info   func() (*debug.BuildInfo, bool)
		want   string
	}{
		{"goreleaser sets the ldflag", "v0.1.0", buildInfo("(devel)"), "v0.1.0"},
		{"ldflag beats build info", "v0.2.0", buildInfo("v0.1.0"), "v0.2.0"},
		{"go install: fall back to the module version", "dev", buildInfo("v0.1.0"), "v0.1.0"},
		{"go build from a working tree", "dev", buildInfo("(devel)"), "dev"},
		{"build info with no version", "dev", buildInfo(""), "dev"},
		{"no build info at all", "dev", noBuildInfo, "dev"},
		{"empty ldflag is not a version", "", buildInfo("v0.1.0"), "v0.1.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveVersion(tc.ldflag, tc.info); got != tc.want {
				t.Errorf("resolveVersion(%q, …) = %q, want %q", tc.ldflag, got, tc.want)
			}
		})
	}
}
