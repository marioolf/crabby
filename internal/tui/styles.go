package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/session"
	"github.com/marioolf/crabby/internal/ui/banner"
	"github.com/marioolf/crabby/internal/ui/theme"
)

// accent is Crabby's brand highlight, drawn from the design system so the coral
// identity is the same everywhere. Every style below is built from theme tokens
// — no colour is defined here directly.
const accent = theme.Primary

// Shared styles for every screen, so the whole app reads as one surface. They
// are the design system rendered into lipgloss: brand, text and semantic tokens
// mapped onto the roles Crabby's interface actually has.
var (
	dividerStyle = lipgloss.NewStyle().Foreground(theme.Border)
	metaStyle    = lipgloss.NewStyle().Foreground(theme.TextMuted)
	helpStyle    = lipgloss.NewStyle().Foreground(theme.TextMuted)
	nameStyle    = lipgloss.NewStyle().Foreground(theme.Text)
	selNameStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.Text)
	barStyle     = lipgloss.NewStyle().Foreground(theme.Primary)
	confirmStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.Warning)
	workingStyle = lipgloss.NewStyle().Foreground(theme.PrimaryBright)
	errorStyle   = lipgloss.NewStyle().Foreground(theme.Error)
	okStyle      = lipgloss.NewStyle().Foreground(theme.Success)

	keyStyle     = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)
	summaryStyle = lipgloss.NewStyle().Foreground(theme.TextMuted)
	headerStyle  = lipgloss.NewStyle().Foreground(theme.Text).Bold(true)
	titleStyle   = lipgloss.NewStyle().Foreground(theme.Text).Bold(true)
	agentStyle   = lipgloss.NewStyle().Foreground(theme.Secondary)

	// Panel chrome for the dashboard's three columns.
	paneTitleStyle   = lipgloss.NewStyle().Foreground(theme.TextMuted).Bold(true)
	paneFocusedTitle = lipgloss.NewStyle().Foreground(theme.Primary).Bold(true)

	stateStyles = map[session.State]lipgloss.Style{
		session.Running: lipgloss.NewStyle().Bold(true).Foreground(theme.Success),
		session.Waiting: lipgloss.NewStyle().Bold(true).Foreground(theme.Warning),
		session.Stopped: lipgloss.NewStyle().Foreground(theme.TextDim),
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
func divider() string { return dividerStyle.Render(strings.Repeat("─", banner.Width())) }

// actionKey styles a keybinding: the key in the accent colour, its description
// faint. It is the single vocabulary for every footer and help line.
func actionKey(k, desc string) string {
	return keyStyle.Render(k) + helpStyle.Render(" "+desc)
}

// Status symbols — a consistent visual language paired with colour so state is
// never carried by colour alone (readable for colour-blind users and in
// no-colour terminals). A working session pulses in the brand coral.
const (
	glyphRunning = "●"
	glyphWaiting = "○"
	glyphStopped = "■"
	glyphError   = "!"
)

// glyph picks the status symbol and colour for a task. Each state has its own
// shape: ● running, ○ waiting, ■ stopped. A working session (producing output)
// shows a filled coral dot so activity stands out from a plain attached one.
func glyph(s session.State, working bool) (string, lipgloss.Style) {
	if working && s != session.Stopped {
		return glyphRunning, workingStyle
	}
	switch s {
	case session.Running:
		return glyphRunning, stateStyles[session.Running]
	case session.Waiting:
		return glyphWaiting, stateStyles[session.Waiting]
	default:
		return glyphStopped, stateStyles[session.Stopped]
	}
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
