package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/marioolf/crabby/internal/version"
)

var (
	sloganStyle = lipgloss.NewStyle().Faint(true)
	authorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// Header renders Crabby's shared, compact identity block: the crab-army logo,
// the name, the slogan, and author attribution. Every full-screen Bubble Tea
// screen renders this so the whole application feels like one thing. This is
// the single source of the ASCII art — do not duplicate it elsewhere.
func Header() string {
	rows := []string{
		logoStyle.Render("  _~_      _~_      _~_"),
		logoStyle.Render("__(o )>  __(o )>  __(o )>"),
		"",
		titleStyle.Render("C R A B B Y"),
		sloganStyle.Render("Prepare projects."),
		sloganStyle.Render("Organize sessions."),
		sloganStyle.Render("Just one terminal."),
		"",
		authorStyle.Render("by " + version.Author + " · " + version.Repo),
	}

	var b strings.Builder
	for i, r := range rows {
		b.WriteString(centered.Render(r))
		if i < len(rows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
