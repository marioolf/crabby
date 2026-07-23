// Package claude is Crabby's Claude Code agent adapter — the first and, for now,
// only implementation of agent.Agent.
//
// It is deliberately thin: Claude Code is a terminal program Crabby launches by
// its command, so the adapter carries the command and reports its identity and
// availability. Everything about running it inside a tmux session is handled by
// the shared runtime in the agent package. A second agent (Codex, Gemini, …)
// would be a sibling package that looks just like this one.
package claude

import (
	"os/exec"

	"github.com/marioolf/crabby/internal/agent"
)

// ID is Claude Code's stable internal identifier, persisted with a task. It is
// intentionally the short slug, not the display name.
const ID = "claude"

// Name is how Claude Code is shown in the interface.
const Name = "Claude Code"

// Agent is the Claude Code adapter. It is bound to the command Crabby is
// configured to launch (usually "claude").
type Agent struct {
	command string
}

// New returns a Claude Code agent that launches the given command. An empty
// command falls back to "claude" so the adapter is always runnable.
func New(command string) agent.Agent {
	if command == "" {
		command = "claude"
	}
	return Agent{command: command}
}

func (a Agent) ID() string      { return ID }
func (a Agent) Name() string    { return Name }
func (a Agent) Command() string { return a.command }

// Available reports whether the configured command can be found on PATH.
func (a Agent) Available() bool {
	_, err := exec.LookPath(a.command)
	return err == nil
}
