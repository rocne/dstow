# dstow bootstrap — https://github.com/rocne/dstow
# Ensure the install dir is on PATH, then install dstow only if missing.
# POSIX "is dir in PATH" idiom (builtin, fork-free):
#   https://unix.stackexchange.com/q/32210
case ":$PATH:" in
  *":$HOME/.local/bin:"*) ;;                 # already on PATH
  *) PATH="$HOME/.local/bin:$PATH" ;;
esac

if ! command -v dstow >/dev/null 2>&1; then
  curl -fsSL https://raw.githubusercontent.com/rocne/dstow/main/install.sh | sh
fi
