// Package tmux wraps the small slice of tmux that Crabby needs.
//
// Crabby is not a tmux replacement: it only creates sessions, inspects them,
// and hands the terminal over via attach.
package tmux

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Client talks to a specific tmux binary.
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

// Available reports whether the tmux binary can be found on PATH.
func (c Client) Available() bool {
	_, err := exec.LookPath(c.Binary)
	return err == nil
}

// HasSession reports whether a session with the given name exists.
func (c Client) HasSession(name string) bool {
	cmd := exec.Command(c.Binary, "has-session", "-t", name)
	return cmd.Run() == nil
}

// Attached reports whether a client is currently attached to the session.
func (c Client) Attached(name string) bool {
	out, err := exec.Command(c.Binary,
		"display-message", "-p", "-t", name, "#{session_attached}").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != "0"
}

// NewSession creates a detached session named name, starting in dir and
// running the given command. If command is empty the session opens a shell.
func (c Client) NewSession(name, dir, command string) error {
	args := []string{"new-session", "-d", "-s", name, "-c", dir}
	if command != "" {
		args = append(args, command)
	}
	cmd := exec.Command(c.Binary, args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Attach replaces the current process with `tmux attach`, dropping the user
// directly into the session. It only returns if exec itself fails.
func (c Client) Attach(name string) error {
	bin, err := exec.LookPath(c.Binary)
	if err != nil {
		return err
	}
	return syscall.Exec(bin, []string{c.Binary, "attach-session", "-t", name}, os.Environ())
}
