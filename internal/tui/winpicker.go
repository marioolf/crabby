package tui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// folderPickedMsg carries the result of the native Windows folder picker: a
// chosen path (already converted to a WSL path), empty on cancel, or an error
// when the picker could not run.
type folderPickedMsg struct {
	path string
	err  error
}

// pickWindowsFolder opens Windows' folder dialog through PowerShell and converts
// the chosen path to a WSL path with wslpath. It runs as a plain command — the
// dialog is a GUI, so the terminal is never taken over — and reports back as a
// message, keeping the TUI responsive while the dialog is open.
//
// This is a convenience for folders on the Windows side; day-to-day paths under
// the Linux home are quicker to reach with Tab completion.
func pickWindowsFolder() tea.Msg {
	const ps = `Add-Type -AssemblyName System.Windows.Forms | Out-Null; ` +
		`$d = New-Object System.Windows.Forms.FolderBrowserDialog; ` +
		`if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Out.Write($d.SelectedPath) }`

	out, err := exec.Command("powershell.exe", "-NoProfile", "-Sta", "-Command", ps).Output()
	if err != nil {
		return folderPickedMsg{err: fmt.Errorf("the Windows folder picker isn't available here")}
	}
	win := strings.TrimSpace(string(out))
	if win == "" {
		return folderPickedMsg{} // cancelled
	}
	conv, err := exec.Command("wslpath", "-u", win).Output()
	if err != nil {
		return folderPickedMsg{err: fmt.Errorf("couldn't convert the Windows path %q", win)}
	}
	return folderPickedMsg{path: strings.TrimSpace(string(conv))}
}
