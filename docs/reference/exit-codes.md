# Exit codes

Four codes. Each has one meaning, mapped in exactly one place in dstow's code,
and the meanings are stable — scripting against them is supported.

| Code | Meaning |
|---|---|
| `0` | Success. |
| `1` | **Negative answer.** The command ran and the answer was no. |
| `2` | **Usage error.** The invocation was malformed. |
| `3` | **Refusal / environment.** dstow declined to act, or could not. |

## `1` — negative answer

The command was well-formed, dstow did the work, and the result is negative:

- A package failed during a deploy verb. Packages are independent, so a bulk
  run continues past the failure and exits `1` at the end if any package
  failed.
- A requested field is applicable but unset or empty — `dstow info zsh -f
  ignores` where no level contributes a pattern. (A field name dstow does not
  recognize is exit `2`: that is a malformed request, not a negative answer.)
- `check` found findings.
- A named thing resolves to nothing: a package, repo, or theme that is not
  there.

That last one is worth stating explicitly, because the alternative reading is
tempting: **a name that does not resolve is exit `1`, not `2`.** The
invocation was perfectly well-formed; the answer is simply that there is no
such thing. Exit `2` is reserved for input dstow could not parse.

## `2` — usage error

The invocation was malformed and dstow never got as far as doing anything:

- An unknown flag, or a flag missing its required value (`--color` with no
  argument).
- A flag used on a command that does not have it — `-v` is the root's, not
  `stow`'s.
- The wrong number of arguments.
- An unknown command, or an unknown subcommand.
- An operand of the wrong shape for the command, including an unknown field
  name for `info -f`.

Every exit `2` prints the error and a `fix:` line pointing at `--help`. If you
are scripting dstow, treat `2` as "my command line is wrong" — it never means
anything about the state of your system.

## `3` — refusal / environment

dstow understood you and declined, or the environment made the operation
impossible. The distinguishing feature is that **re-running the same command
unchanged will produce the same result** until something outside the command
changes.

- **Non-interactive ambiguity.** A name matched more than one thing and there
  is no terminal to ask on. The error lists the qualified spellings.
- **The non-interactive bulk gate.** A deploy verb with no names and no
  `--all` cannot ask "everything?" without a terminal. `--all` is the answer.
- **A corrupt ledger**, or one written by a newer dstow than the one running.
  The `fix:` line names `dstow rebuild` for the former.
- **Lock contention** — another dstow holds the ledger lock.
- **A write command run from inside a hook.** Reads work from inside one; the
  write set refuses. See `dstow manual hooks`.
- **Guards refusing.** `repo remove` on a repo with stowed links; a managed
  clone holding work not present at its source; a fold conflict.
- **Git-side refusals.** git not installed; a diverged clone that
  `repo upgrade` will not fast-forward.
- **A declined guard or ambiguity prompt** — the two prompts where saying no
  *is* the refusal: the `repo remove` still-stowed guard (also covered above)
  and the `repo add` source-interpretation prompt. dstow asked because it could
  not safely proceed on its own, so declining leaves it unable to act.

Declining any *other* prompt is exit `0`, not `3`. The bulk deploy gate
("stow every package of every registered repo?"), a `clean` orphan, and an
`adopt` overwrite of differing content are optional confirmations: say no and
dstow does nothing and reports success, because doing nothing on request is a
valid answer, not a refusal. Only the two guard/ambiguity prompts above turn a
"no" into exit `3`.

## Scripting patterns

Check for the specific case rather than for "nonzero":

```sh
if dstow check --json > findings.json; then
  echo "clean"
else
  case $? in
    1) echo "findings present"; jq . findings.json ;;
    3) echo "dstow could not run: ledger or lock problem" >&2; exit 1 ;;
    *) echo "bad invocation" >&2; exit 2 ;;
  esac
fi
```

Two rules make this reliable:

- **`2` never overlaps with a real answer.** If you see `2`, fix the command
  line; nothing about the machine's state is being reported.
- **`3` is the retry-or-intervene code.** `1` means dstow answered you; `3`
  means it did not, and something — a flag, a lock, a repair — has to change
  first.

Bulk runs deserve one more note: a deploy verb over many packages exits `1` if
*any* package failed, and `0` only if all of them succeeded. The per-package
detail is in the output, and in `--json` where the command has it.
