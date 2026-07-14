#!/usr/bin/env bash
# PROTOTYPE — renders the output-design mockups with real ANSI colors.
# Throwaway asset of https://github.com/rocne/dstow/issues/26
# Honors NO_COLOR. ANSI-16 slots only — your terminal theme owns the look.

if [ -n "${NO_COLOR:-}" ] || [ ! -t 1 ]; then
  B='' D='' RD='' BRD='' GN='' YL='' MG='' CY='' R=''
else
  B=$'\033[1m'; D=$'\033[2m'; RD=$'\033[31m'; BRD=$'\033[1;31m'
  GN=$'\033[32m'; YL=$'\033[33m'; MG=$'\033[35m'; CY=$'\033[36m'; R=$'\033[0m'
fi

hr() { printf '\n%s——— %s ———%s\n\n' "$D" "$1" "$R"; }

hr 'dstow status'
printf '%srocne/dotfiles%s %sgithub · up to date%s\n' "$B" "$R" "$D" "$R"
printf '  zsh      %sstowed%s\n' "$GN" "$R"
printf '  git      %sstowed%s %s~ drifted%s\n' "$GN" "$R" "$CY" "$R"
printf '  tmux     %spartially stowed%s %s(3/5 links)%s\n' "$YL" "$R" "$D" "$R"
printf '  vim      %snot stowed%s\n' "$D" "$R"
printf '\n%swork/dots%s %slocal:~/work/dots · —%s\n' "$B" "$R" "$D" "$R"
printf '  zsh      %soccupied%s %s(1 path)%s\n' "$MG" "$R" "$D" "$R"
printf '  ssh      %sdamaged%s %s(~/.ssh/config: ledgered link replaced)%s\n' "$BRD" "$R" "$D" "$R"

hr 'dstow status ~/.zshrc  (per-path view)'
printf '%s~/.zshrc%s\n' "$B" "$R"
printf '  occupant:  real file %s(1.2 KiB, modified 2026-07-12)%s\n' "$D" "$R"
printf '  ledger:    not dstow'\''s %s(no entry)%s\n' "$D" "$R"
printf '  adoptable by:\n'
printf '    1) %srocne/dotfiles::zsh%s   %sowns 11 neighboring paths%s\n' "$B" "$R" "$D" "$R"
printf '    2) %swork/dots::zsh%s        %sowns 2%s\n' "$B" "$R" "$D" "$R"
printf '%sfix:%s dstow adopt ~/.zshrc dotfiles::zsh\n' "$GN" "$R"

hr 'dstow stow tmux vim work/dots::zsh  (mixed run)'
printf 'stow %stmux%s %slinked 5%s\n' "$B" "$R" "$GN" "$R"
printf 'stow %svim%s %sno-op (already stowed)%s\n' "$B" "$R" "$D" "$R"
printf 'stow %swork/dots::zsh%s %sfailed%s %soccupied%s\n' "$B" "$R" "$BRD" "$R" "$MG" "$R"
printf '  ~/.zshrc is a real file, not dstow'\''s\n'
printf '  %sfix:%s dstow adopt ~/.zshrc work/dots::zsh — or re-run with --adopt\n' "$GN" "$R"
printf '\n%s1 stowed%s, 1 no-op, %s1 failed%s\n' "$B" "$R" "$BRD" "$R"

hr 'bare "dstow stow"  (bulk confirm, interactive)'
printf 'Stow all %s12%s packages? zsh git tmux vim %s… 8 more%s  [y/N]\n' "$B" "$R" "$D" "$R"

hr 'ambiguity — interactive choice'
printf '%szsh%s matches 2 packages:\n' "$B" "$R"
printf '  1) rocne/dotfiles::zsh\n  2) work/dots::zsh\n'
printf 'Stow which? [1/2/q]\n'

hr 'ambiguity — non-interactive error'
printf '%serror:%s %szsh%s is ambiguous — matches rocne/dotfiles::zsh, work/dots::zsh\n' "$BRD" "$R" "$B" "$R"
printf '%sfix:%s qualify it: dstow stow dots::zsh\n' "$GN" "$R"

hr 'announcements (§1.3)'
printf '%snote:%s created target directory ~/.config/foo\n' "$CY" "$R"
printf '%snote:%s folding is off (dstow'\''s default: predictable one-link-per-file).\n' "$CY" "$R"
printf '      To fold like classic stow: [folding] in ~/.config/dstow/config.toml\n'

hr 'D12 knob refusal'
printf '%serror:%s --no-folding is not a dstow flag\n' "$BRD" "$R"
printf '%sfix:%s folding is a global setting: [folding] in ~/.config/dstow/config.toml\n' "$GN" "$R"
printf '      %s(renamed .stowrc fold flags are honored — see docs on stow compat)%s\n' "$D" "$R"

hr 'repo add with percent-encoding (continue-affirmative)'
printf '%snote:%s this path contains '\'':'\'' and will be percent-encoded in names:\n' "$CY" "$R"
printf '      local:/data/weird%%3A%%3Adir/dots\n'
printf 'Register it with this name? [Y/n]  %s(n cancels — rename the directory, then re-add)%s\n' "$D" "$R"

hr 'dstow check'
printf '%sbroken%s    ~/.config/old/rc → %s(gone) rocne/dotfiles/old/rc%s\n' "$RD" "$R" "$D" "$R"
printf '%sorphaned%s  ~/.vimrc → rocne/dotfiles/vim/vimrc %s(ignored since 2026-07-01 config)%s\n' "$YL" "$R" "$D" "$R"
printf '\n%s1 broken%s (clean removes freely), %s1 orphaned%s (clean will ask)\n' "$B" "$R" "$B" "$R"

printf '\n%s——— end of demo ———%s\n' "$D" "$R"
