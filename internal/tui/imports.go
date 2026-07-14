package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/marioolf/crabby/internal/importcmd"
	"github.com/marioolf/crabby/internal/initcmd"
)

// importStep is which step of the import flow is on screen: choosing where to
// scan, or picking which discovered workspaces to adopt.
type importStep int

const (
	importStepPath importStep = iota
	importStepScanning
	importStepPick
)

// discoverDoneMsg carries the result of a background workspace scan. Scanning a
// large tree can take a moment, so it runs off the UI thread and reports back.
type discoverDoneMsg struct {
	found []importcmd.Workspace
	err   error
}

func (m model) startImport() (tea.Model, tea.Cmd) {
	cwd, _ := os.Getwd()
	m.screen = scrImport
	m.importStep = importStepPath
	m.input = newTextInput(cwd)
	m.importList = nil
	m.importSel = nil
	m.importCursor = 0
	m.notice = ""
	return m, nil
}

func (m model) updateImport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.importStep {
	case importStepPath:
		switch msg.String() {
		case "esc":
			m.goDashboard()
		case "enter":
			root := expandHome(strings.TrimSpace(m.input.Value()))
			if root == "" {
				return m, nil
			}
			// Scan off the UI thread — a deep tree must not freeze Crabby.
			m.importStep = importStepScanning
			m.notice = ""
			return m, func() tea.Msg {
				found, err := importcmd.Discover(root)
				return discoverDoneMsg{found, err}
			}
		default:
			m.input = m.input.update(msg)
		}
	case importStepScanning:
		// Only Esc does anything while scanning; a stale result is discarded in
		// the message handler if the user has already left.
		if msg.String() == "esc" {
			m.importStep = importStepPath
		}
	case importStepPick:
		switch msg.String() {
		case "esc":
			m.importStep = importStepPath
		case "up", "k":
			if m.importCursor > 0 {
				m.importCursor--
			}
		case "down", "j":
			if m.importCursor < len(m.importList)-1 {
				m.importCursor++
			}
		case " ":
			if !m.importList[m.importCursor].Imported {
				m.importSel[m.importCursor] = !m.importSel[m.importCursor]
			}
		case "a":
			m.toggleAllImports()
		case "enter":
			return m.performImport()
		}
	}
	return m, nil
}

// applyDiscover consumes a background scan result: it moves to the pick list, or
// reports an empty/failed scan. A result that arrives after the user has left
// the scan is ignored.
func (m model) applyDiscover(msg discoverDoneMsg) (tea.Model, tea.Cmd) {
	if m.screen != scrImport || m.importStep != importStepScanning {
		return m, nil
	}
	if msg.err != nil {
		m.importStep = importStepPath
		m.notice, m.noticeErr = msg.err.Error(), true
		return m, nil
	}
	if len(msg.found) == 0 {
		m.importStep = importStepPath
		m.notice, m.noticeErr = "No workspaces (folders with a CLAUDE.md) found there.", true
		return m, nil
	}
	m.importList = msg.found
	m.importSel = make([]bool, len(msg.found))
	for i, w := range msg.found {
		// Everything importable starts checked — adopting a folder should take
		// one keypress, not one per project.
		m.importSel[i] = !w.Imported
	}
	m.importCursor = 0
	m.importStep = importStepPick
	return m, nil
}

// toggleAllImports selects every importable workspace, or clears them all if
// they are already selected.
func (m *model) toggleAllImports() {
	allOn := true
	for i, w := range m.importList {
		if !w.Imported && !m.importSel[i] {
			allOn = false
			break
		}
	}
	for i, w := range m.importList {
		if !w.Imported {
			m.importSel[i] = !allOn
		}
	}
}

// performImport registers each chosen workspace with init (no pack, so existing
// CLAUDE.md files are left untouched) and returns to the dashboard.
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
	switch m.importStep {
	case importStepPath:
		body := "Scan a folder for workspaces (any folder with a CLAUDE.md):\n\n  " +
			m.input.view("path to scan")
		footer := actionKey("enter", "scan") + "   " + actionKey("esc", "cancel")
		return m.frame("Import workspaces", body, footer)
	case importStepScanning:
		body := metaStyle.Render("Scanning for workspaces… this can take a moment on a large folder.")
		return m.frame("Import workspaces", body, actionKey("esc", "cancel"))
	}

	var b strings.Builder
	b.WriteString(metaStyle.Render(fmt.Sprintf("Found %s.", plural(len(m.importList), "workspace"))) + "\n\n")
	for i, w := range m.importList {
		bar := "  "
		name := nameStyle.Render(w.Name)
		if i == m.importCursor {
			bar = barStyle.Render("▌ ")
			name = selNameStyle.Render(w.Name)
		}
		b.WriteString(bar + importCheckbox(w.Imported, m.importSel[i]) + " " + name)
		switch {
		case w.Imported:
			b.WriteString("  " + metaStyle.Render("already imported"))
		case w.Branch != "":
			b.WriteString("  " + metaStyle.Render(w.Branch))
		}
		b.WriteString("\n")
	}
	footer := strings.Join([]string{
		actionKey("space", "select"), actionKey("a", "all"),
		actionKey("enter", "import"), actionKey("esc", "back"),
	}, "   ")
	return m.frame("Import workspaces", b.String(), footer)
}

// importCheckbox renders a row's selection state: a locked tick for workspaces
// already imported, otherwise a normal on/off box.
func importCheckbox(imported, selected bool) string {
	switch {
	case imported:
		return metaStyle.Render("[✓]")
	case selected:
		return barStyle.Render("[x]")
	default:
		return "[ ]"
	}
}
