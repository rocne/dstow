// Package dstow carries repo-root artifacts that must ride inside the binary.
// The go:embed directive cannot reach above a package's directory, so files
// vendored at the repo root (snippet.sh, owned by release-ci per D26) are
// embedded here and consumed by the internal packages.
package dstow

import "embed"

// RCSnippet is the vendored snippet.sh, verbatim — the canonical rc bootstrap
// (DESIGN §9.1 B1 as amended: authored in release-ci, vendored beside
// install.sh, embedded so `dstow snippet rc` emits the one canonical file
// with zero transcription drift (B2)).
//
//go:embed snippet.sh
var RCSnippet string

// Manual is the tracked docs/ tree, embedded whole — the source the hidden
// `manual` command group mirrors (§2.1's carve-out): directories become
// groups, markdown files become leaves, and every node prints its own file.
// The tree is the single source of truth, so the `all:` prefix is required —
// without it go:embed silently drops files beginning with "." or "_", and a
// dropped file is a topic that exists in the repo and nowhere in the binary.
//
//go:embed all:docs
var Manual embed.FS
