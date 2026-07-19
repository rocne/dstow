// Package dstow carries repo-root artifacts that must ride inside the binary.
// go:embed cannot reach above a package's directory, so files vendored at the
// repo root (snippet.sh, owned by release-ci per D26) are embedded here and
// consumed by the internal packages.
package dstow

import _ "embed"

// RCSnippet is the vendored snippet.sh, verbatim — the canonical rc bootstrap
// (DESIGN §9.1 B1 as amended: authored in release-ci, vendored beside
// install.sh, embedded so `dstow snippet rc` emits the one canonical file
// with zero transcription drift (B2)).
//
//go:embed snippet.sh
var RCSnippet string
