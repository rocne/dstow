# Environment variables

Everything dstow reads from the environment, and the one namespace it writes.

| Variable | Effect |
|---|---|
| `DSTOW_PATH` | Colon-separated absolute local directory paths, registered as session repos for this shell only |
| `DSTOW_COLORS` | Packed per-slot theme overrides |
| `NO_COLOR` | Disables color (the standard signal) |
| `CLICOLOR_FORCE` | Forces color on, below `--color` in precedence |
| `CLICOLOR` | Set to `0` to disable color, below `CLICOLOR_FORCE` |
| `TERM` | `TERM=dumb` disables color at the TTY-detection step |
| `XDG_CONFIG_HOME` | Config, registry, themes, and global hooks root (`…/dstow/`); defaults to `~/.config` |
| `XDG_STATE_HOME` | Ledger location (`…/dstow/`); defaults to `~/.local/state` |
| `XDG_DATA_HOME` | Managed clones (`…/dstow/repos/…`); defaults to `~/.local/share` |
| `HOME` | The default target, and the base for `~` expansion in path values |
| `DSTOW_HOOK_*` | dstow's **output** lane to hooks — set by dstow, read by your hooks |

Config values may also reference **any** environment variable: path-valued
keys expand `$VAR`/`${VAR}` at use time. Those are your variables, not
dstow's.

## `DSTOW_PATH`

Colon-separated directories registered as **session repos** — additions to the
repo set for this shell only, never written to the registry.

    export DSTOW_PATH=/srv/dotfiles:/home/you/scratch-dots

It follows the `PATH` convention, and that comparison is exact in the ways
that matter and in one way that does not:

- **Absolute local directory paths only.** A relative entry is refused loudly.
- **No qualified sources.** `:` is the separator, so a `scheme:coordinate`
  cannot be spelled here. Remote sources need clones, which is `repo add`'s
  job.
- **No dstow-side expansion.** `~` and `$VAR` are not expanded — your shell
  already did that when it built the value, and a second pass would be a
  second grammar.
- **Empty entries warn and are skipped**, rather than being read as the
  current directory. That is the deliberate departure from `PATH`: dstow never
  resolves anything against the working directory.
- **No priority.** The repo set is unordered, so a `DSTOW_PATH` repo does not
  shadow or outrank a registered one. If two repos offer the same package
  name, that is ordinary ambiguity and dstow says so.

Session repos are exactly as real as registered ones for the length of the
shell: they enumerate, they stow, they appear in `list`. When you want one
permanently, `dstow repo add <path>` — and a `fix:` line tells you so when
dstow notices you might.

## `DSTOW_COLORS`

The one theming environment variable, and the top layer of the theming stack:

    export DSTOW_COLORS='error1=bold red:success2=#a6e3a1'
    export DSTOW_COLORS=$(dstow theme emit catppuccin-mocha --format env)

Packed `slot=value` pairs, colon-separated, in the `LS_COLORS` family syntax.
Values use git's `color.*` grammar. See `dstow manual theming values`.

There is deliberately no `DSTOW_THEME`. A name variable would be a second way
to say what the `theme` config key already says, and the packed form can
express everything a name can — including a name, via `theme emit`.

## The color enablement chain

Whether color is on at all is decided before any theme is consulted, in this
precedence:

    --color  >  NO_COLOR  >  CLICOLOR_FORCE  >  CLICOLOR  >  TTY detection (and TERM=dumb)

- `NO_COLOR` disables color when set to any non-empty value.
- `CLICOLOR_FORCE` forces color on when set to any non-empty value other than
  `0`.
- `CLICOLOR=0` disables color.
- Otherwise dstow checks whether the stream is a terminal, and treats
  `TERM=dumb` as not one.

Each stream is decided separately, so redirecting stdout while leaving stderr
on a terminal does the sensible thing on both.

**Theme selection is strictly downstream of this chain.** No theme, config
key, or `DSTOW_COLORS` value can re-enable color that the chain turned off.

## The XDG variables

dstow follows the XDG base directory specification, and the three roots carry
different kinds of thing:

- **`XDG_CONFIG_HOME`** — what you (and dstow's registry) declare: config,
  registry, themes, global hooks. Back this up.
- **`XDG_STATE_HOME`** — the ledger: what dstow currently believes it
  deployed. Machine-local, and reconstructible with `dstow rebuild`.
- **`XDG_DATA_HOME`** — managed clones. **Not a cache**: links point into it
  and its contents are load-bearing, so deleting it breaks live deployments.

## `DSTOW_HOOK_*`

The one namespace dstow *writes* rather than reads. Twelve variables carrying
context to your hooks; see `dstow manual hooks context`.

dstow does read one of them, for one purpose: the presence of
`DSTOW_HOOK_ACTION` is how it detects that it is running inside a hook, and
therefore that write commands must refuse.
