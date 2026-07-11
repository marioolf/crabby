# 🦀 Crabby

**A workspace manager for Claude Code.**

> Prepare projects. Organize sessions. Just one terminal.

_by [marioolf](https://github.com/marioolf) · [github.com/marioolf/crabby](https://github.com/marioolf/crabby)_

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
              _~_      _~_      _~_
           __(o )>  __(o )>  __(o )>

                  C R A B B Y
               Prepare projects.
              Organize sessions.
              Just one terminal.

     by marioolf · github.com/marioolf/crabby
────────────────────────────────────────────────

▌ ● payments
    feature/refunds
    Waiting · working

  ● frontend
    main
    Waiting

  ○ docs
    main
    Stopped

────────────────────────────────────────────────
enter open   n new   x stop   d remove
r refresh   q quit   ·   F12 returns from a session
```

The dashboard **refreshes automatically** (about once a second), sorts projects
by importance (**Running → Waiting → Stopped**, then alphabetically), and marks
sessions that are **currently producing output** with `· working`. State is
colour-coded: green (running), yellow (waiting), grey (stopped).

- **Enter** opens the selected project in Claude. If it wasn't running, Crabby
  starts it for you.
- **F12** (inside a session) brings you straight back to this list — no tmux
  shortcuts to learn. The session keeps running in the background.
- Pick another project and you're in a different Claude in one keypress.
- **n** adds the current directory as a project (see [Packs](#packs)).
- **x** stops the highlighted session (asks first) — no need to exit Claude.
- **d** removes the highlighted project from Crabby (asks first). Your files are
  kept; only Crabby forgets it. Also available as `crabby rm [project]`.
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
| `crabby rm [project]` | Remove a project from Crabby (files are kept). |
| `crabby doctor` | Check that WSL, Ubuntu, tmux, Claude, and crabby are present. |
| `crabby version` | Print the installed version. |

## Packs

A **pack** is a reusable project setup — nothing more than a directory of files
that Crabby copies into a project when you run `crabby init`. No plugins, no
templates, no variables: just folders.

Packs live in:

```text
~/.config/crabby/packs/
```

Each subdirectory is one pack:

```text
~/.config/crabby/packs/
    simple/
    go/
    company/
```

A pack contains a small manifest plus whatever files you want copied in:

```text
simple/
    pack.yaml          # manifest (not copied into the project)
    CLAUDE.md          # copied
    .claude/           # copied (any files you like)
```

`pack.yaml` only needs a name; the rest is optional:

```yaml
name: simple
description: Minimal starter pack
author: Mario Lopez
version: 1.0.0
```

### Using a pack

Run `crabby init` in a project (or press **n** on the home screen):

- **No packs installed** → Crabby writes a minimal default `CLAUDE.md`.
- **One pack** → it's used automatically.
- **Several packs** → Crabby shows a selector.

Files that already exist in the project are **never overwritten** — they're
reported and kept. Crabby always writes its own `.claude/crabby.yaml`
(the registry metadata) regardless of the pack.

### Creating your own pack

Copy one of the bundled examples and edit it — that's the whole workflow. The
repo ships a few in [`examples/`](examples/): `simple-pack`, `go-pack`,
`docs-pack`.

```bash
cp -r examples/go-pack ~/.config/crabby/packs/go
# edit ~/.config/crabby/packs/go/pack.yaml (set the name)
# edit ~/.config/crabby/packs/go/CLAUDE.md, add any other files
```

Next time you run `crabby init`, your pack is available (and with two or more,
you get the selector). Packs are just directories, so managing them is `cp`,
`rm`, and your editor. Crabby prints the packs directory after a plain `init`
and shows it in the selector, so you always know where they go.

## Session states

| State | Meaning |
| --- | --- |
| 🟢 `Running` | You're attached — this is the session you're in. |
| 🟡 `Waiting` | Claude is alive in the background; press Enter to jump back in. |
| ⚪ `Stopped` | Nothing running; press Enter to start fresh. |

A `Waiting` session that is actively producing output is tagged `· working`, so
you can see at a glance which of your Claude workers are busy.

## Notifications

The dashboard is itself the notification surface: because it refreshes every
second, sessions that are working light up with `· working` and reorder to the
top, and a session that has just started producing output rings a short
terminal bell.

**Limitation, stated honestly:** Crabby cannot pop a desktop notification *while
you are inside a Claude session*. When you open a session, Crabby hands the
whole terminal to it and waits — it is not running in the background, so it has
nothing to notify *from*. Reliable cross-session desktop alerts would require a
background daemon, which this project intentionally avoids. The practical,
robust alternative is the live dashboard: leave `crabby` open on a second
terminal (or glance at it between tasks) to see which workers are busy, idle, or
done.

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
    initcmd/           # crabby init (+ pack application)
    pack/              # local project packs
    project/           # projects.json registry
    session/           # project <-> session mapping + state
    tmux/              # tmux driver (dedicated socket, F12 detach, status bar)
    tui/               # bubble tea home screen + pack selector
    doctor/            # environment checks
    config/            # global config
    version/           # version (single source of truth: VERSION file)
    windows/           # wsl forwarding
examples/              # example packs to copy and customize (simple, go, docs)
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
