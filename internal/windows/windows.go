// Package windows implements the thin Windows-side wrapper.
//
// There is never a native Windows implementation of Crabby. The Windows
// executable only forwards the command into WSL, where the real crabby binary
// runs, while making sure the WSL shell starts in the same project directory
// the user is standing in on Windows.
package windows

import (
	"fmt"
	"os"
	"os/exec"
)

// remoteScript converts the Windows working directory (passed as $1) into a
// WSL path, changes into it, and execs the real crabby with the remaining
// arguments ($@). It runs inside a login shell so ~/.local/bin is on PATH.
//
// Everything variable — the path and the user's arguments — is passed as
// separate positional parameters rather than interpolated into this string,
// so there is no shell-quoting to get wrong.
const remoteScript = `dir="$(wslpath -a "$1")" || exit 1; shift; cd "$dir" || exit 1; exec crabby "$@"`

// Forward runs the given crabby arguments inside WSL, in the WSL equivalent of
// the current Windows directory. It returns the child process exit code.
func Forward(args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "crabby: cannot determine current directory:", err)
		return 1
	}

	cmd := exec.Command("wsl", wslArgs(cwd, args)...)
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

// wslArgs builds the argument vector for wsl.exe:
//
//	wsl bash -lc <script> crabby <winCwd> <args...>
//
// The token after the script becomes $0; winCwd becomes $1; the rest are $2…,
// which become "$@" after the script shifts off the directory.
func wslArgs(winCwd string, args []string) []string {
	out := []string{"bash", "-lc", remoteScript, "crabby", winCwd}
	return append(out, args...)
}
