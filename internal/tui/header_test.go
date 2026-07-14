package tui

import "testing"

// The crab art is load-bearing: its exact spacing and the backtick on line 3
// are the design. This locks the bytes so no formatter or edit can drift it.
func TestCrabArtIsExact(t *testing.T) {
	want := " (\\/)    (\\/)\n" +
		"   \\(o..o)/\n" +
		"   /`----'\\"
	if crabArt != want {
		t.Fatalf("crab art changed.\n got: %q\nwant: %q", crabArt, want)
	}
}

func TestIdentityText(t *testing.T) {
	if Wordmark != "CRABBY" {
		t.Errorf("Wordmark = %q", Wordmark)
	}
	if Tagline != "Many Claudes. One shell." {
		t.Errorf("Tagline = %q", Tagline)
	}
	// "claude" is intentionally lowercase and the separator is U+00B7.
	if Subtitle != "prepare projects · orchestrate claude · one terminal" {
		t.Errorf("Subtitle = %q", Subtitle)
	}
}
