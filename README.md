# Crabby

```
 (\/)    (\/)
   \(o..o)/
   /`----'\

     C R A B B Y
```

**Many Claudes. One shell.**

_prepare projects · orchestrate claude · one terminal_

A workspace manager for Claude Code — _by [marioolf](https://github.com/marioolf) · [github.com/marioolf/crabby](https://github.com/marioolf/crabby)_

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

Already have repositories with a `CLAUDE.md`? Adopt them without changing
anything about how they work — see [Adopting existing projects](#adopting-existing-projects).

Then just run Crabby:

```bash
crabby
```

```
                       (\/)    (\/)
                         \(o..o)/
                         /`----'\

                       C R A B B Y
                Many Claudes. One shell.
   prepare projects · orchestrate claude · one terminal

────────────────────────────────────────────────────

  ● test         Waiting for input  ·  1m ago
                 main  ·  up 8m  ·  Sonnet 5  ·  64k tok

  payments       feature/refunds  ·  Opus 4.8  ·  132k tok
▌   ● refactor   Working  ·  up 2h
    ● tests      Idle  ·  up 34m
    ○ docs       Stopped

  ○ SSH-AI       Stopped

────────────────────────────────────────────────────
3 workspaces   ·   3 active   ·   2 stopped   ·   1 working   ·   196k tokens today

enter open   n new workspace   t new task   x stop   d remove
r refresh   q quit   ·   F12 returns from a session
```

The banner stays centred; the list and footer are left-aligned like a normal
CLI, with each row's live data in a column to the right of its name. A workspace
with one task shows as a single line; a workspace with several (like `payments`
above) becomes a header with its tasks beneath it. The dashboard **refreshes
automatically** (about once a second) and sorts by importance (**Running →
Waiting → Stopped**, then alphabetically). See
[Workspaces and tasks](#workspaces-and-tasks) for the model and
[Workspace insights](#workspace-insights) for where the data comes from.

- **Enter** opens the selected task in Claude. If it wasn't running, Crabby
  starts it for you.
- **F12** (inside a session) brings you straight back to this list — no tmux
  shortcuts to learn. The session keeps running in the background.
- **n** adds the current directory as a new workspace (see [Packs](#packs)).
- **t** starts a new task in the selected workspace.
- **x** stops the highlighted task (asks first) — no need to exit Claude.
- **d** removes the highlighted task, or the whole workspace for a single-task
  row (asks first). Your files are kept; only Crabby forgets it.
- **q** quits.

The return key is shown on a small Crabby status bar while you work, so you
never have to remember it.

## Workspaces and tasks

Crabby has three simple concepts:

- **Workspace** — a project directory with a `CLAUDE.md`. This is the source of
  truth; it's what `crabby init` and `crabby import` register.
- **Task** — one Claude Code session working inside a workspace. A workspace can
  have several, each an independent session **sharing the same directory**.
- **Claude session** — the actual running Claude, one per task, kept alive in
  its own tmux session (named automatically, e.g. `crabby_payments_tests`).

One workspace, multiple tasks. This lets you run parallel efforts in the same
repository without cloning it:

```bash
cd ~/repos/payments
crabby task create refactor     # start a "refactor" task and jump in
crabby task create tests        # a second, independent Claude in the same repo
crabby task list                # see them both
crabby attach payments          # asks which task to open
crabby attach payments tests    # or open one directly
crabby task delete refactor     # done with it — the workspace stays
```

Every workspace starts with one default task (`main`), so if you never touch
`crabby task`, nothing changes: a workspace behaves exactly like a single
session did before. Tasks are independent — Crabby does not coordinate them,
share memory between them, or manage git branches; each is just its own Claude.

## Workspace insights

Crabby reads Claude Code's own session transcripts to show, at a glance, what is
happening across every workspace — without attaching to a single one. Each
workspace shows only the information that is actually available:

```
▌ ● payments
    Thinking…  ·  20s ago                          ← activity  ·  last activity
    feature/refunds  ·  up 2h14m  ·  Opus 4.8  ·  132k tok
      branch          uptime        model          session tokens
```

And a global summary sits below the list:

```
3 workspaces   ·   2 waiting   ·   1 stopped   ·   1 working   ·   180k tokens today
```

**Where the data comes from — and how reliable it is:**

| Metric | Source | Notes |
| --- | --- | --- |
| State (running / waiting / stopped) | tmux | Reliable. Running = attached, waiting = alive, stopped = no session. |
| Activity (Thinking / Editing files / Reading files / Running command / Responding / Waiting for input / Idle) | Claude transcript tail + tmux "working" signal | Best-effort. Derived from the last recorded step, so it can lag the live terminal by a moment. Only the fine label; the coarse state is always reliable. |
| Model | Claude transcript | Reliable — the model of the latest turn. |
| Tokens (per workspace) | Claude transcript | Reliable. Input + output for the **current session** (cache tokens excluded). |
| Tokens today (global summary) | Claude transcript | Reliable. Input + output across the workspace's transcripts **dated today**. |
| Last activity | Claude transcript | Reliable — timestamp of the last recorded turn. |
| Uptime | tmux | Reliable — how long the session has been alive. |
| Branch | git (read directly) | Reliable when the folder is a git repository. |

Every figure is **factual**: Crabby never estimates progress percentages or
completion times, and it omits any field it cannot read reliably rather than
showing a placeholder. Transcripts are read incrementally — only the bytes added
since the last refresh — so the dashboard stays fast with many workspaces.

Metrics that depend on Claude Code (activity, model, tokens, last activity)
require Claude's transcript files under `~/.claude/projects/`. If Claude Code has
never run in a workspace, those fields are simply absent and the workspace still
shows its tmux state, branch, and uptime.

## Adopting existing projects

If you already use Claude Code, you probably have repositories with a
`CLAUDE.md` in them. You don't need to recreate them — bring them in as they
are.

**One project** — point `init` at it (no need to `cd` first):

```bash
crabby init ~/repos/payments
```

**A whole folder of them** — let Crabby find them:

```bash
crabby import ~/repos
```

`import` scans the tree for folders that contain a `CLAUDE.md` and shows what it
found — a git repository isn't required. Pick the ones you want and press Enter:

```
       (\/)    (\/)
         \(o..o)/
         /`----'\

       C R A B B Y
Many Claudes. One shell.

Import Claude workspaces
Found 4 workspaces.

▌ [x] ai-security   main
  [x] frontend      main
  [✓] payments      already imported
  [ ] old-test      wip/spike

────────────────────────────────────
space select   a all   enter import   q cancel
```

- **Space** toggles a workspace; **a** toggles all.
- Projects you've already imported show `[✓]` and can't be added twice.
- Everything importable starts checked, so adopting a folder is usually just
  `crabby import ~/repos` then **Enter**.

Import never edits your repositories — it registers them and writes Crabby's own
`.claude/crabby.yaml`, leaving your existing `CLAUDE.md` and workflow untouched.
Imported projects behave exactly like ones created with `crabby init`.

`import` skips the obvious noise while scanning — `.git`, `node_modules`,
`.venv`, `vendor`, and similar — and stops at the first `CLAUDE.md` on each
branch of the tree, so a project's own subdirectory context files aren't
imported as separate workspaces. When a folder is a git repository its current
branch is shown for context; when it isn't, it's still importable.

## Commands

| Command | What it does |
| --- | --- |
| `crabby` | Open the home screen (this is all you normally need). |
| `crabby init [path]` | Register a workspace (the current directory, or `path`). Creates `.claude/crabby.yaml`, and a starter `CLAUDE.md` if none exists. |
| `crabby import [path]` | Discover folders that already have a `CLAUDE.md` and import the ones you pick (the current directory, or `path`). |
| `crabby task create <name>` | Create a task (a parallel Claude session) in the current workspace and open it. |
| `crabby task list [workspace]` | List a workspace's tasks and their state. |
| `crabby task delete <name> [workspace]` | Delete a task (the workspace stays). |
| `crabby task restart <name> [workspace]` | Stop a task's session and start it fresh. |
| `crabby ps` | Alias for `crabby`. |
| `crabby attach [workspace] [task]` | Open a workspace's session; asks which task when several exist, or pass the task name. |
| `crabby start [workspace] [task]` | Same as attach — launches the session if needed. |
| `crabby rm [workspace]` | Remove a workspace from Crabby, stopping its tasks (files are kept). |
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

A live session that is actively producing output shows what it's doing (e.g.
`Working`, `Thinking…`, `Editing files`), so you can see at a glance which of
your Claude workers are busy. See [Workspace insights](#workspace-insights).

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
    importcmd/         # crabby import (workspace discovery)
    insights/          # reads Claude transcripts for dashboard metrics
    pack/              # local project packs
    project/           # projects.json registry
    session/           # project <-> session mapping + state
    tmux/              # tmux driver (dedicated socket, F12 detach, status bar)
    tui/               # bubble tea home screen + pack/import selectors
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

Git integration, worktrees, GitHub/PRs, VSCode, coordination or shared memory
between tasks, plugins, hooks, templates, profiles, snapshots, memory, AI
workflows, context generation, skills, MCP management. See the technical
document for the roadmap.
