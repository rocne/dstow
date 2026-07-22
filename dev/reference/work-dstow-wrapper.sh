#!/usr/bin/env sh
# dstow - stow wrapper for flat dotfiles packages

DOTFILES="$HOME/dotfiles"

# --- Colors (disabled if not a terminal) ---
if [ -t 2 ]; then
  _c_green='\033[32m'
  _c_red='\033[31m'
  _c_yellow='\033[33m'
  _c_dim='\033[2m'
  _c_bold='\033[1m'
  _c_reset='\033[0m'
else
  _c_green='' _c_red='' _c_yellow='' _c_dim='' _c_bold='' _c_reset=''
fi

# Print a status line: _status TAG COLOR PACKAGE TARGET [detail]
_status() {
  tag="$1" color="$2" pkg="$3" tgt="$4" detail="${5:-}"
  # Shorten $HOME to ~
  case "$tgt" in "$HOME"*) tgt="~${tgt#"$HOME"}" ;; esac
  printf "  ${color}[%-8s]${_c_reset}  %-20s -> %s\n" "$tag" "$pkg" "$tgt" >&2
  if [ -n "$detail" ]; then printf "              %s\n" "$detail" >&2; fi
}

show_help() {
  cat <<'EOF'
dstow - stow wrapper for flat dotfiles packages

Usage:
  dstow [stow-options] package...    Stow packages (reads .dstowrc for target)
  dstow --all [stow-options]         Stow all non-ignored packages
  dstow --adopt package...           Adopt existing files into dotfiles
  dstow --list                       List all packages and their targets
  dstow --check                      Find broken symlinks from old stow state
  dstow --clean                      Remove broken symlinks from old stow state
  dstow --help                       Show this help

Examples:
  dstow bash             Stow the bash package
  dstow -D bash          Unstow the bash package
  dstow -R bash zsh      Restow multiple packages
  dstow --all            Stow everything
  dstow -R --all         Restow everything
  dstow --adopt meta     Adopt existing meta files into dotfiles
  dstow --list           List all packages

Adopt:
  When a target file already exists (not a symlink), --adopt moves
  the existing file into the dotfiles package and replaces it with
  a symlink. Useful for importing existing configs.

.dstowrc:
  Optional config file inside a package directory:
    target=~/.claude/skills    Override stow target (default: ~)
    ignore=true                Skip this directory entirely

Dependencies:
  Packages can include a 'deps' file with one dependency per line:
    command_name: install command here
  dstow will check for each command and run the install if missing.

Proxy:
  If installs need a proxy, set dstow_https_proxy and dstow_http_proxy.
EOF
}

# Read .dstowrc from a package directory.
# Sets two variables via eval:
#   pkg_target  - stow target dir (empty = use default ~)
#   pkg_ignore  - "true" if package should be skipped
read_dstowrc() {
  _rc="$1/.dstowrc"
  _target=""
  _ignore="false"
  if [ -f "$_rc" ]; then
    while IFS='=' read -r key value; do
      case "$key" in
        target)  _target="$value" ;;
        ignore)  _ignore="$value" ;;
      esac
    done < "$_rc"
  fi
  case "$_target" in
    '~'*) _target="$HOME${_target#'~'}" ;;
  esac
  printf 'pkg_target=%s\npkg_ignore=%s\n' "$_target" "$_ignore"
}

install_deps() {
  deps_file="$1"
  [ -f "$deps_file" ] || return 0

  while IFS=: read -r cmd install_cmd; do
    case "$cmd" in
      \#*|"") continue ;;
    esac

    cmd=$(echo "$cmd" | tr -d ' ')
    install_cmd=$(echo "$install_cmd" | sed 's/^ *//')

    if command -v "$cmd" >/dev/null 2>&1; then
      continue
    fi

    printf "  ${_c_dim}installing dependency '%s'...${_c_reset}\n" "$cmd" >&2

    (
      if [ -n "$dstow_https_proxy" ]; then
        export https_proxy="$dstow_https_proxy"
        export http_proxy="${dstow_http_proxy:-$dstow_https_proxy}"
      fi
      eval "$install_cmd"
    )

    if ! command -v "$cmd" >/dev/null 2>&1; then
      printf "  ${_c_red}failed to install '%s'${_c_reset}\n" "$cmd" >&2
      echo "  if you need a proxy, set dstow_https_proxy and dstow_http_proxy" >&2
      return 1
    fi
  done < "$deps_file"
}

stow_package() {
  package="$1"
  shift
  opts="$*"

  pkg_dir="$DOTFILES/$package"
  if [ ! -d "$pkg_dir" ]; then
    _status "ERROR" "$_c_red" "$package" "?" "package not found"
    return 1
  fi

  pkg_target="" pkg_ignore="false"
  eval "$(read_dstowrc "$pkg_dir")"

  if [ "$pkg_ignore" = "true" ]; then
    _status "SKIP" "$_c_dim" "$package" "-" "ignored"
    return 0
  fi

  target="${pkg_target:-$HOME}"
  mkdir -p "$target"

  # Install deps on stow (not unstow or restow)
  case "$opts" in
    *-D*) ;;
    *-R*) ;;
    *)    install_deps "$pkg_dir/deps" || return 1 ;;
  esac

  # Dry-run first to detect state (stow outputs to stderr)
  # shellcheck disable=SC2086 # intentional word splitting for stow options
  dry_output=$(stow --dotfiles --no-folding --dir="$DOTFILES" --target="$target" \
    -n -v $opts "$package" 2>&1)

  # Check for conflicts (stow says "cannot stow ... over existing target")
  if echo "$dry_output" | grep -q "over existing target"; then
    _status "CONFLICT" "$_c_red" "$package" "$target"
    echo "$dry_output" | grep "over existing target" | while read -r line; do
      fname=$(echo "$line" | sed 's/.*over existing target //' | sed 's/ since.*//')
      printf "              ${_c_red}%s${_c_reset} already exists\n" "$fname" >&2
    done
    printf "              ${_c_yellow}fix: dstow --adopt %s${_c_reset}\n" "$package" >&2
    return 1
  fi

  # Check if there's anything to do (look for LINK: or UNLINK: in verbose output)
  if ! echo "$dry_output" | grep -qE "^(LINK|UNLINK):"; then
    case "$opts" in
      *-D*) _status "NO-OP" "$_c_dim" "$package" "$target" "not stowed" ;;
      *)    _status "NO-OP" "$_c_dim" "$package" "$target" ;;
    esac
    return 0
  fi

  # Actually run stow
  # shellcheck disable=SC2086 # intentional word splitting for stow options
  stow_output=$(stow --dotfiles --no-folding --dir="$DOTFILES" --target="$target" \
    $opts "$package" 2>&1)
  stow_rc=$?

  if [ $stow_rc -eq 0 ]; then
    case "$opts" in
      *-D*)      _status "UNLINKED" "$_c_yellow" "$package" "$target" ;;
      *-R*)      _status "RESTOWED" "$_c_green" "$package" "$target" ;;
      *--adopt*) _status "ADOPTED" "$_c_green" "$package" "$target" ;;
      *)         _status "LINKED" "$_c_green" "$package" "$target" ;;
    esac
  else
    _status "ERROR" "$_c_red" "$package" "$target"
    echo "$stow_output" | while read -r line; do
      [ -n "$line" ] && printf "              %s\n" "$line" >&2
    done
    return 1
  fi
}

# Collect directories that stow manages (union of all package targets + top-level entries)
stow_search_dirs() {
  for pkg_dir in "$DOTFILES"/*/; do
    [ -d "$pkg_dir" ] || continue
    name="$(basename "$pkg_dir")"
    case "$name" in .*) continue ;; esac

    pkg_target="" pkg_ignore="false"
    eval "$(read_dstowrc "$pkg_dir")"
    [ "$pkg_ignore" = "true" ] && continue

    target="${pkg_target:-$HOME}"

    for entry in "$pkg_dir"*; do
      [ -e "$entry" ] || continue
      ename="$(basename "$entry")"
      case "$ename" in .dstowrc|deps) continue ;; esac
      real_name="${ename#dot-}"
      real_name=".${real_name}"
      # Undo the dot- -> . transform for entries that didn't have dot- prefix
      case "$ename" in dot-*) ;; *) real_name="$ename" ;; esac
      path="$target/$real_name"
      [ -d "$path" ] && echo "$path"
    done
  done | sort -u
}

find_stale() {
  dirs="$(stow_search_dirs)"
  for dir in $dirs; do
    find "$dir" -maxdepth 3 -type l -lname "*dotfiles*" 2>/dev/null
  done | while read -r link; do
    [ ! -e "$link" ] && printf '%s\n' "$link"
  done
}

check_stale() {
  stale="$(find_stale)"
  if [ -z "$stale" ]; then
    echo "dstow: no stale symlinks found"
    return 0
  fi
  echo "$stale" | while read -r link; do
    linktarget="$(readlink "$link")"
    printf "  %s -> %s\n" "$link" "$linktarget"
  done
  count="$(echo "$stale" | wc -l | tr -d ' ')"
  echo "dstow: found $count stale symlink(s)"
  return 1
}

clean_stale() {
  stale="$(find_stale)"
  if [ -z "$stale" ]; then
    echo "dstow: no stale symlinks found"
    return 0
  fi
  echo "$stale" | while read -r link; do
    printf "  removing %s\n" "$link"
    rm "$link"
  done
  count="$(echo "$stale" | wc -l | tr -d ' ')"
  echo "dstow: removed $count stale symlink(s)"
}

list_packages() {
  for pkg_dir in "$DOTFILES"/*/; do
    [ -d "$pkg_dir" ] || continue
    name="$(basename "$pkg_dir")"

    case "$name" in
      .*) continue ;;
    esac

    # shellcheck disable=SC2154 # pkg_target and pkg_ignore set by eval
    eval "$(read_dstowrc "$pkg_dir")"

    if [ "$pkg_ignore" = "true" ]; then
      printf "  %-20s [ignored]\n" "$name"
    elif [ -n "$pkg_target" ]; then
      printf "  %-20s -> %s\n" "$name" "$pkg_target"
    else
      printf "  %-20s -> ~\n" "$name"
    fi
  done
}

all_packages() {
  for pkg_dir in "$DOTFILES"/*/; do
    [ -d "$pkg_dir" ] || continue
    name="$(basename "$pkg_dir")"
    case "$name" in .*) continue ;; esac

    pkg_target="" pkg_ignore="false"
    eval "$(read_dstowrc "$pkg_dir")"
    [ "$pkg_ignore" = "true" ] && continue

    echo "$name"
  done
}

dstow() {
  list=false
  check=false
  clean=false
  all=false
  opts=""
  packages=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --help|-h)    show_help; exit 0 ;;
      --list)       list=true ;;
      --check)      check=true ;;
      --clean)      clean=true ;;
      --all)        all=true ;;
      --adopt)      opts="$opts --adopt" ;;
      -*)           opts="$opts $1" ;;
      *)            packages="$packages $1" ;;
    esac
    shift
  done

  if $list; then
    list_packages
    exit 0
  elif $check; then
    check_stale
    exit 0
  elif $clean; then
    clean_stale
    exit 0
  fi

  if $all; then
    packages="$(all_packages)"
  elif [ -z "$packages" ]; then
    show_help >&2
    exit 1
  fi

  for package in $packages; do
    # shellcheck disable=SC2086 # intentional word splitting for stow options
    stow_package "$package" $opts
  done
}

dstow "$@"
