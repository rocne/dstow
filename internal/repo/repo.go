// Package repo is dstow's repo set (DESIGN.md A9): the repo registry
// (read/write with the same temp-file+fsync+rename discipline as the ledger),
// the source grammar and its internal github/local schemes, the managed-clone
// directory layout (A19), package enumeration (M2/M3), and name resolution
// over the set through the pure name package. Cross-domain guards (still-stowed,
// unsaved-work) compose in ops, never here.
//
// The package returns data and typed errors only (A4): every diagnostic comes
// back as a Warning value and the caller (ops) decides how to render it. It
// depends on the name package for the grammar and on adrg/xdg + BurntSushi/toml
// for the registry; it must not import config — config owns knowing where the
// registry file lives and hands repo the path, keeping the dependency arrow
// config-free both ways.
package repo

// Warning is a diagnostic as data (A4), mirroring config's Warning shape so the
// caller renders both the same way: Source names where it came from (a file
// path, a repo root), Detail is complete prose, and Fix — when set — is the
// remedy line (O2's fix severity). repo defines its own rather than importing
// config's, to keep the dependency arrow config-free.
type Warning struct {
	Source string // where it came from: a file path, a repo root, …
	Detail string // complete prose: what is wrong and that the rest applies
	Fix    string // optional remedy
}
