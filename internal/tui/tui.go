// Package tui implements `crabby ps`: a small Bubble Tea list of projects and
// their session state.
//
// It does one thing: let the user pick a project and attach. No mouse, no
// tabs, no filters.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/session"
	"github.com/marioolf/crabby/internal/tmux"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	branchStyle   = lipgloss.NewStyle().Faint(true).PaddingLeft(4)
	helpStyle     = lipgloss.NewStyle().Faint(true).PaddingTop(1)

	runningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // green
	waitingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // amber
	stoppedStyle = lipgloss.NewStyle().Faint(true)
)

type item struct {
	project project.Project
	state   session.State
}

type model struct {
	tmux     tmux.Client
	items    []item
	cursor   int
	selected *project.Project // non-nil once the user chose to attach
	quit     bool
}

// Run displays the TUI and returns the project the user chose to attach to, or
// nil if they quit without choosing.
func Run(t tmux.Client) (*project.Project, error) {
	m := model{tmux: t}
	m.refresh()

	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	return final.(model).selected, nil
}

func (m *model) refresh() {
	projects, err := project.Load()
	if err != nil {
		projects = nil
	}
	items := make([]item, 0, len(projects))
	for _, p := range projects {
		items = append(items, item{project: p, state: session.Detect(m.tmux, p)})
	}
	m.items = items
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "q", "ctrl+c", "esc":
		m.quit = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "r":
		m.refresh()
	case "enter":
		if len(m.items) > 0 {
			p := m.items[m.cursor].project
			m.selected = &p
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🦀 Crabby"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 46))
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		b.WriteString("  No projects registered.\n")
		b.WriteString("  Run `crabby init` inside a project to get started.\n")
		b.WriteString(helpStyle.Render("q  Quit"))
		return b.String()
	}

	for i, it := range m.items {
		dot, dotStyle := stateGlyph(it.state)
		name := it.project.Name

		cursor := "  "
		if i == m.cursor {
			cursor = "❯ "
			name = selectedStyle.Render(name)
		}

		b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, dotStyle.Render(dot), name))
		b.WriteString(branchStyle.Render("state: "+string(it.state)) + "\n\n")
	}

	b.WriteString(strings.Repeat("─", 46))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓  Move    Enter  Attach    r  Refresh    q  Quit"))
	return b.String()
}

func stateGlyph(s session.State) (string, lipgloss.Style) {
	switch s {
	case session.Running:
		return "●", runningStyle
	case session.Waiting:
		return "●", waitingStyle
	default:
		return "○", stoppedStyle
	}
}
