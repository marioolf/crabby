package agent_test

import (
	"os/exec"
	"testing"

	"github.com/marioolf/crabby/internal/agent"
	"github.com/marioolf/crabby/internal/agent/claude"
)

// fakeAgent is a controllable Agent for exercising the runtime.
type fakeAgent struct {
	id, name, cmd string
	available     bool
}

func (f fakeAgent) ID() string      { return f.id }
func (f fakeAgent) Name() string    { return f.name }
func (f fakeAgent) Command() string { return f.cmd }
func (f fakeAgent) Available() bool { return f.available }

// fakeTmux records calls and pretends to hold a set of sessions.
type fakeTmux struct {
	sessions map[string]bool
	attached map[string]bool
	started  []string
	renamed  map[string]string
	killed   []string
}

func newFakeTmux() *fakeTmux {
	return &fakeTmux{
		sessions: map[string]bool{},
		attached: map[string]bool{},
		renamed:  map[string]string{},
	}
}

func (f *fakeTmux) HasSession(name string) bool { return f.sessions[name] }
func (f *fakeTmux) Attached(name string) bool   { return f.attached[name] }
func (f *fakeTmux) NewSession(name, dir, command string) error {
	f.sessions[name] = true
	f.started = append(f.started, command)
	return nil
}
func (f *fakeTmux) RenameWindow(session, name string) error { f.renamed[session] = name; return nil }
func (f *fakeTmux) AttachCmd(name string) *exec.Cmd         { return exec.Command("true", name) }
func (f *fakeTmux) KillSession(name string) error {
	delete(f.sessions, name)
	f.killed = append(f.killed, name)
	return nil
}

func TestRegistryLookupAndDefault(t *testing.T) {
	c := claude.New("claude")
	other := fakeAgent{id: "codex", name: "Codex", cmd: "codex", available: true}
	r := agent.NewRegistry(c, other)

	if got := r.Lookup("codex"); got.ID() != "codex" {
		t.Errorf("Lookup(codex) = %q, want codex", got.ID())
	}
	// Empty and unknown IDs fall back to the default (claude) — the compatibility
	// hinge for tasks written before agents existed.
	if got := r.Lookup(""); got.ID() != agent.DefaultID {
		t.Errorf("Lookup(\"\") = %q, want %q", got.ID(), agent.DefaultID)
	}
	if got := r.Lookup("nope"); got.ID() != agent.DefaultID {
		t.Errorf("Lookup(nope) = %q, want default", got.ID())
	}
	if r.Default().ID() != agent.DefaultID {
		t.Errorf("Default = %q, want %q", r.Default().ID(), agent.DefaultID)
	}
	if len(r.All()) != 2 {
		t.Errorf("All() = %d agents, want 2", len(r.All()))
	}
}

func TestClaudeIdentity(t *testing.T) {
	a := claude.New("")
	if a.ID() != "claude" {
		t.Errorf("ID = %q, want claude", a.ID())
	}
	if a.Name() != "Claude Code" {
		t.Errorf("Name = %q, want Claude Code", a.Name())
	}
	if a.Command() != "claude" {
		t.Errorf("empty command should default to claude, got %q", a.Command())
	}
}

func TestSessionLifecycle(t *testing.T) {
	ft := newFakeTmux()
	sess := agent.Session{
		Agent: fakeAgent{id: "claude", name: "Claude Code", cmd: "claude", available: true},
		Tmux:  ft,
		Name:  "crabby_demo",
		Dir:   "/repo",
		Label: "demo",
	}

	if sess.State() != agent.Stopped {
		t.Fatalf("fresh session state = %q, want Stopped", sess.State())
	}
	if err := sess.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !sess.Running() {
		t.Fatal("session should be running after Start")
	}
	if len(ft.started) != 1 || ft.started[0] != "claude" {
		t.Fatalf("Start should launch the agent command, got %v", ft.started)
	}
	if ft.renamed["crabby_demo"] != "demo" {
		t.Errorf("window not labelled, got %q", ft.renamed["crabby_demo"])
	}
	if sess.State() != agent.Waiting {
		t.Errorf("running-but-detached state = %q, want Waiting", sess.State())
	}
	ft.attached["crabby_demo"] = true
	if sess.State() != agent.Running {
		t.Errorf("attached state = %q, want Running", sess.State())
	}

	// Starting again is a no-op — no second launch.
	if err := sess.Start(); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	if len(ft.started) != 1 {
		t.Errorf("second Start relaunched the agent: %v", ft.started)
	}

	if err := sess.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if sess.Running() {
		t.Error("session should be stopped after Stop")
	}
}

func TestSessionStartUnavailable(t *testing.T) {
	ft := newFakeTmux()
	sess := agent.Session{
		Agent: fakeAgent{id: "claude", cmd: "nope", available: false},
		Tmux:  ft,
		Name:  "crabby_x",
	}
	if err := sess.Start(); err != agent.ErrUnavailable {
		t.Fatalf("Start with missing command = %v, want ErrUnavailable", err)
	}
	if ft.HasSession("crabby_x") {
		t.Error("no session should be created when the agent is unavailable")
	}
}
