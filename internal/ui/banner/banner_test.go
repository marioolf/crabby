package banner

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestLogoShape checks the wordmark renders as a compact five-row block — the
// vertical budget the old crab art occupied — and never blows up into a giant
// banner.
func TestLogoShape(t *testing.T) {
	logo := Logo()
	if got := lipgloss.Height(logo); got != 5 {
		t.Fatalf("logo height = %d, want 5", got)
	}
	if w := lipgloss.Width(logo); w < 20 || w > 40 {
		t.Errorf("logo width = %d, want a compact 20–40 columns", w)
	}
}

// TestFullHasSloganAndAttribution makes sure the high-level identity block
// carries the brand's words.
func TestFullHasSloganAndAttribution(t *testing.T) {
	full := Full()
	if !strings.Contains(full, Slogan) {
		t.Error("Full() is missing the slogan")
	}
	if !strings.Contains(full, Attribution) {
		t.Error("Full() is missing the attribution")
	}
}

// TestIdentityText locks the brand strings so they cannot drift silently.
func TestIdentityText(t *testing.T) {
	if Wordmark != "CRABBY" {
		t.Errorf("Wordmark = %q", Wordmark)
	}
	if Slogan != "Prepare projects. Organize sessions. Just one terminal." {
		t.Errorf("Slogan = %q", Slogan)
	}
	if Attribution != "github.com/marioolf" {
		t.Errorf("Attribution = %q", Attribution)
	}
}
