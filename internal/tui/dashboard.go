package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/session"
)

// taskRow is one task's live state within a workspace.
type taskRow struct {
	task    project.Task
	state   session.State
	working bool
	uptime  time.Duration
}

// wsRow is a workspace and its tasks, with the workspace-level insight (model,
// tokens, activity) that is shared across tasks in the same directory.
type wsRow struct {
	proj    project.Project
	branch  string
	insight insights.Insight
	tasks   []taskRow
	best    int // best (lowest) task rank, for ordering workspaces
}

// repr picks a representative state and working flag for the workspace, used for
// its dot in the list: the best task's state, working if any task is working.
func (r wsRow) repr() (session.State, bool) {
	if len(r.tasks) == 0 {
		return session.Stopped, false
	}
	working := false
	for _, t := range r.tasks {
		if t.working {
			working = true
		}
	}
	return r.tasks[0].state, working
}

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmStop
	confirmRemoveTask
	confirmRemoveWorkspace
)

// refresh reloads workspaces and their live tmux state in a single call, sorts
// them by importance, and flags a bell when a background task starts working.
func (m *model) refresh() {
	projects, err := project.Load()
	if err != nil {
		projects = nil
	}
	sessions := m.tmux.ListSessions()
	m.sessions = sessions

	rising := false
	rows := make([]wsRow, 0, len(projects))
	for _, p := range projects {
		r := wsRow{proj: p, branch: p.Branch(), insight: m.insights.For(p.Path), best: 99}
		for _, tk := range p.Tasks {
			info, present := sessions[tk.Session]
			tr := taskRow{task: tk, state: session.Classify(present, info.Attached)}
			if present {
				prev, seen := m.lastActivity[tk.Session]
				tr.working = seen && info.Activity.After(prev)
				if tr.working && !m.working[tk.Session] && !m.firstLoad {
					rising = true
				}
				if !info.Created.IsZero() {
					tr.uptime = time.Since(info.Created)
				}
				m.lastActivity[tk.Session] = info.Activity
				m.working[tk.Session] = tr.working
			} else {
				delete(m.lastActivity, tk.Session)
				delete(m.working, tk.Session)
			}
			if rk := session.Rank(tr.state); rk < r.best {
				r.best = rk
			}
			r.tasks = append(r.tasks, tr)
		}
		sort.SliceStable(r.tasks, func(i, j int) bool {
			ri, rj := session.Rank(r.tasks[i].state), session.Rank(r.tasks[j].state)
			if ri != rj {
				return ri < rj
			}
			return r.tasks[i].task.Name < r.tasks[j].task.Name
		})
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].best != rows[j].best {
			return rows[i].best < rows[j].best
		}
		return rows[i].proj.Name < rows[j].proj.Name
	})

	m.rows = rows
	m.ringBell = rising
	m.firstLoad = false
	m.clampCursors()
}

// clampCursors keeps both cursors within range after a refresh.
func (m *model) clampCursors() {
	if m.wsCursor >= len(m.rows) {
		m.wsCursor = len(m.rows) - 1
	}
	if m.wsCursor < 0 {
		m.wsCursor = 0
	}
	if ws, ok := m.selectedWs(); ok {
		if m.taskCursor >= len(ws.tasks) {
			m.taskCursor = len(ws.tasks) - 1
		}
		if m.taskCursor < 0 {
			m.taskCursor = 0
		}
	} else {
		m.taskCursor = 0
	}
}

func (m model) selectedWs() (wsRow, bool) {
	if m.wsCursor < 0 || m.wsCursor >= len(m.rows) {
		return wsRow{}, false
	}
	return m.rows[m.wsCursor], true
}

func (m model) selectedTask() (wsRow, taskRow, bool) {
	ws, ok := m.selectedWs()
	if !ok || m.taskCursor < 0 || m.taskCursor >= len(ws.tasks) {
		return wsRow{}, taskRow{}, false
	}
	return ws, ws.tasks[m.taskCursor], true
}

// --- Update ----------------------------------------------------------------

func (m model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirm != confirmNone {
		return m.handleConfirm(msg)
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		if m.notice != "" {
			m.notice = ""
		} else if m.focus == paneTasks {
			m.focus = paneWorkspaces
		}
	case "up", "k":
		m.moveUp()
	case "down", "j":
		m.moveDown()
	case "tab", "right", "l":
		m.focus = paneTasks
	case "shift+tab", "left", "h":
		m.focus = paneWorkspaces
	case "enter":
		if ws, tr, ok := m.selectedTask(); ok {
			proj, task := ws.proj, tr.task
			return m, func() tea.Msg { return openTaskMsg{proj, task} }
		}
	case "n":
		return m.startNewWorkspace()
	case "o":
		if ws, ok := m.selectedWs(); ok {
			return m, openFolderCmd(ws.proj.Path)
		}
	case "i":
		return m.startImport()
	case "t":
		if ws, ok := m.selectedWs(); ok {
			return m.startNewTask(ws.proj)
		}
	case "x":
		if _, tr, ok := m.selectedTask(); ok && tr.state != session.Stopped {
			m.confirm = confirmStop
		}
	case "d":
		if ws, _, ok := m.selectedTask(); ok {
			if m.focus == paneTasks && len(ws.tasks) > 1 {
				m.confirm = confirmRemoveTask
			} else {
				m.confirm = confirmRemoveWorkspace
			}
		}
	case "r":
		m.refresh()
	case "p":
		return m.startPacks()
	case "s":
		return m.startSettings()
	case "?":
		m.screen = scrHelp
	}
	return m, nil
}

func (m *model) moveUp() {
	if m.focus == paneWorkspaces {
		if m.wsCursor > 0 {
			m.wsCursor--
			m.taskCursor = 0
		}
		return
	}
	if m.taskCursor > 0 {
		m.taskCursor--
	}
}

func (m *model) moveDown() {
	if m.focus == paneWorkspaces {
		if m.wsCursor < len(m.rows)-1 {
			m.wsCursor++
			m.taskCursor = 0
		}
		return
	}
	if ws, ok := m.selectedWs(); ok && m.taskCursor < len(ws.tasks)-1 {
		m.taskCursor++
	}
}

func (m model) handleConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if s := msg.String(); s == "y" || s == "Y" {
		if ws, tr, ok := m.selectedTask(); ok {
			switch m.confirm {
			case confirmStop:
				_ = m.tmux.KillSession(tr.task.Session)
			case confirmRemoveTask:
				_ = m.tmux.KillSession(tr.task.Session)
				_ = project.RemoveTask(ws.proj.Name, tr.task.Name)
			case confirmRemoveWorkspace:
				for _, t := range ws.proj.Tasks {
					_ = m.tmux.KillSession(t.Session)
				}
				_ = project.Remove(ws.proj.Name)
			}
			m.refresh()
		}
	}
	m.confirm = confirmNone
	return m, nil
}

// --- View ------------------------------------------------------------------

func (m model) viewDashboard() string {
	var b strings.Builder
	b.WriteString(center(m.width, BannerFrame(m.anim)))
	b.WriteString("\n\n")

	if len(m.rows) == 0 {
		b.WriteString(center(m.width, divider()) + "\n\n")
		b.WriteString(center(m.width, "No workspaces yet.") + "\n\n")
		b.WriteString(center(m.width, metaStyle.Render("Press  n  to create one — point it at any project folder.")) + "\n\n")
		b.WriteString(center(m.width, divider()) + "\n")
		b.WriteString(center(m.width, actionKey("n", "new workspace")+"   "+actionKey("p", "packs")+"   "+actionKey("?", "help")+"   "+actionKey("q", "quit")))
		if n := m.noticeLine(); n != "" {
			b.WriteString("\n\n" + center(m.width, n))
		}
		return b.String()
	}

	leftW, midW, rightW := m.paneWidths()
	h := m.paneHeight()
	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		m.workspaceColumn(leftW, h), " ",
		m.taskColumn(midW, h), " ",
		m.detailColumn(rightW, h),
	)
	b.WriteString(center(m.width, cols))
	b.WriteString("\n")
	if s := m.summaryLine(); s != "" {
		b.WriteString(center(m.width, s) + "\n")
	}

	switch m.confirm {
	case confirmStop:
		_, tr, _ := m.selectedTask()
		b.WriteString(center(m.width, confirmStyle.Render(fmt.Sprintf("Stop %q? (y/n)", tr.task.Name))))
		return b.String()
	case confirmRemoveTask:
		ws, tr, _ := m.selectedTask()
		b.WriteString(center(m.width, confirmStyle.Render(fmt.Sprintf("Remove task %q from %q? (y/n)", tr.task.Name, ws.proj.Name))))
		return b.String()
	case confirmRemoveWorkspace:
		ws, _ := m.selectedWs()
		b.WriteString(center(m.width, confirmStyle.Render(fmt.Sprintf("Remove workspace %q from Crabby? Files are kept. (y/n)", ws.proj.Name))))
		return b.String()
	}

	b.WriteString(center(m.width, m.dashboardFooter()))
	if n := m.noticeLine(); n != "" {
		b.WriteString("\n" + center(m.width, n))
	}
	return b.String()
}

// paneWidths splits the terminal width into the three content columns.
func (m model) paneWidths() (int, int, int) {
	w := m.width
	if w <= 0 {
		w = 80
	}
	budget := w - 2 - 12 - 2 // outer margin, 3×(border+padding), 2 gaps
	if budget < 36 {
		budget = 36
	}
	left := budget * 26 / 100
	mid := budget * 32 / 100
	right := budget - left - mid
	if left < 12 {
		left = 12
	}
	if mid < 14 {
		mid = 14
	}
	if right < 18 {
		right = 18
	}
	return left, mid, right
}

// paneHeight is how tall the three columns may be, given the banner and footer.
func (m model) paneHeight() int {
	h := m.height
	if h <= 0 {
		h = 24
	}
	avail := h - lipgloss.Height(Banner()) - 8
	if avail < 6 {
		avail = 6
	}
	return avail
}

// box wraps inner content in a rounded border of the given content size, the
// border accented when the pane holds focus.
func box(inner string, w, h int, focused bool) string {
	s := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(w).Height(h)
	if focused {
		return s.BorderForeground(accent).Render(inner)
	}
	return s.BorderForeground(lipgloss.Color("240")).Render(inner)
}

// paneTitle renders a column's heading, accented when the column is focused.
func paneTitle(label string, focused bool) string {
	if focused {
		return paneFocusedTitle.Render(label)
	}
	return paneTitleStyle.Render(label)
}

// window returns the [start,end) slice of a list of n items that keeps cursor
// visible within a window of the given size.
func window(n, cursor, size int) (int, int) {
	if size < 1 {
		size = 1
	}
	if n <= size {
		return 0, n
	}
	start := cursor - size/2
	if start < 0 {
		start = 0
	}
	if start+size > n {
		start = n - size
	}
	return start, start + size
}

func (m model) workspaceColumn(w, h int) string {
	focused := m.focus == paneWorkspaces
	var lines []string
	lines = append(lines, paneTitle("Workspaces", focused), "")
	budget := h - 2
	start, end := window(len(m.rows), m.wsCursor, budget)
	for i := start; i < end; i++ {
		r := m.rows[i]
		selected := i == m.wsCursor
		st, working := r.repr()
		dot, dotStyle := glyph(st, working)
		name := r.proj.Name
		nameR := nameStyle.Render(name)
		if selected {
			nameR = selNameStyle.Render(name)
		}
		count := ""
		if len(r.tasks) > 1 {
			count = metaStyle.Render(fmt.Sprintf(" (%d)", len(r.tasks)))
		}
		gutter := "  "
		if selected && focused {
			gutter = barStyle.Render("▌ ")
		}
		lines = append(lines, gutter+dotStyle.Render(dot)+" "+nameR+count)
	}
	return box(strings.Join(lines, "\n"), w, h, focused)
}

func (m model) taskColumn(w, h int) string {
	focused := m.focus == paneTasks
	ws, ok := m.selectedWs()
	var lines []string
	lines = append(lines, paneTitle("Tasks", focused), "")
	if !ok {
		lines = append(lines, metaStyle.Render("—"))
		return box(strings.Join(lines, "\n"), w, h, focused)
	}
	budget := h - 2
	start, end := window(len(ws.tasks), m.taskCursor, budget)
	for i := start; i < end; i++ {
		tr := ws.tasks[i]
		selected := i == m.taskCursor
		dot, dotStyle := glyph(tr.state, tr.working)
		nameR := nameStyle.Render(tr.task.Name)
		if selected {
			nameR = selNameStyle.Render(tr.task.Name)
		}
		gutter := "  "
		if selected && focused {
			gutter = barStyle.Render("▌ ")
		}
		lines = append(lines, gutter+dotStyle.Render(dot)+" "+nameR)
	}
	return box(strings.Join(lines, "\n"), w, h, focused)
}

func (m model) detailColumn(w, h int) string {
	var lines []string
	lines = append(lines, paneTitle("Details", false), "")
	ws, tr, ok := m.selectedTask()
	if !ok {
		lines = append(lines, metaStyle.Render("—"))
		return box(strings.Join(lines, "\n"), w, h, false)
	}
	ins := ws.insight
	add := func(label, value string) {
		if value == "" {
			return
		}
		lines = append(lines, metaStyle.Render(fmt.Sprintf("%-9s", label))+value)
	}
	add("status", stateLabel(tr.state, tr.working))
	add("task", nameStyle.Render(tr.task.Name))
	add("branch", ws.branch)
	if ins.Model != "" {
		add("model", ins.Model)
	}
	if ins.SessionTokens > 0 {
		add("tokens", formatTokens(ins.SessionTokens)+" session")
	}
	if ins.TodayTokens > 0 {
		add("today", formatTokens(ins.TodayTokens)+" tokens")
	}
	if tr.uptime > 0 {
		add("uptime", formatDuration(tr.uptime))
	}
	if tr.working {
		add("activity", workingActivity(ins))
	}
	if !ins.LastActivity.IsZero() {
		add("last", formatAgo(time.Since(ins.LastActivity)))
	}
	add("path", metaStyle.Render(ws.proj.Path))
	return box(strings.Join(lines, "\n"), w, h, false)
}

// summaryLine is the global usage bar: workspace and task counts plus tokens.
func (m model) summaryLine() string {
	if len(m.rows) == 0 {
		return ""
	}
	var active, stopped, working, tokens int
	for _, r := range m.rows {
		tokens += r.insight.TodayTokens
		for _, t := range r.tasks {
			if t.state == session.Stopped {
				stopped++
			} else {
				active++
			}
			if t.working {
				working++
			}
		}
	}
	parts := []string{plural(len(m.rows), "workspace")}
	if active > 0 {
		parts = append(parts, fmt.Sprintf("%d active", active))
	}
	if stopped > 0 {
		parts = append(parts, fmt.Sprintf("%d stopped", stopped))
	}
	if working > 0 {
		parts = append(parts, fmt.Sprintf("%d working", working))
	}
	if tokens > 0 {
		parts = append(parts, formatTokens(tokens)+" tokens today")
	}
	return summaryStyle.Render(strings.Join(parts, "   ·   "))
}

// dashboardFooter shows the actions and the on-screen hint for returning from a
// Claude session — the single, discoverable answer to "how do I get back?".
func (m model) dashboardFooter() string {
	line1 := strings.Join([]string{
		actionKey("↑↓", "move"), actionKey("tab", "pane"),
		actionKey("enter", "open"), actionKey("n", "new"),
		actionKey("t", "task"), actionKey("o", "folder"),
		actionKey("x", "stop"), actionKey("d", "remove"),
	}, "   ")
	line2 := strings.Join([]string{
		actionKey("F12", "mission control"), actionKey("i", "import"),
		actionKey("p", "packs"), actionKey("s", "settings"),
		actionKey("?", "help"), actionKey("q", "quit"),
	}, "   ")
	return line1 + "\n" + line2
}
