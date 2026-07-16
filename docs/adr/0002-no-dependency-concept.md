# dstow has no dependency concept

dstow v1 ships with no dependency concept of any kind: no declarations, no
presence checks, no dependency surfaces. v2 will carry no *first-class*
dependency concept (a generic property store may host dependencies as an
endorsed convention). No further claims are made.

This supersedes a fully designed subsystem — scoped declarations, warn-only
presence-on-PATH checks ordered around pre-hooks, status detail, a machine
query — preserved verbatim in
[docs/attic/dependencies.md](../attic/dependencies.md). Decided on
[Bootstrap snippet and installer design (#28)](https://github.com/rocne/dstow/issues/28)
after the design was complete, when shaping its bundled hook exposed the
incoherence.

## Considered options

Keeping the designed system (declare + verify in core, act in user hooks) was
genuinely defensible — facts in the binary, policy in userland, and a real
product moment (fresh box, `dstow stow zsh`, "zsh wants fzf — hint:
`apt install fzf`"). It lost on three grounds:

1. **Arbitrary privilege.** Command-on-PATH is one member of a family of
   package-applicability predicates — fonts installed, terminfo entries, OS,
   tool versions, env vars — all of which a dotfiles system genuinely meets.
   Baking in exactly one member as first-class while the rest wait for
   user-side conventions privileges it arbitrarily. Principled first-class
   support means the general predicate system (v2 store territory); v1's
   honest choices were one privileged instance or none.
2. **The genre agrees.** GNU stow, chezmoi, yadm, dotbot: none bakes in a
   dependency checker; all answer with hooks/scripts/plugins. git — the
   design conscience here — shipped hooks and *sample* hooks for decades and
   never grew a built-in checker for what people check in hooks.
3. **The levee holds better at the top of the hill.** Checker-in-binary invites
   warn UX → "just run the hints" → OS/package-manager maps → half a package
   manager. "Never installs" mid-slope needs constant defending;
   "no dependency concept" is defensible by ownership: everything below the
   line is user-authored and user-installed.

## Consequences

- REQUIREMENTS amended: §9.2 deleted (short pointer remains), §5.5
  unmet-deps line dropped, §1.5/§2.7/§4.2/§11 dependency mentions removed,
  §9 retitled "Hooks". CONTEXT.md retires *dependency / names / hint /
  dependency query*.
- The use case survives user-side: a hook is a list of
  `command -v <tool> || <install command>` lines the user authors. Hooks
  remain the mechanism dstow ships for it; convenience skeletons
  (`snippet hook`, git-sample style) are a candidate v1.1 addition.
- The C10 names+hint declaration shape is preserved in the attic as the
  natural format should v2's store endorse a dependencies convention.
- Re-baking dependencies into core later is foreclosed for practical
  purposes: once users build their own hook conventions on an agnostic v1,
  core semantics would compete with them.
