package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/marioolf/crabby/internal/config"
	"github.com/marioolf/crabby/internal/initcmd"
	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/tmux"
)

type TestHarness struct {
	Tm      *teatest.TestModel
	HomeDir string
	DataDir string
	stopped bool
}

func newTestModelHarness(t *testing.T) *TestHarness {
	t.Helper()

	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_DATA_HOME", tempDir)

	// Isolate PATH so that teatest flows do not detect host agent binaries
	// (like claude) and launch live host tmux sessions during testing.
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err == nil {
		if tmuxPath, err := exec.LookPath("tmux"); err == nil {
			_ = os.Symlink(tmuxPath, filepath.Join(binDir, "tmux"))
		}
		t.Setenv("PATH", binDir)
	}

	dataDir := filepath.Join(tempDir, ".local", "share", "crabby")
	configDir := filepath.Join(tempDir, ".config", "crabby")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("failed to create isolated data dir: %v", err)
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create isolated config dir: %v", err)
	}

	cfg := config.Default()
	cfg.ClaudeCommand = "missing-claude-cmd"
	tClient := tmux.New("tmux")
	coll := insights.New()

	m := model{
		cfg:          cfg,
		tmux:         tClient,
		insights:     coll,
		width:        120,
		height:       40,
		lastActivity: map[string]time.Time{},
		working:      map[string]bool{},
		firstLoad:    true,
	}
	m.refresh()

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})

	h := &TestHarness{
		Tm:      tm,
		HomeDir: tempDir,
		DataDir: dataDir,
	}
	// Stop the model before t.Setenv restores HOME and PATH. This prevents a
	// late refresh from writing to the real user's registry after a failed test.
	t.Cleanup(func() { h.stop(t) })
	return h
}

func (h *TestHarness) stop(t *testing.T) {
	t.Helper()
	if h.stopped {
		return
	}
	h.stopped = true
	h.Tm.Quit()
	h.Tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func (h *TestHarness) Finish(t *testing.T) model {
	t.Helper()
	h.stop(t)
	finalM, ok := h.Tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second)).(model)
	if !ok {
		return model{}
	}
	return finalM
}

func sendKeyRune(tm *teatest.TestModel, r rune) {
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
}

func sendEnter(tm *teatest.TestModel) {
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
}

func sendEsc(tm *teatest.TestModel) {
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
}

func sendTab(tm *teatest.TestModel) {
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
}

func sendShiftTab(tm *teatest.TestModel) {
	tm.Send(tea.KeyMsg{Type: tea.KeyShiftTab})
}

func sendF12(tm *teatest.TestModel) {
	tm.Send(tea.KeyMsg{Type: tea.KeyF12})
}

func sendSpace(tm *teatest.TestModel) {
	tm.Send(tea.KeyMsg{Type: tea.KeySpace})
}

func sendUp(tm *teatest.TestModel) {
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})
}

func sendDown(tm *teatest.TestModel) {
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
}

func setupTestWorkspace(t *testing.T, dirPath string) project.Project {
	t.Helper()
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("failed to create test workspace dir %s: %v", dirPath, err)
	}
	res, err := initcmd.Init(dirPath, nil, "")
	if err != nil {
		t.Fatalf("failed to init workspace at %s: %v", dirPath, err)
	}
	return res.Project
}

func setupMockAgentBinary(t *testing.T, homeDir string, name string) {
	t.Helper()
	binDir := filepath.Join(homeDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("failed to create binDir: %v", err)
	}
	agentPath := filepath.Join(binDir, name)
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(agentPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write mock agent script %s: %v", agentPath, err)
	}
}

func waitForOutput(t *testing.T, tm *teatest.TestModel, substr string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return strings.Contains(string(out), substr)
	}, teatest.WithDuration(2*time.Second))
}

func TestTeatestHarnessSanity(t *testing.T) {
	h := newTestModelHarness(t)
	waitForOutput(t, h.Tm, "CRABBY")
	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected screen scrDashboard, got %v", finalM.screen)
	}
}

func TestTeatest_GoroutineLeakVerification(t *testing.T) {
	time.Sleep(1500 * time.Millisecond)
	runtime.GC()
	initialGoroutines := runtime.NumGoroutine()

	const iterations = 2
	for i := 0; i < iterations; i++ {
		h := newTestModelHarness(t)
		waitForOutput(t, h.Tm, "CRABBY")
		sendKeyRune(h.Tm, '?')
		waitForOutput(t, h.Tm, "Help")
		sendEsc(h.Tm)
		h.Finish(t)
	}

	// Wait 2.5 seconds (longer than 1s refreshEach tick) and run GC
	time.Sleep(2500 * time.Millisecond)
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	buf := make([]byte, 64*1024)
	n := runtime.Stack(buf, true)
	t.Logf("REMAINING GOROUTINES AFTER 2.5s (%d vs initial %d):\n%s", finalGoroutines, initialGoroutines, string(buf[:n]))

	// Each teatest.NewTestModel creates 1 background signal listener (iterations = 2).
	if finalGoroutines > initialGoroutines+iterations+1 {
		t.Fatalf("possible goroutine leak detected: started with %d goroutines, ended with %d", initialGoroutines, finalGoroutines)
	}
}
