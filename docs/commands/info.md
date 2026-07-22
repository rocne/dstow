# dstow info

<!-- dstow:short -->
Everything dstow knows about one repo or package
<!-- /dstow:short -->

<!-- dstow:long -->
One scope's fields — the facts and effective config dstow holds about it,
from configuration and metadata, never by inspecting targets (that is
status's job). The scope is the global installation (named by no operand),
a repo, or a package.

With no name, details the global scope. Fields come in two groups: inherent
facts of the thing as it exists (version and paths; a package's repo, scheme,
managed path, qualified name) — permanently read-only — and
configured values from the config chain (effective target, dot-translation,
fold, ignores). The full human view prints the inherent group first, then
the configured group.

Exit status:
  0   the requested field(s) carry a value
  1   applicable but unset or empty (shown as (unset) or [])
  2   unknown field, or a field illegal for the scope (names a suggestion)
  3   global refusal
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow info                       # the global scope
  dstow info zsh
  dstow info dotfiles -f qualified-name -f scheme
  dstow info -r -f target          # target across every scope
<!-- /dstow:examples -->
