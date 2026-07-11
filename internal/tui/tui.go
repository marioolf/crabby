// Package tui implements Crabby's home screen and pack selector.
//
// The home screen is the application: pick a project to open it in Claude,
// leave the session to come straight back, start or stop sessions, and add new
// projects — all without touching tmux.
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/marioolf/crabby/internal/pack"
	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/session"
	"github.com/marioolf/crabby/internal/tmux"
)

const (
	crab  = lipgloss.Color("209") // Crabby's orange
	width = 46
)

var (
	crabRowStyle  = lipgloss.NewStyle().Foreground(crab)
	logoStyle     = lipgloss.NewStyle().Bold(true).Foreground(crab)
	subtitleStyle = lipgloss.NewStyle().Faint(true)
	dividerStyle  = lipgloss.NewStyle().Faint(true)
	metaStyle     = lipgloss.NewStyle().Faint(true)
	helpStyle     = lipgloss.NewStyle().Faint(true)
	nameStyle     = lipgloss.NewStyle()
	selNameStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231"))
	barStyle      = lipgloss.NewStyle().Foreground(crab)
	confirmStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))

	stateStyles = map[session.State]lipgloss.Style{
		session.Running: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),
		session.Waiting: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")),
		session.Stopped: lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("244")),
	}

	centered = lipgloss.NewStyle().Width(width).Align(lipgloss.Center)
)

// header is the compact "crab army" banner shown at the top of the home screen.
func header() string {
	var b strings.Builder
	b.WriteString(centered.Render(crabRowStyle.Render("🦀   🦀   🦀")))
	b.WriteString("\n")
	b.WriteString(centered.Render(logoStyle.Render("C R A B B Y")))
	b.WriteString("\n")
	b.WriteString(centered.Render(subtitleStyle.Render("managing your Claude army")))
	return b.String()
}

func divider() string {
	return dividerStyle.Render(strings.Repeat("─", width))
}

// Action is what the user chose to do on the home screen.
type Action int

const (
	ActionQuit Action = iota
	ActionOpen
	ActionNewProject
)

// Result is returned from Run.
type Result struct {
	Action  Action
	Project *project.Project // set when Action == ActionOpen
}

type item struct {
	project  project.Project
	state    session.State
	branch   string
	activity time.Time
	hasAct   bool
}

type model struct {
	tmux       tmux.Client
	detachKey  string
	items      []item
	cursor     int
	confirming bool // showing "stop this session?" prompt
	result     Result
}

// Run displays the home screen and returns the chosen action.
func Run(t tmux.Client, detachKey string) (Result, error) {
	m := model{tmux: t, detachKey: detachKey, result: Result{Action: ActionQuit}}
	m.refresh()

	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return Result{Action: ActionQuit}, err
	}
	return final.(model).result, nil
}

func (m *model) refresh() {
	projects, err := project.Load()
	if err != nil {
		projects = nil
	}
	items := make([]item, 0, len(projects))
	for _, p := range projects {
		it := item{project: p, state: session.Detect(m.tmux, p), branch: p.Branch()}
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

	// Confirmation prompt for stopping a session captures all keys.
	if m.confirming {
		switch key.String() {
		case "y", "Y":
			_ = m.tmux.KillSession(m.items[m.cursor].project.Session)
			m.confirming = false
			m.refresh()
		default:
			m.confirming = false
		}
		return m, nil
	}

	switch key.String() {
	case "q", "ctrl+c", "esc":
		m.result = Result{Action: ActionQuit}
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
	case "n":
		m.result = Result{Action: ActionNewProject}
		return m, tea.Quit
	case "x":
		if len(m.items) > 0 && m.items[m.cursor].state != session.Stopped {
			m.confirming = true
		}
	case "enter":
		if len(m.items) > 0 {
			p := m.items[m.cursor].project
			m.result = Result{Action: ActionOpen, Project: &p}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(header())
	b.WriteString("\n\n")
	b.WriteString(divider())
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		b.WriteString(metaStyle.Render("  No projects yet."))
		b.WriteString("\n")
		b.WriteString(metaStyle.Render("  Press n, or run `crabby init` inside a project."))
		b.WriteString("\n\n")
		b.WriteString(divider())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("n new project   q quit"))
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

		b.WriteString(fmt.Sprintf("%s%s %s   %s\n", bar, dotStyle.Render(dot), name, stateLabel))

		meta := []string{it.project.Path}
		if it.branch != "" {
			meta = append(meta, "⎇ "+it.branch)
		}
		if it.hasAct {
			meta = append(meta, "active "+humanize(it.activity))
		}
		b.WriteString("    ")
		b.WriteString(metaStyle.Render(strings.Join(meta, "   ")))
		b.WriteString("\n\n")
	}

	b.WriteString(divider())
	b.WriteString("\n")
	if m.confirming {
		b.WriteString(confirmStyle.Render(fmt.Sprintf("Stop \"%s\"? (y/n)", m.items[m.cursor].project.Name)))
		return b.String()
	}
	b.WriteString(helpStyle.Render("enter open   n new   x stop   r refresh   q quit"))
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

// --- Pack selector ---------------------------------------------------------

type packModel struct {
	packs  []pack.Pack
	cursor int
	chosen *pack.Pack
}

// SelectPack shows a list of packs and returns the chosen one, or nil if the
// user cancels.
func SelectPack(packs []pack.Pack) (*pack.Pack, error) {
	final, err := tea.NewProgram(packModel{packs: packs}, tea.WithAltScreen()).Run()
	if err != nil {
		return nil, err
	}
	return final.(packModel).chosen, nil
}

func (m packModel) Init() tea.Cmd { return nil }

func (m packModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		if m.cursor < len(m.packs)-1 {
			m.cursor++
		}
	case "enter":
		p := m.packs[m.cursor]
		m.chosen = &p
		return m, tea.Quit
	}
	return m, nil
}

func (m packModel) View() string {
	var b strings.Builder
	b.WriteString(logoStyle.Render("Select a pack"))
	b.WriteString("\n\n")
	for i, p := range m.packs {
		bar := "  "
		name := nameStyle.Render(p.Name)
		if i == m.cursor {
			bar = barStyle.Render("▌ ")
			name = selNameStyle.Render(p.Name)
		}
		b.WriteString(bar + name + "\n")
		if p.Description != "" {
			b.WriteString("    " + metaStyle.Render(p.Description) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move   enter select   q cancel"))
	return b.String()
}
