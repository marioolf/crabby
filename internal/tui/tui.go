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
	uptime  time.Duration
	insight insights.Insight
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
			insight: m.insights.For(p.Path),
		}
		if present {
			prev, seen := m.lastActivity[p.Session]
			it.working = seen && info.Activity.After(prev)
			if it.working && !m.working[p.Session] && !m.firstLoad {
				risingEdge = true
			}
			if !info.Created.IsZero() {
				it.uptime = time.Since(info.Created)
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
	b.WriteString(Banner())
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
		return center(m.width, b.String())
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

		// What it's doing now, with how long ago Claude last wrote.
		activity := m.activityLabel(it)
		if ago := lastActivityAgo(it.insight); ago != "" {
			activity += metaStyle.Render("  ·  " + ago)
		}
		b.WriteString("    " + activity + "\n")

		// Factual metadata: only the parts we actually have.
		if meta := metaLine(it); meta != "" {
			b.WriteString("    " + metaStyle.Render(meta) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(divider())
	b.WriteString("\n")
	if summary := m.summaryLine(); summary != "" {
		b.WriteString(summary + "\n")
	}
	switch m.confirm {
	case confirmStop:
		b.WriteString(confirmStyle.Render(fmt.Sprintf("Stop \"%s\"? (y/n)", m.items[m.cursor].project.Name)))
		return center(m.width, b.String())
	case confirmRemove:
		b.WriteString(confirmStyle.Render(fmt.Sprintf("Remove \"%s\" from Crabby? Files are kept. (y/n)", m.items[m.cursor].project.Name)))
		return center(m.width, b.String())
	}
	b.WriteString(helpStyle.Render("enter open   n new   x stop   d remove"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("r refresh   q quit   ·   %s returns from a session", m.detachKey)))
	return center(m.width, b.String())
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

// activityLabel describes what a session is doing, combining tmux's live
// "working" signal with the transcript's last recorded step. It never invents
// state: with no transcript it falls back to the plain session state.
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

// summaryLine is the dashboard's global usage bar. Token totals appear only
// when transcripts actually provided them, never as a placeholder.
func (m model) summaryLine() string {
	if len(m.items) == 0 {
		return ""
	}
	var running, waiting, stopped, working, tokens int
	for _, it := range m.items {
		switch it.state {
		case session.Running:
			running++
		case session.Waiting:
			waiting++
		default:
			stopped++
		}
		if it.working {
			working++
		}
		tokens += it.insight.TodayTokens
	}
	parts := []string{fmt.Sprintf("%d workspaces", len(m.items))}
	if running > 0 {
		parts = append(parts, fmt.Sprintf("%d running", running))
	}
	if waiting > 0 {
		parts = append(parts, fmt.Sprintf("%d waiting", waiting))
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
	return metaStyle.Render(strings.Join(parts, "   ·   "))
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
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
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
