// Package tmux wraps the small slice of tmux that Crabby needs.
//
// tmux is an implementation detail: Crabby drives it so the user never has to.
// Everything runs on a dedicated tmux server (socket "crabby") so Crabby's key
// bindings and status bar never touch the user's own tmux.
package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// socket is the dedicated tmux server Crabby owns.
const socket = "crabby"

// Client talks to Crabby's tmux server.
type Client struct {
	Binary string
}

// New returns a Client for the given tmux binary (usually just "tmux").
func New(binary string) Client {
	if binary == "" {
		binary = "tmux"
	}
	return Client{Binary: binary}
}

// args prefixes the dedicated socket to every tmux invocation.
func (c Client) args(extra ...string) []string {
	return append([]string{"-L", socket}, extra...)
}

func (c Client) run(extra ...string) error {
	cmd := exec.Command(c.Binary, c.args(extra...)...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c Client) output(extra ...string) (string, error) {
	out, err := exec.Command(c.Binary, c.args(extra...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// Available reports whether the tmux binary can be found on PATH.
func (c Client) Available() bool {
	_, err := exec.LookPath(c.Binary)
	return err == nil
}

// HasSession reports whether a session with the given name exists.
func (c Client) HasSession(name string) bool {
	return exec.Command(c.Binary, c.args("has-session", "-t", name)...).Run() == nil
}

// Attached reports whether a client is currently attached to the session.
func (c Client) Attached(name string) bool {
	out, err := c.output("display-message", "-p", "-t", name, "#{session_attached}")
	if err != nil {
		return false
	}
	return out != "0"
}

// Activity returns the time of the session's last activity.
func (c Client) Activity(name string) (time.Time, bool) {
	out, err := c.output("display-message", "-p", "-t", name, "#{session_activity}")
	if err != nil {
		return time.Time{}, false
	}
	sec, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(sec, 0), true
}

// NewSession creates a detached session named name, starting in dir and running
// the given command (empty opens a shell).
func (c Client) NewSession(name, dir, command string) error {
	a := []string{"new-session", "-d", "-s", name, "-c", dir}
	if command != "" {
		a = append(a, command)
	}
	return c.run(a...)
}

// RenameWindow sets the session's window name, which Crabby uses to show the
// project name in the status bar instead of the running command.
func (c Client) RenameWindow(session, name string) error {
	return c.run("rename-window", "-t", session, name)
}

// AttachChild attaches to the session as a child process, inheriting the
// terminal. It blocks until the user leaves the session (detaches or Claude
// exits) and then returns — which is what lets Crabby take back control.
func (c Client) AttachChild(name string) error {
	cmd := exec.Command(c.Binary, c.args("attach-session", "-t", name)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Configure makes the session feel like part of Crabby rather than tmux:
//   - a single, prefix-free key (detachKey) returns to Crabby;
//   - a small, Crabby-branded status bar shows how to get back.
//
// Options are global to Crabby's server and safe to re-apply on every attach.
func (c Client) Configure(detachKey string) error {
	if detachKey == "" {
		detachKey = "F12"
	}
	steps := [][]string{
		{"bind-key", "-n", detachKey, "detach-client"},
		// Keep the window name Crabby sets (the project) rather than letting the
		// running command rename it.
		{"set-option", "-g", "allow-rename", "off"},
		{"set-option", "-g", "automatic-rename", "off"},
		{"set-option", "-g", "status", "on"},
		{"set-option", "-g", "status-justify", "centre"},
		{"set-option", "-g", "status-style", "bg=colour235,fg=colour250"},
		{"set-option", "-g", "status-left-length", "40"},
		{"set-option", "-g", "status-right-length", "50"},
		{"set-option", "-g", "status-left", "#[fg=colour209,bold] 🦀 crabby #[default]"},
		{"set-option", "-g", "status-right",
			fmt.Sprintf("#[fg=colour209,bold] %s #[fg=colour250,nobold]return to crabby ", detachKey)},
		{"set-option", "-g", "window-status-current-format", "#[fg=colour252,bold]#W"},
		{"set-option", "-g", "window-status-format", "#[fg=colour244]#W"},
	}
	for _, s := range steps {
		if err := c.run(s...); err != nil {
			return err
		}
	}
	return nil
}
