# dstow repo add

<!-- dstow:short -->
Register a repo from a source (path, URL, github:owner/name)
<!-- /dstow:short -->

<!-- dstow:long -->
Register a repo — where packages come from.

Sources: a local directory path, a full URL/ssh form, or a qualified source
like github:owner/name (bare owner/name asks first; in scripts, qualify).
Remote sources clone into the managed directory; local paths are registered
in place and never modified.

Adding stows nothing: dstow announces the repo's packages and any bare
names that now need qualification. If the path would need percent-encoding
in name expressions, dstow shows the encoded form and asks whether to
continue or rename first. Re-adding a present repo is a safe, announced
no-op.
<!-- /dstow:long -->

<!-- dstow:examples -->
  dstow repo add ~/dotfiles
  dstow repo add github:rocne/dotfiles
  dstow repo add rocne/dotfiles --stow
<!-- /dstow:examples -->

## How the source form is decided

The source's form is read **from the string alone**, in this order, with no
filesystem access — the string decides the kind, and only the bare form below
ever looks at disk:

1. **path** — starts with `/`, `~/`, `./`, or `../`.
2. **URL** — contains `://`, or matches the scp-like `user@host:path` form.
3. **qualified** — still contains a `:`, so it is read as `scheme:coordinate`
   (`local:/home/you/dots`, `github:owner/name`). An unknown scheme is a clean
   error listing the known ones.
4. **bare** — anything else: an `owner/name` shape, or a plain name.

### path and qualified-local forms

A path source (or a `local:` qualified source) registers a directory **in
place**. `~` expands, the path is made absolute and canonicalized at add time,
and dstow records that location — it never copies the directory and never
modifies it. Removing the repo later only forgets it.

### URL forms

The owner and name are read out of the URL, and the URL is cloned **exactly as
given**, so ssh and credential-carrying forms keep working unchanged. v1 clones
github-hosted sources into the managed directory (`dstow manual reference
files`).

### bare forms — the only ones that consult disk

A bare source is the one case where dstow looks at the filesystem before
deciding, and where it confirms unless the answer is unambiguous:

- an `owner/name` shape **and** an existing directory of that same
  `owner/name` → **ambiguous**: dstow refuses and names both qualified
  spellings (`github:owner/name` or `local:/abs/path`) so you can re-run with
  the one you meant;
- an `owner/name` shape with no such directory → dstow **confirms the
  github.com interpretation** before acting. This prompt is a confirmation of
  stated intent, so `-y` answers it; non-interactively without `-y` it refuses
  and names the `github:` spelling;
- a plain name that is an existing directory → **that directory**;
- anything else → a refusal naming the two qualifications you could have
  written.

`dstow manual concepts naming` covers the qualified-name grammar these
spellings belong to.

## The encoding prompt

If the source contains characters that must percent-encode to be spellable in a
name expression (a `:` in a local path, for instance), dstow shows the
canonical encoded form. Interactively it asks whether to continue with it or
rename first; **non-interactively it proceeds** with the encoded form and a loud
announcement, rather than refusing. See `dstow manual concepts naming` for the
encoding itself.

## `--stow`

`--stow` composes a stow of the newly-added repo's packages right after adding,
with the usual bulk exclusions applied (`dstow manual configuration keys`).
