package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/marioolf/crabby/internal/pack"
)

// packAction is the sub-mode of the packs screen: browsing, naming a new or
// duplicated pack, or confirming a deletion.
type packAction int

const (
	packActionNone packAction = iota
	packActionCreate
	packActionDuplicate
	packActionConfirmDelete
)

// editMsg asks the app to open a pack directory in the user's editor.
type editMsg struct{ dir string }

// editDoneMsg is delivered after the editor exits, returning to the packs list.
type editDoneMsg struct{ err error }

func (m model) startPacks() (tea.Model, tea.Cmd) {
	m.screen = scrPacks
	m.packs, _ = pack.List()
	m.packsCursor = 0
	m.packAction = packActionNone
	m.notice = ""
	return m, nil
}

func (m *model) clampPacksCursor() {
	if m.packsCursor >= len(m.packs) {
		m.packsCursor = len(m.packs) - 1
	}
	if m.packsCursor < 0 {
		m.packsCursor = 0
	}
}

func (m model) updatePacks(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.packAction {
	case packActionCreate, packActionDuplicate:
		switch msg.String() {
		case "esc":
			m.packAction = packActionNone
		case "enter":
			return m.performPackName()
		default:
			m.input = m.input.update(msg)
		}
		return m, nil
	case packActionConfirmDelete:
		if s := msg.String(); s == "y" || s == "Y" {
			return m.performPackDelete()
		}
		m.packAction = packActionNone
		return m, nil
	}

	// Browsing.
	switch msg.String() {
	case "esc", "q":
		m.goDashboard()
	case "up", "k":
		if m.packsCursor > 0 {
			m.packsCursor--
		}
	case "down", "j":
		if m.packsCursor < len(m.packs)-1 {
			m.packsCursor++
		}
	case "n":
		m.packAction = packActionCreate
		m.input = newTextInput("")
	case "c":
		if len(m.packs) > 0 {
			m.packAction = packActionDuplicate
			m.input = newTextInput(m.packs[m.packsCursor].Name + "-copy")
		}
	case "e":
		if len(m.packs) > 0 {
			dir := m.packs[m.packsCursor].Dir
			return m, func() tea.Msg { return editMsg{dir} }
		}
	case "d":
		if len(m.packs) > 0 {
			m.packAction = packActionConfirmDelete
		}
	case "r":
		m.packs, _ = pack.List()
		m.clampPacksCursor()
	}
	return m, nil
}

// performPackName creates or duplicates a pack from the typed name.
func (m model) performPackName() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.input.Value())
	if name == "" {
		return m, nil
	}
	var (
		p   pack.Pack
		err error
	)
	switch m.packAction {
	case packActionDuplicate:
		p, err = pack.Duplicate(m.packs[m.packsCursor], name)
	default:
		p, err = pack.Create(name, "")
	}
	if err != nil {
		m.notice, m.noticeErr = err.Error(), true
		return m, nil
	}
	m.packAction = packActionNone
	m.packs, _ = pack.List()
	m.selectPackByName(p.Name)
	m.notice, m.noticeErr = "Created pack "+p.Name+" — press e to edit it", false
	return m, nil
}

func (m model) performPackDelete() (tea.Model, tea.Cmd) {
	p := m.packs[m.packsCursor]
	if err := pack.Delete(p); err != nil {
		m.notice, m.noticeErr = err.Error(), true
	} else {
		m.notice, m.noticeErr = "Deleted pack "+p.Name, false
	}
	m.packAction = packActionNone
	m.packs, _ = pack.List()
	m.clampPacksCursor()
	return m, nil
}

// launchEditor opens a directory in $EDITOR (falling back to vi) through
// tea.ExecProcess, so the screen is released for the editor and the packs list
// restored when it exits.
func (m model) launchEditor(dir string) (tea.Model, tea.Cmd) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	// Honour editors configured with flags, e.g. EDITOR="code -w".
	fields := strings.Fields(editor)
	args := append(fields[1:], dir)
	cmd := exec.Command(fields[0], args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return editDoneMsg{err} })
}

func (m *model) selectPackByName(name string) {
	for i, p := range m.packs {
		if p.Name == name {
			m.packsCursor = i
			return
		}
	}
}

func (m model) viewPacks() string {
	var b strings.Builder
	if len(m.packs) == 0 {
		b.WriteString(metaStyle.Render("No packs yet.") + "\n\n")
	}
	for i, p := range m.packs {
		b.WriteString(m.selectLine(i == m.packsCursor && m.packAction == packActionNone, p.Name))
		if p.Version != "" {
			b.WriteString(metaStyle.Render("  v" + p.Version))
		}
		b.WriteString("\n")
		if p.Description != "" {
			b.WriteString("    " + metaStyle.Render(p.Description) + "\n")
		}
	}
	b.WriteString("\n" + metaStyle.Render("packs live in "+pack.DisplayDir()))

	var footer string
	switch m.packAction {
	case packActionCreate:
		b.WriteString("\n\n" + "New pack name:  " + m.input.view("letters, digits, - and _"))
		footer = actionKey("enter", "create") + "   " + actionKey("esc", "cancel")
	case packActionDuplicate:
		src := m.packs[m.packsCursor].Name
		b.WriteString("\n\n" + fmt.Sprintf("Copy %q to:  ", src) + m.input.view("new pack name"))
		footer = actionKey("enter", "duplicate") + "   " + actionKey("esc", "cancel")
	case packActionConfirmDelete:
		p := m.packs[m.packsCursor]
		b.WriteString("\n\n" + confirmStyle.Render(fmt.Sprintf("Delete pack %q from disk? (y/n)", p.Name)))
		footer = actionKey("y", "delete") + "   " + actionKey("n", "cancel")
	default:
		footer = strings.Join([]string{
			actionKey("↑↓", "move"), actionKey("n", "new"), actionKey("c", "duplicate"),
			actionKey("e", "edit"), actionKey("d", "delete"), actionKey("esc", "back"),
		}, "   ")
	}
	return m.frame("Packs", b.String(), footer)
}
