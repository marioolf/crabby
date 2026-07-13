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

// TestExactTargetsAvoidPrefixMatching guards the multi-task bug: a task's
// session name is a prefix of another's (crabby_x vs crabby_x_task), and tmux
// matches targets by prefix unless forced exact. HasSession/KillSession must
// treat them as distinct so opening one task never lands on another.
func TestExactTargetsAvoidPrefixMatching(t *testing.T) {
	c := New("tmux")
	if !c.Available() {
		t.Skip("tmux not installed")
	}

	const base = "crabby_itest_prefix"
	const sub = base + "_docs"
	for _, n := range []string{base, sub} {
		_ = c.run("kill-session", "-t", exact(n))
	}
	t.Cleanup(func() {
		for _, n := range []string{base, sub} {
			_ = c.run("kill-session", "-t", exact(n))
		}
	})

	// Only the longer session exists; the shorter name must NOT match it.
	if err := c.NewSession(sub, "/tmp", "sleep 60"); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if c.HasSession(base) {
		t.Fatalf("HasSession(%q) matched %q by prefix", base, sub)
	}

	// Create the prefix session too; killing it must leave the other alive.
	if err := c.NewSession(base, "/tmp", "sleep 60"); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if err := c.KillSession(base); err != nil {
		t.Fatalf("KillSession: %v", err)
	}
	if !c.HasSession(sub) {
		t.Fatalf("KillSession(%q) also killed %q", base, sub)
	}
	if c.HasSession(base) {
		t.Fatalf("KillSession(%q) did not remove it", base)
	}
}
