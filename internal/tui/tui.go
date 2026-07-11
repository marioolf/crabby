// Package tui implements Crabby's home screen: a small Bubble Tea list of
// projects and their session state. It is the application — selecting a project
// launches Claude, and leaving Claude returns straight here.
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/session"
	"github.com/marioolf/crabby/internal/tmux"
)

const crab = lipgloss.Color("209") // Crabby's orange

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(crab)
	subtitleStyle = lipgloss.NewStyle().Faint(true)
	metaStyle     = lipgloss.NewStyle().Faint(true)
	helpStyle     = lipgloss.NewStyle().Faint(true)
	nameStyle     = lipgloss.NewStyle()
	selNameStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231"))
	barStyle      = lipgloss.NewStyle().Foreground(crab)

	stateStyles = map[session.State]lipgloss.Style{
		session.Running: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),  // green
		session.Waiting: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")), // amber
		session.Stopped: lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("244")),
	}
)

type item struct {
	project  project.Project
	state    session.State
	branch   string
	activity time.Time
	hasAct   bool
}

type model struct {
	tmux      tmux.Client
	detachKey string
	items     []item
	cursor    int
	selected  *project.Project
}

// Run displays the home screen and returns the project the user chose to open,
// or nil if they quit.
func Run(t tmux.Client, detachKey string) (*project.Project, error) {
	m := model{tmux: t, detachKey: detachKey}
	m.refresh()

	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
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
		it := item{
			project: p,
			state:   session.Detect(m.tmux, p),
			branch:  p.Branch(),
		}
		if it.state != session.Stopped {
			it.activity, it.hasAct = m.tmux.Activity(p.Session)
		}
		items = append(items, it)
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
	b.WriteString("  ")
	b.WriteString(subtitleStyle.Render("workspace manager for Claude Code"))
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		b.WriteString(metaStyle.Render("  No projects yet."))
		b.WriteString("\n")
		b.WriteString(metaStyle.Render("  Run `crabby init` inside a project to add it."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("  q  quit"))
		return b.String()
	}

	for i, it := range m.items {
		selected := i == m.cursor

		bar := "  "
		name := nameStyle.Render(it.project.Name)
		if selected {
			bar = barStyle.Render("▌ ")
			name = selNameStyle.Render(it.project.Name)
		}
		dot, dotStyle := glyph(it.state)
		stateLabel := stateStyles[it.state].Render(string(it.state))

		b.WriteString(fmt.Sprintf("%s%s %s   %s\n",
			bar, dotStyle.Render(dot), name, stateLabel))

		// Second line: quiet metadata.
		meta := []string{it.project.Path}
		if it.branch != "" {
			meta = append(meta, "⎇ "+it.branch)
		}
		if it.hasAct && it.state != session.Stopped {
			meta = append(meta, "active "+humanize(it.activity))
		}
		b.WriteString("    ")
		b.WriteString(metaStyle.Render(strings.Join(meta, "   ")))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("↑/↓ move   enter open   r refresh   q quit"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("inside a session, press %s to return here", m.detachKey)))
	return b.String()
}

func glyph(s session.State) (string, lipgloss.Style) {
	if s == session.Stopped {
		return "○", stateStyles[s]
	}
	return "●", stateStyles[s]
}

func humanize(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
