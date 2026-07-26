# 05 — Build, release, distribution, and the musl constraint

The headline: **the release pipeline is far more portable than expected.**
GoReleaser has supported Rust since v2.5, which means most of the machinery
below survives with a changed `builds:` block rather than a rewrite.

---

## What exists today

dstow's distribution is not a `go build` — it is a full signed-release
pipeline, and reproducing it from scratch in Rust would be a substantial
project on its own.

### Pipeline

```
Conventional-Commit PRs on main
  └─> release-please  (googleapis/release-please-action@v5)
        maintains a standing release PR: next version + CHANGELOG
        merging it = Rocne's button; creates tag + GitHub release
  └─> rocne/release-ci .github/workflows/release.yml@v0.1.5   (Rocne's own reusable workflow)
        └─> GoReleaser  (.goreleaser/dstow.yaml, 153 lines)
              ├─ builds: linux + darwin × amd64 + arm64, CGO_ENABLED=0
              ├─ archives: tar.gz, <name>_<tag>_<os>_<arch>
              ├─ nfpms: signed .deb + .rpm (GPG)
              ├─ homebrew_casks: → rocne/homebrew-tap (with a Gatekeeper xattr hook)
              ├─ checksum: SHA-256 manifest
              ├─ signs: cosign v3, single Sigstore bundle (.sigstore.json)
              ├─ publishers: cloudsmith CLI → apt/dnf repo (any-distro/any-version)
              └─ release.extra_files: the GPG public key
```

Plus `install.sh` — vendored from `rocne/release-ci`, POSIX sh, detects
OS/arch, downloads the matching tarball, **verifies SHA-256 before touching
anything**, verifies the cosign signature if cosign is present, installs to
`~/.local/bin`. And `snippet.sh`, also vendored from release-ci, embedded in
the binary and emitted verbatim by `dstow snippet rc`.

### CI workflows (7)

| Workflow | Job | Port impact |
|---|---|---|
| `ci.yml` | gitleaks full-history secret scan; test matrix on `ubuntu-latest`, `ubuntu-24.04-arm`, `macos-latest`; e2e | Toolchain swap only. Matrix rationale (one runner per shipped platform axis) is unchanged. |
| `lint.yml` | golangci-lint | → `cargo clippy` + `cargo fmt --check`. |
| `pr-title.yml` | Conventional-Commit title enforcement | **Unchanged.** |
| `docs-release-guard.yml` | fails a `docs/**` PR whose commit type doesn't release (ADR 0003) | **Unchanged** — it reads commit titles and changed paths, not code. |
| `release-please.yml` | the release path | Config change only; release-please supports Rust manifests. |
| `release-dryrun.yml` | builds + signs a snapshot on PRs touching signing files, then verifies | Toolchain swap; the signing mechanics are language-agnostic. |
| `release-manual.yml` | break-glass tag-push path | Same. |

---

## GoReleaser builds Rust — this is the key finding

**Since GoReleaser v2.5**, `builds:` accepts `builder: rust`. It detects a
Rust project from `Cargo.toml`, uses **`cargo-zigbuild`** by default (with
`cross` selectable via `tool`), and runs `rustup target add` for declared
targets. v2.15 added custom glibc-version targeting for zigbuild.

**What this preserves, unchanged:** archives, nfpms (deb/rpm + GPG signing),
homebrew_casks, checksums, cosign signing, the cloudsmith publisher, and
`release.extra_files`. These are artifact-stage features that operate on
built binaries and do not care what produced them. The whole bottom two-thirds
of `.goreleaser/dstow.yaml` — the hard-won part, the part that took issues
#48/#49/#50 and several release cycles to get right — carries over.

**Caveats, stated honestly:**

- The documented **default** targets are the `-gnu` and `-apple-darwin`
  triples. **musl targets are not among the defaults and are not mentioned in
  the Rust-builder docs.** Targets are configurable, and `cargo-zigbuild`
  does support musl triples — but *"GoReleaser + Rust + musl + nfpm + cosign
  end to end"* is *not verified* by this pack. Treat it as a prototype task,
  not an assumption.
- The build environment is no longer self-contained: cargo, rustup, zig, and
  cargo-zigbuild must be installed on the runner. Go needed only
  `actions/setup-go`.
- Some GoReleaser build options are documented as not yet implemented for
  Rust. An audit against dstow's actual `builds:` block is needed — though
  that block is currently *tiny* (binary name, main path, `CGO_ENABLED=0`,
  ldflags for version injection). Version injection is the one item to check:
  Go uses `-X main.version=`; the Rust equivalent is typically
  `env!("CARGO_PKG_VERSION")` or a `build.rs`, not a linker flag.

### The one hard coupling: `rocne/release-ci`

`release-ci`'s reusable `release.yml` hardcodes:

```yaml
- uses: actions/setup-go@…v7.0.0
  with:
    go-version-file: go.mod
```

So a Rust dstow cannot call `release-ci@v0.1.5` as-is. This is a small,
well-scoped change to a repo Rocne owns — parameterize the toolchain step, or
add a Rust variant — but it is **a change to a shared dependency with another
consumer (gostow)**, so it is a cross-repo decision, not a dstow-local one.

The **artifact-shape contract (D34)** that `install.sh` consumes is already
language-agnostic and documented in the script itself:

```
<bin>_<tag>_<os>_<arch>.tar.gz          archive, binary at its root
<bin>_<tag>_checksums.txt (+ .sigstore.json)
man/ and completions/ inside the archive when shipped
```

Any builder producing those shapes works with the existing installer,
unchanged. **`install.sh` and `snippet.sh` need no port at all** — they are
POSIX sh, vendored from release-ci, and byte-identical across consumers.

### Alternative if GoReleaser is dropped

The Rust-native stack is `release-plz` (Conventional-Commit release PRs,
CHANGELOG via git-cliff, crates.io publishing) + `cargo-dist` (multi-platform
builds, archives, installers, checksums, GitHub releases). The idiomatic
pairing is release-plz creating the tag and cargo-dist reacting to it.

**Assessment: not obviously better here.** It replaces release-please with
release-plz (a lateral move — release-please already works and enforces the
same conventions), and cargo-dist's deb/rpm + Cloudsmith + GPG-signed-package
story is weaker than nfpm's. dstow publishes to an apt/dnf repo; that is not
cargo-dist's strong suit. Keeping GoReleaser looks like the lower-risk path,
with the release-ci change as its cost.

---

## The musl constraint

**Rocne's tentative direction: target musl, not glibc.** Recorded here as
tentative and pending further judgment.

### Why it is a bigger question in Rust than in Go

Today, `CGO_ENABLED=0` already makes dstow a fully static, libc-free binary
on Linux. Go's runtime provides its own syscall layer, so "static" is the
default and costs nothing. **Rust does not get this for free**: the default
Linux target `x86_64-unknown-linux-gnu` dynamically links glibc and inherits
its version-floor problem. Choosing musl is how a Rust port *recovers a
property the Go build already has.*

Framed that way, musl is not a new ambition — it is the price of parity.

### How to build it

- Targets: `x86_64-unknown-linux-musl`, `aarch64-unknown-linux-musl`.
- Toolchain: **`cross`** (Docker-based, the community standard) or
  **`cargo-zigbuild`** (which GoReleaser's Rust builder already uses by
  default — a reason to prefer it here for consistency).
- `.cargo/config.toml` with `rustflags = ["-C", "target-feature=+crt-static"]`;
  `-C strip=symbols` matches the current `-s -w` ldflags.
- Verify with `ldd` — if it lists libraries, it is not static.

### Two real caveats

**1. The allocator.** musl's default `malloc` is significantly slower than
glibc's, and this is the most commonly reported musl regression. The standard
mitigation is linking `mimalloc` or `tikv-jemallocator` on musl targets only
(gated by `#[cfg(target_env = "musl")]`). Sources note mimalloc brings
performance on par with glibc at some memory cost.

**Is it relevant to dstow?** Probably not much — dstow is I/O-bound
(directory walks, symlink syscalls, small TOML/JSON parses), not
allocation-bound. Measure before adding a dependency. Worth a note in the
plan, not a preemptive fix.

**2. NSS / `getpwnam`.** The classic static-musl failure is user and host
lookups: statically linked glibc breaks NSS, and musl's implementations are
limited. Two places this could touch dstow:

- **DNS / network** — *not applicable.* dstow shells out to the user's `git`
  binary (A17) and never opens a socket itself. The subprocess uses the
  system's own resolver. **This is an underrated benefit of the A17 decision:
  it makes the static-musl story clean.**
- **`~user` home expansion** — *applicable, and verified.*
  `internal/config/expand.go` matches `^~([^/]*)` — a leading tilde **with an
  optional username** — and resolves the named form via Go's `os/user`
  `user.Lookup`, i.e. `getpwnam` semantics. So this concern is real, not
  hypothetical.

  **But the Go build already solved it, and the solution transfers.** Because
  dstow builds with `CGO_ENABLED=0`, Go's `os/user` falls back to its **pure-Go
  `/etc/passwd` parser** rather than calling into libc NSS. The current binary
  therefore never depends on NSS either. A Rust port should do the same thing
  deliberately: **parse `/etc/passwd` directly rather than reaching for a
  `getpwnam`-backed crate** (`uzers`/`users` bind libc and are exactly the
  thing that breaks under static musl).

  Severity is further reduced by the failure shape: `expandTilde` returns the
  **unexpanded literal** when the lookup fails — a `~unknown` stays `~unknown`
  rather than erroring. So a degraded lookup is soft, not fatal. Still, silently
  changing this from "resolves" to "never resolves" would be a behavior
  regression, so it belongs in the plan rather than being discovered later.

### Scope note

macOS targets are unaffected — there is no musl on Darwin, and
`*-apple-darwin` links the system libSystem either way. The musl decision is
**Linux-only** and touches two of the four shipped platform triples.

---

## Sources

- [GoReleaser — Rust builder](https://goreleaser.com/customization/builds/rust) · [Announcing v2.5 — multi-language](https://goreleaser.com/blog/goreleaser-v2.5/) · [Announcing v2.15](https://goreleaser.com/blog/goreleaser-v2.15/) · [Using GoReleaser to release Rust and Zig projects](https://goreleaser.com/blog/rust-zig/) · [goreleaser/example-rust](https://github.com/goreleaser/example-rust)
- [release-plz](https://release-plz.dev/) · [cargo-dist](https://crates.io/crates/cargo-dist) · [Fully Automated Releases for Rust Projects](https://blog.orhun.dev/automated-rust-releases/)
- [How to Target musl for Fully Static Linux Binaries](https://www.rustfaq.org/en/how-to-target-musl-for-fully-static-linux-binaries/) · [Performance of static Rust with MUSL](https://raniz.blog/2025-02-06_rust-musl-malloc/) · [clux/muslrust](https://github.com/clux/muslrust) · [emk/rust-musl-builder](https://github.com/emk/rust-musl-builder)
