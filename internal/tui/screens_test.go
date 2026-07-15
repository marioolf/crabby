package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/marioolf/crabby/internal/config"
	"github.com/marioolf/crabby/internal/importcmd"
	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/pack"
	"github.com/marioolf/crabby/internal/session"
	"github.com/marioolf/crabby/internal/tmux"
)

// baseModel is a model with the services a View needs and a sane terminal size.
func baseModel() model {
	return model{
		cfg:          config.Default(),
		tmux:         tmux.New("tmux"),
		insights:     insights.New(),
		width:        120,
		height:       40,
		lastActivity: map[string]time.Time{},
		working:      map[string]bool{},
	}
}

// TestEveryScreenRenders makes sure no screen's View panics, whatever sub-state
// it is in — the failure mode most likely to slip past the compiler.
func TestEveryScreenRenders(t *testing.T) {
	variants := []struct {
		name  string
		setup func(m *model)
	}{
		{"dashboard-empty", func(m *model) { m.screen = scrDashboard }},
		{"mission-empty", func(m *model) { m.screen = scrMission }},
		{"mission-cards", func(m *model) {
			m.screen = scrMission
			m.mc.cards = []missionCard{
				{label: "payments", state: session.Waiting, working: true, branch: "main",
					insight: insights.Insight{Model: "Opus 4.8", SessionTokens: 132000, Activity: insights.Editing},
					preview: []string{"Updated auth.go", "Running tests..."}},
				{label: "docs", state: session.Stopped},
			}
		}},
		{"new-workspace-dir", func(m *model) {
			m.screen = scrNewWorkspace
			m.wsStep = wsStepDir
			m.input = newTextInput("/tmp/x")
		}},
		{"new-workspace-pack", func(m *model) {
			m.screen = scrNewWorkspace
			m.wsStep = wsStepPack
			m.wsDir = "/tmp/x"
			m.packList = []pack.Pack{{Name: "go", Description: "Go"}}
		}},
		{"import-scanning", func(m *model) {
			m.screen = scrImport
			m.importStep = importStepScanning
		}},
		{"import-pick", func(m *model) {
			m.screen = scrImport
			m.importStep = importStepPick
			m.importList = []importcmd.Workspace{{Name: "a", Branch: "main"}, {Name: "b", Imported: true}}
			m.importSel = []bool{true, false}
		}},
		{"new-task", func(m *model) {
			m.screen = scrNewTask
			m.input = newTextInput("")
		}},
		{"packs-empty", func(m *model) { m.screen = scrPacks }},
		{"packs-create", func(m *model) {
			m.screen = scrPacks
			m.packAction = packActionCreate
			m.input = newTextInput("new")
		}},
		{"packs-duplicate", func(m *model) {
			m.screen = scrPacks
			m.packs = []pack.Pack{{Name: "base"}}
			m.packAction = packActionDuplicate
			m.input = newTextInput("base-copy")
		}},
		{"packs-delete", func(m *model) {
			m.screen = scrPacks
			m.packs = []pack.Pack{{Name: "base"}}
			m.packAction = packActionConfirmDelete
		}},
		{"settings-menu", func(m *model) { m.screen = scrSettings }},
		{"settings-diagnostics", func(m *model) {
			m.screen = scrSettings
			m.settingsPane = settingsDiagnostics
		}},
		{"settings-about", func(m *model) {
			m.screen = scrSettings
			m.settingsPane = settingsAbout
		}},
		{"help", func(m *model) { m.screen = scrHelp }},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			m := baseModel()
			v.setup(&m)
			out := m.View()
			if strings.TrimSpace(out) == "" {
				t.Fatalf("%s rendered empty", v.name)
			}
		})
	}
}

// TestDashboardTransitions checks the keys that open each screen land on it.
func TestDashboardTransitions(t *testing.T) {
	cases := []struct {
		key  string
		want screen
	}{
		{"n", scrNewWorkspace},
		{"i", scrImport},
		{"p", scrPacks},
		{"s", scrSettings},
		{"?", scrHelp},
	}
	for _, c := range cases {
		m := baseModel()
		m.screen = scrDashboard
		next, _ := m.updateDashboard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(c.key)})
		got := next.(model).screen
		if got != c.want {
			t.Errorf("key %q -> screen %d, want %d", c.key, got, c.want)
		}
	}
}

func TestF12TogglesMissionControl(t *testing.T) {
	m := baseModel()
	m.screen = scrDashboard
	f12 := tea.KeyMsg{Type: tea.KeyF12}

	next, _ := m.handleKey(f12)
	if s := next.(model).screen; s != scrMission {
		t.Fatalf("F12 from dashboard -> screen %d, want mission", s)
	}
	back, _ := next.(model).handleKey(f12)
	if s := back.(model).screen; s != scrDashboard {
		t.Fatalf("F12 from mission -> screen %d, want dashboard", s)
	}
}
