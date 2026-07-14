// Package tui implements Crabby's terminal application.
//
// From v0.8 Crabby is TUI-first: `crabby` opens a persistent, multi-panel home
// screen and everything — creating workspaces and tasks, attaching to Claude,
// managing packs, diagnostics — happens inside it. The shell is no longer part
// of the normal workflow. A single Bubble Tea program owns the whole session;
// attaching to a Claude session is done through tea.ExecProcess so the screen is
// released and restored around it, without ever leaving Crabby.
package tui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/marioolf/crabby/internal/config"
	"github.com/marioolf/crabby/internal/doctor"
	"github.com/marioolf/crabby/internal/importcmd"
	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/pack"
	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/tmux"
)

const refreshEach = time.Second

// animEach is how often the banner redraws. It only rebuilds the view (no disk
// or tmux reads), so it can be brisk without being costly.
const animEach = 500 * time.Millisecond

// screen is which view the app is currently showing.
type screen int

const (
	scrDashboard screen = iota
	scrNewWorkspace
	scrImport
	scrNewTask
	scrPacks
	scrSettings
	scrHelp
)

// pane identifies the focused column of the dashboard.
type pane int

const (
	paneWorkspaces pane = iota
	paneTasks
)

// model is the whole application state. A single struct keeps the shared
// services (tmux, insights, config) and the transitions between screens in one
// place, which suits an app where every screen acts on the same workspaces.
type model struct {
	// Services, shared by every screen.
	cfg      config.Config
	tmux     tmux.Client
	insights *insights.Collector

	width, height int
	screen        screen

	// --- Dashboard ---------------------------------------------------------
	rows       []wsRow
	wsCursor   int
	taskCursor int
	focus      pane
	confirm    confirmKind
	// Live activity tracking, persisted across refreshes so a session that
	// starts working rings the bell exactly once.
	lastActivity map[string]time.Time
	working      map[string]bool
	firstLoad    bool
	ringBell     bool

	// --- New-workspace flow ------------------------------------------------
	wsStep     wsStep
	wsDir      string
	wsMatches  []string // sub-directories matching the typed path, for completion
	packList   []pack.Pack
	packCursor int

	// --- Import flow -------------------------------------------------------
	importStep   importStep
	importList   []importcmd.Workspace
	importSel    []bool
	importCursor int

	// --- New-task flow -----------------------------------------------------
	formProj project.Project

	// --- Packs screen ------------------------------------------------------
	packs       []pack.Pack
	packsCursor int
	packAction  packAction

	// input is the shared single-line editor, reused by the flows above. It is
	// reset whenever a flow begins.
	input textInput

	// --- Settings screen ---------------------------------------------------
	settingsCursor int
	settingsPane   settingsPane
	checks         []doctor.Check

	// anim is the banner animation counter, advanced by its own light tick.
	anim int

	// notice is a transient message shown in the status bar (errors, results).
	notice    string
	noticeErr bool
}

// Run starts the persistent application and blocks until the user quits. The
// collector is reused across the whole session so its incremental transcript
// cache survives returning from a Claude session.
func Run(cfg config.Config, t tmux.Client, coll *insights.Collector) error {
	if coll == nil {
		coll = insights.New()
	}
	m := model{
		cfg:          cfg,
		tmux:         t,
		insights:     coll,
		lastActivity: map[string]time.Time{},
		working:      map[string]bool{},
		firstLoad:    true,
	}
	m.refresh()
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// --- Messages --------------------------------------------------------------

type tickMsg time.Time

// animMsg drives the banner animation, separately from the data refresh so the
// crab can move without re-reading the filesystem or tmux.
type animMsg time.Time

// openTaskMsg asks the app to attach to a task, creating its session first if
// needed. Every "open a Claude session" path funnels through here.
type openTaskMsg struct {
	proj project.Project
	task project.Task
}

// execDoneMsg is delivered after an external program (a Claude attach, or an
// editor) finishes and the screen has been restored.
type execDoneMsg struct{ err error }

// noticeMsg sets the status-bar message.
type noticeMsg struct {
	text  string
	isErr bool
}

func tick() tea.Cmd {
	return tea.Tick(refreshEach, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func animTick() tea.Cmd {
	return tea.Tick(animEach, func(t time.Time) tea.Msg { return animMsg(t) })
}

// bellCmd rings the terminal bell out-of-band (BEL does not disturb the screen).
func bellCmd() tea.Msg {
	fmt.Fprint(os.Stderr, "\a")
	return nil
}

func (m model) Init() tea.Cmd { return tea.Batch(tick(), animTick()) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		m.refresh()
		if m.screen == scrDashboard && m.ringBell {
			return m, tea.Batch(tick(), bellCmd)
		}
		return m, tick()

	case animMsg:
		m.anim++
		return m, animTick()

	case noticeMsg:
		m.notice, m.noticeErr = msg.text, msg.isErr
		return m, nil

	case openTaskMsg:
		return m.attach(msg.proj, msg.task)

	case discoverDoneMsg:
		return m.applyDiscover(msg)

	case folderPickedMsg:
		if m.screen != scrNewWorkspace {
			return m, nil
		}
		switch {
		case msg.err != nil:
			m.notice, m.noticeErr = msg.err.Error(), true
		case msg.path == "":
			m.notice, m.noticeErr = "Folder picker cancelled.", false
		default:
			m.wsStep = wsStepDir
			m.input = newTextInput(msg.path)
			_, m.wsMatches = dirCandidates(msg.path)
			m.notice = ""
		}
		return m, nil

	case editMsg:
		return m.launchEditor(msg.dir)

	case execDoneMsg:
		m.screen = scrDashboard
		m.refresh()
		if msg.err != nil {
			m.notice, m.noticeErr = "session ended with an error: "+msg.err.Error(), true
		}
		return m, nil

	case editDoneMsg:
		// Editing a pack returns to the packs list, not the dashboard.
		m.screen = scrPacks
		m.packs, _ = pack.List()
		m.clampPacksCursor()
		if msg.err != nil {
			m.notice, m.noticeErr = "editor exited with an error: "+msg.err.Error(), true
		}
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey routes a keypress to the active screen.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case scrNewWorkspace:
		return m.updateNewWorkspace(msg)
	case scrImport:
		return m.updateImport(msg)
	case scrNewTask:
		return m.updateNewTask(msg)
	case scrPacks:
		return m.updatePacks(msg)
	case scrSettings:
		return m.updateSettings(msg)
	case scrHelp:
		return m.updateHelp(msg)
	default:
		return m.updateDashboard(msg)
	}
}

func (m model) View() string {
	switch m.screen {
	case scrNewWorkspace:
		return m.viewNewWorkspace()
	case scrImport:
		return m.viewImport()
	case scrNewTask:
		return m.viewNewTask()
	case scrPacks:
		return m.viewPacks()
	case scrSettings:
		return m.viewSettings()
	case scrHelp:
		return m.viewHelp()
	default:
		return m.viewDashboard()
	}
}

// attach opens a task's Claude session. Quick tmux calls (create, configure,
// rename) run inline; the blocking attach is handed to tea.ExecProcess so Bubble
// Tea releases the screen for Claude and restores the dashboard afterwards.
func (m model) attach(p project.Project, task project.Task) (tea.Model, tea.Cmd) {
	if !m.tmux.HasSession(task.Session) {
		if _, err := exec.LookPath(m.cfg.ClaudeCommand); err != nil {
			m.notice = fmt.Sprintf("Claude Code not found on PATH (looked for %q)", m.cfg.ClaudeCommand)
			m.noticeErr = true
			return m, nil
		}
		if err := m.tmux.NewSession(task.Session, p.Path, m.cfg.ClaudeCommand); err != nil {
			m.notice = fmt.Sprintf("could not start task %q: %v", task.Name, err)
			m.noticeErr = true
			return m, nil
		}
		_ = m.tmux.RenameWindow(task.Session, windowLabel(p, task))
	}
	// Best-effort branding + the prefix-free return key; a failure here only
	// means a plain status bar, so never block the attach on it.
	_ = m.tmux.Configure(m.cfg.DetachKey)
	m.notice, m.noticeErr = "", false
	ex := &attachExec{cmd: m.tmux.AttachCmd(task.Session)}
	return m, tea.Exec(ex, func(err error) tea.Msg { return execDoneMsg{err} })
}

// clearNormalBuffer is the escape sequence that clears the terminal's normal
// (non-alternate) screen and homes the cursor.
const clearNormalBuffer = "\033[2J\033[H"

// attachExec runs the tmux attach as a tea.Exec command, clearing the terminal's
// normal buffer either side of it. Attaching drops out of Crabby's alternate
// screen, and on detach tmux prints "[detached (from session …)]" to the normal
// buffer; without the clears that message accumulates in the shell and the
// buffer-switch shows the leftover session for a moment. Clearing turns both into
// a clean, blank transition.
type attachExec struct{ cmd *exec.Cmd }

func (a *attachExec) Run() error {
	fmt.Fprint(os.Stdout, clearNormalBuffer)
	err := a.cmd.Run()
	fmt.Fprint(os.Stdout, clearNormalBuffer)
	return err
}

// Bubble Tea wires its own I/O into a command only when unset; the attach
// command is already bound to the real terminal, so these keep that binding.
func (a *attachExec) SetStdin(r io.Reader) {
	if a.cmd.Stdin == nil {
		a.cmd.Stdin = r
	}
}

func (a *attachExec) SetStdout(w io.Writer) {
	if a.cmd.Stdout == nil {
		a.cmd.Stdout = w
	}
}

func (a *attachExec) SetStderr(w io.Writer) {
	if a.cmd.Stderr == nil {
		a.cmd.Stderr = w
	}
}

// windowLabel is what shows in the tmux status bar: just the workspace for the
// default task, workspace:task otherwise.
func windowLabel(p project.Project, task project.Task) string {
	if task.Name == project.DefaultTask {
		return p.Name
	}
	return p.Name + ":" + task.Name
}

// noticeLine renders the status-bar message, coloured by whether it is an error.
func (m model) noticeLine() string {
	if m.notice == "" {
		return ""
	}
	if m.noticeErr {
		return errorStyle.Render(m.notice)
	}
	return okStyle.Render(m.notice)
}

// goDashboard returns to the home screen, clearing any transient state.
func (m *model) goDashboard() {
	m.screen = scrDashboard
	m.confirm = confirmNone
}

// frame is the shared chrome for every non-dashboard screen: the identity
// banner, a title, the screen's body, and a footer of keys — so every screen
// reads as the same application. The status notice, if any, sits at the bottom.
func (m model) frame(title, body, footer string) string {
	var b strings.Builder
	b.WriteString(center(m.width, BannerFrame(m.anim)))
	b.WriteString("\n\n")
	b.WriteString(center(m.width, divider()))
	b.WriteString("\n\n")
	if title != "" {
		b.WriteString(center(m.width, titleStyle.Render(title)))
		b.WriteString("\n\n")
	}
	b.WriteString(blockCenter(m.width, body))
	b.WriteString("\n\n")
	b.WriteString(center(m.width, divider()))
	b.WriteString("\n")
	b.WriteString(center(m.width, footer))
	if n := m.noticeLine(); n != "" {
		b.WriteString("\n\n" + center(m.width, n))
	}
	return b.String()
}

// blockCenter centres a multi-line block as a unit: every line gets the same
// left margin, so the block sits in the middle while its internal left-aligned
// columns stay aligned (unlike per-line centring, which lets each line drift).
func blockCenter(width int, s string) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	blockWidth := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > blockWidth {
			blockWidth = w
		}
	}
	margin := (width - blockWidth) / 2
	if margin <= 0 {
		return s
	}
	pad := strings.Repeat(" ", margin)
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}
