// Package cli is dstow's command-line front end.
package cli

import "io"

// Run is scaffold plumbing: it pins the entry signature
// (args, version, stdin, stdout, stderr) that cmd/dstow wires against so the
// rest of the scaffold (build, CI, release) has something real to compile and
// ship. The actual command surface — cobra wiring, verbs, exit-code mapping —
// is built out by the cli ticket (#47); this stub does nothing but return 0.
func Run(args []string, version string, stdin io.Reader, stdout, stderr io.Writer) int {
	return 0
}
