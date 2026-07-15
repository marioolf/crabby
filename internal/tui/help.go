package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "?", "enter":
		m.goDashboard()
	}
	return m, nil
}

// helpEntry is one documented key and what it does.
type helpEntry struct{ key, desc string }

// helpSection groups related keys under a heading.
type helpSection struct {
	title   string
	entries []helpEntry
}

var helpSections = []helpSection{
	{"Navigation", []helpEntry{
		{"↑ ↓ / j k", "move within the focused column"},
		{"tab / →", "focus the Tasks column"},
		{"shift+tab / ←", "focus the Workspaces column"},
		{"esc", "step back / clear a message"},
	}},
	{"Workspaces & tasks", []helpEntry{
		{"enter", "open the selected task in Claude"},
		{"n", "new workspace (point it at a folder)"},
		{"i", "import workspaces found under a folder"},
		{"t", "new task in the selected workspace"},
		{"o", "open the workspace's folder in the file manager"},
		{"x", "stop the selected task's session"},
		{"d", "remove the selected task or workspace"},
		{"r", "refresh now"},
	}},
	{"Elsewhere", []helpEntry{
		{"F12", "Mission Control — live view of every task"},
		{"p", "manage packs (new / duplicate / edit / delete)"},
		{"s", "settings — packs, diagnostics, about"},
		{"?", "this help"},
		{"q", "quit Crabby"},
	}},
}

func (m model) viewHelp() string {
	var b strings.Builder
	// Lay keys out in an aligned two-column block per section.
	width := 0
	for _, s := range helpSections {
		for _, e := range s.entries {
			if w := lipgloss.Width(e.key); w > width {
				width = w
			}
		}
	}
	for i, s := range helpSections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(headerStyle.Render(s.title) + "\n")
		for _, e := range s.entries {
			pad := strings.Repeat(" ", width-lipgloss.Width(e.key)+2)
			b.WriteString("  " + keyStyle.Render(e.key) + pad + helpStyle.Render(e.desc) + "\n")
		}
	}
	body := strings.TrimRight(b.String(), "\n") + "\n\n" +
		metaStyle.Render("Inside a Claude session, press "+m.cfg.DetachKey+" to return here.")
	return m.frame("Help", body, actionKey("esc", "back"))
}
