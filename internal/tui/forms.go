package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/marioolf/crabby/internal/initcmd"
	"github.com/marioolf/crabby/internal/pack"
	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/session"
)

// wsStep is which step of the new-workspace flow is on screen.
type wsStep int

const (
	wsStepDir wsStep = iota
	wsStepPack
)

// --- New workspace ---------------------------------------------------------

func (m model) startNewWorkspace() (tea.Model, tea.Cmd) {
	cwd, _ := os.Getwd()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepDir
	m.input = newTextInput(cwd)
	m.notice = ""
	return m, nil
}

func (m model) updateNewWorkspace(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.wsStep {
	case wsStepDir:
		switch msg.String() {
		case "esc":
			m.goDashboard()
		case "enter":
			dir := strings.TrimSpace(m.input.Value())
			if dir == "" {
				return m, nil
			}
			m.wsDir = expandHome(dir)
			packs, _ := pack.List()
			switch len(packs) {
			case 0:
				return m.createWorkspace(nil)
			case 1:
				p := packs[0]
				return m.createWorkspace(&p)
			default:
				m.packList = packs
				m.packCursor = 0
				m.wsStep = wsStepPack
			}
		default:
			m.input = m.input.update(msg)
		}
	case wsStepPack:
		switch msg.String() {
		case "esc":
			m.wsStep = wsStepDir
		case "up", "k":
			if m.packCursor > 0 {
				m.packCursor--
			}
		case "down", "j":
			if m.packCursor < len(m.packList) { // last index is the "No pack" choice
				m.packCursor++
			}
		case "enter":
			if m.packCursor >= len(m.packList) {
				return m.createWorkspace(nil)
			}
			p := m.packList[m.packCursor]
			return m.createWorkspace(&p)
		}
	}
	return m, nil
}

// createWorkspace runs init for the captured directory and pack, returning to
// the dashboard with the new workspace selected. On failure it keeps the flow
// open so the path can be corrected.
func (m model) createWorkspace(p *pack.Pack) (tea.Model, tea.Cmd) {
	res, err := initcmd.Init(m.wsDir, p)
	if err != nil {
		m.notice, m.noticeErr = err.Error(), true
		m.wsStep = wsStepDir
		return m, nil
	}
	m.goDashboard()
	m.refresh()
	m.selectByName(res.Project.Name)
	label := res.Project.Name
	if res.PackID != "" {
		label += fmt.Sprintf(" (pack %q)", res.PackID)
	}
	m.notice, m.noticeErr = "Created workspace "+label, false
	return m, nil
}

func (m model) viewNewWorkspace() string {
	switch m.wsStep {
	case wsStepPack:
		var b strings.Builder
		b.WriteString(metaStyle.Render(m.wsDir) + "\n\n")
		b.WriteString("Choose a pack:\n\n")
		for i, p := range m.packList {
			b.WriteString(m.selectLine(i == m.packCursor, p.Name))
			if p.Description != "" {
				b.WriteString("  " + metaStyle.Render(p.Description))
			}
			b.WriteString("\n")
		}
		b.WriteString(m.selectLine(m.packCursor >= len(m.packList), "No pack — default CLAUDE.md"))
		footer := actionKey("↑↓", "move") + "   " + actionKey("enter", "create") + "   " + actionKey("esc", "back")
		return m.frame("New workspace", b.String(), footer)
	default:
		body := "Directory to initialize:\n\n  " + m.input.view("path to a project folder")
		footer := actionKey("enter", "continue") + "   " + actionKey("esc", "cancel")
		return m.frame("New workspace", body, footer)
	}
}

// selectLine renders one selectable row with the accent bar when chosen.
func (m model) selectLine(selected bool, label string) string {
	if selected {
		return barStyle.Render("▌ ") + selNameStyle.Render(label)
	}
	return "  " + nameStyle.Render(label)
}

// --- New task --------------------------------------------------------------

func (m model) startNewTask(p project.Project) (tea.Model, tea.Cmd) {
	m.screen = scrNewTask
	m.formProj = p
	m.input = newTextInput("")
	m.notice = ""
	return m, nil
}

func (m model) updateNewTask(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.goDashboard()
	case "enter":
		name := strings.TrimSpace(m.input.Value())
		if name == "" {
			return m, nil
		}
		tk, err := createTask(m.formProj, name)
		if err != nil {
			m.notice, m.noticeErr = err.Error(), true
			return m, nil
		}
		proj := m.formProj
		m.goDashboard()
		m.refresh()
		m.selectByName(proj.Name)
		// Open the new task straight away — a parallel session without leaving Crabby.
		return m, func() tea.Msg { return openTaskMsg{proj, tk} }
	default:
		m.input = m.input.update(msg)
	}
	return m, nil
}

func (m model) viewNewTask() string {
	body := fmt.Sprintf("New task in %q.\n\nTask name:\n\n  ", m.formProj.Name) + m.input.view("e.g. tests, docs, refactor")
	footer := actionKey("enter", "create & open") + "   " + actionKey("esc", "cancel")
	return m.frame("New task", body, footer)
}

// --- Shared helpers --------------------------------------------------------

// createTask validates a name, builds its session, and registers it in the
// workspace. It does not start the session — the caller opens it.
func createTask(p project.Project, name string) (project.Task, error) {
	if err := validateTaskName(name); err != nil {
		return project.Task{}, err
	}
	tk := project.Task{
		Name:    name,
		Session: session.TaskSession(p.Name, name),
		Created: time.Now(),
	}
	if err := project.AddTask(p.Name, tk); err != nil {
		return project.Task{}, fmt.Errorf("creating task %q: %w", name, err)
	}
	return tk, nil
}

// validateTaskName keeps task names safe for tmux session names and readable in
// the tree.
func validateTaskName(name string) error {
	if name == "" {
		return fmt.Errorf("a task name is required")
	}
	for _, r := range name {
		ok := r == '-' || r == '_' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return fmt.Errorf("task names may only contain letters, digits, '-' and '_' (got %q)", name)
		}
	}
	return nil
}

// selectByName moves the workspace cursor onto the named workspace, if present.
func (m *model) selectByName(name string) {
	for i, r := range m.rows {
		if r.proj.Name == name {
			m.wsCursor = i
			m.taskCursor = 0
			return
		}
	}
}

// expandHome turns a leading ~ into the user's home directory and makes the path
// absolute, so a relative path typed on the dashboard resolves as expected.
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
