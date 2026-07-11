package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// The project's face. Colours are foreground-only so the user's real terminal
// background shows through.
var (
	crabStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B4A")).Bold(true)
	wordmarkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F5C4B3")).Bold(true)
	taglineStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#C9C9C2"))
	subStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
)

// Identity text, kept verbatim by design:
//   - the support line's separator is "·" (U+00B7) with surrounding spaces;
//   - "claude" is intentionally lowercase in the support line.
const (
	Wordmark = "C R A B B Y"
	Tagline  = "Many Claudes. One shell."
	Subtitle = "prepare projects · orchestrate claude · one terminal"
)

// crabArt is the crab. Its 3rd line contains a backtick, and Go raw strings
// cannot contain one, so the literal is split and the backtick concatenated as
// a normal string. Do NOT reformat or re-indent — the spacing is the art.
const crabArt = ` (\/)    (\/)
   \(o..o)/
   /` + "`" + `----'\`

// Banner renders Crabby's identity block: crab art, wordmark, tagline, and the
// support line, stacked and centred on the widest line (the support line).
// This is the single source of the ASCII art — do not duplicate it elsewhere.
func Banner() string {
	return lipgloss.JoinVertical(lipgloss.Center,
		crabStyle.Render(crabArt),
		"",
		wordmarkStyle.Render(Wordmark),
		taglineStyle.Render(Tagline),
		subStyle.Render(Subtitle),
	)
}

// BannerWidth is the display width of the banner (its widest line).
func BannerWidth() int { return lipgloss.Width(Banner()) }

// Splash centres the banner horizontally in the given terminal width (from
// tea.WindowSizeMsg) with a little vertical breathing room.
func Splash(width int) string {
	block := Banner()
	if width <= 0 {
		return block
	}
	return lipgloss.Place(width, lipgloss.Height(block)+2,
		lipgloss.Center, lipgloss.Center, block)
}
