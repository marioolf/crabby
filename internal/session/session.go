// Package session maps Crabby projects onto tmux sessions.
//
// Each project owns exactly one session named crabby_<project_name>.
package session

import (
	"strings"

	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/tmux"
)

// State describes what a project's session is currently doing.
type State string

const (
	// Running: the session exists and a client is attached to it.
	Running State = "Running"
	// Waiting: the session exists but nobody is attached (idle, ready).
	Waiting State = "Waiting"
	// Stopped: no session exists for the project.
	Stopped State = "Stopped"
)

func sanitize(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Name returns the tmux session name for a workspace's default task. Kept as
// "crabby_<workspace>" so single-task workspaces match their pre-tasks name.
func Name(projectName string) string {
	return "crabby_" + sanitize(projectName)
}

// TaskSession builds the tmux session name for a named task, e.g.
// "crabby_payments_tests". Users never type these — Crabby manages them.
func TaskSession(workspace, task string) string {
	return "crabby_" + sanitize(workspace) + "_" + sanitize(task)
}

// Classify maps a session's presence and attachment into a State. Used by the
// dashboard, which fetches all sessions in one batch call.
func Classify(present, attached bool) State {
	switch {
	case !present:
		return Stopped
	case attached:
		return Running
	default:
		return Waiting
	}
}

// Rank orders states by importance for sorting (Running first).
func Rank(s State) int {
	switch s {
	case Running:
		return 0
	case Waiting:
		return 1
	default:
		return 2
	}
}

// Detect returns the current state of a project's session.
func Detect(t tmux.Client, p project.Project) State {
	name := sessionName(p)
	if !t.HasSession(name) {
		return Stopped
	}
	if t.Attached(name) {
		return Running
	}
	return Waiting
}

// sessionName returns the stored session name, falling back to the derived one
// for older registry entries.
func sessionName(p project.Project) string {
	if p.Session != "" {
		return p.Session
	}
	return Name(p.Name)
}
