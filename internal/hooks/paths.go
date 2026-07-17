package hooks

import (
	"path/filepath"

	"github.com/rocne/dstow/internal/config"
)

// ScopeHooksDir returns the hooks directory of a repo or package root:
// <scopeRoot>/.dstow/hooks (M6). Discovery goes through config's metadata
// accessor (A11), so hooks and config can never disagree about where metadata
// lives. The global level's hooks directory is not this — it is
// GlobalScope.Dir/hooks (the caller supplies the global config dir for
// testability; see GlobalScope).
func ScopeHooksDir(scopeRoot string) string {
	return filepath.Join(config.MetadataDir(scopeRoot), "hooks")
}
