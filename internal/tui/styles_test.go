package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/marioolf/crabby/internal/session"
	"github.com/marioolf/crabby/internal/ui/theme"
)

// colorOf returns the hex string behind a style's foreground colour, or "" if it
// is not a plain colour.
func colorOf(c lipgloss.TerminalColor) string {
	if col, ok := c.(lipgloss.Color); ok {
		return string(col)
	}
	return ""
}

// TestStylesDrawFromTheme locks the design-system contract: the shared styles
// take their colours from the central tokens, not from stray literals. If a
// component is re-coloured with a raw hex value, this catches the drift.
func TestStylesDrawFromTheme(t *testing.T) {
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"bar", colorOf(barStyle.GetForeground()), string(theme.Primary)},
		{"key", colorOf(keyStyle.GetForeground()), string(theme.Primary)},
		{"error", colorOf(errorStyle.GetForeground()), string(theme.Error)},
		{"ok", colorOf(okStyle.GetForeground()), string(theme.Success)},
		{"meta", colorOf(metaStyle.GetForeground()), string(theme.TextMuted)},
		{"divider", colorOf(dividerStyle.GetForeground()), string(theme.Border)},
		{"running", colorOf(stateStyles[session.Running].GetForeground()), string(theme.Success)},
		{"waiting", colorOf(stateStyles[session.Waiting].GetForeground()), string(theme.Warning)},
		{"stopped", colorOf(stateStyles[session.Stopped].GetForeground()), string(theme.TextDim)},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s style foreground = %q, want theme token %q", c.name, c.got, c.want)
		}
	}

	if string(accent) != string(theme.Primary) {
		t.Errorf("accent = %q, want theme.Primary %q", accent, theme.Primary)
	}
}
