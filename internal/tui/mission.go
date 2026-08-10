package tui

import (
	"sort"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/session"
)

// Mission Control is a live overview of every AI Agent task at once: one card per
// task, each showing its status and a short preview of what its tmux session is
// currently showing. It is an observation surface — never a terminal emulator —
// and is kept entirely separate from the dashboard's rendering.

// missionState is Mission Control's own state, independent of the dashboard.
type missionState struct {
	cards    []missionCard
	cursor   int
	captures map[string]capture // session name -> cached pane preview
}

// capture is a cached pane preview, keyed on the session's activity time so a
// session is only re-captured when it has actually produced new output.
type capture struct {
	activity time.Time
	lines    []string
}

// missionCard is one task's live snapshot.
type missionCard struct {
	proj    project.Project
	task    project.Task
	label   string
	state   session.State
	working bool
	branch  string
	insight insights.Insight
	uptime  time.Duration
	preview []string
}

const (
	missionPreviewLines = 3
	missionCardBody     = missionPreviewLines + 4 // title, status, meta, divider
)

func (m model) startMission() (tea.Model, tea.Cmd) {
	m.screen = scrMission
	m.mc.cursor = 0
	m.notice = ""
	m.refresh() // make sure the session snapshot is current
	m.refreshMission()
	return m, nil
}

// refreshMission rebuilds the cards from the current projects and the session
// snapshot taken by refresh(), capturing a fresh pane preview only for sessions
// whose activity has advanced since last time — so idle tasks cost nothing and
// the view scales to dozens of them.
func (m *model) refreshMission() {
	if m.mc.captures == nil {
		m.mc.captures = map[string]capture{}
	}
	projects, err := project.Load()
	if err != nil {
		projects = nil
	}
	sessions := m.sessions
	if sessions == nil {
		sessions = m.tmux.ListSessions()
	}

	var cards []missionCard
	live := map[string]bool{}
	for _, p := range projects {
		ins := m.insights.For(p.Path)
		branch := p.Branch()
		for _, tk := range p.Tasks {
			info, present := sessions[tk.Session]
			c := missionCard{
				proj:    p,
				task:    tk,
				label:   windowLabel(p, tk),
				state:   session.Classify(present, info.Attached),
				branch:  branch,
				insight: ins,
			}
			if present {
				live[tk.Session] = true
				// refresh() has already computed the working flag for this tick.
				c.working = m.working[tk.Session]
				if !info.Created.IsZero() {
					c.uptime = time.Since(info.Created)
				}
				c.preview = m.previewFor(tk.Session, info.Activity)
			}
			cards = append(cards, c)
		}
	}
	// Forget previews for sessions that have gone away.
	for s := range m.mc.captures {
		if !live[s] {
			delete(m.mc.captures, s)
		}
	}

	sort.SliceStable(cards, func(i, j int) bool {
		if ri, rj := session.Rank(cards[i].state), session.Rank(cards[j].state); ri != rj {
			return ri < rj
		}
		return cards[i].label < cards[j].label
	})
	m.mc.cards = cards
	if m.mc.cursor >= len(cards) {
		m.mc.cursor = len(cards) - 1
	}
	if m.mc.cursor < 0 {
		m.mc.cursor = 0
	}
}

// previewFor returns the cached preview for a session, re-capturing only when
// its activity time has advanced.
func (m *model) previewFor(name string, activity time.Time) []string {
	if c, ok := m.mc.captures[name]; ok && c.activity.Equal(activity) {
		return c.lines
	}
	lines := previewLines(m.tmux.CapturePane(name))
	m.mc.captures[name] = capture{activity: activity, lines: lines}
	return lines
}

// previewLines reduces a captured pane to its last few useful lines. the Agent's
// UI fills the bottom of the pane with its input box and a persistent status
// line, so a naive "last N lines" shows only chrome; this strips box-drawing,
// blank and status lines and keeps the most recent lines that actually carry
// words — the closest reliable signal to what Agent last said or is being asked.
func previewLines(raw string) []string {
	if raw == "" {
		return nil
	}
	var useful []string
	for _, l := range strings.Split(raw, "\n") {
		if t := trimBox(l); usefulLine(t) {
			useful = append(useful, t)
		}
	}
	if len(useful) > missionPreviewLines {
		useful = useful[len(useful)-missionPreviewLines:]
	}
	return useful
}

// boxRunes are the frame characters Agent (and other TUIs) draw around content.
const boxRunes = "─│╭╮╰╯┌┐└┘├┤┬┴┼═║╔╗╚╝▌▐▕▏"

// trimBox strips surrounding box-drawing characters and spaces, leaving the
// content a boxed line actually holds.
func trimBox(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || strings.ContainsRune(boxRunes, r)
	})
}

// usefulLine reports whether a line carries real content rather than decoration
// or the Agent's persistent status chrome.
func usefulLine(t string) bool {
	if t == "" {
		return false
	}
	hasWord := false
	for _, r := range t {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			hasWord = true
			break
		}
	}
	if !hasWord {
		return false
	}
	low := strings.ToLower(t)
	for _, chrome := range []string{"mode on", "for shortcuts", "auto-accept", "esc to interrupt"} {
		if strings.Contains(low, chrome) {
			return false
		}
	}
	return true
}

func (m model) selectedCard() (missionCard, bool) {
	if m.mc.cursor < 0 || m.mc.cursor >= len(m.mc.cards) {
		return missionCard{}, false
	}
	return m.mc.cards[m.mc.cursor], true
}

func (m model) updateMission(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cols := m.missionCols()
	switch msg.String() {
	case "esc":
		m.goDashboard()
	case "q":
		return m, tea.Quit
	case "left", "h", "shift+tab":
		if m.mc.cursor > 0 {
			m.mc.cursor--
		}
	case "right", "l", "tab":
		if m.mc.cursor < len(m.mc.cards)-1 {
			m.mc.cursor++
		}
	case "up", "k":
		if m.mc.cursor-cols >= 0 {
			m.mc.cursor -= cols
		}
	case "down", "j":
		if m.mc.cursor+cols < len(m.mc.cards) {
			m.mc.cursor += cols
		}
	case "r":
		m.refresh()
		m.refreshMission()
	case "enter":
		if c, ok := m.selectedCard(); ok {
			proj, task := c.proj, c.task
			return m, func() tea.Msg { return openTaskMsg{proj, task} }
		}
	}
	return m, nil
}

// --- View ------------------------------------------------------------------

func (m model) viewMission() string {
	if len(m.mc.cards) == 0 {
		body := metaStyle.Render("No AI Agent tasks yet.\nCreate one with  t  on the dashboard, then F12 to watch them here.")
		return m.frame("Mission Control", body, m.missionFooter())
	}

	cols := m.missionCols()
	cw := m.missionCardWidth(cols)

	boxes := make([]string, len(m.mc.cards))
	for i, c := range m.mc.cards {
		boxes[i] = m.renderCard(c, i == m.mc.cursor, cw)
	}

	// Group into rows, then show only the rows that fit around the cursor.
	var rows []string
	for i := 0; i < len(boxes); i += cols {
		end := i + cols
		if end > len(boxes) {
			end = len(boxes)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, withGaps(boxes[i:end])...))
	}
	rowH := missionCardBody + 2 // + border
	maxRows := m.missionRowBudget() / rowH
	if maxRows < 1 {
		maxRows = 1
	}
	cursorRow := m.mc.cursor / cols
	start, end := window(len(rows), cursorRow, maxRows)

	var b strings.Builder
	if start > 0 {
		b.WriteString(metaStyle.Render("↑ more") + "\n")
	}
	b.WriteString(strings.Join(rows[start:end], "\n"))
	if end < len(rows) {
		b.WriteString("\n" + metaStyle.Render("↓ more"))
	}
	return m.frame("Mission Control", b.String(), m.missionFooter())
}

// missionCols is how many cards sit side by side, from the terminal width.
func (m model) missionCols() int {
	w := m.width
	if w <= 0 {
		w = 80
	}
	cols := w / 46
	if cols < 1 {
		cols = 1
	}
	if cols > 4 {
		cols = 4
	}
	if n := len(m.mc.cards); n > 0 && cols > n {
		cols = n
	}
	return cols
}

// missionCardWidth is the inner content width of each card for the column count.
func (m model) missionCardWidth(cols int) int {
	w := m.width
	if w <= 0 {
		w = 80
	}
	inner := (w - 4 - (cols - 1) - cols*4) / cols // outer margin, gaps, per-card chrome
	if inner < 22 {
		inner = 22
	}
	if inner > 56 {
		inner = 56
	}
	return inner
}

// missionRowBudget is the vertical space available for the card grid.
func (m model) missionRowBudget() int {
	h := m.height
	if h <= 0 {
		h = 24
	}
	avail := h - lipgloss.Height(Banner()) - 8
	if avail < missionCardBody+2 {
		avail = missionCardBody + 2
	}
	return avail
}

// renderCard draws one task card: name and status, a line of metadata, then a
// short live preview of its pane.
func (m model) renderCard(c missionCard, focused bool, cw int) string {
	// lipgloss counts the horizontal padding within Width, so the usable text
	// area is cw minus the 1-column padding on each side. Everything is drawn to
	// that width so nothing wraps inside the card.
	inner := cw - 2
	dot, dotStyle := glyph(c.state, c.working)
	label := truncate(c.label, inner-2)
	name := nameStyle.Render(label)
	if focused {
		name = selNameStyle.Render(label)
	}
	statusText, statusStyle := cardStatus(c)
	var lines []string
	lines = append(lines, dotStyle.Render(dot)+" "+name)
	lines = append(lines, statusStyle.Render(truncate(statusText, inner)))
	lines = append(lines, metaStyle.Render(truncate(cardMeta(c), inner)))
	lines = append(lines, dividerStyle.Render(strings.Repeat("─", inner)))
	for i := 0; i < missionPreviewLines; i++ {
		if i < len(c.preview) {
			lines = append(lines, metaStyle.Render(truncate(c.preview[i], inner)))
		} else if i == 0 && len(c.preview) == 0 {
			lines = append(lines, metaStyle.Render("—"))
		} else {
			lines = append(lines, "")
		}
	}

	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).
		Width(cw).Height(missionCardBody)
	if focused {
		box = box.BorderForeground(accent)
	} else {
		box = box.BorderForeground(lipgloss.Color("240"))
	}
	return box.Render(strings.Join(lines, "\n"))
}

// cardStatus is the one-line status as plain text plus the style to render it in
// — the live activity when working, otherwise the reliable tmux-derived state.
// Returning the two separately lets the caller truncate the text without
// corrupting the styling.
func cardStatus(c missionCard) (string, lipgloss.Style) {
	switch {
	case c.state == session.Stopped:
		return "Stopped", stateStyles[session.Stopped]
	case c.working:
		return workingActivity(c.insight), workingStyle
	case c.state == session.Running:
		return "Attached", stateStyles[session.Running]
	case c.insight.Activity == insights.Responding:
		return "Waiting for input", stateStyles[session.Waiting]
	default:
		return "Idle", metaStyle
	}
}

// cardMeta is the branch/uptime/model/tokens line, omitting anything unknown.
func cardMeta(c missionCard) string {
	var parts []string
	if c.branch != "" {
		parts = append(parts, sanitizeRender(c.branch))
	}
	if c.uptime > 0 {
		parts = append(parts, "up "+formatDuration(c.uptime))
	}
	if c.insight.Model != "" {
		parts = append(parts, sanitizeRender(c.insight.Model))
	}
	if c.insight.SessionTokens > 0 {
		parts = append(parts, formatTokens(c.insight.SessionTokens)+" tok")
	}
	return strings.Join(parts, "  ·  ")
}

func (m model) missionFooter() string {
	return strings.Join([]string{
		actionKey("↑↓←→", "move"), actionKey("enter", "open"),
		actionKey("r", "refresh"), actionKey("F12", "dashboard"), actionKey("q", "quit"),
	}, "   ")
}

// withGaps inserts a single-space spacer between boxes for JoinHorizontal.
func withGaps(boxes []string) []string {
	if len(boxes) <= 1 {
		return boxes
	}
	out := make([]string, 0, len(boxes)*2-1)
	for i, b := range boxes {
		if i > 0 {
			out = append(out, " ")
		}
		out = append(out, b)
	}
	return out
}

// truncate shortens plain text to at most w display columns, marking the cut
// with an ellipsis. It measures display width (not rune count), so wide glyphs
// that Agent prints — ✻, ※, box art — don't overflow the card and wrap.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
