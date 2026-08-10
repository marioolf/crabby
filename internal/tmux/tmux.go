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

// exact turns a session name into an exact-match target. Without the leading
// "=", tmux matches targets by prefix, so "crabby_test" would resolve to
// "crabby_test_docs" — opening the wrong task, or the wrong workspace when one
// name is a prefix of another.
func exact(session string) string { return "=" + session }

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

// isValidSessionName ensures the session name contains only safe characters.
func isValidSessionName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == ':') {
			return false
		}
	}
	return true
}

// HasSession reports whether a session with the given name exists.
func (c Client) HasSession(name string) bool {
	if !isValidSessionName(name) {
		return false
	}
	return exec.Command(c.Binary, c.args("has-session", "-t", exact(name))...).Run() == nil
}

// Attached reports whether a client is currently attached to the session.
//
// This uses display-message, whose -t is a target-pane and does not accept the
// "=" exact-match form, so callers must pass a full session name. The dashboard
// reads attachment from ListSessions instead, which keys on exact names.
func (c Client) Attached(name string) bool {
	if !isValidSessionName(name) {
		return false
	}
	out, err := c.output("display-message", "-p", "-t", name, "#{session_attached}")
	if err != nil {
		return false
	}
	return out != "0"
}

// Session is a snapshot of one tmux session's live state.
type Session struct {
	Attached bool
	Activity time.Time
	Created  time.Time // when the session was started, for uptime
}

// ListSessions returns every session on Crabby's server in a single call, so
// the dashboard can refresh cheaply regardless of how many projects exist. A
// missing server (no sessions) yields an empty map, not an error.
//
// Activity uses the window's activity timestamp, which — unlike
// session_activity — advances when a detached session produces output. That is
// what lets the dashboard show which background sessions are working.
func (c Client) ListSessions() map[string]Session {
	out, err := c.output("list-sessions", "-F",
		"#{session_name}\t#{session_attached}\t#{window_activity}\t#{session_created}")
	sessions := map[string]Session{}
	if err != nil {
		return sessions
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			continue
		}
		activity, _ := strconv.ParseInt(fields[2], 10, 64)
		created, _ := strconv.ParseInt(fields[3], 10, 64)
		sessions[fields[0]] = Session{
			Attached: fields[1] != "0",
			Activity: time.Unix(activity, 0),
			Created:  time.Unix(created, 0),
		}
	}
	return sessions
}

// Activity returns the time of the session's last activity.
func (c Client) Activity(name string) (time.Time, bool) {
	if !isValidSessionName(name) {
		return time.Time{}, false
	}
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
	if !isValidSessionName(name) {
		return fmt.Errorf("invalid session name: %q", name)
	}
	a := []string{"new-session", "-d", "-s", name, "-c", dir}
	if command != "" {
		a = append(a, command)
	}
	return c.run(a...)
}

// KillSession ends a session (and the program running in it). Used by the home
// screen's stop key so the user never has to touch tmux directly.
func (c Client) KillSession(name string) error {
	if !isValidSessionName(name) {
		return fmt.Errorf("invalid session name: %q", name)
	}
	return c.run("kill-session", "-t", exact(name))
}

// RenameWindow sets the session's window name, which Crabby uses to show the
// project name in the status bar instead of the running command.
func (c Client) RenameWindow(session, name string) error {
	if !isValidSessionName(session) {
		return fmt.Errorf("invalid session name: %q", session)
	}
	return c.run("rename-window", "-t", exact(session), name)
}

// AttachCmd builds the command that attaches to a session, wired to the real
// terminal. It does not run: the TUI wraps it for tea.Exec so Bubble Tea can
// release the screen, run the attach, and restore itself afterwards — a seamless
// step in and out of a AI Agent session without ever leaving Crabby. The session
// name is passed as a single argv element, so names with unusual characters are
// never word-split or interpreted by a shell.
func (c Client) AttachCmd(name string) *exec.Cmd {
	if !isValidSessionName(name) {
		return nil // Should not happen in normal flow, but defends against injection
	}
	cmd := exec.Command(c.Binary, c.args("attach-session", "-t", exact(name))...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// CapturePane returns the current visible content of a session's active pane,
// used by Mission Control for a live preview. The session name is passed plain
// (capture-pane's target-pane does not accept the "=" exact form); callers only
// capture sessions that exist, and tmux resolves an existing exact name before
// any prefix, so this cannot land on the wrong session. A missing session or any
// error yields "".
func (c Client) CapturePane(name string) string {
	if !isValidSessionName(name) {
		return ""
	}
	out, err := c.output("capture-pane", "-t", name, "-p")
	if err != nil {
		return ""
	}
	return out
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
		// Let the wheel scroll the pane's history instead of being forwarded to
		// AI Agent as arrow keys — otherwise scrolling up cycles through old
		// prompts rather than the response you're trying to read.
		{"set-option", "-g", "mouse", "on"},
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
