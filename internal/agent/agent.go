// Package agent is Crabby's agent abstraction: the seam between Crabby and the
// terminal AI it drives.
//
// Historically Crabby was built around Claude Code. From v0.10 that coupling is
// gone: Crabby talks to an Agent — a tool that does the work inside a task's
// session — and Claude Code is simply the first (and, for now, only)
// implementation. New agents (Codex, Gemini, OpenCode, …) can be added later by
// writing another Agent adapter, without touching Crabby's core.
//
// An Agent captures only what actually varies between tools: a stable internal
// ID, a human-facing name, the command Crabby launches, and whether that command
// is available. Everything else — creating the tmux session, attaching, watching
// its status — is identical across agents and lives in the runtime (see
// session.go), parameterised by the agent's command.
package agent

// DefaultID is the agent assumed when a task does not name one — every existing
// task, and every new one for now. It is the stable internal identifier for
// Claude Code. Note the split the whole package rests on: the *ID* is a stable
// slug ("claude"), never the display *Name* ("Claude Code").
const DefaultID = "claude"

// Agent is a terminal AI Crabby can run inside a task's session. It abstracts
// only the operations that genuinely differ between tools; the session
// lifecycle is shared and lives in the runtime.
type Agent interface {
	// ID is the stable internal identifier persisted with a task, e.g.
	// "claude". It never changes and is never shown to the user.
	ID() string
	// Name is the human-facing label shown in the interface, e.g. "Claude
	// Code".
	Name() string
	// Command is the executable Crabby launches to start a session for this
	// agent.
	Command() string
	// Available reports whether the agent's command can actually be run
	// (resolvable on PATH), so Crabby can refuse gracefully rather than open a
	// broken session.
	Available() bool
}

// Registry is the set of agents Crabby knows about. It is built once at startup
// from configuration and consulted whenever a task needs its agent resolved.
type Registry struct {
	byID  map[string]Agent
	order []string
}

// NewRegistry builds a registry from the given agents, in order. The first
// agent is treated as the default when a task names an unknown or empty agent —
// so an installation always resolves to something runnable.
func NewRegistry(agents ...Agent) *Registry {
	r := &Registry{byID: make(map[string]Agent, len(agents))}
	for _, a := range agents {
		if _, seen := r.byID[a.ID()]; seen {
			continue
		}
		r.byID[a.ID()] = a
		r.order = append(r.order, a.ID())
	}
	return r
}

// Get returns the agent with the given ID.
func (r *Registry) Get(id string) (Agent, bool) {
	a, ok := r.byID[id]
	return a, ok
}

// Lookup resolves a task's stored agent ID to a runnable agent, falling back to
// the default for an empty or unknown ID. This is the compatibility hinge:
// tasks written before agents existed carry no ID and resolve to the default.
func (r *Registry) Lookup(id string) Agent {
	if a, ok := r.byID[id]; ok {
		return a
	}
	return r.Default()
}

// Default is the fallback agent — the first registered, or the "claude" entry
// if present. A registry is never expected to be empty in practice.
func (r *Registry) Default() Agent {
	if a, ok := r.byID[DefaultID]; ok {
		return a
	}
	if len(r.order) > 0 {
		return r.byID[r.order[0]]
	}
	return nil
}

// All returns every registered agent in registration order.
func (r *Registry) All() []Agent {
	out := make([]Agent, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.byID[id])
	}
	return out
}
