// Package windows implements the thin Windows-side wrapper.
//
// There is never a native Windows implementation of Crabby. The Windows
// executable only forwards the command into WSL, where the real crabby binary
// runs, while making sure the WSL shell starts in the same project directory
// the user is standing in on Windows.
package windows

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Forward runs the given crabby arguments inside WSL, in the WSL equivalent of
// the current Windows directory. It returns the child process exit code.
func Forward(args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "crabby: cannot determine current directory:", err)
		return 1
	}

	cmd := exec.Command("wsl", "bash", "-lc", remoteCommand(cwd, args))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "crabby: failed to forward into WSL:", err)
		fmt.Fprintln(os.Stderr, "  Is WSL installed and is 'crabby' installed inside it?")
		fmt.Fprintln(os.Stderr, "  Try:  wsl -- crabby doctor")
		return 1
	}
	return 0
}

// remoteCommand builds the single string handed to `wsl bash -lc`.
//
// wsl.exe rebuilds and re-parses the command line it receives, which mangles
// embedded quotes and can drop arguments. To make the payload immune to that,
// the real script — which converts the Windows directory with wslpath, changes
// into it, and execs crabby with the original arguments — is base64-encoded and
// decoded inside WSL. The command line wsl.exe sees therefore contains only the
// base64 alphabet plus a few shell operators, and no embedded double quotes.
//
// It runs via `bash <(...)` (process substitution) rather than a pipe so that
// crabby inherits the real terminal on stdin, which `crabby ps` and
// `crabby attach` need. The inner bash inherits PATH (including ~/.local/bin)
// from the outer login shell.
func remoteCommand(winCwd string, args []string) string {
	enc := base64.StdEncoding.EncodeToString([]byte(innerScript(winCwd, args)))
	return "bash <(echo '" + enc + "' | base64 -d)"
}

// innerScript is the bash script that actually runs inside WSL. Every dynamic
// value is single-quoted here (safe because this whole script is base64-encoded
// before it reaches wsl.exe).
func innerScript(winCwd string, args []string) string {
	var b strings.Builder
	b.WriteString("d=$(wslpath -a ")
	b.WriteString(shellQuote(winCwd))
	b.WriteString(") || exit 1\n")
	b.WriteString(`cd "$d" || exit 1` + "\n")
	b.WriteString("exec crabby")
	for _, a := range args {
		b.WriteByte(' ')
		b.WriteString(shellQuote(a))
	}
	b.WriteByte('\n')
	return b.String()
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes, so
// it is a single literal bash word.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
