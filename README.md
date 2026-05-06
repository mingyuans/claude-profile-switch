# ccs

Switch between multiple Claude Code accounts on a single machine by toggling `CLAUDE_CONFIG_DIR`.

`ccs` registers named profiles (each backed by its own config directory) and provides a `ccs use <name>` shortcut that takes effect **immediately in the current shell session** when paired with the auto-sourced shell wrapper.

---

## Why

Claude Code reads its credentials, sessions and settings from `$CLAUDE_CONFIG_DIR` (defaulting to `~/.claude`). To run multiple accounts side-by-side you need to flip that variable, but a child process can't mutate its parent shell's environment. `ccs` solves this with the standard "binary + sourced shell function + `eval`" pattern (à la `pyenv`, `direnv`, `nvm`).

---

## Install

### Quick install (curl | bash)

```bash
curl -fsSL https://raw.githubusercontent.com/mingyuans/claude-profile-switch/main/install.sh | bash
```

The installer detects your platform (darwin/linux × amd64/arm64), downloads the matching binary from GitHub Releases, and installs it into `/usr/local/bin` (or `~/.local/bin` if that isn't writable).

Customise via flags or env vars:

```bash
# pin a release tag
curl -fsSL https://raw.githubusercontent.com/mingyuans/claude-profile-switch/main/install.sh | bash -s -- --version v0.1.0

# install to a user-local path (no sudo)
CCS_BIN_DIR="$HOME/.local/bin" \
  curl -fsSL https://raw.githubusercontent.com/mingyuans/claude-profile-switch/main/install.sh | bash

# install from a fork
CCS_REPO=your-fork/claude-profile-switch \
  curl -fsSL https://raw.githubusercontent.com/your-fork/claude-profile-switch/main/install.sh | bash
```

Prefer to inspect the script first (recommended whenever piping a remote script into a shell):

```bash
curl -fsSL https://raw.githubusercontent.com/mingyuans/claude-profile-switch/main/install.sh -o install.sh
less install.sh
bash install.sh
```

### Build from source

```bash
git clone https://github.com/mingyuans/claude-profile-switch.git
cd claude-profile-switch
make build
sudo cp bin/ccs /usr/local/bin/
```

### Enable shell integration (one-time, all install methods)

```bash
echo 'eval "$(ccs init zsh)"' >> ~/.zshrc
exec zsh
```

Bash and fish are also supported: `ccs init bash` / `ccs init fish | source`.

---

## Usage

```bash
# 1. register profiles
ccs add work                       # defaults to ~/.claude-work
ccs add personal /path/to/dir      # custom path
ccs register oldsetup ~/.claude-oldsetup  # adopt an existing dir as-is

# 2. inspect
ccs list
ccs current

# 3. switch (instant in current shell, thanks to the wrapper)
ccs use work
echo $CLAUDE_CONFIG_DIR            # /Users/you/.claude-work

# 4. cleanup
ccs rm personal                    # unregister, leave directory intact
ccs rm personal --purge --yes      # also delete the directory
```

### Subcommands

| Command | Description |
|---|---|
| `ccs add <name> [path]` | Register a profile, auto-create the directory, and symlink shareable items from `~/.claude` (see [Sharing](#sharing-extensions-across-profiles)). `--no-create` skips dir creation, `--no-share` skips the symlinks. |
| `ccs register <name> <path>` (alias `import`) | Register an *existing* Claude config directory under a name. Refuses to create or modify the directory; pass `--share` to opt in to symlinking shareable items from `~/.claude`. |
| `ccs rm <name>` | Unregister a profile. `--purge --yes` also deletes its directory. |
| `ccs list` (alias `ls`) | Show all profiles in a table. |
| `ccs current` | Show live `$CLAUDE_CONFIG_DIR` and the last-switched profile. |
| `ccs use <name>` (alias `switch`) | Activate a profile. With `--export`, prints `export CLAUDE_CONFIG_DIR=...` for `eval`. |
| `ccs path <name>` | Print the profile's directory (script-friendly, no decoration). |
| `ccs init <shell>` | Print shell integration script. Supports `zsh`, `bash`, `fish`. |
| `ccs version` | Print version. |

---

## Sharing extensions across profiles

By default `ccs add` symlinks a small whitelist of "extension definition" or "preference" entries from `~/.claude` into the new profile, so things like your custom skills and slash commands are automatically available under every account without duplicating files.

**Symlinked by default** (only the entries that actually exist in `~/.claude` are linked):

| Path | Why share |
|---|---|
| `CLAUDE.md` | Global user instructions |
| `agents/` | Custom subagent definitions |
| `commands/` | Custom slash commands |
| `skills/` | User-defined skills |
| `output-styles/` | Output style presets |
| `keybindings.json` | Key binding overrides |
| `hooks/` | Hook scripts |
| `plugins/` | Installed plugin packages |
| `settings.json` | Model / theme / permission preferences |

**Never shared** (stays per-profile, as expected):

`.credentials.json`, `projects/`, `todos/`, `statsig/`, `settings.local.json`, session files, lock files, and anything else not in the whitelist above. The list is a *whitelist*, so any new credential/state file Claude Code adds in the future stays isolated automatically.

**Safeguards**:

- An entry already present in the new profile dir is **never overwritten** — symlink creation skips it.
- If the new profile dir resolves to the same path as `~/.claude` itself, the share step is skipped entirely (no self-links).
- Pass `--no-share` to skip the symlink step on a per-profile basis.

```bash
ccs add work                # symlinks shareable items by default
ccs add isolated --no-share # fully empty profile, nothing shared
```

---

## How the live-switch works

1. `ccs init zsh` prints a shell function called `ccs`.
2. After you `eval` it (typically from `.zshrc`), every `ccs <subcommand>` call goes through the function.
3. For `use` / `switch`, the function runs `command ccs use <name> --export`, which prints exactly one line: `export CLAUDE_CONFIG_DIR='/abs/path'`. (`command` bypasses the wrapper function and invokes the binary directly, even though they share the same name.)
4. The function `eval`s that line, mutating the **current** shell's environment.
5. Other subcommands are forwarded to the binary unchanged.

Without the wrapper, `ccs use work` only persists the choice in the on-disk store and prints a hint — the parent shell's `$CLAUDE_CONFIG_DIR` won't change.

---

## Config storage

Profiles are persisted as JSON. Lookup order:

1. `$CCS_CONFIG_PATH` (full path override, useful for tests)
2. `$XDG_CONFIG_HOME/ccs/profiles.json`
3. `~/.config/ccs/profiles.json` (default)

```json
{
  "current": "work",
  "profiles": [
    { "name": "work",     "dir": "/Users/you/.claude-work" },
    { "name": "personal", "dir": "/Users/you/.claude-personal" }
  ]
}
```

Saves are atomic (write to `*.tmp`, then rename).

---

## Development

```bash
make help        # list all targets
make install     # go mod tidy
make test        # go test ./...
make lint        # gofmt + go vet
make build       # produce bin/ccs
make release     # cross-compile dist/ccs-{darwin,linux}-{amd64,arm64}.tar.gz
make run ARGS="list"
```

### Cutting a release

1. Tag and push: `git tag v0.1.0 && git push --tags`
2. `make release` → produces `dist/ccs-*.tar.gz`
3. Upload the four tarballs as assets on the GitHub Release named after the tag
4. `install.sh` automatically picks them up via `https://github.com/$repo/releases/download/$tag/ccs-$os-$arch.tar.gz`

### Layout

```
.
├── main.go
├── cmd/                   # cobra subcommands (add, rm, list, current, use, path, init, version)
├── internal/
│   ├── config/            # JSON-backed Store; CRUD + atomic save
│   ├── output/            # ▸ ✓ ! ✗ – icons, table, summary, TTY-aware colour
│   └── shellfs/           # embed.FS for shell integration templates
└── shell-templates*       # source shell scripts: see internal/shellfs/
```

---

## Uninstall

```bash
sudo rm /usr/local/bin/ccs
# remove the `eval "$(ccs init zsh)"` line from ~/.zshrc
rm -rf ~/.config/ccs
```
