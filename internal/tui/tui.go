// Package tui implements Crabby's dashboard and pack selector.
//
// The dashboard is the application: a live, auto-refreshing view of every
// project and its Claude session, ordered by importance. Selecting a project
// opens it in Claude; leaving the session returns straight here.
package tui

import (
	"fmt"
	"os"
	"sort"
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
	crab        = lipgloss.Color("209") // Crabby's orange
	width       = 48
	refreshEach = time.Second
)

var (
	logoStyle     = lipgloss.NewStyle().Foreground(crab)
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(crab)
	subtitleStyle = lipgloss.NewStyle().Faint(true)
	dividerStyle  = lipgloss.NewStyle().Faint(true)
	metaStyle     = lipgloss.NewStyle().Faint(true)
	helpStyle     = lipgloss.NewStyle().Faint(true)
	nameStyle     = lipgloss.NewStyle()
	selNameStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231"))
	barStyle      = lipgloss.NewStyle().Foreground(crab)
	confirmStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	workingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	stateStyles = map[session.State]lipgloss.Style{
		session.Running: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),  // green
		session.Waiting: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")), // yellow
		session.Stopped: lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("244")),
	}

	centered = lipgloss.NewStyle().Width(width).Align(lipgloss.Center)
)

// logo is the compact "crab army" banner.
func logo() string {
	rows := []string{
		logoStyle.Render("  _~_      _~_      _~_"),
		logoStyle.Render("__(o )>  __(o )>  __(o )>"),
		"",
		titleStyle.Render("C R A B B Y"),
		subtitleStyle.Render("manage your Claude army"),
	}
	var b strings.Builder
	for i, r := range rows {
		b.WriteString(centered.Render(r))
		if i < len(rows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func divider() string { return dividerStyle.Render(strings.Repeat("─", width)) }

// Action is what the user chose on the dashboard.
type Action int

const (
	ActionQuit Action = iota
	ActionOpen
	ActionNewProject
)

// Result is returned from Run.
type Result struct {
	Action  Action
	Project *project.Project
}

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmStop
	confirmRemove
)

type item struct {
	project project.Project
	state   session.State
	branch  string
	working bool // producing output right now
}

type tickMsg time.Time

type model struct {
	tmux      tmux.Client
	detachKey string
	items     []item
	cursor    int
	confirm   confirmKind
	result    Result

	// activity/working tracking, persisted across refreshes
	lastActivity map[string]time.Time
	working      map[string]bool
	ringBell     bool
	firstLoad    bool
}

// Run displays the dashboard and returns the chosen action.
func Run(t tmux.Client, detachKey string) (Result, error) {
	m := model{
		tmux:         t,
		detachKey:    detachKey,
		result:       Result{Action: ActionQuit},
		lastActivity: map[string]time.Time{},
		working:      map[string]bool{},
		firstLoad:    true,
	}
	m.refresh()

	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return Result{Action: ActionQuit}, err
	}
	return final.(model).result, nil
}

// refresh reloads projects and session state in a single tmux call, tracks
// which sessions are actively producing output, and flags a bell on a rising
// edge (a session that just started working while you're on the dashboard).
func (m *model) refresh() {
	projects, err := project.Load()
	if err != nil {
		projects = nil
	}
	sessions := m.tmux.ListSessions()

	items := make([]item, 0, len(projects))
	risingEdge := false
	for _, p := range projects {
		info, present := sessions[p.Session]
		it := item{
			project: p,
			state:   session.Classify(present, info.Attached),
			branch:  p.Branch(),
		}
		if present {
			prev, seen := m.lastActivity[p.Session]
			it.working = seen && info.Activity.After(prev)
			if it.working && !m.working[p.Session] && !m.firstLoad {
				risingEdge = true
			}
			m.lastActivity[p.Session] = info.Activity
			m.working[p.Session] = it.working
		} else {
			delete(m.lastActivity, p.Session)
			delete(m.working, p.Session)
		}
		items = append(items, it)
	}

	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := session.Rank(items[i].state), session.Rank(items[j].state)
		if ri != rj {
			return ri < rj
		}
		return items[i].project.Name < items[j].project.Name
	})

	m.items = items
	m.ringBell = risingEdge
	m.firstLoad = false
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func tick() tea.Cmd {
	return tea.Tick(refreshEach, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// bellCmd rings the terminal bell out-of-band (BEL does not disturb the screen).
func bellCmd() tea.Msg {
	fmt.Fprint(os.Stderr, "\a")
	return nil
}

func (m model) Init() tea.Cmd { return tick() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.refresh()
		if m.ringBell {
			return m, tea.Batch(tick(), bellCmd)
		}
		return m, tick()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirm != confirmNone {
		if s := key.String(); s == "y" || s == "Y" {
			p := m.items[m.cursor].project
			switch m.confirm {
			case confirmStop:
				_ = m.tmux.KillSession(p.Session)
			case confirmRemove:
				_ = m.tmux.KillSession(p.Session)
				_ = project.Remove(p.Name)
			}
			m.refresh()
		}
		m.confirm = confirmNone
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
			m.confirm = confirmStop
		}
	case "d":
		if len(m.items) > 0 {
			m.confirm = confirmRemove
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
	b.WriteString(logo())
	b.WriteString("\n\n")
	b.WriteString(divider())
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		b.WriteString("  No projects yet.\n\n")
		b.WriteString(metaStyle.Render("  Run  crabby init  inside a project,\n"))
		b.WriteString(metaStyle.Render("  or press  n  to add the current directory.\n\n"))
		b.WriteString(divider())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("n new project    q quit"))
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
		dot, dotStyle := glyph(it.state, it.working)
		b.WriteString(fmt.Sprintf("%s%s %s\n", bar, dotStyle.Render(dot), name))
		if it.branch != "" {
			b.WriteString("    " + metaStyle.Render(it.branch) + "\n")
		}
		b.WriteString("    " + stateLabel(it.state, it.working) + "\n\n")
	}

	b.WriteString(divider())
	b.WriteString("\n")
	switch m.confirm {
	case confirmStop:
		b.WriteString(confirmStyle.Render(fmt.Sprintf("Stop \"%s\"? (y/n)", m.items[m.cursor].project.Name)))
		return b.String()
	case confirmRemove:
		b.WriteString(confirmStyle.Render(fmt.Sprintf("Remove \"%s\" from Crabby? Files are kept. (y/n)", m.items[m.cursor].project.Name)))
		return b.String()
	}
	b.WriteString(helpStyle.Render("enter open   n new   x stop   d remove"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("r refresh   q quit   ·   %s returns from a session", m.detachKey)))
	return b.String()
}

// glyph picks the status symbol and colour. A working session pulses green.
func glyph(s session.State, working bool) (string, lipgloss.Style) {
	if s == session.Stopped {
		return "○", stateStyles[s]
	}
	if working {
		return "●", workingStyle
	}
	return "●", stateStyles[s]
}

func stateLabel(s session.State, working bool) string {
	label := stateStyles[s].Render(string(s))
	if working {
		return label + workingStyle.Render(" · working")
	}
	return label
}

// --- Pack selector ---------------------------------------------------------

type packModel struct {
	packs  []pack.Pack
	cursor int
	chosen *pack.Pack
}

// SelectPack shows the available packs and returns the chosen one, or nil if
// the user cancels.
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
	b.WriteString(titleStyle.Render("Select a pack"))
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
	b.WriteString(metaStyle.Render("packs live in " + pack.DisplayDir()))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move   enter select   q cancel"))
	return b.String()
}
