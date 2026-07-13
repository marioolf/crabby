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

	"github.com/marioolf/crabby/internal/importcmd"
	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/pack"
	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/session"
	"github.com/marioolf/crabby/internal/tmux"
)

const refreshEach = time.Second

// accent is Crabby's primary highlight colour (the crab's orange).
const accent = lipgloss.Color("#FF6B4A")

var (
	dividerStyle = lipgloss.NewStyle().Faint(true)
	metaStyle    = lipgloss.NewStyle().Faint(true)
	helpStyle    = lipgloss.NewStyle().Faint(true)
	nameStyle    = lipgloss.NewStyle()
	selNameStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231"))
	barStyle     = lipgloss.NewStyle().Foreground(accent)
	confirmStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	workingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	stateStyles = map[session.State]lipgloss.Style{
		session.Running: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),  // green
		session.Waiting: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")), // yellow
		session.Stopped: lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("244")),
	}
)

// divider spans the banner's width so the whole card lines up.
func divider() string { return dividerStyle.Render(strings.Repeat("─", BannerWidth())) }

// center places content horizontally in the given terminal width (0 = as-is).
func center(width int, content string) string {
	if width <= 0 {
		return content
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}

// Action is what the user chose on the dashboard.
type Action int

const (
	ActionQuit Action = iota
	ActionOpen
	ActionNewProject
	ActionNewTask
)

// Result is returned from Run. Project is the chosen workspace; Task is the
// chosen task within it (for ActionOpen) or nil.
type Result struct {
	Action  Action
	Project *project.Project
	Task    *project.Task
}

// Styles specific to the dashboard footer, so informational data (the usage
// summary) reads differently from the action keys.
var (
	keyStyle     = lipgloss.NewStyle().Foreground(accent).Bold(true)
	summaryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	headerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
)

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmStop
	confirmRemoveTask
	confirmRemoveWorkspace
)

// rowKind distinguishes the three shapes a dashboard line can take.
type rowKind int

const (
	rowSingle rowKind = iota // a single-task workspace, shown as one line
	rowHeader                // a multi-task workspace's name (not selectable)
	rowTask                  // one task beneath a header
)

type item struct {
	workspace  project.Project
	task       project.Task
	kind       rowKind
	state      session.State
	branch     string
	working    bool // producing output right now
	uptime     time.Duration
	insight    insights.Insight
	groupStart bool // first line of a workspace group, for spacing
}

type tickMsg time.Time

type model struct {
	tmux      tmux.Client
	detachKey string
	insights  *insights.Collector
	width     int
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

// Run displays the dashboard and returns the chosen action. The collector
// supplies transcript-derived insights and is reused across refreshes so its
// incremental cache survives; pass nil to run without them.
func Run(t tmux.Client, detachKey string, coll *insights.Collector) (Result, error) {
	if coll == nil {
		coll = insights.New()
	}
	m := model{
		tmux:         t,
		detachKey:    detachKey,
		insights:     coll,
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

// refresh reloads workspaces and session state in a single tmux call, groups
// each workspace's tasks together, tracks which sessions are actively producing
// output, and flags a bell on a rising edge (a task that just started working
// while you're on the dashboard).
func (m *model) refresh() {
	projects, err := project.Load()
	if err != nil {
		projects = nil
	}
	sessions := m.tmux.ListSessions()

	type group struct {
		p       project.Project
		branch  string
		insight insights.Insight
		rows    []item
		best    int // best (lowest) task rank, for ordering workspaces
	}

	risingEdge := false
	groups := make([]group, 0, len(projects))
	for _, p := range projects {
		g := group{p: p, branch: p.Branch(), insight: m.insights.For(p.Path), best: 99}
		for _, tk := range p.Tasks {
			info, present := sessions[tk.Session]
			it := item{
				workspace: p,
				task:      tk,
				state:     session.Classify(present, info.Attached),
				branch:    g.branch,
				insight:   g.insight,
			}
			if present {
				prev, seen := m.lastActivity[tk.Session]
				it.working = seen && info.Activity.After(prev)
				if it.working && !m.working[tk.Session] && !m.firstLoad {
					risingEdge = true
				}
				if !info.Created.IsZero() {
					it.uptime = time.Since(info.Created)
				}
				m.lastActivity[tk.Session] = info.Activity
				m.working[tk.Session] = it.working
			} else {
				delete(m.lastActivity, tk.Session)
				delete(m.working, tk.Session)
			}
			if r := session.Rank(it.state); r < g.best {
				g.best = r
			}
			g.rows = append(g.rows, it)
		}
		sort.SliceStable(g.rows, func(i, j int) bool {
			ri, rj := session.Rank(g.rows[i].state), session.Rank(g.rows[j].state)
			if ri != rj {
				return ri < rj
			}
			return g.rows[i].task.Name < g.rows[j].task.Name
		})
		groups = append(groups, g)
	}

	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].best != groups[j].best {
			return groups[i].best < groups[j].best
		}
		return groups[i].p.Name < groups[j].p.Name
	})

	items := make([]item, 0, len(groups))
	for _, g := range groups {
		if len(g.rows) == 1 {
			it := g.rows[0]
			it.kind = rowSingle
			it.groupStart = true
			items = append(items, it)
			continue
		}
		items = append(items, item{
			workspace: g.p, kind: rowHeader, branch: g.branch,
			insight: g.insight, groupStart: true,
		})
		for _, it := range g.rows {
			it.kind = rowTask
			items = append(items, it)
		}
	}

	m.items = items
	m.ringBell = risingEdge
	m.firstLoad = false
	m.clampCursor()
}

// selectable reports whether a row can hold the cursor (headers cannot).
func (m model) selectable(i int) bool {
	return i >= 0 && i < len(m.items) && m.items[i].kind != rowHeader
}

// nextSelectable returns the next selectable row from i in the given direction,
// or -1 if there is none.
func (m model) nextSelectable(i, dir int) int {
	for j := i + dir; j >= 0 && j < len(m.items); j += dir {
		if m.selectable(j) {
			return j
		}
	}
	return -1
}

// clampCursor keeps the cursor in range and never resting on a header.
func (m *model) clampCursor() {
	if len(m.items) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.selectable(m.cursor) {
		return
	}
	if n := m.nextSelectable(m.cursor, 1); n >= 0 {
		m.cursor = n
	} else if p := m.nextSelectable(m.cursor, -1); p >= 0 {
		m.cursor = p
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
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
			it := m.items[m.cursor]
			switch m.confirm {
			case confirmStop:
				_ = m.tmux.KillSession(it.task.Session)
			case confirmRemoveTask:
				_ = m.tmux.KillSession(it.task.Session)
				_ = project.RemoveTask(it.workspace.Name, it.task.Name)
			case confirmRemoveWorkspace:
				for _, tk := range it.workspace.Tasks {
					_ = m.tmux.KillSession(tk.Session)
				}
				_ = project.Remove(it.workspace.Name)
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
		if n := m.nextSelectable(m.cursor, -1); n >= 0 {
			m.cursor = n
		}
	case "down", "j":
		if n := m.nextSelectable(m.cursor, 1); n >= 0 {
			m.cursor = n
		}
	case "r":
		m.refresh()
	case "n":
		m.result = Result{Action: ActionNewProject}
		return m, tea.Quit
	case "t":
		if m.selectable(m.cursor) {
			ws := m.items[m.cursor].workspace
			m.result = Result{Action: ActionNewTask, Project: &ws}
			return m, tea.Quit
		}
	case "x":
		if m.selectable(m.cursor) && m.items[m.cursor].state != session.Stopped {
			m.confirm = confirmStop
		}
	case "d":
		if m.selectable(m.cursor) {
			// A single-task row stands for the whole workspace; a task row
			// removes just that task.
			if m.items[m.cursor].kind == rowTask {
				m.confirm = confirmRemoveTask
			} else {
				m.confirm = confirmRemoveWorkspace
			}
		}
	case "enter":
		if m.selectable(m.cursor) {
			it := m.items[m.cursor]
			ws, tk := it.workspace, it.task
			m.result = Result{Action: ActionOpen, Project: &ws, Task: &tk}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	// Everything is centred on the terminal. Standalone lines (banner, dividers,
	// summary, footer) centre individually; the workspace list centres as one
	// block so its columns stay left-aligned.
	var b strings.Builder
	b.WriteString(center(m.width, Banner()))
	b.WriteString("\n\n")
	b.WriteString(center(m.width, divider()))
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		b.WriteString(center(m.width, "No workspaces yet.") + "\n\n")
		b.WriteString(center(m.width, metaStyle.Render("Run  crabby init  inside a project,")) + "\n")
		b.WriteString(center(m.width, metaStyle.Render("or press  n  to add the current directory.")) + "\n\n")
		b.WriteString(center(m.width, divider()) + "\n")
		b.WriteString(center(m.width, actionKey("n", "new")+"   "+actionKey("q", "quit")))
		return b.String()
	}

	b.WriteString(blockCenter(m.width, m.renderRows()))
	b.WriteString("\n")
	b.WriteString(center(m.width, divider()))
	b.WriteString("\n")
	if summary := m.summaryLine(); summary != "" {
		b.WriteString(center(m.width, summary) + "\n\n")
	}
	switch m.confirm {
	case confirmStop:
		b.WriteString(center(m.width, confirmStyle.Render(fmt.Sprintf("Stop \"%s\"? (y/n)", m.items[m.cursor].displayName()))))
		return b.String()
	case confirmRemoveTask:
		it := m.items[m.cursor]
		b.WriteString(center(m.width, confirmStyle.Render(fmt.Sprintf("Remove task \"%s\" from %q? (y/n)", it.task.Name, it.workspace.Name))))
		return b.String()
	case confirmRemoveWorkspace:
		b.WriteString(center(m.width, confirmStyle.Render(fmt.Sprintf("Remove workspace \"%s\" from Crabby? Files are kept. (y/n)", m.items[m.cursor].workspace.Name))))
		return b.String()
	}
	b.WriteString(center(m.width, m.helpLines()))
	return b.String()
}

// renderRows lays every workspace/task line out in two columns: the name on the
// left, its live data aligned to the right.
func (m model) renderRows() string {
	plain := make([]string, len(m.items))
	labelWidth := 0
	for i, it := range m.items {
		plain[i] = it.plainLabel()
		if w := lipgloss.Width(plain[i]); w > labelWidth {
			labelWidth = w
		}
	}

	var b strings.Builder
	for i, it := range m.items {
		if it.groupStart && i > 0 {
			b.WriteString("\n")
		}
		gutter := "  "
		if m.selectable(i) && i == m.cursor {
			gutter = barStyle.Render("▌ ")
		}
		pad := strings.Repeat(" ", labelWidth-lipgloss.Width(plain[i])+2)
		b.WriteString(gutter + it.styledLabel(i == m.cursor) + pad + m.rowData(it) + "\n")
		if meta := m.rowMeta(it); meta != "" {
			b.WriteString("  " + strings.Repeat(" ", labelWidth+2) + meta + "\n")
		}
	}
	return b.String()
}

// blockCenter centres a multi-line block as a unit: every line gets the same
// left margin, so the block sits in the middle while its internal left-aligned
// columns stay aligned (unlike per-line centring, which lets each line drift).
func blockCenter(width int, s string) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	blockWidth := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > blockWidth {
			blockWidth = w
		}
	}
	margin := (width - blockWidth) / 2
	if margin <= 0 {
		return s
	}
	pad := strings.Repeat(" ", margin)
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}

// helpLines renders the footer with action keys highlighted so they read as
// controls, distinct from the informational summary above them.
func (m model) helpLines() string {
	line1 := strings.Join([]string{
		actionKey("enter", "open"), actionKey("n", "new workspace"),
		actionKey("t", "new task"), actionKey("x", "stop"), actionKey("d", "remove"),
	}, "   ")
	line2 := actionKey("r", "refresh") + "   " + actionKey("q", "quit") +
		"   " + helpStyle.Render("·  "+m.detachKey+" returns from a session")
	return line1 + "\n" + line2
}

// actionKey styles a keybinding: the key in the accent colour, its description
// faint.
func actionKey(k, desc string) string {
	return keyStyle.Render(k) + helpStyle.Render(" "+desc)
}

func (it item) plainLabel() string {
	switch it.kind {
	case rowHeader:
		return it.workspace.Name
	case rowTask:
		return "  " + glyphRune(it.state) + " " + it.task.Name
	default:
		return glyphRune(it.state) + " " + it.workspace.Name
	}
}

func (it item) styledLabel(selected bool) string {
	if it.kind == rowHeader {
		return headerStyle.Render(it.workspace.Name)
	}
	name := it.workspace.Name
	indent := ""
	if it.kind == rowTask {
		name = it.task.Name
		indent = "  "
	}
	dot, dotStyle := glyph(it.state, it.working)
	rendered := nameStyle.Render(name)
	if selected {
		rendered = selNameStyle.Render(name)
	}
	return indent + dotStyle.Render(dot) + " " + rendered
}

func (it item) displayName() string {
	if it.kind == rowTask {
		return it.task.Name
	}
	return it.workspace.Name
}

// rowData is the primary line to the right of a row's name.
func (m model) rowData(it item) string {
	switch it.kind {
	case rowHeader:
		return metaStyle.Render(headerData(it))
	case rowTask:
		data := coarseActivity(it)
		if it.uptime > 0 {
			data += metaStyle.Render("  ·  up " + formatDuration(it.uptime))
		}
		return data
	default:
		data := m.activityLabel(it)
		if ago := lastActivityAgo(it.insight); ago != "" {
			data += metaStyle.Render("  ·  " + ago)
		}
		return data
	}
}

// rowMeta is the optional faint second line, used only by single-task rows to
// carry branch/uptime/model/tokens.
func (m model) rowMeta(it item) string {
	if it.kind != rowSingle {
		return ""
	}
	if meta := metaLine(it); meta != "" {
		return metaStyle.Render(meta)
	}
	return ""
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

// glyphRune is glyph's symbol only, for measuring label widths.
func glyphRune(s session.State) string {
	if s == session.Stopped {
		return "○"
	}
	return "●"
}

// headerData is the dir-level summary shown on a multi-task workspace header:
// its branch, model, and current-session tokens. These come from Claude's
// transcript for the shared project directory, so they describe the workspace,
// not any single task.
func headerData(it item) string {
	var parts []string
	if it.branch != "" {
		parts = append(parts, it.branch)
	}
	if it.insight.Model != "" {
		parts = append(parts, it.insight.Model)
	}
	if it.insight.SessionTokens > 0 {
		parts = append(parts, formatTokens(it.insight.SessionTokens)+" tok")
	}
	return strings.Join(parts, "  ·  ")
}

// coarseActivity is the state shown per task when several tasks share a
// directory: transcript-derived detail cannot be attributed to one task, so
// only the reliable tmux state is shown.
func coarseActivity(it item) string {
	switch {
	case it.state == session.Stopped:
		return stateStyles[session.Stopped].Render("Stopped")
	case it.working:
		return workingStyle.Render("Working")
	case it.state == session.Running:
		return stateStyles[session.Running].Render("Attached")
	default:
		return metaStyle.Render("Idle")
	}
}

// activityLabel describes what a single-task workspace is doing, combining
// tmux's live "working" signal with the transcript's last recorded step. It
// never invents state: with no transcript it falls back to the plain state.
func (m model) activityLabel(it item) string {
	if it.state == session.Stopped {
		return stateStyles[session.Stopped].Render("Stopped")
	}
	if it.working {
		return workingStyle.Render(workingActivity(it.insight))
	}
	if it.insight.Activity == insights.Responding {
		return stateStyles[session.Waiting].Render("Waiting for input")
	}
	return metaStyle.Render("Idle")
}

// workingActivity turns a transcript activity into a label, always returning
// something — it falls back to a plain "Working" when the tail gives no hint.
func workingActivity(ins insights.Insight) string {
	switch ins.Activity {
	case insights.Thinking:
		return "Thinking…"
	case insights.Editing:
		return "Editing files"
	case insights.Reading:
		return "Reading files"
	case insights.Running:
		if ins.Detail == "" || ins.Detail == "Bash" {
			return "Running command"
		}
		return "Running " + ins.Detail
	case insights.Responding:
		return "Responding"
	default:
		return "Working"
	}
}

// metaLine joins the factual bits we have for a project into one faint line,
// omitting anything unavailable.
func metaLine(it item) string {
	var parts []string
	if it.branch != "" {
		parts = append(parts, it.branch)
	}
	if it.uptime > 0 {
		parts = append(parts, "up "+formatDuration(it.uptime))
	}
	if it.insight.Model != "" {
		parts = append(parts, it.insight.Model)
	}
	if it.insight.SessionTokens > 0 {
		parts = append(parts, formatTokens(it.insight.SessionTokens)+" tok")
	}
	return strings.Join(parts, "  ·  ")
}

func lastActivityAgo(ins insights.Insight) string {
	if !ins.Found || ins.LastActivity.IsZero() {
		return ""
	}
	return formatAgo(time.Since(ins.LastActivity))
}

// summaryLine is the dashboard's global usage bar. Workspaces are counted once
// each (the first line of each group); task states are counted per session.
// Token totals appear only when transcripts provided them, never as a
// placeholder, and are summed once per workspace to avoid double counting.
func (m model) summaryLine() string {
	if len(m.items) == 0 {
		return ""
	}
	var workspaces, waiting, stopped, working, tokens int
	for _, it := range m.items {
		if it.groupStart {
			workspaces++
			tokens += it.insight.TodayTokens
		}
		if it.kind == rowHeader {
			continue
		}
		switch it.state {
		case session.Stopped:
			stopped++
		default: // Running or Waiting are both "alive"
			waiting++
		}
		if it.working {
			working++
		}
	}
	parts := []string{plural(workspaces, "workspace")}
	if waiting > 0 {
		parts = append(parts, fmt.Sprintf("%d active", waiting))
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

// formatTokens renders a token count compactly: 950, 12k, 428k.
func formatTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%dk", (n+500)/1000)
}

// formatDuration renders an uptime as 5m, 2h14m, or 3d4h.
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "<1m"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		if m := int(d.Minutes()) % 60; m > 0 {
			return fmt.Sprintf("%dh%dm", int(d.Hours()), m)
		}
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		if h := int(d.Hours()) % 24; h > 0 {
			return fmt.Sprintf("%dd%dh", int(d.Hours())/24, h)
		}
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	}
}

// formatAgo renders how long ago something happened: 20s ago, 5m ago, 2h ago.
func formatAgo(d time.Duration) string {
	switch {
	case d < 0:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours())/24)
	}
}

// --- Task selector ---------------------------------------------------------

type taskModel struct {
	workspace string
	tasks     []project.Task
	cursor    int
	width     int
	chosen    *project.Task
}

// SelectTask shows a workspace's tasks and returns the chosen one, or nil if the
// user cancels. Used by `crabby attach` when a workspace holds several tasks.
func SelectTask(workspace string, tasks []project.Task) (*project.Task, error) {
	final, err := tea.NewProgram(
		taskModel{workspace: workspace, tasks: tasks},
		tea.WithAltScreen(),
	).Run()
	if err != nil {
		return nil, err
	}
	return final.(taskModel).chosen, nil
}

func (m taskModel) Init() tea.Cmd { return nil }

func (m taskModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		return m, nil
	}
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
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
		}
	case "enter":
		t := m.tasks[m.cursor]
		m.chosen = &t
		return m, tea.Quit
	}
	return m, nil
}

func (m taskModel) View() string {
	var b strings.Builder
	b.WriteString(Banner())
	b.WriteString("\n\n")
	b.WriteString(divider())
	b.WriteString("\n\n")
	b.WriteString(wordmarkStyle.Render(m.workspace) + metaStyle.Render("  ·  choose a task"))
	b.WriteString("\n\n")
	for i, t := range m.tasks {
		bar := "  "
		name := nameStyle.Render(t.Name)
		if i == m.cursor {
			bar = barStyle.Render("▌ ")
			name = selNameStyle.Render(t.Name)
		}
		b.WriteString(bar + name + "\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move   enter attach   q cancel"))
	return center(m.width, b.String())
}

// --- Pack selector ---------------------------------------------------------

type packModel struct {
	packs  []pack.Pack
	cursor int
	width  int
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
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		return m, nil
	}
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
	b.WriteString(Banner())
	b.WriteString("\n\n")
	b.WriteString(divider())
	b.WriteString("\n\n")
	b.WriteString(wordmarkStyle.Render("Select a pack"))
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
	return center(m.width, b.String())
}

// --- Import selector -------------------------------------------------------

type importModel struct {
	workspaces []importcmd.Workspace
	selected   []bool
	cursor     int
	width      int
	confirmed  bool // Enter pressed — import the selection
}

// SelectImports shows the discovered workspaces and lets the user choose which
// to import. It returns the chosen workspaces and whether the user cancelled.
// Already-imported workspaces are shown for context but can never be selected.
func SelectImports(workspaces []importcmd.Workspace) (chosen []importcmd.Workspace, cancelled bool, err error) {
	selected := make([]bool, len(workspaces))
	for i, w := range workspaces {
		// Default everything importable to checked — adopting a whole folder of
		// repos should take one keypress, not one per project.
		selected[i] = !w.Imported
	}

	final, runErr := tea.NewProgram(
		importModel{workspaces: workspaces, selected: selected},
		tea.WithAltScreen(),
	).Run()
	if runErr != nil {
		return nil, false, runErr
	}
	m := final.(importModel)
	if !m.confirmed {
		return nil, true, nil
	}
	for i, w := range m.workspaces {
		if m.selected[i] && !w.Imported {
			chosen = append(chosen, w)
		}
	}
	return chosen, false, nil
}

func (m importModel) Init() tea.Cmd { return nil }

func (m importModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		return m, nil
	}
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
		if m.cursor < len(m.workspaces)-1 {
			m.cursor++
		}
	case " ":
		// Already-imported workspaces are locked — there is nothing to toggle.
		if !m.workspaces[m.cursor].Imported {
			m.selected[m.cursor] = !m.selected[m.cursor]
		}
	case "a":
		m.toggleAll()
	case "enter":
		m.confirmed = true
		return m, tea.Quit
	}
	return m, nil
}

// toggleAll selects every importable workspace, or clears them if all are
// already selected.
func (m *importModel) toggleAll() {
	allOn := true
	for i, w := range m.workspaces {
		if !w.Imported && !m.selected[i] {
			allOn = false
			break
		}
	}
	for i, w := range m.workspaces {
		if !w.Imported {
			m.selected[i] = !allOn
		}
	}
}

func (m importModel) View() string {
	var b strings.Builder
	b.WriteString(Banner())
	b.WriteString("\n\n")
	b.WriteString(divider())
	b.WriteString("\n\n")
	b.WriteString(wordmarkStyle.Render("Import Claude workspaces"))
	b.WriteString("\n")
	b.WriteString(metaStyle.Render(fmt.Sprintf("Found %s.", plural(len(m.workspaces), "workspace"))))
	b.WriteString("\n\n")

	for i, w := range m.workspaces {
		bar := "  "
		name := nameStyle.Render(w.Name)
		if i == m.cursor {
			bar = barStyle.Render("▌ ")
			name = selNameStyle.Render(w.Name)
		}
		b.WriteString(fmt.Sprintf("%s%s %s", bar, checkbox(w.Imported, m.selected[i]), name))
		if w.Imported {
			b.WriteString("  " + metaStyle.Render("already imported"))
		} else if w.Branch != "" {
			b.WriteString("  " + metaStyle.Render(w.Branch))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(divider())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("space select   a all   enter import   q cancel"))
	return center(m.width, b.String())
}

// checkbox renders an item's selection state: a locked tick for workspaces that
// are already imported, otherwise a normal on/off box.
func checkbox(imported, selected bool) string {
	switch {
	case imported:
		return metaStyle.Render("[✓]")
	case selected:
		return barStyle.Render("[x]")
	default:
		return "[ ]"
	}
}

// plural formats a count with its noun, adding an "s" for anything but one.
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
