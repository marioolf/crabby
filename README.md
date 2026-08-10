<img width="967" height="141" alt="image" src="https://github.com/user-attachments/assets/63df5554-ea50-4a4e-9abd-4784df9ffcec" />


A workspace manager for AI Agent — _by [marioolf](https://github.com/marioolf) · [github.com/marioolf/crabby](https://github.com/marioolf/crabby)_

Run `crabby`, pick a project, and you're in Agent. Leave the session with a
single key and you're back on the project list, ready to jump into the next one.
Crabby is the home screen for all your AI Agent work.

```
crabby                 ← your projects
   │  Enter
   ▼
Agent (Project A)     ← work
   │  F12
   ▼
crabby                 ← back on the list
   │  Enter
   ▼
Agent (Project B)     ← switch, instantly
```

No shell commands between sessions. No session names to remember. You never
leave Crabby.

## Philosophy

> Make working with multiple AI Agent sessions effortless.

Crabby is small and opinionated. It assumes **WSL + AI Agent** and needs no
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
checks tmux and AI Agent.

Then confirm everything is ready:

```bash
crabby doctor
```

> Building from source is only for contributors — see
> [CONTRIBUTING.md](CONTRIBUTING.md).

## Everyday use

Crabby is a terminal application. Open it once and stay inside it — creating
workspaces and tasks, jumping into Agent, and managing packs all happen in the
interface. There are no commands to remember.

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

╭──────────────────╮ ╭──────────────────╮ ╭──────────────────────────╮
│ Workspaces       │ │ Tasks            │ │ Details                  │
│                  │ │                  │ │                          │
│ ▌ ● payments (3) │ │ ▌ ● refactor     │ │ status   Working         │
│   ● test         │ │   ● tests        │ │ task     refactor        │
│   ○ SSH-AI       │ │   ○ docs         │ │ branch   feature/refunds │
│                  │ │                  │ │ model    Opus 4.8         │
│                  │ │                  │ │ tokens   132k session     │
│                  │ │                  │ │ uptime   2h14m            │
│                  │ │                  │ │ activity Editing files    │
╰──────────────────╯ ╰──────────────────╯ ╰──────────────────────────╯
        5 workspaces   ·   2 active   ·   4 stopped   ·   196k tokens today
        ↑↓ move   tab pane   enter open   n new   t task   x stop   d remove
   F12 mission control   i import   p packs   s settings   ? help   q quit
```

The home screen is three columns — **Workspaces**, **Tasks**, **Details** — with
the identity banner above and the actions and return-key hint below. Move the
cursor with the arrows, switch columns with **Tab**, and press **Enter** to open
the selected task. The dashboard **refreshes automatically** (about once a
second) and sorts workspaces by importance (**Running → Waiting → Stopped**, then
alphabetically). See [Workspaces and tasks](#workspaces-and-tasks) for the model
and [Workspace insights](#workspace-insights) for where the Details come from.

- **↑ ↓** move within the focused column; **Tab / Shift+Tab** (or **← →**) switch
  between Workspaces and Tasks.
- **Enter** opens the selected task in Agent. If it wasn't running, Crabby
  starts it for you.
- **F12** toggles [Mission Control](#mission-control), the live overview of every
  AI Agent task. Inside a AI Agent session, **F12** brings you straight back here —
  no tmux shortcuts to learn; the session keeps running in the background.
- **n** creates a new workspace from any folder (see [Packs](#packs)) — the path
  field completes with **Tab** and lists matching sub-directories as you type, and
  **Ctrl+O** opens the native Windows folder picker. You are also asked **which AI
  agent** (e.g. `claude`, `opencode`, `agy`) to associate with the workspace;
  this keeps separate workspaces at the same path from overwriting each other (see
  [Multi-agent workspaces](#multi-agent-workspaces)).
- **i** [imports existing Agent workspaces](#adopting-existing-projects) — it
  scans your machine automatically.
- **t** starts a new task in the selected workspace. You will be asked to choose
  the **agent** for that task; it can differ from the workspace's default agent
  (see [Per-task agent selection](#per-task-agent-selection)).
- **o** opens the selected workspace's folder in the file manager (Windows
  Explorer on WSL), so you can drop files straight into it.
- **e** relinks a workspace whose folder you've moved or renamed: Crabby flags it
  as `⚠ missing`, and **e** points it at the new folder (updating its name and
  path, keeping its tasks). Crabby can't guess where a folder went, so this is how
  you tell it.
- **x** stops the highlighted task (asks first) — no need to exit Agent.
- **d** removes the highlighted task, or the whole workspace (asks first). Your
  files are kept; only Crabby forgets it.
- **p** manages packs, **s** opens Settings, **?** shows the full key list, and
  **q** quits.

Every shortcut is on screen, and the return key is shown on a small Crabby status
bar while you work — so you never have to remember anything.

## Workspaces and tasks

Crabby has three simple concepts:

- **Workspace** — a project directory with a `CLAUDE.md`. This is the source of
  truth; it's what **n** (new) and **i** (import) register.
- **Task** — one AI Agent session working inside a workspace. A workspace can
  have several, each an independent session **sharing the same directory**.
- **AI Agent session** — the actual running Agent, one per task, kept alive in
  its own tmux session (named automatically, e.g. `crabby_payments_tests`).

One workspace, multiple tasks. This lets you run parallel efforts in the same
repository without cloning it — all from the home screen:

- Select the workspace and press **t** to start a new task (say `refactor`); it
  opens straight away in its own Agent.
- Press **t** again for a second, independent task (say `tests`) in the same repo.
- Both appear in the **Tasks** column; **Tab** into it, pick one, **Enter** to
  open, **d** to remove it (the workspace stays).

Every workspace starts with one default task (`main`), so a workspace with a
single task behaves exactly like a single session. Tasks are independent — Crabby
does not coordinate them, share memory between them, or manage git branches; each
is just its own Agent.

## Multi-agent workspaces

Crabby supports running **multiple AI agents side by side**, even on the same
project directory. When you open a second workspace pointing at a folder that is
already registered, Crabby checks whether the agent differs:

- **Same agent** → the existing workspace is reused (no duplication).
- **Different agent** → a new, independent workspace is created with a unique
  name derived from the path and the agent binary (e.g. `payments-opencode`).

This means you can have, for example, a `claude` workspace and an `opencode`
workspace both targeting `/repos/payments` and switch between them from the
dashboard without either one overwriting the other.

## Per-task agent selection

When you press **t** to create a task, Crabby asks you which agent should run it.
The selected agent is saved with the task in `projects.json` and used whenever
that task is opened, independently of the workspace's default agent. This lets
you run a `claude` task and an `opencode` task inside the same workspace at the
same time, each in its own tmux session.

## Workspace insights

Crabby reads AI Agent's own session transcripts to show, at a glance, what is
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
| Activity (Thinking / Editing files / Reading files / Running command / Responding / Waiting for input / Idle) | Agent transcript tail + tmux "working" signal | Best-effort. Derived from the last recorded step, so it can lag the live terminal by a moment. Only the fine label; the coarse state is always reliable. |
| Model | Agent transcript | Reliable — the model of the latest turn. |
| Tokens (per workspace) | Agent transcript | Reliable. Input + output for the **current session** (cache tokens excluded). |
| Tokens today (global summary) | Agent transcript | Reliable. Input + output across the workspace's transcripts **dated today**. |
| Last activity | Agent transcript | Reliable — timestamp of the last recorded turn. |
| Uptime | tmux | Reliable — how long the session has been alive. |
| Branch | git (read directly) | Reliable when the folder is a git repository. |

Every figure is **factual**: Crabby never estimates progress percentages or
completion times, and it omits any field it cannot read reliably rather than
showing a placeholder. Transcripts are read incrementally — only the bytes added
since the last refresh — so the dashboard stays fast with many workspaces.

Metrics that depend on AI Agent (activity, model, tokens, last activity)
require the Agent's transcript files under `~/.claude/projects/`. If AI Agent has
never run in a workspace, those fields are simply absent and the workspace still
shows its tmux state, branch, and uptime.

## Adopting existing projects

If you already use AI Agent, you probably have repositories with a `CLAUDE.md`
in them. You don't need to recreate them — bring them in as they are, from inside
Crabby.

Press **i** and Crabby **scans your machine straight away** — no path to type. It
looks through your WSL home and the common project folders under each Windows user
on your mounted drives (`Desktop`, `Documents`, `source`, `repos`, `projects`, …),
finds every folder with a `CLAUDE.md` (or `claude.md`), and lists them:

```
────────────────────────────────────────────────────

                   Import workspaces

        Found 4 Agent workspaces   ·   Selected: 2

        ▌ [✔] payments
              ✔ already imported  ·  /repos/payments
          [✔] frontend
              ✔ already imported  ·  /mnt/c/Users/you/repos/frontend
          [x] docs
              main  ·  /repos/docs
          [x] ai-security
              wip/spike  ·  /repos/ai-security

────────────────────────────────────────────────────
   ↑↓ move   space toggle   a all   n none   i import   esc back
```

- The scan runs off the UI thread, so it stays responsive; press **r** to rescan.
- **Space** toggles a workspace; **a** selects all, **n** selects none. The
  `Selected` count is always visible. Everything not-yet-imported starts checked,
  so adopting a machine full of projects is often just **i** then **i** again.
- Projects you've already imported show `✔` and can't be added twice — press **d**
  on one to un-import it (Crabby forgets it; your files are kept).
- Press **i** (or **Enter**) to import the selection; the workspaces appear on the
  dashboard immediately.

Import never edits your repositories — it registers them and writes Crabby's own
`.claude/crabby.yaml`, leaving your existing `CLAUDE.md` and workflow untouched.

Scanning skips the obvious noise — `.git`, `node_modules`, `.venv`, `vendor`,
`build`, `dist`, and hidden config folders like `~/.claude` — and stops at the
first context file on each branch of the tree, so a project's own subdirectory
context files aren't imported as separate workspaces. When a folder is a git
repository its current branch is shown for context; when it isn't, it's still
importable.

## Mission Control

Press **F12** to open **Mission Control** — a live overview of *every* AI Agent task
at once, so you can see what all your Agent workers are doing without attaching to
each one. **F12** again returns to the dashboard.

```
────────────────────────────────────────────────────

                   Mission Control

╭──────────────────────────╮ ╭──────────────────────────╮
│ ● payments:refactor      │ │ ● frontend               │
│ Editing files            │ │ Waiting for input        │
│ feature/refunds · Opus…  │ │ main · up 3h · 42k tok   │
│ ─────────────────────────│ │ ─────────────────────────│
│ Updated auth.go          │ │ ❯ add a dark theme       │
│ ✻ Churned for 2m 15s     │ │ —                        │
│ ❯ now run the tests      │ │                          │
╰──────────────────────────╯ ╰──────────────────────────╯
╭──────────────────────────╮ ╭──────────────────────────╮
│ ● docs                   │ │ ○ ai-security            │
│ Running command          │ │ Stopped                  │
│ main · up 1h · 12k tok   │ │                          │
│ ─────────────────────────│ │ ─────────────────────────│
│ Writing summary...       │ │ —                        │
│ Executing grep...        │ │                          │
╰──────────────────────────╯ ╰──────────────────────────╯

  ↑↓←→ move   enter open   r refresh   F12 dashboard   q quit
```

Each card is one task. It shows the status Crabby can read reliably — **Working**
(with the current activity, when it can tell), **Waiting for input**, **Attached**,
**Idle** or **Stopped** — plus the branch, uptime, model and token count when a
transcript is available, and a short **live preview** of the session's screen.

The preview comes from `tmux capture-pane`, reduced to the last few lines that
actually carry content: Crabby strips the box art, blank lines and the Agent's
persistent status bar, so the card shows recent output and the current prompt
rather than the whole terminal. Mission Control is an **observation surface**, not
a terminal emulator — it never lets you type into multiple sessions at once.

It refreshes automatically and stays cheap: a session's preview is only
re-captured when that session has actually produced new output, so watching dozens
of tasks costs almost nothing. Move between cards with the arrows or **Tab**; press
**Enter** to attach to the highlighted task, and leaving it drops you back in
Mission Control.

## Commands

Crabby is TUI-first: everything you do day to day lives inside the app, so the
command line is intentionally tiny.

| Command | What it does |
| --- | --- |
| `crabby` | Open the application — the home screen and everything from there. |
| `crabby doctor` | Check that WSL, Ubuntu, tmux, Agent, and crabby are present. |
| `crabby version` | Print the installed version. |

Creating and importing workspaces, managing tasks, and managing packs all moved
into the interface — press **?** inside Crabby for the full key list.

## Packs

A **pack** is a reusable project setup — nothing more than a directory of files
that Crabby copies into a project when you create a workspace. No plugins, no
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

Press **n** to create a workspace and point it at a folder:

- **No packs installed** → Crabby writes a minimal default `CLAUDE.md`.
- **One pack** → it's used automatically.
- **Several packs** → Crabby shows a pack picker in the flow.

Files that already exist in the project are **never overwritten** — they're
reported and kept. Crabby always writes its own `.claude/crabby.yaml`
(the registry metadata) regardless of the pack.

### Managing packs

Press **p** (or **s** → Packs) to manage packs without leaving Crabby:

- **n** creates a new pack (a starter `pack.yaml` and `CLAUDE.md`).
- **c** duplicates the selected pack under a new name.
- **e** opens the pack directory in your `$EDITOR` (falling back to `vi`) so you
  can edit its `CLAUDE.md` and add any files you want copied in.
- **d** deletes a pack from disk (asks first).

Packs are still just directories under `~/.config/crabby/packs/`, so you can also
copy one of the bundled examples in [`examples/`](examples/) (`simple-pack`,
`go-pack`, `docs-pack`) and edit it by hand — but you never have to.

## Session states

| State | Meaning |
| --- | --- |
| 🟢 `Running` | You're attached — this is the session you're in. |
| 🟡 `Waiting` | Agent is alive in the background; press Enter to jump back in. |
| ⚪ `Stopped` | Nothing running; press Enter to start fresh. |

A live session that is actively producing output shows what it's doing (e.g.
`Working`, `Thinking…`, `Editing files`), so you can see at a glance which of
your Agent workers are busy. See [Workspace insights](#workspace-insights).

## Notifications

The dashboard is itself the notification surface: because it refreshes every
second, sessions that are working light up with `· working` and reorder to the
top, and a session that has just started producing output rings a short
terminal bell.

**Limitation, stated honestly:** Crabby cannot pop a desktop notification *while
you are inside a AI Agent session*. When you open a session, Crabby hands the
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
claude_command: claude   # default agent command (overridable per workspace / task)
tmux_binary: tmux        # tmux binary to use
detach_key: F12          # key that returns you to Crabby
```

Projects are recorded in `~/.local/share/crabby/projects.json`. Each entry
includes the agent command used for that workspace and for each individual task,
so the right agent is launched automatically on `Enter`.

If your keyboard sends F12 somewhere else, set `detach_key` to another
[tmux key name](https://man.openbsd.org/tmux#KEY_BINDINGS) such as `C-g`.

## Project layout

```
cmd/
    crabby/            # Linux (WSL) entrypoint
    crabby-windows/    # Windows wrapper -> crabby.exe
internal/
    cli/               # cobra: `crabby`, `version`, `doctor`
    initcmd/           # workspace initialization (+ pack application)
    importcmd/         # workspace discovery for import
    insights/          # reads Agent transcripts for dashboard metrics
    pack/              # local project packs (list + create/duplicate/delete)
    project/           # projects.json registry
    session/           # project <-> session mapping + state
    tmux/              # tmux driver (dedicated socket, F12 detach, status bar)
    tui/               # bubble tea application (dashboard, forms, packs, settings)
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
