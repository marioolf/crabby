package tmux

import (
	"strings"
	"testing"
	"time"
)

// These tests exercise a real tmux on the dedicated "crabby" socket. They are
// skipped automatically where tmux is unavailable (e.g. CI without tmux).
func TestSessionLifecycleAndConfigure(t *testing.T) {
	c := New("tmux")
	if !c.Available() {
		t.Skip("tmux not installed")
	}

	const name = "crabby_itest_v02"
	_ = c.run("kill-session", "-t", name) // clean slate
	t.Cleanup(func() { _ = c.run("kill-session", "-t", name) })

	if err := c.NewSession(name, "/tmp", "sleep 60"); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if !c.HasSession(name) {
		t.Fatal("HasSession = false after NewSession")
	}
	if c.Attached(name) {
		t.Fatal("Attached = true for a detached session")
	}
	if ts, ok := c.Activity(name); !ok || time.Since(ts) > time.Minute {
		t.Fatalf("Activity = (%v, %v), want a recent time", ts, ok)
	}

	if err := c.Configure("F12"); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	// The prefix-free F12 -> detach-client binding must exist.
	keys, err := c.output("list-keys", "-T", "root")
	if err != nil {
		t.Fatalf("list-keys: %v", err)
	}
	if !strings.Contains(keys, "F12") || !strings.Contains(keys, "detach-client") {
		t.Fatalf("F12 detach binding not found in root key table:\n%s", keys)
	}

	// The status bar must be branded, not default tmux.
	right, err := c.output("show-options", "-g", "-v", "status-right")
	if err != nil {
		t.Fatalf("show-options status-right: %v", err)
	}
	if !strings.Contains(right, "return to crabby") || !strings.Contains(right, "F12") {
		t.Fatalf("status-right not branded: %q", right)
	}
}
