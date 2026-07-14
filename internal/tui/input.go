package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// textInput is a minimal single-line text editor. Crabby only ever asks for a
// short string (a task name, a path, a pack name), so a full input widget would
// be more dependency than the job needs — this covers typing, deleting, and
// moving the caret, and nothing more.
type textInput struct {
	value  string
	cursor int // caret position, in runes, within value
}

// newTextInput returns an input primed with an optional starting value, with
// the caret at the end so the user can keep typing straight away.
func newTextInput(value string) textInput {
	return textInput{value: value, cursor: len([]rune(value))}
}

// Value returns the current text.
func (t textInput) Value() string { return t.value }

// update applies a key to the input and returns the new state. It only handles
// editing keys; the caller deals with Enter/Esc.
func (t textInput) update(msg tea.KeyMsg) textInput {
	runes := []rune(t.value)
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace:
		in := msg.Runes
		if msg.Type == tea.KeySpace {
			in = []rune{' '}
		}
		runes = append(runes[:t.cursor], append(append([]rune{}, in...), runes[t.cursor:]...)...)
		t.cursor += len(in)
	case tea.KeyBackspace:
		if t.cursor > 0 {
			runes = append(runes[:t.cursor-1], runes[t.cursor:]...)
			t.cursor--
		}
	case tea.KeyDelete:
		if t.cursor < len(runes) {
			runes = append(runes[:t.cursor], runes[t.cursor+1:]...)
		}
	case tea.KeyLeft:
		if t.cursor > 0 {
			t.cursor--
		}
	case tea.KeyRight:
		if t.cursor < len(runes) {
			t.cursor++
		}
	case tea.KeyHome, tea.KeyCtrlA:
		t.cursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		t.cursor = len(runes)
	case tea.KeyCtrlU:
		runes = runes[:0]
		t.cursor = 0
	}
	t.value = string(runes)
	return t
}

var caretStyle = lipgloss.NewStyle().Reverse(true)

// view renders the field with a block caret, so the user can see where they are
// typing. An empty field shows the placeholder faintly.
func (t textInput) view(placeholder string) string {
	runes := []rune(t.value)
	if len(runes) == 0 && placeholder != "" {
		return caretStyle.Render(" ") + metaStyle.Render(placeholder)
	}
	var b strings.Builder
	for i := 0; i <= len(runes); i++ {
		if i == t.cursor {
			if i < len(runes) {
				b.WriteString(caretStyle.Render(string(runes[i])))
			} else {
				b.WriteString(caretStyle.Render(" "))
			}
			continue
		}
		if i < len(runes) {
			b.WriteString(string(runes[i]))
		}
	}
	return b.String()
}
