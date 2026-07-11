# 🦀 Crabby

**A workspace manager for Claude Code.**

Run `crabby`, pick a project, and you're in Claude. Leave the session with a
single key and you're back on the project list, ready to jump into the next one.
Crabby is the home screen for all your Claude Code work.

```
crabby                 ← your projects
   │  Enter
   ▼
Claude (Project A)     ← work
   │  F12
   ▼
crabby                 ← back on the list
   │  Enter
   ▼
Claude (Project B)     ← switch, instantly
```

No shell commands between sessions. No session names to remember. You never
leave Crabby.

## Philosophy

> Make working with multiple Claude Code sessions effortless.

Crabby is small and opinionated. It assumes **WSL + Claude Code** and needs no
configuration. Under the hood it uses tmux to keep your sessions alive, but you
never have to think about it — Crabby owns the whole experience and tmux stays
out of sight.

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

## Everyday use

Add a project once:

```bash
cd my-project
crabby init
```

Then just run Crabby:

```bash
crabby
```

```
🦀 Crabby  workspace manager for Claude Code

▌ ● payments     Waiting
    ~/work/payments   ⎇ feature/refunds   active 2m ago

  ● frontend     Waiting
    ~/work/frontend   ⎇ main   active 1h ago

  ○ docs         Stopped
    ~/work/docs   ⎇ main

↑/↓ move   enter open   r refresh   q quit
inside a session, press F12 to return here
```

- **Enter** opens the selected project in Claude. If it wasn't running, Crabby
  starts it for you.
- **F12** (inside a session) brings you straight back to this list — no tmux
  shortcuts to learn. The session keeps running in the background.
- Pick another project and you're in a different Claude in one keypress.
- **q** quits.

The return key is shown on a small Crabby status bar while you work, so you
never have to remember it.

## Commands

| Command | What it does |
| --- | --- |
| `crabby` | Open the home screen (this is all you normally need). |
| `crabby init` | Register the current directory as a project (creates `.claude/crabby.yaml` and `CLAUDE.md`). |
| `crabby ps` | Alias for `crabby`. |
| `crabby attach [project]` | Open a specific project's session directly. |
| `crabby start [project]` | Same as attach — launches the session if needed. |
| `crabby doctor` | Check that WSL, Ubuntu, tmux, Claude, and crabby are present. |
| `crabby version` | Print the installed version. |

## Session states

| State | Meaning |
| --- | --- |
| 🟢 `Running` | You're attached — this is the session you're in. |
| 🟡 `Waiting` | Claude is alive in the background; press Enter to jump back in. |
| ⚪ `Stopped` | Nothing running; press Enter to start fresh. |

## Configuration

Crabby needs no configuration. If you want to change something, edit
`~/.config/crabby/config.yaml`:

```yaml
claude_command: claude   # how to launch Claude Code
tmux_binary: tmux        # tmux binary to use
detach_key: F12          # key that returns you to Crabby
```

Projects are recorded in `~/.local/share/crabby/projects.json`.

If your keyboard sends F12 somewhere else, set `detach_key` to another
[tmux key name](https://man.openbsd.org/tmux#KEY_BINDINGS) such as `C-g`.

## Project layout

```
cmd/
    crabby/            # Linux (WSL) entrypoint
    crabby-windows/    # Windows wrapper -> crabby.exe
internal/
    cli/               # cobra commands
    initcmd/           # crabby init
    project/           # projects.json registry
    session/           # project <-> session mapping + state
    tmux/              # tmux driver (dedicated socket, F12 detach, status bar)
    tui/               # bubble tea home screen
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
