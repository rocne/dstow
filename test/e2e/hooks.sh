#!/bin/sh
set -e

# H7 (DESIGN §5), exercised through a real hook: write commands refuse from
# inside a hook, reads stay fully allowed. The unit suite can assert the guard
# with DSTOW_HOOK_ACTION set by hand; only e2e can assert it fires when dstow
# itself is the one setting that variable, on a command the ledger lock does
# not incidentally cover.
export HOME=/home/e2e
export XDG_CONFIG_HOME=/home/e2e/.config
export XDG_STATE_HOME=/home/e2e/.local/state
export XDG_DATA_HOME=/home/e2e/.local/share
mkdir -p /home/e2e

# A local repo with one package, and a second directory the hook will try to
# register (a write that never touches the ledger lock).
mkdir -p /home/e2e/hookdots/zsh
printf 'export E2E_HOOKS=1\n' > /home/e2e/hookdots/zsh/dot-zshrc
mkdir -p /home/e2e/otherdots/vim
printf '" e2e\n' > /home/e2e/otherdots/vim/dot-vimrc

# A repo-level pre-stow hook that runs one write and one read, records both
# outcomes, and exits 0 so the deploy proceeds either way.
mkdir -p /home/e2e/hookdots/.dstow/hooks
cat > /home/e2e/hookdots/.dstow/hooks/pre-stow <<'HOOK'
#!/bin/sh
dstow repo add /home/e2e/otherdots > /home/e2e/write.out 2>&1
printf '%s' "$?" > /home/e2e/write.code
dstow status --json > /home/e2e/read.out 2>&1
printf '%s' "$?" > /home/e2e/read.code
exit 0
HOOK
chmod +x /home/e2e/hookdots/.dstow/hooks/pre-stow

dstow repo add /home/e2e/hookdots >/dev/null 2>&1 \
  || { printf 'FAIL: repo add exited nonzero\n'; exit 1; }

# Stowing fires the repo-level pre-stow hook, which is where the assertions
# below are actually produced.
dstow stow zsh >/dev/null 2>&1 \
  || { printf 'FAIL: stow exited nonzero\n'; exit 1; }

if [ ! -f /home/e2e/write.code ]; then
  printf 'FAIL: the pre-stow hook never ran\n'
  exit 1
fi

# The write refused with exit 3 (A3 refusal/environment).
code=$(cat /home/e2e/write.code)
if [ "$code" != "3" ]; then
  printf 'FAIL: repo add inside a hook exit = %s, want 3\n' "$code"
  cat /home/e2e/write.out
  exit 1
fi

# It refused for the right reason. The ledger lock does not cover repo add, and
# naming lock contention here would be the wrong error entirely — it suggests
# retrying, when the correct answer is "never, from here".
grep -q 'hook' /home/e2e/write.out \
  || { printf 'FAIL: refusal does not name the hook\n'; cat /home/e2e/write.out; exit 1; }
grep -q 'DSTOW_HOOK_ACTION' /home/e2e/write.out \
  || { printf 'FAIL: refusal does not name DSTOW_HOOK_ACTION\n'; cat /home/e2e/write.out; exit 1; }
grep -q 'fix:' /home/e2e/write.out \
  || { printf 'FAIL: refusal printed no fix: line\n'; cat /home/e2e/write.out; exit 1; }
if grep -q 'ledger.lock' /home/e2e/write.out; then
  printf 'FAIL: the ledger lock refused, not the H7 guard\n'
  cat /home/e2e/write.out
  exit 1
fi

# The refusal was real: the repo was not registered.
if dstow list | grep -q 'otherdots'; then
  printf 'FAIL: repo add inside a hook registered the repo anyway\n'
  exit 1
fi

# Reads stay fully allowed — the other half of H7.
code=$(cat /home/e2e/read.code)
if [ "$code" != "0" ]; then
  printf 'FAIL: status --json inside a hook exit = %s, want 0\n' "$code"
  cat /home/e2e/read.out
  exit 1
fi
grep -q '"state"' /home/e2e/read.out \
  || { printf 'FAIL: status --json inside a hook produced no report\n'; cat /home/e2e/read.out; exit 1; }

# The deploy itself completed: the hook blocked nothing.
if [ ! -L /home/e2e/.zshrc ]; then
  printf 'FAIL: stow did not create the link\n'
  exit 1
fi

# Outside a hook the same write works — the guard is the environment, not the
# command.
dstow repo add /home/e2e/otherdots >/dev/null 2>&1 \
  || { printf 'FAIL: repo add outside a hook exited nonzero\n'; exit 1; }
dstow list | grep -q 'otherdots' \
  || { printf 'FAIL: repo add outside a hook did not register the repo\n'; exit 1; }

printf 'PASS: H7 write-refusal inside a hook (writes 3, reads 0)\n'
