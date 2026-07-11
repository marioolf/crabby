package windows

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		`C:\Users\mario`: `'C:\Users\mario'`,
		"my proj":        `'my proj'`,
		"it's":           `'it'\''s'`,
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInnerScriptConvertsAndForwardsArgs(t *testing.T) {
	got := innerScript(`C:\Users\mario\work`, []string{"attach", "my proj"})
	want := "d=$(wslpath -a 'C:\\Users\\mario\\work') || exit 1\n" +
		"cd \"$d\" || exit 1\n" +
		"exec crabby 'attach' 'my proj'\n"
	if got != want {
		t.Errorf("innerScript mismatch\n got: %q\nwant: %q", got, want)
	}
}

// remoteCommand must contain no double quotes, so wsl.exe cannot mis-parse it,
// and its base64 blob must decode back to the inner script.
func TestRemoteCommandHasNoDoubleQuotesAndRoundTrips(t *testing.T) {
	args := []string{"doctor"}
	cwd := `C:\Users\mario.lopez`
	remote := remoteCommand(cwd, args)

	if strings.Contains(remote, `"`) {
		t.Fatalf("remoteCommand contains a double quote (wsl.exe would mangle it): %q", remote)
	}

	m := regexp.MustCompile(`echo '([A-Za-z0-9+/=]+)'`).FindStringSubmatch(remote)
	if m == nil {
		t.Fatalf("could not find base64 blob in %q", remote)
	}
	decoded, err := base64.StdEncoding.DecodeString(m[1])
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if string(decoded) != innerScript(cwd, args) {
		t.Fatalf("decoded payload does not match innerScript")
	}
}
