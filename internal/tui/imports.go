package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/marioolf/crabby/internal/importcmd"
	"github.com/marioolf/crabby/internal/initcmd"
)

// importStep is which step of the import flow is on screen: the machine scan, or
// the pick list of what it found.
type importStep int

const (
	importStepScanning importStep = iota
	importStepPick
)

// discoverDoneMsg carries the result of a background workspace scan. Scanning
// can take a moment, so it runs off the UI thread and reports back.
type discoverDoneMsg struct {
	found []importcmd.Workspace
	err   error
}

// startImport opens the import view and immediately begins scanning the machine
// — no path to type, no shell. The scan runs off the UI thread.
func (m model) startImport() (tea.Model, tea.Cmd) {
	m.screen = scrImport
	m.importStep = importStepScanning
	m.importList = nil
	m.importSel = nil
	m.importCursor = 0
	m.notice = ""
	return m, scanMachine
}

// scanMachine runs the default multi-location scan and reports the result.
func scanMachine() tea.Msg {
	found, err := importcmd.DiscoverDefault()
	return discoverDoneMsg{found, err}
}

func (m model) updateImport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.importStep {
	case importStepScanning:
		if msg.String() == "esc" {
			m.goDashboard()
		}
		return m, nil
	case importStepPick:
		switch msg.String() {
		case "esc", "q":
			m.goDashboard()
		case "up", "k":
			if m.importCursor > 0 {
				m.importCursor--
			}
		case "down", "j":
			if m.importCursor < len(m.importList)-1 {
				m.importCursor++
			}
		case " ":
			if len(m.importList) > 0 && !m.importList[m.importCursor].Imported {
				m.importSel[m.importCursor] = !m.importSel[m.importCursor]
			}
		case "a":
			m.setAllImports(true)
		case "n":
			m.setAllImports(false)
		case "r":
			m.importStep = importStepScanning
			m.importList = nil
			m.importSel = nil
			m.importCursor = 0
			return m, scanMachine
		case "i", "I", "enter":
			return m.performImport()
		}
	}
	return m, nil
}

// applyDiscover consumes a background scan result, moving to the pick list. A
// result that arrives after the user has left the scan is ignored.
func (m model) applyDiscover(msg discoverDoneMsg) (tea.Model, tea.Cmd) {
	if m.screen != scrImport || m.importStep != importStepScanning {
		return m, nil
	}
	if msg.err != nil {
		m.goDashboard()
		m.notice, m.noticeErr = msg.err.Error(), true
		return m, nil
	}
	m.importList = msg.found
	m.importSel = make([]bool, len(msg.found))
	for i, w := range msg.found {
		// Everything not-yet-imported starts checked, so adopting a machine full
		// of projects is one keypress.
		m.importSel[i] = !w.Imported
	}
	m.importCursor = 0
	m.importStep = importStepPick
	return m, nil
}

// setAllImports selects or clears every importable workspace at once.
func (m *model) setAllImports(on bool) {
	for i, w := range m.importList {
		if !w.Imported {
			m.importSel[i] = on
		}
	}
}

// importSelectedCount is how many not-yet-imported workspaces are checked.
func (m model) importSelectedCount() int {
	n := 0
	for i, w := range m.importList {
		if m.importSel[i] && !w.Imported {
			n++
		}
	}
	return n
}

// performImport registers each chosen workspace with init (no pack, so existing
// context files are left untouched) and returns to the dashboard.
func (m model) performImport() (tea.Model, tea.Cmd) {
	imported := 0
	for i, w := range m.importList {
		if !m.importSel[i] || w.Imported {
			continue
		}
		if _, err := initcmd.Init(w.Path, nil); err == nil {
			imported++
		}
	}
	m.goDashboard()
	m.refresh()
	if imported == 0 {
		m.notice, m.noticeErr = "Nothing imported.", false
	} else {
		m.notice, m.noticeErr = fmt.Sprintf("Imported %s.", plural(imported, "workspace")), false
	}
	return m, nil
}

func (m model) viewImport() string {
	if m.importStep == importStepScanning {
		body := metaStyle.Render("Scanning your machine for Claude workspaces…") + "\n" +
			metaStyle.Render("(your WSL home and the common project folders on your Windows drives)")
		return m.frame("Import workspaces", body, actionKey("esc", "cancel"))
	}

	var b strings.Builder
	if len(m.importList) == 0 {
		b.WriteString(metaStyle.Render("No Claude workspaces found.") + "\n\n")
		b.WriteString(metaStyle.Render("A workspace is any folder with a CLAUDE.md.") + "\n")
		footer := actionKey("r", "rescan") + "   " + actionKey("esc", "back")
		return m.frame("Import workspaces", b.String(), footer)
	}

	b.WriteString(headerStyle.Render(fmt.Sprintf("Found %s", plural(len(m.importList), "Claude workspace"))))
	b.WriteString(metaStyle.Render(fmt.Sprintf("   ·   Selected: %d", m.importSelectedCount())))
	b.WriteString("\n\n")

	start, end := window(len(m.importList), m.importCursor, m.importListHeight())
	if start > 0 {
		b.WriteString(metaStyle.Render("  ↑ more") + "\n")
	}
	for i := start; i < end; i++ {
		w := m.importList[i]
		bar := "  "
		name := nameStyle.Render(w.Name)
		if i == m.importCursor {
			bar = barStyle.Render("▌ ")
			name = selNameStyle.Render(w.Name)
		}
		b.WriteString(bar + importCheckbox(w.Imported, m.importSel[i]) + " " + name + "\n")
		detail := w.Path
		if w.Imported {
			detail = "✔ already imported  ·  " + w.Path
		} else if w.Branch != "" {
			detail = w.Branch + "  ·  " + w.Path
		}
		b.WriteString("      " + metaStyle.Render(detail) + "\n")
	}
	if end < len(m.importList) {
		b.WriteString(metaStyle.Render("  ↓ more") + "\n")
	}

	footer := strings.Join([]string{
		actionKey("↑↓", "move"), actionKey("space", "toggle"), actionKey("a", "all"),
		actionKey("n", "none"), actionKey("i", "import"), actionKey("esc", "back"),
	}, "   ")
	return m.frame("Import workspaces", b.String(), footer)
}

// importListHeight is how many workspaces to show at once, leaving room for the
// banner, header and footer.
func (m model) importListHeight() int {
	h := m.height
	if h <= 0 {
		h = 24
	}
	// Each workspace uses two lines (name + detail); reserve the chrome.
	rows := (h - 16) / 2
	if rows < 3 {
		rows = 3
	}
	return rows
}

// importCheckbox renders a row's selection state: a locked tick for workspaces
// already imported, otherwise a normal on/off box.
func importCheckbox(imported, selected bool) string {
	switch {
	case imported:
		return okStyle.Render("[✔]")
	case selected:
		return barStyle.Render("[x]")
	default:
		return "[ ]"
	}
}
