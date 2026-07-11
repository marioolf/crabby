Crabby - Technical Design Document (MVP v0.1)
Overview

Crabby is a lightweight workspace manager for Claude Code.

It has two goals only:

Prepare projects so they are immediately ready to work with Claude Code.
Manage multiple Claude Code sessions from a single terminal.

Crabby is not:

a Git client
a tmux replacement
a project manager
an IDE
an AI orchestrator

It should stay intentionally small.

The entire philosophy is:

"Make working with multiple Claude Code sessions effortless."

Primary User

A developer working on multiple repositories simultaneously.

Typical workflow:

Start a new project
Initialize it for Claude
Launch Claude
Switch between multiple Claude sessions throughout the day
Never worry about where each session is running
Core Principles
Simplicity

Every feature must solve a real daily pain.

No speculative features.

Opinionated

Crabby has opinions.

It uses:

WSL
tmux
Claude Code

No configuration required.

Single implementation

Crabby only has one real implementation:

Linux (inside WSL).

Windows simply forwards commands into WSL.

This keeps the codebase simple.

Architecture
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

There is never a native Windows implementation.

Windows only launches WSL.

Supported environments

MVP:

Windows 11
WSL2
Ubuntu
tmux
Claude Code

Nothing else.

Commands
crabby init

Run inside a project.

Example:

crabby init

Responsibilities:

Create .claude/
Create CLAUDE.md
Create project metadata
Register project
Generated structure
project/

    .claude/
        crabby.yaml

    CLAUDE.md

Nothing more.

Keep it minimal.

CLAUDE.md

Initial content:

# Project

Describe the project.

# Architecture

Describe the architecture.

# Coding conventions

Describe conventions.

# Important notes

Write important information here.

Very small.

Users will customize it.

crabby ps

Main entrypoint.

Launches a Bubble Tea TUI.

Example:

┌────────────────────────────────────────────┐
│ Crabby                                    │
├────────────────────────────────────────────┤

● payments

    branch: feature/refunds
    state: Running

● frontend

    branch: main
    state: Waiting

○ docs

    state: Stopped

─────────────────────────────────────────────

Enter  Attach

q      Quit

r      Refresh

└────────────────────────────────────────────┘

Selecting a project and pressing Enter executes:

crabby attach <project>
crabby attach
crabby attach payments

Responsibilities:

Find tmux session
Attach user to Claude immediately

User must land directly inside Claude.

No intermediate shell.

crabby doctor

Checks:

✓ WSL installed

✓ Ubuntu available

✓ tmux installed

✓ Claude installed

✓ crabby installed

Useful for installation.

Project Registration

Each initialized project becomes registered.

Database location:

~/.local/share/crabby/projects.json

Example:

[
  {
    "name": "payments",
    "path": "/home/mario/work/payments",
    "session": "payments"
  }
]

No SQLite.

JSON is enough.

Session model

Each project owns exactly one Claude session.

Session name:

crabby_<project_name>

Example:

crabby_payments
Starting Claude

Future command:

crabby start

For MVP this is optional.

If missing:

User simply executes:

claude

inside the tmux session.

Detecting sessions

Crabby should inspect tmux.

If:

crabby_payments

exists

↓

Running

Otherwise

↓

Stopped

Windows wrapper

Windows executable responsibilities:

Detect PowerShell
Convert current path into WSL path
Forward command

Equivalent to:

wsl crabby ...

If command needs current directory:

wsl bash -lc "cd <converted_path> && crabby ..."

Use wslpath.

TUI

Library:

Bubble Tea

Requirements:

Arrow navigation
Enter
q
r

Nothing else.

No mouse.

No tabs.

No filters.

Configuration

Global configuration:

~/.config/crabby/config.yaml

Example:

claude_command: claude

tmux_binary: tmux

No more settings.

Logging

None.

Errors only.

Error handling

Example:

Project not initialized.

Run:

crabby init

Example:

tmux not installed.

Example:

Claude Code not found.
Internal structure
cmd/

    crabby/

internal/

    cli/

    init/

    project/

    session/

    tmux/

    tui/

    doctor/

    config/

    windows/

pkg/

Keep everything internal.

Dependencies

Required:

cobra

bubbletea

lipgloss

Only if truly necessary.

Prefer standard library.

Out of scope

Do NOT implement:

Git integration
Worktrees
GitHub
Pull requests
VSCode
Multiple Claude sessions per project
Plugins
Hooks
Templates
Profiles
Snapshots
Memory
AI workflows
Context generation
Skills
MCP management

These belong to future versions.

Future roadmap

v0.2

crabby start
Automatic Claude launch
Session creation

v0.3

Project templates
Better CLAUDE.md
Team templates

v0.4

Skills management
Reusable project presets
Acceptance criteria

The MVP is considered complete when the following workflow works:

Open PowerShell.
Navigate to any project.
Run:
crabby init
Crabby initializes the project.
Open Claude.
Repeat for multiple projects.
Run:
crabby ps
Bubble Tea displays every project.
Select one.
Press Enter.
User is immediately attached to the correct Claude Code session.
Detach.
Return to crabby ps.
Switch to another session.

If this works smoothly, the MVP is complete.

Una sugerencia final

Hay una única cosa que sí añadiría al MVP porque creo que es muy barata de implementar y aporta mucho valor:

crabby start

En lugar de hacer:

crabby init
tmux new -s proyecto
claude

Simplemente:

crabby start

Y que haga automáticamente:

Cree la sesión tmux si no existe.
Entre en el directorio del proyecto.
Lance claude.
Deje la sesión preparada.
Si la sesión ya existe, simplemente haga attach.

Con eso, el flujo completo quedaría reducido a solo tres comandos:

crabby init
crabby start
crabby ps

Creo que esa experiencia es mucho más redonda y sigue siendo perfectamente compatible con el objetivo de mantener Crabby pequeño, simple y muy enfocado.