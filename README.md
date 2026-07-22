# dstow

**Deploy dotfiles and configuration as symlinks, from packages in repos.**

dstow links the files in a *package* into a *target* directory (your `$HOME` by
default), the same way GNU Stow does — but it remembers what it deployed, works
across many repos at once, names things by a stable qualified identity, and
never depends on the directory you happen to be standing in.

- **Repos** are where packages come from — a local directory, or a remote
  clone (`github:owner/name`).
- **Packages** are the top-level directories inside a repo. Each package is a
  tree of files to link into the target.
- **Targets** are where the links go. The default is `$HOME`; any level of
  config can point elsewhere.

```
github:yourname/dotfiles::starship
└─┬──┘ └───────┬───────┘  └──┬───┘
  │            │             package
  │            coordinate
  scheme
```

That fully-qualified name (FQN) is a package's stable identity. You rarely type
the whole thing — **any unambiguous suffix works** (`starship`,
`dotfiles::starship`, `yourname/dotfiles::starship`), and **the working
directory never changes what a command does.**

---

## The manual is in the binary

**dstow's documentation ships inside dstow.** Once it is installed, you never
need this page or any website:

```sh
dstow manual                     # the whole manual, navigable as commands
dstow manual concepts            # the model: repos, packages, targets, states
dstow manual configuration keys  # every config key and where it is legal
dstow manual reference           # exit codes, environment, file locations
dstow <command> --help           # any command's help
```

`dstow manual <TAB>` walks the tree. A command's manual page *is* the source of
its `--help`, so the two can never disagree.

This README covers only what you need **before** you have dstow: how to install
it, and what the first five minutes look like.

---

## Install

### Bootstrap snippet (recommended)

Add dstow's bootstrap to your shell rc. It puts `~/.local/bin` on `PATH` and
installs dstow **only if it is missing** — when dstow is already present the
snippet is silent, invisible, and offline (the installer is never even
fetched):

```sh
dstow snippet rc >> ~/.bashrc      # or ~/.zshrc, ~/.profile, …
```

The emitted snippet is plain POSIX sh — dstow never edits your rc files, it only
prints the text to stdout for you to redirect. On a fresh machine where dstow is
not yet installed, run the snippet's install line directly:

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dstow/main/install.sh | sh -s --
```

The installer drops `dstow` into `~/.local/bin`. It is idempotent: run against a
machine that already has dstow, it prints one status line and exits 0.

**Installer flags:** `--force` reinstalls even when present; `--version vX.Y.Z`
ensures exactly that release — already installed at that version it exits 0,
otherwise it installs it (no implied force). The install dir is tunable
(`--install-dir`, `DSTOW_INSTALL_DIR`, `XDG_BIN_HOME`); `~/.local/bin` is the
default the snippet relies on. Downloads are checksum-verified always, and
cosign-verified when cosign is available. See `install.sh --help` for the
full surface.

Add installer args right after the `--`, such as `--force` or
`--version <version>`:

```sh
curl -fsSL https://raw.githubusercontent.com/rocne/dstow/main/install.sh | sh -s -- --force
```

### Linux packages (dnf / apt)

Signed `.rpm` and `.deb` packages are published to a hosted repo, so dstow
installs by name and stays current through your normal system upgrades. Add the
repo once, then install:

```sh
# Fedora, RHEL, CentOS Stream, openSUSE, … (dnf/yum)
curl -1sLf 'https://dl.cloudsmith.io/public/rocne/releases/setup.rpm.sh' | sudo -E bash
sudo dnf install dstow
```

```sh
# Debian, Ubuntu, … (apt)
curl -1sLf 'https://dl.cloudsmith.io/public/rocne/releases/setup.deb.sh' | sudo -E bash
sudo apt install dstow
```

The setup script drops a repo file into `/etc/yum.repos.d` (or
`/etc/apt/sources.list.d`) and imports the repo's index-signing key, so
`dnf upgrade` / `apt upgrade` pick up new dstow releases automatically. The
packages themselves are GPG-signed by a separate key (fingerprint `64894FE3…`),
whose public half is attached to every GitHub release as
`dstow-signing-key.asc` if you want to verify a download by hand.

### With Go

If you have a Go toolchain, install straight from source — the binary reports
the module version it was built from:

```sh
go install github.com/rocne/dstow/cmd/dstow@latest
```

### Shell completion

```sh
dstow completion bash        # also: zsh, fish, powershell
```

See `dstow completion --help` for where to place the output for your shell.

---

## Quickstart

```sh
# 1. Register a repo of packages (clones a remote; registers a local path in place).
dstow repo add github:rocne/dotfiles
dstow repo add ~/my-dotfiles

# 2. See what you have (reads config only — never touches disk).
dstow list                       # your repos
dstow list dotfiles              # a repo's packages

# 3. Deploy. Name packages, or a whole repo, by any unambiguous suffix.
dstow stow zsh git tmux
dstow stow dotfiles              # a repo: all of its packages

# 4. Check reality.
dstow status                     # what is actually deployed, live

# 5. Later: refresh, or remove.
dstow restow zsh                 # unstow then stow (pick up changes)
dstow unstow tmux                # remove tmux's links
```

First run on a machine that already has live config files? Adopt them into the
package as you stow, so nothing is destroyed:

```sh
dstow stow --all --adopt
```

Coming from GNU Stow? dstow reads your existing `.stowrc` and
`.stow-local-ignore` — you do not have to rewrite anything to start:

```sh
dstow manual configuration stow-compat
```

---

## Where to go next

Everything else lives in the binary:

| You want | Run |
|---|---|
| The model: repos, packages, targets, the ledger | `dstow manual concepts` |
| What `occupied` or `orphaned` means | `dstow manual concepts states` |
| The naming grammar and suffix matching | `dstow manual concepts naming` |
| Every command | `dstow manual commands`, or `dstow <cmd> --help` |
| Config keys and where each is legal | `dstow manual configuration keys` |
| Ignore patterns | `dstow manual configuration ignores` |
| Migrating from GNU Stow | `dstow manual configuration stow-compat` |
| Theming, slots, color values | `dstow manual theming` |
| Writing a hook, and its context contract | `dstow manual hooks` |
| Exit codes, environment variables, file locations | `dstow manual reference` |

The manual's source is the [`docs/`](docs/) directory of this repository, if you
would rather read it here — but the copy in your binary is the one that matches
your version.

## Contributing

Design and requirements documents, architecture decision records, and agent
workflow docs live in [`dev/`](dev/).

## License

dstow is licensed under the [GNU General Public License v3.0](LICENSE).
Copyright (c) 2026 Rocne Scribner.
