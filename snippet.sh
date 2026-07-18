#!/usr/bin/env sh
# snippet.sh — the canonical rc snippet (DESIGN.md §6.7, D26). This is the line
# that lands in users' shell rc files and stays there for years: it keeps the
# contractual install dir on PATH and performs a one-time bootstrap install.
#
# Canonical source: rocne/release-ci, installer/snippet.sh. Vendored beside
# install.sh to consumers that surface an rc snippet (today: dstow). Everything
# below the config block is byte-identical across consumers.
#
# This file is *sourced* from a user's rc, so it must never break shell
# startup and must leave nothing behind: the config vars are RELEASE_CI_-
# prefixed (a bare REPO/TOOL would clobber the user's own variables — the same
# collision class D8/D21 killed for INSTALL_DIR) and are unset on every path.

# ---- vendored config (per-consumer; everything below is canonical) ----
RELEASE_CI_REPO="rocne/dstow"   # GitHub slug, e.g. rocne/dstow. Set at vendor time.
RELEASE_CI_TOOL="dstow"   # installed binary name. Set at vendor time.
# ---- end vendored config ----

# The PATH line bakes the contractual default (F3): consumer snippets may rely
# on the *default* resolving to ~/.local/bin; the installer's dir is otherwise
# fully tunable (D9). Prepended before the guard, so the guard sees it (B1).
case ":$PATH:" in
  *":$HOME/.local/bin:"*) ;;
  *) PATH="$HOME/.local/bin:$PATH" ;;
esac

# One-time bootstrap: a quiet no-op once the tool is present. Skips silently
# when curl is absent (an rc file must never break shell startup) and on the
# unvendored canonical copy (empty config). The installer's own presence check
# never runs here — this guard short-circuits before any network.
if [ -n "$RELEASE_CI_TOOL" ] \
  && ! command -v "$RELEASE_CI_TOOL" >/dev/null 2>&1 \
  && command -v curl >/dev/null 2>&1; then
  # || true: a failed bootstrap must not abort sourcing in a `set -e` rc —
  # the installer's own stderr still reports what went wrong.
  curl -fsSL --proto '=https' --tlsv1.2 --retry 3 \
    "https://raw.githubusercontent.com/$RELEASE_CI_REPO/main/install.sh" | sh || true
fi

unset RELEASE_CI_REPO RELEASE_CI_TOOL
