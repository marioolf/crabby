package tui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// openFolderCmd opens a workspace's directory in the file manager, so files can
// be dropped into it without copying paths around. It runs off the UI thread and
// reports the result in the status bar.
func openFolderCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if err := openInFileManager(path); err != nil {
			return noticeMsg{"couldn't open the folder: " + err.Error(), true}
		}
		return noticeMsg{"Opened " + path + " in the file manager", false}
	}
}

// openInFileManager launches the platform's file manager at path. On WSL — the
// only environment Crabby targets — that is Windows Explorer, reached by
// converting the path with wslpath (a Linux path opens over \\wsl.localhost).
// explorer.exe is launched fire-and-forget: it returns a non-zero exit status
// even on success, so its result is deliberately not awaited. A couple of Linux
// openers are tried as a fallback for non-WSL use.
func openInFileManager(path string) error {
	if win, err := exec.Command("wslpath", "-w", path).Output(); err == nil {
		return exec.Command("explorer.exe", strings.TrimSpace(string(win))).Start()
	}
	for _, opener := range []string{"wslview", "xdg-open", "open"} {
		if _, err := exec.LookPath(opener); err == nil {
			return exec.Command(opener, path).Start()
		}
	}
	return fmt.Errorf("no file manager found")
}
