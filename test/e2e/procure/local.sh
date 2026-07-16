#!/bin/sh
set -e

# ~/.local/bin is dstow's contractual install directory (DESIGN.md B6: "the
# installer contract is a floor ... ~/.local/bin default bound as *contract*
# (the snippet hardcodes it)"), so staging the binary there mirrors where a
# real install lands it.
mkdir -p "${HOME}/.local/bin"
cp /staged/dstow "${HOME}/.local/bin/dstow"
chmod +x "${HOME}/.local/bin/dstow"

export PATH="$HOME/.local/bin:$PATH"
