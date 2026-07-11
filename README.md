# 🦀 Crabby

A lightweight workspace manager for Claude Code.

Crabby has two goals only:

1. **Prepare projects** so they are immediately ready to work with Claude Code.
2. **Manage multiple Claude Code sessions** from a single terminal.

It is intentionally small. Crabby is *not* a Git client, a tmux replacement, a
project manager, an IDE, or an AI orchestrator.

## Philosophy

> Make working with multiple Claude Code sessions effortless.

Crabby is opinionated. It assumes **WSL + tmux + Claude Code** and requires no
configuration. There is only one real implementation — Linux, inside WSL.
Windows simply forwards commands into WSL.

```
Windows PowerShell
        │
        ▼
     crabby.exe
        │
        ▼
       WSL
        │
        ▼
      crabby
        │
        ▼
      tmux
        │
        ▼
   Claude Code
```

## Install

No compilation required — one command per platform.

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/marioolf/crabby/main/install.ps1 | iex
```

This installs `crabby.exe`, adds it to your PATH, checks WSL + Ubuntu, and
installs the Linux binary inside WSL for you.

### Linux / WSL

```bash
curl -fsSL https://raw.githubusercontent.com/marioolf/crabby/main/install.sh | bash
```

This downloads the right binary for your architecture into `~/.local/bin` and
checks tmux and Claude Code.

Then confirm everything is ready:

```bash
crabby doctor
```

> Building from source is only for contributors — see
> [CONTRIBUTING.md](CONTRIBUTING.md).

## Commands

### `crabby init`

Run inside a project. Creates:

```
project/
    .claude/
        crabby.yaml     # project metadata
    CLAUDE.md           # small, ready to customize
```

…and registers the project.

### `crabby start [project]`

Creates the tmux session if it does not exist, moves into the project
directory, launches Claude, and attaches. If the session already exists it just
attaches. With no argument it uses the current directory.

### `crabby ps`

The main entrypoint. Launches a Bubble Tea TUI listing every project and its
session state:

```
🦀 Crabby
──────────────────────────────────────────────
❯ ● payments
    state: Running

  ● frontend
    state: Waiting

  ○ docs
    state: Stopped

──────────────────────────────────────────────
↑/↓  Move    Enter  Attach    r  Refresh    q  Quit
```

Select a project and press **Enter** to be dropped directly into its Claude
session — no intermediate shell.

### `crabby attach [project]`

Attach to a project's session directly. With no argument it uses the current
directory.

### `crabby doctor`

Checks that WSL, Ubuntu, tmux, Claude, and crabby are all present.

### `crabby version`

Prints the installed version, e.g. `Crabby v0.1.0`.

## The complete workflow

```bash
crabby init      # prepare the project
crabby start     # launch Claude in a tmux session
crabby ps        # switch between all your Claude sessions
```

## Session model

Each project owns exactly one Claude session named `crabby_<project_name>`.
State is read live from tmux:

| State     | Meaning                                    |
| --------- | ------------------------------------------ |
| `Running` | session exists and a client is attached    |
| `Waiting` | session exists but detached (idle, ready)  |
| `Stopped` | no session                                 |

## Data & configuration

- **Project registry:** `~/.local/share/crabby/projects.json`
- **Global config:** `~/.config/crabby/config.yaml`

```yaml
claude_command: claude
tmux_binary: tmux
```

## Project layout

```
cmd/
    crabby/            # Linux (WSL) entrypoint
    crabby-windows/    # Windows wrapper -> crabby.exe
internal/
    cli/               # cobra commands
    initcmd/           # crabby init
    project/           # projects.json registry
    session/           # project <-> tmux session mapping + state
    tmux/              # thin tmux wrapper
    tui/               # bubble tea list (crabby ps)
    doctor/            # environment checks
    config/            # global config
    version/           # version (single source of truth: VERSION file)
    windows/           # wsl forwarding
.github/workflows/     # CI + release automation
docs/RELEASING.md      # how to cut a release
install.sh             # Linux / WSL installer
install.ps1            # Windows installer
```

## Releasing

See [docs/RELEASING.md](docs/RELEASING.md). In short: bump `VERSION`, update
`CHANGELOG.md`, then push a matching `v*` tag — GitHub Actions builds and
publishes the release automatically.

## Out of scope (for now)

Git integration, worktrees, GitHub/PRs, VSCode, multiple Claude sessions per
project, plugins, hooks, templates, profiles, snapshots, memory, AI workflows,
context generation, skills, MCP management. See the technical document for the
roadmap.
