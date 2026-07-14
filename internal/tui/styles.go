package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/session"
)

// accent is Crabby's primary highlight colour (the crab's orange).
const accent = lipgloss.Color("#FF6B4A")

// Shared styles for every screen, so the whole app reads as one surface.
var (
	dividerStyle = lipgloss.NewStyle().Faint(true)
	metaStyle    = lipgloss.NewStyle().Faint(true)
	helpStyle    = lipgloss.NewStyle().Faint(true)
	nameStyle    = lipgloss.NewStyle()
	selNameStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231"))
	barStyle     = lipgloss.NewStyle().Foreground(accent)
	confirmStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	workingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	okStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	keyStyle     = lipgloss.NewStyle().Foreground(accent).Bold(true)
	summaryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	headerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
	titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)

	// Panel chrome for the dashboard's three columns.
	paneTitleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true)
	paneBorderStyle  = lipgloss.NewStyle().Faint(true)
	paneFocusedTitle = lipgloss.NewStyle().Foreground(accent).Bold(true)

	stateStyles = map[session.State]lipgloss.Style{
		session.Running: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),  // green
		session.Waiting: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")), // yellow
		session.Stopped: lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("244")),
	}
)

// center places content horizontally in the given terminal width (0 = as-is).
func center(width int, content string) string {
	if width <= 0 {
		return content
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}

// divider spans the banner's width so headers line up with the identity block.
func divider() string { return dividerStyle.Render(strings.Repeat("─", BannerWidth())) }

// actionKey styles a keybinding: the key in the accent colour, its description
// faint. It is the single vocabulary for every footer and help line.
func actionKey(k, desc string) string {
	return keyStyle.Render(k) + helpStyle.Render(" "+desc)
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

// stateLabel is the short, reliable status word for a task, derived only from
// tmux (never from a transcript, which cannot be attributed to one task).
func stateLabel(s session.State, working bool) string {
	switch {
	case s == session.Stopped:
		return stateStyles[session.Stopped].Render("Stopped")
	case working:
		return workingStyle.Render("Working")
	case s == session.Running:
		return stateStyles[session.Running].Render("Attached")
	default:
		return metaStyle.Render("Idle")
	}
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

// plural formats a count with its noun, adding an "s" for anything but one.
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
