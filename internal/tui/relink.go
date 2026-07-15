package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/marioolf/crabby/internal/project"
)

// The relink flow points an existing workspace at a new folder — for when its
// directory has been moved or renamed. It reuses the directory input and Tab
// completion from the new-workspace flow.

func (m model) startRelink(p project.Project) (tea.Model, tea.Cmd) {
	m.screen = scrRelink
	m.relinkProj = p
	m.input = newTextInput(p.Path)
	_, m.wsMatches = dirCandidates(p.Path)
	m.notice = ""
	return m, nil
}

func (m model) updateRelink(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.goDashboard()
	case "tab":
		m.input = newTextInput(completePath(m.input.Value()))
		_, m.wsMatches = dirCandidates(m.input.Value())
	case "ctrl+o":
		m.notice, m.noticeErr = "Opening the Windows folder picker…", false
		return m, pickWindowsFolder
	case "enter":
		path := strings.TrimSpace(m.input.Value())
		if path == "" {
			return m, nil
		}
		if err := project.Relink(m.relinkProj.Name, expandHome(path)); err != nil {
			m.notice, m.noticeErr = err.Error(), true
			return m, nil
		}
		newName := m.relinkProj.Name
		m.goDashboard()
		m.refresh()
		// The workspace's name follows its new folder; select it by its path.
		for i, r := range m.rows {
			if r.proj.Path == expandHome(path) {
				m.wsCursor, m.taskCursor = i, 0
				newName = r.proj.Name
			}
		}
		m.notice, m.noticeErr = "Relinked to "+newName, false
	default:
		m.input = m.input.update(msg)
		_, m.wsMatches = dirCandidates(m.input.Value())
	}
	return m, nil
}

func (m model) viewRelink() string {
	body := fmt.Sprintf("Point %q at its new folder:\n\n  ", m.relinkProj.Name) +
		m.input.view("path to the moved/renamed folder")
	if len(m.wsMatches) > 0 {
		body += "\n\n  " + metaStyle.Render("subdirectories:") + "\n  " +
			metaStyle.Render(wrapMatches(m.wsMatches, 60))
	}
	footer := strings.Join([]string{
		actionKey("tab", "complete"), actionKey("ctrl+o", "windows folder"),
		actionKey("enter", "relink"), actionKey("esc", "cancel"),
	}, "   ")
	return m.frame("Relink workspace", body, footer)
}
