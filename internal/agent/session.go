package agent

import (
	"errors"
	"os/exec"
)

// ErrUnavailable is returned when a session is asked to start but its agent's
// command cannot be found, so the caller can report it clearly instead of
// opening a broken session.
var ErrUnavailable = errors.New("agent command not available")

// State is the live status of an agent session, derived from tmux. It is the
// per-session lifecycle view; the dashboard classifies many sessions at once
// through its own batch path.
type State string

const (
	// Running: the session exists and a client is attached.
	Running State = "Running"
	// Waiting: the session exists but nobody is attached — idle, ready.
	Waiting State = "Waiting"
	// Stopped: no session exists.
	Stopped State = "Stopped"
)

// Tmux is the slice of tmux the agent runtime needs. Crabby's tmux.Client
// satisfies it structurally, so the runtime stays decoupled from that package
// and is trivial to fake in tests.
type Tmux interface {
	HasSession(name string) bool
	Attached(name string) bool
	NewSession(name, dir, command string) error
	RenameWindow(session, name string) error
	AttachCmd(name string) *exec.Cmd
	KillSession(name string) error
}

// Session couples an Agent with the tmux session that backs one task: the live
// instance of the agent doing that task's work. The lifecycle here is identical
// for every agent — only the command differs — which is exactly why it lives in
// one place rather than in each adapter.
type Session struct {
	Agent Agent
	Tmux  Tmux
	Name  string // tmux session name
	Dir   string // working directory the agent runs in
	Label string // window label shown in the status bar
}

// Running reports whether the session currently exists.
func (s Session) Running() bool { return s.Tmux.HasSession(s.Name) }

// State returns the session's live status.
func (s Session) State() State {
	if !s.Tmux.HasSession(s.Name) {
		return Stopped
	}
	if s.Tmux.Attached(s.Name) {
		return Running
	}
	return Waiting
}

// Start creates the session running the agent's command, if it is not already
// up, and labels its window. It returns ErrUnavailable if the agent's command
// cannot be found. Starting an already-running session is a no-op.
func (s Session) Start() error {
	if s.Tmux.HasSession(s.Name) {
		return nil
	}
	if !s.Agent.Available() {
		return ErrUnavailable
	}
	if err := s.Tmux.NewSession(s.Name, s.Dir, s.Agent.Command()); err != nil {
		return err
	}
	if s.Label != "" {
		_ = s.Tmux.RenameWindow(s.Name, s.Label)
	}
	return nil
}

// AttachCmd returns the command that attaches to the session, for the caller to
// run (Crabby hands it to Bubble Tea's Exec so the screen is released and
// restored around it).
func (s Session) AttachCmd() *exec.Cmd { return s.Tmux.AttachCmd(s.Name) }

// Stop ends the session and the agent running in it.
func (s Session) Stop() error { return s.Tmux.KillSession(s.Name) }
