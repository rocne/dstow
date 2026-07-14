# Release pipeline and installer survey (2026-07-14)

**Question.** What does dstow inherit for free from the `rocne/release-ci`
pattern, and what does the gostow installer precedent already provide? Gathered
for the "Bootstrap snippet and installer design" ticket (REQUIREMENTS.md §10,
[gostow#31](https://github.com/rocne/gostow/issues/31) era, feature requests
gostow#33–36).

Sources: local clones `~/git/rocne/release-ci`, `~/git/gostow`,
`~/git/rocne/dot-dagger`; live state via `gh` (checked 2026-07-14).

---

## 1. What `release-ci` is and produces

`rocne/release-ci` is a **single reusable GitHub Actions workflow** (not a
composite action, not a template repo) — `.github/workflows/release.yml`, called
via `workflow_call`. It needs **zero secrets of its own**; callers pass
`secrets: inherit` (release-ci/README.md:4; docs/SECRETS.md:34).

**Inputs a caller supplies** (release-ci/.github/workflows/release.yml:10-50):
`version` (tag, already exists), `goreleaser_config` (default
`.goreleaser.yaml`), `project_name` (required — drives `dist/<project_name>_*`
names), `package_name` (apt/dnf/brew identity, defaults to `project_name`),
`binary` (for the smoke `--version` check), `cloudsmith_repo` (default
`rocne/releases`), `language` (default `go`), `e2e_script` (optional,
tool-specific), `smoke_distros` (default ubuntu:24.04/apt + fedora:41/dnf).

**What it does, in one job graph** (release-ci/.github/workflows/release.yml):
1. `release` job: checkout caller's repo (full history), set up Go, install
   cosign, write the caller's `GPG_PRIVATE_KEY` secret to a temp file, install
   the Cloudsmith CLI, then run **GoReleaser** (`goreleaser-action@v7`,
   `release --clean --skip=validate`) against the caller's own
   `goreleaser_config`. Attests build provenance (SLSA) over
   `dist/<project_name>_*.tar.gz`.
2. `verify` job: re-downloads the just-published checksums/sig/pem/archive from
   `https://github.com/<repo>/releases/download/<tag>/...` and runs
   `cosign verify-blob` against the **release-ci workflow's own Fulcio
   identity** (`^https://github\.com/rocne/release-ci/\.github/workflows/release\.yml@refs/tags/v`,
   OIDC issuer `token.actions.githubusercontent.com`) plus
   `gh attestation verify --signer-workflow rocne/release-ci/...`. Opens an
   issue on failure.
3. `package-repo-smoke` job: matrix over `smoke_distros`, installs the package
   from the **Cloudsmith** apt/dnf repo (`dl.cloudsmith.io/public/<repo_slug>/setup.{deb,rpm}.sh`)
   inside a clean container and checks `$BINARY --version` contains the
   released version.
4. `e2e` job: runs the caller's own `e2e_script` if given.

**What GoReleaser itself produces**, driven by each caller's own
`.goreleaser/<tool>.yaml` (e.g. `gostow/.goreleaser/gostow.yaml`):
- per-OS/arch **tarballs**: `{{.ProjectName}}_{{.Tag}}_{{.Os}}_{{.Arch}}.tar.gz`
  (gostow.yaml:35) — linux/darwin × amd64/arm64.
- **`.deb`/`.rpm`** via nfpm, GPG-signed with the caller's `GPG_PRIVATE_KEY`
  (gostow.yaml:53-94), published to the shared Cloudsmith repo
  `rocne/releases` (gostow.yaml:145-156).
- a **Homebrew cask** pushed to `rocne/homebrew-tap` via
  `HOMEBREW_TAP_GITHUB_TOKEN` (gostow.yaml:106-132).
- a **checksums file** `{{.ProjectName}}_{{.Tag}}_checksums.txt`
  (gostow.yaml:158-159), **cosign-signed** (`.sig`/`.pem`, gostow.yaml:161-171).
- two GPG public keys attached as release extra files (gostow.yaml:185-188).
- **all of this lands as GitHub Release assets** on the caller's own repo —
  release-ci publishes nothing to a separate host. There is no GitHub Pages
  and no release-ci-owned domain anywhere in this pipeline.

**What a consuming repo must add** (traced via gostow's and dot-dagger's
wiring, both otherwise-identical):
- its own `.goreleaser/<tool>.yaml` (build/archive/nfpm/cask/publisher/sign
  stanzas).
- a **release-please** workflow (`.github/workflows/release-please.yml`) as
  the primary release path: on push to `main`, maintains a standing release PR
  from Conventional Commits; merging it tags + creates the GitHub release in
  the same run, then calls release-ci
  (gostow/.github/workflows/release-please.yml:84-98).
- a **break-glass tag-push workflow** (`.github/workflows/release.yml` in the
  *consumer* repo, confusingly same filename as release-ci's) that also calls
  release-ci, for manual re-cuts (gostow/.github/workflows/release.yml:35-48,
  dot-dagger/.github/workflows/release.yml:17-31).
- three repo secrets: `GPG_PRIVATE_KEY`, `HOMEBREW_TAP_GITHUB_TOKEN`,
  `CLOUDSMITH_API_KEY` (docs/SECRETS.md:10-18) — `GITHUB_TOKEN` is free/automatic.
- a version-guard step if the tool is pre-1.0 (both gostow and dot-dagger have
  a hand-rolled "refuse v1.0.0+" guard; this is tool-specific policy, not part
  of release-ci itself).
- pins the reusable workflow **by release-ci's own tag**:
  `uses: rocne/release-ci/.github/workflows/release.yml@v0.1.1` — release-ci
  is tagged independently (`v0.1.1` is its only tag today,
  `~/git/rocne/release-ci` `git tag -l`).

**dot-dagger is a second, near-identical consumer** — same release-please +
break-glass + secrets structure (dot-dagger/.github/workflows/release.yml,
release-please.yml), confirming the wiring cost is fixed and repeatable, not
gostow-specific. dot-dagger additionally supplies an `e2e_script`
(`./test/run-e2e-release.sh`, dot-dagger/.github/workflows/release.yml:29-30).

---

## 2. gostow's installer: mechanics, hosting, live state

### 2.1 The commit and its shape

`57e1b63` (gostow, "feat: add a self-contained curl | sh installer (#31)",
2026-07-10) added `install.sh` (221 lines), a README section, and a shellcheck
CI step. Explicitly modelled on dot-dagger's pre-existing installer ("modelled
on the sibling `dot-dagger`'s and adapted for gostow" — commit message).
`dot-dagger/install.sh` is essentially the same script minus the man
page/completions block.

### 2.2 How it's hosted — the exact URL

```
curl -fsSL https://raw.githubusercontent.com/rocne/gostow/main/install.sh | sh
```
(gostow/install.sh:5; verified live: `curl -sI` on that URL returns `HTTP/2
200`, `content-type: text/plain`.)

**This is a raw-file-on-`main` URL, not a release asset, not GitHub Pages, not
a custom domain.** The installer script itself is version-*resolving*, not
version-*pinned*: it always fetches whatever revision of `install.sh` sits on
the `main` branch right now, and that script then independently resolves which
**release tag** to install (latest, or `--version` override). The two version
axes — "which installer script" and "which gostow release" — are deliberately
decoupled.

### 2.3 Platform detection, install location, verification

- OS: `uname -s` → `linux`/`darwin`; arch: `uname -m` → `amd64`/`arm64` (both
  overridable: `--os`, `--arch`) (install.sh:57-75).
- Version resolution: `--version vX.Y.Z` (normalizes a leading `v`), else hits
  `https://api.github.com/repos/rocne/gostow/releases/latest` and greps
  `tag_name` (install.sh:86-100).
- Asset naming assumed: `gostow_<TAG>_<os>_<arch>.tar.gz` +
  `gostow_<TAG>_checksums.txt` (install.sh:104-108) — **matches** GoReleaser's
  `name_template` in gostow.yaml:35 and the checksum `name_template` in
  gostow.yaml:159, and matches the live asset list (§2.4 below).
- Download base: `https://github.com/rocne/gostow/releases/download/<TAG>/...`
  (install.sh:122) — i.e. the installer *fetches from* GitHub Release assets
  even though the *installer script itself* is hosted as a raw file.
- **Checksum verification is mandatory** — no `sha256sum`/`shasum` on the
  machine is a hard abort (install.sh:126-137).
- **Signature verification is conditional**: only runs if `cosign` is present
  locally; otherwise prints a notice and proceeds on checksum alone
  (install.sh:139-162). Verifies against the same release-ci Fulcio identity
  regex used in release-ci's own `verify` job.
- Install target: `${INSTALL_DIR:-$HOME/.local/bin}`, no root
  (install.sh:36,166-168). Best-effort man page + completions into XDG dirs
  unless `--bin-only` (install.sh:174-207). PATH reminder printed if
  `$INSTALL_DIR` isn't on `$PATH` (install.sh:212-221).

### 2.4 Live release state (checked via `gh`, 2026-07-14)

```
$ gh release list -R rocne/gostow
v0.2.0  Latest  v0.2.0  2026-07-10T18:57:12Z
v0.1.1          v0.1.1  2026-07-10T06:05:58Z
v0.1.0          v0.1.0  2026-07-09T19:35:11Z
```

`gh release view v0.2.0 -R rocne/gostow` assets: `.rpm` ×2 (arch),
`.deb` ×2 (arch), two `.asc` signing-key files, `_checksums.txt`,
`_checksums.txt.pem`, `_checksums.txt.sig`, and four `.tar.gz` archives
(linux/darwin × amd64/arm64). This exactly matches what `install.sh` expects
to find.

### 2.5 Idempotence — the gap

**`install.sh` has no presence check at all.** Reading the full script
top to bottom: there is no `command -v gostow` (or any equivalent) anywhere,
and no early-exit path when gostow is already installed. Every invocation:
prints `tool:`, `platform:`, `release:`, `asset:` lines; downloads; verifies
checksum; extracts; overwrites `$INSTALL_DIR/gostow`; prints `installed:`
lines; runs `--version`. Re-running it is **safe** (re-downloads and
re-installs the same or newer version, no error) but **never silent** — it
always touches the network and always produces substantial stdout. Same is
true of `dot-dagger/install.sh` (grepped for `command -v`/presence-style
checks: only `curl`, `sha256sum`/`shasum`, `cosign` — tool *dependency*
checks, never a "is dotd already here" check).

This is **not** what REQUIREMENTS.md §10.2 asks for. The requirement: *"The
script is itself idempotent — it too checks for [the tool] and exits cleanly
when present, so direct `curl | sh` is always safe."* Gostow's script is
idempotent in the weaker "safe to re-run" sense, not in the "checks first and
exits quietly" sense the requirement specifies.

---

## 3. The dstow delta: inherit vs. add

### Inherits for free (zero new dstow-specific engineering)

- The entire **build → sign → publish → verify → smoke** pipeline as a
  `workflow_call` — dstow's consumer-side workflow files are a copy-paste of
  gostow's/dot-dagger's shape (release-please.yml + release.yml + a
  `.goreleaser/dstow.yaml`), pinned to `rocne/release-ci/...@v0.1.1`.
- **GitHub Releases as the artifact host** — tarballs, checksums, cosign
  signatures, `.deb`/`.rpm`, Homebrew cask: all "established release pipeline"
  per §10.2, with no new hosting decision needed.
- The **raw.githubusercontent.com/<owner>/<repo>/main/<script>** hosting
  pattern for the installer script itself — no Pages, no custom domain, no new
  infra. dstow's URL under this scheme would be:
  ```
  https://raw.githubusercontent.com/rocne/dstow/main/install.sh
  ```
- The **cosign/Fulcio verify-blob pattern** keyed to release-ci's own
  workflow identity — reusable as-is if dstow's installer wants signature
  verification.
- The **version-resolve-via-GitHub-API** pattern (`releases/latest` →
  `tag_name`) for "install latest" without a hosted redirect.

### Must add (dstow-specific)

1. **`.goreleaser/dstow.yaml`** — build/archive/nfpm/cask/publisher/checksum/sign
   stanzas, modelled on gostow's file, with dstow's own `project_name`,
   `binary`, and asset contents (no man page/completions assumption carries
   over automatically — copy only what applies).
2. **Two workflow files** (`release-please.yml`, `release.yml`) wired to
   `rocne/release-ci/.github/workflows/release.yml@v0.1.1`, each supplying
   `version`, `goreleaser_config`, `project_name: dstow`, `binary: dstow`,
   `secrets: inherit`.
3. **Three repo secrets** (`GPG_PRIVATE_KEY`, `HOMEBREW_TAP_GITHUB_TOKEN`,
   `CLOUDSMITH_API_KEY`) set on `rocne/dstow` — mechanical, per
   docs/SECRETS.md's existing runbook (same GPG key and Homebrew PAT can
   likely be reused across repos, not regenerated).
4. **`install.sh`** at repo root, adapted from gostow's/dot-dagger's — BUT
   with a **presence check added that gostow's precedent lacks** (see gap
   below). This is new work, not something to copy verbatim.
5. **The bootstrap-snippet command** (§10.1) itself — a dstow CLI subcommand
   emitting POSIX sh with a **local** presence check (`command -v dstow`) and
   zero output/zero network when found. Nothing in release-ci or gostow
   provides this; it is pure dstow application logic. The snippet's own
   "install iff absent" body would most naturally be
   `command -v dstow >/dev/null 2>&1 || curl -fsSL <installer-url> | sh`.

### Whether gostow's installer shape satisfies the snippet's contract

**Not as-is.** §10.1 requires the *emitted snippet's* presence check to be
local and silent — that part dstow controls directly and gostow's precedent
is irrelevant to it. But §10.2 separately requires the *hosted installer
script* to be self-idempotent (checks for the tool, exits cleanly when
present). Gostow's `install.sh` does not do this (§2.5), so **it cannot be
copied as-is** if dstow's installer is meant to satisfy §10.2 literally — a
plain `curl | sh` of a gostow-style script always re-downloads and re-installs,
never "exits cleanly" early. If the design ticket wants direct `curl | sh`
safety independent of the snippet's own guard, dstow's `install.sh` needs a
`command -v dstow` early-exit that gostow's/dot-dagger's precedent does not
demonstrate.

---

## 4. Gaps flagged for the design ticket

1. **No idempotent-quiet precedent exists yet.** Both sibling installers
   (gostow, dot-dagger) always print output and always touch the network,
   contradicting §10.2's "exits cleanly when present" wording if read
   strictly. Design must decide: add a presence check to dstow's script (net
   new behavior, no copy-paste precedent), or interpret "idempotent" as
   "safe/idempotent-in-effect" (re-running converges, matches gostow's
   existing bar) rather than "silent no-op". These are materially different
   asks and the requirement text is compatible with either reading.
2. **The installer URL is raw-file-based, not release-asset-based** — good
   news for embedding in a snippet (stable, tag-independent URL), but it means
   the *installer script's* correctness is decoupled from any given release:
   a bug fixed in `install.sh` on `main` instantly affects everyone re-running
   the one-liner, bypassing the release pipeline's verify/smoke gates
   entirely. Worth deciding whether dstow wants that same script-vs-release
   decoupling or a versioned/pinned installer URL instead.
3. **Signature verification is opportunistic, not enforced** — cosign only
   runs if already present on the user's machine; otherwise it's skipped with
   a notice. If dstow's threat model wants stronger guarantees, this precedent
   under-delivers.
4. **`--dir`/`INSTALL_DIR` default is `~/.local/bin`** — matches XDG
   convention but is a design choice, not inherited automatically; needs
   confirming against dstow's own install-location expectations (relevant if
   dstow's snippet later needs to reference where the binary landed).
5. **Secrets are per-repo, not inherited from an org** (`rocne` is a personal
   account — docs/SECRETS.md:28-32) — dstow's repo needs its own three
   secrets set before its first release; this is a one-time manual setup step
   external to any code the design ticket produces.
