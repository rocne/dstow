#!/usr/bin/env sh
# Shell rc snippet: adds ~/.local/bin to PATH and bootstrap-installs a tool on first use.

# ---- vendored config (per-consumer) ----
RELEASE_CI_REPO="rocne/dstow"   # GitHub slug, e.g. rocne/dstow.
RELEASE_CI_BIN=""    # installed binary name; only set if it diverges from the repo name.
# ---- end vendored config ----

RELEASE_CI_BIN="${RELEASE_CI_BIN:-${RELEASE_CI_REPO##*/}}"

case ":$PATH:" in
  *":$HOME/.local/bin:"*) ;;
  *) PATH="$HOME/.local/bin:$PATH" ;;
esac

if [ -n "$RELEASE_CI_BIN" ] \
  && ! command -v "$RELEASE_CI_BIN" >/dev/null 2>&1 \
  && command -v curl >/dev/null 2>&1; then
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 \
    "https://raw.githubusercontent.com/$RELEASE_CI_REPO/main/install.sh" | sh || true
fi

unset RELEASE_CI_REPO RELEASE_CI_BIN
