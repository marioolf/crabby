# Agents in Crabby

Crabby was born around Claude Code. From v0.10 that assumption is gone from the
core: Crabby manages **agents**, and Claude Code is simply the first one. This
document explains the model and how to add another agent later.

> Status: Claude Code is the only functional agent today. The architecture is
> ready for more (Codex, Gemini, OpenCode, …); those adapters are intentionally
> not written yet.

## The model

Crabby has four concepts, layered:

```
Workspace          a registered project folder
   └── Task        a unit of work in that folder (e.g. refactor-auth)
         └── Agent the tool that does the work (e.g. Claude Code)
               └── Session the live instance of that agent, in tmux
```

- A **Workspace** is a folder Crabby knows about (the registry unit, a
  `project.Project`).
- A **Task** is an independent piece of work inside a workspace
  (`project.Task`). A workspace can hold several. **A task is not "a Claude
  window"** — it records *which agent* runs it via a stable ID (`agent: claude`).
- An **Agent** is the terminal AI that performs a task (`agent.Agent`). It is
  the seam between Crabby and the tool it drives.
- A **Session** is the running instance of an agent for a task
  (`agent.Session`), backed by one tmux session on Crabby's dedicated socket.

The key inversion from earlier versions: a task no longer *is* Claude; a task
*uses* an agent, and the agent has a session.

## What an Agent is

`internal/agent/agent.go` defines the interface. It abstracts **only what
actually differs between tools** — deliberately small:

```go
type Agent interface {
    ID() string        // stable internal slug, persisted with a task ("claude")
    Name() string      // human-facing label ("Claude Code")
    Command() string   // the executable Crabby launches
    Available() bool    // is that command runnable (on PATH)?
}
```

Two rules the whole design rests on:

1. **ID is not Name.** The ID is a stable slug stored in the registry
   (`claude`) and never shown; the Name is display text (`Claude Code`) and may
   change freely. Never persist the display name.
2. **Only abstract what varies.** Creating the tmux session, attaching to it,
   reading its status and stopping it are *identical* for every agent — the only
   difference is the command launched. So that lifecycle is **not** part of the
   interface; it lives once in the shared runtime.

## What a Session is

`internal/agent/session.go` holds the shared lifecycle. A `Session` couples an
`Agent` with the tmux session backing one task:

```go
type Session struct {
    Agent Agent
    Tmux  Tmux   // the small slice of tmux the runtime needs; tmux.Client fits
    Name  string // tmux session name
    Dir   string // working directory
    Label string // window label
}

func (s Session) Start() error         // create + label if not already up
func (s Session) AttachCmd() *exec.Cmd // command to attach (run via tea.Exec)
func (s Session) Stop() error          // end the session
func (s Session) Running() bool
func (s Session) State() State         // Running / Waiting / Stopped
```

`Start()` launches `Agent.Command()` in a fresh tmux session, or does nothing if
it is already running; it returns `ErrUnavailable` when the agent's command
cannot be found, so Crabby refuses gracefully rather than opening a broken
session. `Tmux` is an interface, so the runtime is decoupled from the tmux
package and trivial to fake in tests.

## The registry

`agent.Registry` is the set of agents Crabby knows about, built once at startup
(see `tui.defaultRegistry`). A task's stored ID is resolved through it:

```go
r := agent.NewRegistry(claude.New(cfg.ClaudeCommand))
ag := r.Lookup(task.Agent) // empty or unknown -> the default agent
```

`Lookup` is the **compatibility hinge**: a task written before agents existed
carries no ID and resolves to the default (`agent.DefaultID`, `"claude"`). This
is why old workspaces keep working with no migration.

## The Claude Code adapter

`internal/agent/claude/claude.go` is the reference adapter, and it is thin:

```go
package claude

const ID = "claude"
const Name = "Claude Code"

func New(command string) agent.Agent { /* binds the configured command */ }

func (a Agent) ID() string      { return ID }
func (a Agent) Name() string    { return Name }
func (a Agent) Command() string { return a.command }
func (a Agent) Available() bool { /* exec.LookPath(a.command) */ }
```

That is the whole adapter. Everything about *running* Claude Code is handled by
the shared `agent.Session` runtime, parameterised by `Command()`.

## Adding a new agent (e.g. Codex)

When the time comes to add another terminal agent, the recipe is:

1. **Create the adapter package**, mirroring `claude`:

   ```
   internal/agent/
       agent.go
       session.go
       claude/claude.go
       codex/codex.go   ← new
   ```

   ```go
   package codex

   const ID = "codex"
   const Name = "Codex"

   func New(command string) agent.Agent { /* default command "codex" */ }
   // ID/Name/Command/Available, exactly like the Claude adapter.
   ```

2. **Register it** in `tui.defaultRegistry`:

   ```go
   func defaultRegistry(cfg config.Config) *agent.Registry {
       return agent.NewRegistry(
           claude.New(cfg.ClaudeCommand),
           codex.New(cfg.CodexCommand), // add a config key for its command
       )
   }
   ```

3. **Let tasks choose it.** Persist `agent: codex` on the task (extend the
   new-task flow with an agent picker) — the field already exists and the
   registry already resolves it.

That is all. No change to the session lifecycle, the dashboard, Mission Control,
or persistence: they all speak in terms of `agent.Agent` and `agent.Session`,
never Claude specifically.

## What an adapter must *not* do

- Do not re-implement session creation, attach, or status — that is the
  runtime's job, shared by every agent.
- Do not store or compare on the display name; use the ID.
- Do not assume a particular UI. An agent is a command plus an identity; how its
  output is previewed (Mission Control) is Crabby's concern, not the adapter's.
