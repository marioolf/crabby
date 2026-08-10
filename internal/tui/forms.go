package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	wsStepAgent
)

func getInstalledAgents() []AgentItem {
	candidates := []AgentItem{
		{Name: "Claude Code", Command: "claude", AutoApproveFlag: "--permission-mode auto"},
		{Name: "Antigravity CLI", Command: "agy", AutoApproveFlag: "--dangerously-skip-permissions"},
		{Name: "Aider", Command: "aider", AutoApproveFlag: "--yes-always"},
		{Name: "OpenCode", Command: "opencode", AutoApproveFlag: "--auto"},
	}
	var installed []AgentItem
	for _, c := range candidates {
		if _, err := exec.LookPath(c.Command); err == nil {
			installed = append(installed, c)
		}
	}
	return installed
}

// --- New workspace ---------------------------------------------------------

func (m model) startNewWorkspace() (tea.Model, tea.Cmd) {
	cwd, _ := os.Getwd()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepDir
	m.input = newTextInput(cwd)
	_, m.wsMatches = dirCandidates(cwd)
	m.notice = ""
	return m, nil
}

func (m model) updateNewWorkspace(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.wsStep {
	case wsStepDir:
		switch msg.String() {
		case "esc":
			m.goDashboard()
		case "tab":
			// Complete the typed path to the matching sub-directory, shell-style.
			m.input = newTextInput(completePath(m.input.Value()))
			_, m.wsMatches = dirCandidates(m.input.Value())
		case "ctrl+o":
			// Optional native folder picker (WSL → Windows dialog).
			m.notice, m.noticeErr = "Opening the Windows folder picker…", false
			return m, pickWindowsFolder
		case "enter":
			dir := strings.TrimSpace(m.input.Value())
			if dir == "" {
				return m, nil
			}
			m.wsDir = expandHome(dir)
			packs, _ := pack.List()
			switch len(packs) {
			case 0:
				return m.nextAfterPack(nil)
			case 1:
				p := packs[0]
				return m.nextAfterPack(&p)
			default:
				m.packList = packs
				m.packCursor = 0
				m.wsStep = wsStepPack
			}
		default:
			m.input = m.input.update(msg)
			_, m.wsMatches = dirCandidates(m.input.Value())
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
			var p *pack.Pack
			if m.packCursor < len(m.packList) {
				p = &m.packList[m.packCursor]
			}
			return m.nextAfterPack(p)
		}
	case wsStepAgent:
		switch msg.String() {
		case " ":
			m.agentAutoApprove = !m.agentAutoApprove
		case "esc":
			if len(m.packList) > 1 {
				m.wsStep = wsStepPack
			} else {
				m.wsStep = wsStepDir
			}
		case "up", "k":
			if m.agentCursor > 0 {
				m.agentCursor--
			}
		case "down", "j":
			if m.agentCursor < len(m.agentList) {
				m.agentCursor++
			}
		case "enter":
			agentCmd := ""
			if m.agentCursor < len(m.agentList) {
				agentCmd = m.agentList[m.agentCursor].Command
				if m.agentAutoApprove && m.agentList[m.agentCursor].AutoApproveFlag != "" {
					agentCmd += " " + m.agentList[m.agentCursor].AutoApproveFlag
				}
			}
			return m.createWorkspace(m.wsPack, agentCmd)
		}
	}
	return m, nil
}

// createWorkspace runs init for the captured directory and pack, returning to
// the dashboard with the new workspace selected. On failure it keeps the flow
// open so the path can be corrected.
func (m model) nextAfterPack(p *pack.Pack) (tea.Model, tea.Cmd) {
	m.wsPack = p
	agents := getInstalledAgents()
	if len(agents) == 0 {
		return m.createWorkspace(p, "")
	}
	m.agentList = agents
	m.agentCursor = 0
	m.agentAutoApprove = false
	m.wsStep = wsStepAgent
	return m, nil
}

func (m model) createWorkspace(p *pack.Pack, agentCmd string) (tea.Model, tea.Cmd) {
	res, err := initcmd.Init(m.wsDir, p, agentCmd)
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
	case wsStepAgent:
		var b strings.Builder
		b.WriteString(metaStyle.Render(m.wsDir) + "\n\n")
		b.WriteString("Choose an AI Agent:\n\n")
		for i, a := range m.agentList {
			b.WriteString(m.selectLine(i == m.agentCursor, a.Name))
			b.WriteString("  " + metaStyle.Render("Command: "+a.Command) + "\n")
		}
		b.WriteString(m.selectLine(m.agentCursor >= len(m.agentList), "Default global agent"))

		b.WriteString("\n\n")
		if m.agentAutoApprove {
			b.WriteString(okStyle.Render("  [x] Auto-approve ON"))
		} else {
			b.WriteString(metaStyle.Render("  [ ] Auto-approve OFF"))
		}

		footer := actionKey("↑↓", "move") + "   " + actionKey("space", "toggle auto-approve") + "   " + actionKey("enter", "create") + "   " + actionKey("esc", "back")
		return m.frame("New workspace", b.String(), footer)
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
		if len(m.wsMatches) > 0 {
			body += "\n\n  " + metaStyle.Render("subdirectories:") + "\n  " +
				metaStyle.Render(wrapMatches(m.wsMatches, 60))
		}
		footer := strings.Join([]string{
			actionKey("tab", "complete"), actionKey("ctrl+o", "windows folder"),
			actionKey("enter", "continue"), actionKey("esc", "cancel"),
		}, "   ")
		return m.frame("New workspace", body, footer)
	}
}

// dirCandidates returns the directory currently being completed and the names of
// its sub-directories that match what has been typed — directory-only, so it
// only ever offers valid workspace locations. It powers both the live suggestion
// list and Tab completion.
func dirCandidates(input string) (dir string, matches []string) {
	raw := strings.TrimSpace(input)
	// Detect the trailing separator on the raw text: expandHome cleans the path
	// (via filepath.Abs), which drops it — but it is what distinguishes "list this
	// directory's children" from "complete this last component".
	trailing := strings.HasSuffix(raw, string(os.PathSeparator))
	p := expandHome(raw)
	var prefix string
	switch {
	case raw == "":
		dir = "."
	case trailing:
		dir = p
	default:
		dir, prefix = filepath.Dir(p), filepath.Base(p)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return dir, nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Hide dot-directories unless the user has started typing one.
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}
	sort.Strings(matches)
	return dir, matches
}

// completePath extends a typed path toward its matching sub-directory: a single
// match is filled in with a trailing separator so drilling can continue; several
// matches extend to their longest common prefix.
func completePath(input string) string {
	dir, matches := dirCandidates(input)
	if len(matches) == 0 {
		return input
	}
	if len(matches) == 1 {
		return filepath.Join(dir, matches[0]) + string(os.PathSeparator)
	}
	lcp := matches[0]
	for _, m := range matches[1:] {
		lcp = commonPrefix(lcp, m)
	}
	return filepath.Join(dir, lcp)
}

func commonPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return a[:i]
}

// wrapMatches lays sub-directory names out across lines within width, each shown
// with a trailing separator, capping the list so it never floods the screen.
func wrapMatches(matches []string, width int) string {
	const cap = 24
	extra := 0
	if len(matches) > cap {
		extra = len(matches) - cap
		matches = matches[:cap]
	}
	var lines []string
	cur := ""
	for _, name := range matches {
		item := name + "/"
		switch {
		case cur == "":
			cur = item
		case len(cur)+2+len(item) > width:
			lines = append(lines, cur)
			cur = item
		default:
			cur += "  " + item
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	out := strings.Join(lines, "\n  ")
	if extra > 0 {
		out += fmt.Sprintf("\n  … (+%d more)", extra)
	}
	return out
}

// selectLine renders one selectable row with the accent bar when chosen.
func (m model) selectLine(selected bool, label string) string {
	if selected {
		return barStyle.Render("▌ ") + selNameStyle.Render(sanitizeRender(label))
	}
	return "  " + nameStyle.Render(sanitizeRender(label))
}

type newTaskStep int

const (
	newTaskStepName newTaskStep = iota
	newTaskStepAgent
)

// --- New task --------------------------------------------------------------

func (m model) startNewTask(p project.Project) (tea.Model, tea.Cmd) {
	m.screen = scrNewTask
	m.formProj = p
	m.newTaskStep = newTaskStepName
	m.agentAutoApprove = false
	m.input = newTextInput("")
	m.notice = ""
	return m, nil
}

func (m model) updateNewTask(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.newTaskStep {
	case newTaskStepName:
		switch msg.String() {
		case "esc":
			m.goDashboard()
		case "enter":
			name := strings.TrimSpace(m.input.Value())
			if name == "" {
				return m, nil
			}
			if err := validateTaskName(name); err != nil {
				m.notice, m.noticeErr = err.Error(), true
				return m, nil
			}

			agents := getInstalledAgents()
			if len(agents) == 0 {
				return m.finishNewTask(name, "")
			}
			m.newTaskName = name
			m.agentList = agents
			m.agentCursor = 0
			m.agentAutoApprove = false
			m.newTaskStep = newTaskStepAgent
		default:
			m.input = m.input.update(msg)
		}
	case newTaskStepAgent:
		switch msg.String() {
		case " ":
			m.agentAutoApprove = !m.agentAutoApprove
		case "esc":
			m.newTaskStep = newTaskStepName
		case "up", "k":
			if m.agentCursor > 0 {
				m.agentCursor--
			}
		case "down", "j":
			if m.agentCursor < len(m.agentList) { // Allow selecting "Default global agent"
				m.agentCursor++
			}
		case "enter":
			agentCmd := ""
			if m.agentCursor < len(m.agentList) {
				agentCmd = m.agentList[m.agentCursor].Command
				if m.agentAutoApprove && m.agentList[m.agentCursor].AutoApproveFlag != "" {
					agentCmd += " " + m.agentList[m.agentCursor].AutoApproveFlag
				}
			}
			return m.finishNewTask(m.newTaskName, agentCmd)
		}
	}
	return m, nil
}

func (m model) finishNewTask(name, agentCmd string) (tea.Model, tea.Cmd) {
	tk, err := createTask(m.formProj, name, agentCmd)
	if err != nil {
		m.notice, m.noticeErr = err.Error(), true
		m.newTaskStep = newTaskStepName
		return m, nil
	}
	proj := m.formProj
	m.goDashboard()
	m.refresh()
	m.selectByName(proj.Name)
	// Open the new task straight away — a parallel session without leaving Crabby.
	return m, func() tea.Msg { return openTaskMsg{proj, tk} }
}

func (m model) viewNewTask() string {
	if m.newTaskStep == newTaskStepAgent {
		var b strings.Builder
		b.WriteString(metaStyle.Render(fmt.Sprintf("New task %q in %q", m.newTaskName, m.formProj.Name)) + "\n\n")
		b.WriteString("Choose an AI Agent:\n\n")
		for i, a := range m.agentList {
			b.WriteString(m.selectLine(i == m.agentCursor, a.Name))
			b.WriteString("  " + metaStyle.Render("Command: "+a.Command) + "\n")
		}
		b.WriteString(m.selectLine(m.agentCursor >= len(m.agentList), "Default global agent"))

		b.WriteString("\n\n")
		if m.agentAutoApprove {
			b.WriteString(okStyle.Render("  [x] Auto-approve ON"))
		} else {
			b.WriteString(metaStyle.Render("  [ ] Auto-approve OFF"))
		}

		footer := actionKey("↑↓", "move") + "   " + actionKey("space", "toggle auto-approve") + "   " + actionKey("enter", "create & open") + "   " + actionKey("esc", "back")
		return m.frame("New task", b.String(), footer)
	}

	body := fmt.Sprintf("New task in %q.\n\nTask name:\n\n  ", m.formProj.Name) + m.input.view("e.g. tests, docs, refactor")
	footer := actionKey("enter", "continue") + "   " + actionKey("esc", "cancel")
	return m.frame("New task", body, footer)
}

// --- Shared helpers --------------------------------------------------------

// createTask validates a name, builds its session, and registers it in the
// workspace. It does not start the session — the caller opens it.
func createTask(p project.Project, name, agentCmd string) (project.Task, error) {
	if err := validateTaskName(name); err != nil {
		return project.Task{}, err
	}
	tk := project.Task{
		Name:         name,
		Session:      session.TaskSession(p.Name, name),
		Created:      time.Now(),
		AgentCommand: agentCmd,
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
