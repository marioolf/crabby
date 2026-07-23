// Package theme is Crabby's design system: the single source of colour for the
// whole application. Every screen builds its styles from these tokens, so the
// interface reads as one coherent surface and the look can be tuned in one
// place rather than chased through dozens of scattered hex literals.
//
// The palette is retro-futurist and arcade: a dark, warm base with a coral red
// as the brand colour. Brand and status are deliberately separate ideas — the
// coral identifies Crabby, while the semantic colours (success, warning, error,
// info) speak about a session's state. They may look close, but they are
// different tokens with different jobs, so error is never "the brand red".
package theme

import "github.com/charmbracelet/lipgloss"

// Surfaces — the dark, layered background the interface sits on. Crabby only
// ever sets foreground colours in practice (so the user's own terminal
// background shows through), but these are kept as tokens so any surface that
// does need a fill draws from the same well.
const (
	Background = lipgloss.Color("#080B12") // the deepest layer
	Surface    = lipgloss.Color("#10141D") // cards, panels
	SurfaceAlt = lipgloss.Color("#171D29") // a raised surface / selection
	Border     = lipgloss.Color("#292F3A") // hairlines, unfocused chrome
)

// Text — the reading colours, from the brightest heading down to the faintest
// hint.
const (
	Text      = lipgloss.Color("#F2F0ED") // primary reading colour
	TextMuted = lipgloss.Color("#858894") // secondary / metadata
	TextDim   = lipgloss.Color("#555A66") // faint hints, disabled
)

// Brand — the coral identity of Crabby. Primary is the resting brand colour;
// PrimaryBright lifts it for emphasis (a lit edge, a focused accent); Secondary
// is the warm amber companion used sparingly for contrast.
const (
	Primary       = lipgloss.Color("#FF5A52")
	PrimaryBright = lipgloss.Color("#FF6B61")
	Secondary     = lipgloss.Color("#FF9F68")
)

// Semantic — status colours, a vocabulary distinct from the brand. Error is a
// clear red of its own rather than the brand coral, so "something is wrong"
// never reads as "this is Crabby".
const (
	Success = lipgloss.Color("#7DD6A3") // running / healthy
	Warning = lipgloss.Color("#FFD166") // waiting / needs attention
	Error   = lipgloss.Color("#E5484D") // failed / missing
	Info    = lipgloss.Color("#8AB4FF") // neutral information
)
