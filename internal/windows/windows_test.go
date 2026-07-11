package windows

import "testing"

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"/mnt/c/work":     `'/mnt/c/work'`,
		"has space":       `'has space'`,
		"it's":            `'it'\''s'`,
		"/mnt/c/a b/it's": `'/mnt/c/a b/it'\''s'`,
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuoteAll(t *testing.T) {
	got := quoteAll([]string{"start", "payments"})
	want := []string{"'start'", "'payments'"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("quoteAll[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
