package theme

import (
	"regexp"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// TestTokensAreValidHex guards the palette: every token must be a real 6-digit
// hex colour, so a typo can never ship a broken style.
func TestTokensAreValidHex(t *testing.T) {
	tokens := map[string]lipgloss.Color{
		"Background": Background, "Surface": Surface, "SurfaceAlt": SurfaceAlt,
		"Border": Border, "Text": Text, "TextMuted": TextMuted, "TextDim": TextDim,
		"Primary": Primary, "PrimaryBright": PrimaryBright, "Secondary": Secondary,
		"Success": Success, "Warning": Warning, "Error": Error, "Info": Info,
	}
	for name, c := range tokens {
		if !hexColor.MatchString(string(c)) {
			t.Errorf("token %s = %q is not a #RRGGBB colour", name, c)
		}
	}
}

// TestBrandAndErrorAreDistinct locks the design rule that the brand colour and
// the error colour are different concepts, not the same red reused. If someone
// collapses them, the interface loses the distinction between "this is Crabby"
// and "something failed".
func TestBrandAndErrorAreDistinct(t *testing.T) {
	if Primary == Error {
		t.Errorf("Primary and Error must be distinct tokens, both are %q", Primary)
	}
}
