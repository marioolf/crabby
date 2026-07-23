// Package banner renders Crabby's identity: a compact, pixel-art "CRABBY"
// wordmark plus the slogan and attribution. It is the single home of the logo —
// no screen draws it directly — so the brand looks the same everywhere and can
// be reshaped in one place.
//
// The wordmark is drawn from a small block font written in Go: no FIGlet, no
// external fonts, no binaries. Each letter is a 5-row pixel glyph, lit slightly
// brighter along its top two rows for a warm, arcade cabinet feel.
package banner

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/marioolf/crabby/internal/ui/theme"
)

// Slogan is Crabby's official one-line pitch, shown on the high-level screens
// (Dashboard, Welcome, About) and kept off the dense ones.
const Slogan = "Prepare projects. Organize sessions. Just one terminal."

// Attribution is the discreet author credit shown alongside the slogan.
const Attribution = "github.com/marioolf"

// Wordmark is the brand name the pixel logo spells.
const Wordmark = "CRABBY"

// glyphs is the 5-row pixel font for the six letters the wordmark needs. A '#'
// is a lit pixel, a space is dark; every glyph is exactly four columns wide so
// they align into a clean block.
var glyphs = map[rune][5]string{
	'C': {
		"####",
		"#   ",
		"#   ",
		"#   ",
		"####",
	},
	'R': {
		"### ",
		"#  #",
		"### ",
		"# # ",
		"#  #",
	},
	'A': {
		" ## ",
		"#  #",
		"####",
		"#  #",
		"#  #",
	},
	'B': {
		"### ",
		"#  #",
		"### ",
		"#  #",
		"### ",
	},
	'Y': {
		"#  #",
		"#  #",
		" ## ",
		" #  ",
		" #  ",
	},
}

const (
	litPixel = "█"
	glyphGap = " " // one dark column between letters
)

// rowStyles lights the wordmark from above: the top rows glow with the brighter
// brand colour, the lower rows settle into the resting coral, so the logo reads
// like an illuminated arcade sign rather than a flat block of text.
var rowStyles = [5]lipgloss.Style{
	lipgloss.NewStyle().Foreground(theme.PrimaryBright).Bold(true),
	lipgloss.NewStyle().Foreground(theme.PrimaryBright).Bold(true),
	lipgloss.NewStyle().Foreground(theme.Primary).Bold(true),
	lipgloss.NewStyle().Foreground(theme.Primary).Bold(true),
	lipgloss.NewStyle().Foreground(theme.Primary).Bold(true),
}

var (
	sloganStyle = lipgloss.NewStyle().Foreground(theme.TextMuted)
	attribStyle = lipgloss.NewStyle().Foreground(theme.TextDim).Italic(true)
)

// Logo returns the pixel wordmark on its own (5 lines), coloured but with no
// slogan or attribution — the compact identity for dense screens.
func Logo() string {
	var rows [5]strings.Builder
	letters := []rune(Wordmark)
	for li, r := range letters {
		g, ok := glyphs[r]
		if !ok {
			continue
		}
		for row := 0; row < 5; row++ {
			if li > 0 {
				rows[row].WriteString(glyphGap)
			}
			// Draw the row's pixels, then colour the whole row at once so the
			// styling adds no per-pixel escape noise.
			rows[row].WriteString(strings.ReplaceAll(g[row], "#", litPixel))
		}
	}
	var out []string
	for row := 0; row < 5; row++ {
		out = append(out, rowStyles[row].Render(rows[row].String()))
	}
	return strings.Join(out, "\n")
}

// Full returns the complete identity block — logo, slogan and attribution — for
// the high-level screens where the brand should be present in full.
func Full() string {
	return Logo() + "\n\n" +
		center(sloganStyle.Render(Slogan)) + "\n" +
		center(attribStyle.Render(Attribution))
}

// Compact is an alias for Logo, named for its use: the identity on dense views
// where the slogan would only cost space.
func Compact() string { return Logo() }

// Width is the display width of the identity block (its widest line), which the
// slogan sets. Screens use it to size dividers and centre their content.
func Width() int { return lipgloss.Width(Slogan) }

// center pads a single line to the identity width so slogan and attribution sit
// centred beneath the logo rather than flush-left under it.
func center(s string) string {
	w := Width()
	pad := (w - lipgloss.Width(s)) / 2
	if pad <= 0 {
		return s
	}
	return strings.Repeat(" ", pad) + s
}
