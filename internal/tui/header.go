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
	Wordmark = "CRABBY"
	Tagline  = "Many Claudes. One shell."
	Subtitle = "prepare projects · orchestrate claude · one terminal"
)

// crabArt is the crab in its resting pose. Its 3rd line contains a backtick and
// its slashes are literal, so the string is written with escapes rather than a
// raw literal. Do NOT reformat — the spacing is the art. This is the canonical
// pose; the animation frames below are small variations of it.
const crabArt = " (\\/)    (\\/)\n" +
	"   \\(o..o)/\n" +
	"   /`----'\\"

// crabWave flips the claws, and crabBlink closes the eyes — the two frames that,
// alternated with the resting pose, make the crab look alive.
const (
	crabWave = " (/\\)    (/\\)\n" +
		"   \\(o..o)/\n" +
		"   /`----'\\"
	crabBlink = " (\\/)    (\\/)\n" +
		"   \\(-..-)/\n" +
		"   /`----'\\"
)

var crabFrames = []string{crabArt, crabWave, crabBlink}

// crabLane is the fixed width the crab is laid out in, so it can sway left and
// right inside it without moving the rest of the banner. It is the widest crab
// line (13) plus the sway range (4).
const crabLane = 17

// swayHome centres the crab in its lane for the still banner.
const swayHome = 2

// crabCycle is one loop of the animation: for each step, which frame to show and
// how far to sway it. A gentle triangle sway with an occasional claw wave and a
// blink — calm, not busy.
var crabCycle = []struct{ frame, sway int }{
	{0, 0}, {0, 1}, {1, 2}, {0, 3}, {0, 4}, {2, 3}, {1, 2}, {0, 1},
}

// layoutCrab places a crab pose at a horizontal offset within the fixed lane, so
// every frame is exactly crabLane wide and only the crab moves.
func layoutCrab(art string, sway int) string {
	return crabStyle.Width(crabLane).PaddingLeft(sway).Render(art)
}

// joinBanner stacks the crab, wordmark, tagline and support line, centred on the
// widest line (the support line).
func joinBanner(crab string) string {
	return lipgloss.JoinVertical(lipgloss.Center,
		crab,
		"",
		wordmarkStyle.Render(Wordmark),
		taglineStyle.Render(Tagline),
		subStyle.Render(Subtitle),
	)
}

// Banner renders Crabby's identity block in its resting pose. This is the still
// banner used outside the TUI (e.g. CLI help). Inside the app, BannerFrame gives
// the animated version.
func Banner() string {
	return joinBanner(layoutCrab(crabArt, swayHome))
}

// BannerFrame renders the banner at animation step t (any non-negative counter);
// successive values sway the crab and cycle its pose. The lane is fixed width, so
// the wordmark and support line never move.
func BannerFrame(t int) string {
	step := crabCycle[t%len(crabCycle)]
	return joinBanner(layoutCrab(crabFrames[step.frame], step.sway))
}

// BannerWidth is the display width of the banner (its widest line), stable across
// animation frames.
func BannerWidth() int { return lipgloss.Width(Banner()) }
