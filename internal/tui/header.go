package tui

import (
	"strings"

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
	Wordmark = "CRABBY"
	Tagline  = "Many Claudes. One shell."
	Subtitle = "prepare projects · orchestrate claude · one terminal"
)

// crabArt is the crab in its resting pose: claws on top, then face, body, and
// legs. Slashes are literal and the body line holds a backtick, so the strings
// are written with escapes rather than raw literals. Do NOT reformat — the
// spacing is the art. The animation poses below are variations of it.
const crabArt = " (\\/)    (\\/)\n" +
	"   \\(o..o)/\n" +
	"   /`----'\\\n" +
	"  //      \\\\"

// crabWave flips the claws; crabBlink closes the eyes. The legs stay put in
// every pose — only the claws and eyes move, so the crab reads as alive without
// the busier leg motion.
const (
	crabWave = " (/\\)    (/\\)\n" +
		"   \\(o..o)/\n" +
		"   /`----'\\\n" +
		"  //      \\\\"
	crabBlink = " (\\/)    (\\/)\n" +
		"   \\(-..-)/\n" +
		"   /`----'\\\n" +
		"  //      \\\\"
)

var crabFrames = []string{crabArt, crabWave, crabBlink}

// crabCycle is one loop of the animation: for each step, which pose to show and
// how far to sway it. It starts centred (sway 0) and swings symmetrically either
// side, so the crab both begins and rests centred over the wordmark.
var crabCycle = []struct{ frame, sway int }{
	{0, 0}, {0, 1}, {1, 2}, {0, 1}, {0, 0}, {2, -1}, {1, -2}, {0, -1},
}

// bannerWidth is the width of the widest line (the support line); every line is
// centred within it, and the crab sways around its centre.
func bannerWidth() int { return lipgloss.Width(Subtitle) }

// blockWidth is the width of the widest line in a multi-line block.
func blockWidth(s string) int {
	w := 0
	for _, l := range strings.Split(s, "\n") {
		if x := lipgloss.Width(l); x > w {
			w = x
		}
	}
	return w
}

// placeIn positions a (possibly styled) line at the given left offset within a
// line that is exactly width wide, right-padding to fill it. Every banner line
// is made the same width this way: lipgloss.PlaceHorizontal centres each line by
// its own width, so lines of differing widths would be re-centred independently
// and drift apart — a uniform width keeps them aligned as one block.
func placeIn(width, left int, s string) string {
	right := width - left - lipgloss.Width(s)
	if right < 0 {
		right = 0
	}
	if left < 0 {
		left = 0
	}
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// centerIn places a line centred within width (both sides padded).
func centerIn(width int, s string) string {
	return placeIn(width, (width-lipgloss.Width(s))/2, s)
}

// renderBanner assembles the identity block with the crab at a given pose and
// horizontal sway. Every line is exactly bannerWidth wide, so the block stays
// coherent when centred; the crab is centred over the wordmark at sway 0 and
// shifts by sway columns either side while the text lines never move.
func renderBanner(frame, sway int) string {
	bw := bannerWidth()
	crab := crabFrames[frame]
	left := (bw-blockWidth(crab))/2 + sway

	var lines []string
	for _, l := range strings.Split(crab, "\n") {
		lines = append(lines, placeIn(bw, left, crabStyle.Render(l)))
	}
	lines = append(lines,
		strings.Repeat(" ", bw),
		centerIn(bw, wordmarkStyle.Render(Wordmark)),
		centerIn(bw, taglineStyle.Render(Tagline)),
		centerIn(bw, subStyle.Render(Subtitle)),
	)
	return strings.Join(lines, "\n")
}

// Banner renders Crabby's identity block in its resting, centred pose. This is
// the still banner used outside the TUI (e.g. CLI help); inside the app,
// BannerFrame gives the animated version.
func Banner() string { return renderBanner(0, 0) }

// BannerFrame renders the banner at animation step t (any non-negative counter);
// successive values sway the crab and cycle its pose.
func BannerFrame(t int) string {
	step := crabCycle[t%len(crabCycle)]
	return renderBanner(step.frame, step.sway)
}

// BannerWidth is the display width of the banner (its widest line), stable across
// animation frames.
func BannerWidth() int { return bannerWidth() }
