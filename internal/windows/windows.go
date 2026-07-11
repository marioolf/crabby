// Package windows implements the thin Windows-side wrapper.
//
// There is never a native Windows implementation of Crabby. The Windows
// executable only converts the current directory into a WSL path and forwards
// the command into WSL, where the real crabby binary runs.
package windows

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Forward converts the current working directory to a WSL path and runs
//
//	wsl bash -lc "cd <converted_path> && crabby <args...>"
//
// so the Linux crabby binary sees the same project directory the user is
// standing in on Windows. It returns the child process exit code.
func Forward(args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "crabby: cannot determine current directory:", err)
		return 1
	}

	wslPath, err := toWSLPath(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "crabby: cannot convert path to WSL:", err)
		return 1
	}

	remote := fmt.Sprintf("cd %s && crabby %s", shellQuote(wslPath), strings.Join(quoteAll(args), " "))
	cmd := exec.Command("wsl", "bash", "-lc", remote)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "crabby: failed to forward into WSL:", err)
		return 1
	}
	return 0
}

// toWSLPath uses `wslpath` to convert a Windows path into its WSL equivalent.
func toWSLPath(winPath string) (string, error) {
	out, err := exec.Command("wsl", "wslpath", "-a", winPath).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// quoteAll shell-quotes every argument.
func quoteAll(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = shellQuote(a)
	}
	return out
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes,
// so it survives `bash -lc`.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
