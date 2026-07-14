package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/marioolf/crabby/internal/config"
	"github.com/marioolf/crabby/internal/doctor"
	"github.com/marioolf/crabby/internal/pack"
	"github.com/marioolf/crabby/internal/version"
)

// settingsPane is which part of Settings is on screen: the menu, or one of the
// leaf views it opens.
type settingsPane int

const (
	settingsMenu settingsPane = iota
	settingsDiagnostics
	settingsAbout
)

// settingsItems is the menu, intentionally small for now.
var settingsItems = []string{"Packs", "Diagnostics", "About"}

func (m model) startSettings() (tea.Model, tea.Cmd) {
	m.screen = scrSettings
	m.settingsPane = settingsMenu
	m.settingsCursor = 0
	m.notice = ""
	return m, nil
}

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settingsPane != settingsMenu {
		// The leaf views only need a way back.
		if s := msg.String(); s == "esc" || s == "q" || s == "enter" {
			m.settingsPane = settingsMenu
		}
		return m, nil
	}

	switch msg.String() {
	case "esc", "q":
		m.goDashboard()
	case "up", "k":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case "down", "j":
		if m.settingsCursor < len(settingsItems)-1 {
			m.settingsCursor++
		}
	case "enter":
		switch settingsItems[m.settingsCursor] {
		case "Packs":
			return m.startPacks()
		case "Diagnostics":
			m.checks = doctor.Run(m.cfg)
			m.settingsPane = settingsDiagnostics
		case "About":
			m.settingsPane = settingsAbout
		}
	}
	return m, nil
}

func (m model) viewSettings() string {
	switch m.settingsPane {
	case settingsDiagnostics:
		return m.frame("Diagnostics", m.diagnosticsBody(), actionKey("esc", "back"))
	case settingsAbout:
		return m.frame("About", m.aboutBody(), actionKey("esc", "back"))
	default:
		var b strings.Builder
		for i, it := range settingsItems {
			b.WriteString(m.selectLine(i == m.settingsCursor, it) + "\n")
		}
		footer := actionKey("↑↓", "move") + "   " + actionKey("enter", "open") + "   " + actionKey("esc", "back")
		return m.frame("Settings", b.String(), footer)
	}
}

func (m model) diagnosticsBody() string {
	var b strings.Builder
	for _, c := range m.checks {
		mark := okStyle.Render("✓")
		if !c.OK {
			mark = errorStyle.Render("✗")
		}
		b.WriteString(mark + " " + c.Name)
		if c.Note != "" {
			b.WriteString("  " + metaStyle.Render("("+c.Note+")"))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m model) aboutBody() string {
	cfg := m.cfg
	def := config.Default()
	lines := []string{
		fmt.Sprintf("%s v%s", version.App, version.String()),
		metaStyle.Render(version.Tagline),
		"",
		metaStyle.Render("by " + version.Author + " · " + version.Repo),
		"",
		metaStyle.Render("claude command   ") + valueOrDefault(cfg.ClaudeCommand, def.ClaudeCommand),
		metaStyle.Render("tmux binary      ") + valueOrDefault(cfg.TmuxBinary, def.TmuxBinary),
		metaStyle.Render("return key       ") + cfg.DetachKey,
		metaStyle.Render("packs directory  ") + pack.DisplayDir(),
	}
	return strings.Join(lines, "\n")
}

// valueOrDefault labels a config value, marking the built-in default so the
// About view shows what is configured versus what is implied.
func valueOrDefault(v, def string) string {
	if v == def {
		return v + metaStyle.Render("  (default)")
	}
	return v
}
