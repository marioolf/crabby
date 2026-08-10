package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/marioolf/crabby/internal/config"
	"github.com/marioolf/crabby/internal/importcmd"
	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/pack"
	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/session"
	"github.com/marioolf/crabby/internal/tmux"
)

// baseModel is a model with the services a View needs and a sane terminal size.
func baseModel() model {
	return model{
		cfg:          config.Default(),
		tmux:         tmux.New("tmux"),
		insights:     insights.New(),
		width:        120,
		height:       40,
		lastActivity: map[string]time.Time{},
		working:      map[string]bool{},
	}
}

// sendKey sends a single tea.KeyMsg to model.Update and returns the updated model and command.
func sendKey(m model, k tea.KeyMsg) (model, tea.Cmd) {
	res, cmd := m.Update(k)
	return res.(model), cmd
}

// typeString simulates typing a full string char-by-char into the model's active input.
func typeString(m model, s string) (model, tea.Cmd) {
	var lastCmd tea.Cmd
	for _, r := range s {
		var cmd tea.Cmd
		m, cmd = sendKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if cmd != nil {
			lastCmd = cmd
		}
	}
	return m, lastCmd
}

// TestEveryScreenRenders makes sure no screen's View panics, whatever sub-state
// it is in — the failure mode most likely to slip past the compiler.
func TestEveryScreenRenders(t *testing.T) {
	variants := []struct {
		name  string
		setup func(m *model)
	}{
		{"dashboard-empty", func(m *model) { m.screen = scrDashboard }},
		{"mission-empty", func(m *model) { m.screen = scrMission }},
		{"mission-cards", func(m *model) {
			m.screen = scrMission
			m.mc.cards = []missionCard{
				{label: "payments", state: session.Waiting, working: true, branch: "main",
					insight: insights.Insight{Model: "Opus 4.8", SessionTokens: 132000, Activity: insights.Editing},
					preview: []string{"Updated auth.go", "Running tests..."}},
				{label: "docs", state: session.Stopped},
			}
		}},
		{"new-workspace-dir", func(m *model) {
			m.screen = scrNewWorkspace
			m.wsStep = wsStepDir
			m.input = newTextInput("/tmp/x")
		}},
		{"new-workspace-pack", func(m *model) {
			m.screen = scrNewWorkspace
			m.wsStep = wsStepPack
			m.wsDir = "/tmp/x"
			m.packList = []pack.Pack{{Name: "go", Description: "Go"}}
		}},
		{"relink", func(m *model) {
			m.screen = scrRelink
			m.relinkProj = project.Project{Name: "payments", Path: "/repos/payments"}
			m.input = newTextInput("/repos/payments-new")
		}},
		{"import-scanning", func(m *model) {
			m.screen = scrImport
			m.importStep = importStepScanning
		}},
		{"import-pick", func(m *model) {
			m.screen = scrImport
			m.importStep = importStepPick
			m.importList = []importcmd.Workspace{{Name: "a", Branch: "main"}, {Name: "b", Imported: true}}
			m.importSel = []bool{true, false}
		}},
		{"new-task", func(m *model) {
			m.screen = scrNewTask
			m.input = newTextInput("")
		}},
		{"packs-empty", func(m *model) { m.screen = scrPacks }},
		{"packs-create", func(m *model) {
			m.screen = scrPacks
			m.packAction = packActionCreate
			m.input = newTextInput("new")
		}},
		{"packs-duplicate", func(m *model) {
			m.screen = scrPacks
			m.packs = []pack.Pack{{Name: "base"}}
			m.packAction = packActionDuplicate
			m.input = newTextInput("base-copy")
		}},
		{"packs-delete", func(m *model) {
			m.screen = scrPacks
			m.packs = []pack.Pack{{Name: "base"}}
			m.packAction = packActionConfirmDelete
		}},
		{"settings-menu", func(m *model) { m.screen = scrSettings }},
		{"settings-diagnostics", func(m *model) {
			m.screen = scrSettings
			m.settingsPane = settingsDiagnostics
		}},
		{"settings-about", func(m *model) {
			m.screen = scrSettings
			m.settingsPane = settingsAbout
		}},
		{"help", func(m *model) { m.screen = scrHelp }},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			m := baseModel()
			v.setup(&m)
			out := m.View()
			if strings.TrimSpace(out) == "" {
				t.Fatalf("%s rendered empty", v.name)
			}
		})
	}
}

// TestDashboardTransitions checks the keys that open each screen land on it.
func TestDashboardTransitions(t *testing.T) {
	cases := []struct {
		key  string
		want screen
	}{
		{"n", scrNewWorkspace},
		{"i", scrImport},
		{"p", scrPacks},
		{"s", scrSettings},
		{"?", scrHelp},
	}
	for _, c := range cases {
		m := baseModel()
		m.screen = scrDashboard
		next, _ := m.updateDashboard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(c.key)})
		got := next.(model).screen
		if got != c.want {
			t.Errorf("key %q -> screen %d, want %d", c.key, got, c.want)
		}
	}
}

func TestF12TogglesMissionControl(t *testing.T) {
	m := baseModel()
	m.screen = scrDashboard
	f12 := tea.KeyMsg{Type: tea.KeyF12}

	next, _ := m.handleKey(f12)
	if s := next.(model).screen; s != scrMission {
		t.Fatalf("F12 from dashboard -> screen %d, want mission", s)
	}
	back, _ := next.(model).handleKey(f12)
	if s := back.(model).screen; s != scrDashboard {
		t.Fatalf("F12 from mission -> screen %d, want dashboard", s)
	}
}

// --- R1.1 Model Msg Simulation (T1 & T2) -----------------------------------

func TestModelMsg_TextInputTypingAndDeleting(t *testing.T) {
	inp := newTextInput("")
	m := baseModel()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepDir
	m.input = inp

	m, _ = typeString(m, "abc d")
	if m.input.Value() != "abc d" {
		t.Fatalf("input value = %q, want 'abc d'", m.input.Value())
	}

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.input.Value() != "abc " {
		t.Fatalf("after Backspace value = %q, want 'abc '", m.input.Value())
	}

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.input.Value() != "" {
		t.Fatalf("after Ctrl+U value = %q, want empty string", m.input.Value())
	}
}

func TestModelMsg_DashboardKeyNavigation(t *testing.T) {
	m := baseModel()
	m.screen = scrDashboard
	m.rows = []wsRow{
		{
			proj: project.Project{Name: "p1", Path: "/tmp/p1"},
			tasks: []taskRow{
				{task: project.Task{Name: "t1"}},
				{task: project.Task{Name: "t2"}},
			},
		},
		{
			proj: project.Project{Name: "p2", Path: "/tmp/p2"},
		},
	}
	m.focus = paneWorkspaces

	if m.focus != paneWorkspaces {
		t.Fatalf("initial focus = %v, want paneWorkspaces", m.focus)
	}

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.wsCursor != 1 {
		t.Fatalf("wsCursor = %d, want 1", m.wsCursor)
	}

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.wsCursor != 0 {
		t.Fatalf("wsCursor = %d, want 0", m.wsCursor)
	}

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != paneTasks {
		t.Fatalf("focus after Tab = %v, want paneTasks", m.focus)
	}

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.taskCursor != 1 {
		t.Fatalf("taskCursor = %d, want 1", m.taskCursor)
	}

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focus != paneWorkspaces {
		t.Fatalf("focus after ShiftTab = %v, want paneWorkspaces", m.focus)
	}
}

func TestModelMsg_ScreenTransitions(t *testing.T) {
	cases := []struct {
		key  string
		want screen
	}{
		{"n", scrNewWorkspace},
		{"i", scrImport},
		{"p", scrPacks},
		{"s", scrSettings},
		{"?", scrHelp},
	}

	for _, c := range cases {
		t.Run("key_"+c.key, func(t *testing.T) {
			m := baseModel()
			m.screen = scrDashboard
			mRes, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(c.key)})
			if mRes.screen != c.want {
				t.Fatalf("key %q -> screen %v, want %v", c.key, mRes.screen, c.want)
			}
		})
	}
}

func TestModelMsg_MissionControlToggle(t *testing.T) {
	m := baseModel()
	m.screen = scrDashboard

	m1, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyF12})
	if m1.screen != scrMission {
		t.Fatalf("F12 from dashboard -> screen %v, want scrMission", m1.screen)
	}

	m2, _ := sendKey(m1, tea.KeyMsg{Type: tea.KeyF12})
	if m2.screen != scrDashboard {
		t.Fatalf("F12 from mission -> screen %v, want scrDashboard", m2.screen)
	}
}

func TestModelMsg_ConfirmationDialogDismissal(t *testing.T) {
	m := baseModel()
	m.screen = scrDashboard
	m.rows = []wsRow{
		{
			proj:  project.Project{Name: "p1"},
			tasks: []taskRow{{task: project.Task{Name: "t1"}, state: session.Running}},
		},
	}

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.confirm != confirmStop {
		t.Fatalf("confirm = %v, want confirmStop", m.confirm)
	}

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.confirm != confirmNone {
		t.Fatalf("confirm after 'n' = %v, want confirmNone", m.confirm)
	}
}

func TestModelMsg_ZeroDimensionsView(t *testing.T) {
	screens := []screen{
		scrDashboard, scrMission, scrNewWorkspace, scrRelink,
		scrImport, scrNewTask, scrPacks, scrSettings, scrHelp,
	}

	for _, s := range screens {
		t.Run("screen_"+fmt.Sprintf("%d", s), func(t *testing.T) {
			m := baseModel()
			m.width = 0
			m.height = 0
			m.screen = s
			out := m.View()
			_ = out
		})
	}
}

func TestModelMsg_CursorBoundaryClamping(t *testing.T) {
	m := baseModel()
	m.screen = scrDashboard
	m.rows = []wsRow{{}, {}}

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.wsCursor != 0 {
		t.Fatalf("wsCursor = %d, want 0", m.wsCursor)
	}

	for i := 0; i < 50; i++ {
		m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.wsCursor != len(m.rows)-1 {
		t.Fatalf("wsCursor = %d, want clamped max %d", m.wsCursor, len(m.rows)-1)
	}
}

func TestModelMsg_CtrlCQuitsFromAnyScreen(t *testing.T) {
	screens := []screen{
		scrDashboard, scrMission, scrNewWorkspace, scrRelink,
		scrImport, scrNewTask, scrPacks, scrSettings, scrHelp,
	}

	for _, s := range screens {
		t.Run("ctrl_c_screen_"+fmt.Sprintf("%d", s), func(t *testing.T) {
			m := baseModel()
			m.screen = s
			_, cmd := sendKey(m, tea.KeyMsg{Type: tea.KeyCtrlC})
			if cmd == nil {
				t.Fatalf("expected non-nil cmd for Ctrl+C on screen %v", s)
			}
			msg := cmd()
			if _, ok := msg.(tea.QuitMsg); !ok {
				t.Fatalf("cmd returned %T, want tea.QuitMsg", msg)
			}
		})
	}
}

func TestModelMsg_EmptyInputSubmit(t *testing.T) {
	m := baseModel()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepDir
	m.input = newTextInput("")

	mRes, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if mRes.wsStep != wsStepDir {
		t.Fatalf("expected step to remain wsStepDir when submitting empty dir, got %v", mRes.wsStep)
	}
}

func TestModelMsg_RapidTabKeyPathCompletion(t *testing.T) {
	m := baseModel()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepDir
	m.input = newTextInput("/nonexistent/path/xyz")

	for i := 0; i < 5; i++ {
		m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyTab})
	}
	if m.input.Value() != "/nonexistent/path/xyz" {
		t.Fatalf("input value = %q, want preserved original path", m.input.Value())
	}
}

// --- R1.5 Agent Command Cascade Unit Tests (T1 & T2) -----------------------

func uniqueSessionName(t *testing.T) string {
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, t.Name())
	return fmt.Sprintf("sess_%s_%d", clean, time.Now().UnixNano())
}

func cleanupSession(t *testing.T, m *model, sess string) {
	t.Cleanup(func() {
		if sess != "" && m != nil && m.tmux.HasSession(sess) {
			_ = m.tmux.KillSession(sess)
		}
	})
}

func TestAttachCascade_TaskCommandOverride(t *testing.T) {
	sess := uniqueSessionName(t)
	m := baseModel()
	cleanupSession(t, &m, sess)
	p := project.Project{Name: "p1", Path: t.TempDir(), AgentCommand: "missing-proj-cmd"}
	tk := project.Task{Name: "t1", Session: sess, AgentCommand: "missing-task-cmd --auto"}

	resModel, _ := m.attach(p, tk)
	res := resModel.(model)
	if !res.noticeErr {
		t.Fatalf("expected noticeErr=true for synthetic agent binary")
	}
	wantNotice := `Agent not found on PATH (looked for "missing-task-cmd --auto")`
	if res.notice != wantNotice {
		t.Fatalf("notice = %q, want %q", res.notice, wantNotice)
	}
}

func TestAttachCascade_ProjectCommandFallback(t *testing.T) {
	sess := uniqueSessionName(t)
	m := baseModel()
	cleanupSession(t, &m, sess)
	p := project.Project{Name: "p1", Path: t.TempDir(), AgentCommand: "missing-proj-cmd"}
	tk := project.Task{Name: "t1", Session: sess, AgentCommand: ""}

	resModel, _ := m.attach(p, tk)
	res := resModel.(model)
	if !res.noticeErr {
		t.Fatalf("expected noticeErr=true for synthetic agent binary")
	}
	wantNotice := `Agent not found on PATH (looked for "missing-proj-cmd")`
	if res.notice != wantNotice {
		t.Fatalf("notice = %q, want %q", res.notice, wantNotice)
	}
}

func TestAttachCascade_GlobalConfigFallback(t *testing.T) {
	sess := uniqueSessionName(t)
	m := baseModel()
	cleanupSession(t, &m, sess)
	m.cfg.ClaudeCommand = "missing-global-cmd"
	p := project.Project{Name: "p1", Path: t.TempDir(), AgentCommand: ""}
	tk := project.Task{Name: "t1", Session: sess, AgentCommand: ""}

	resModel, _ := m.attach(p, tk)
	res := resModel.(model)
	if !res.noticeErr {
		t.Fatalf("expected noticeErr=true for synthetic agent binary")
	}
	wantNotice := `Agent not found on PATH (looked for "missing-global-cmd")`
	if res.notice != wantNotice {
		t.Fatalf("notice = %q, want %q", res.notice, wantNotice)
	}
}

func TestAttachCascade_ExecutableNotFoundNotice(t *testing.T) {
	sess := uniqueSessionName(t)
	m := baseModel()
	cleanupSession(t, &m, sess)
	p := project.Project{Name: "p1", Path: t.TempDir()}
	tk := project.Task{Name: "t1", Session: sess, AgentCommand: "nonexistent-cmd"}

	resModel, _ := m.attach(p, tk)
	res := resModel.(model)
	if !res.noticeErr {
		t.Fatalf("expected noticeErr=true")
	}
	if !strings.Contains(res.notice, "Agent not found on PATH") {
		t.Fatalf("notice = %q, expected 'Agent not found on PATH'", res.notice)
	}
}

func TestAttachCascade_CommandWithArguments(t *testing.T) {
	sess := uniqueSessionName(t)
	m := baseModel()
	cleanupSession(t, &m, sess)
	p := project.Project{Name: "p1", Path: t.TempDir()}
	tk := project.Task{Name: "t1", Session: sess, AgentCommand: "missing-aider-cmd --yes-always --dark-mode"}

	resModel, _ := m.attach(p, tk)
	res := resModel.(model)
	wantNotice := `Agent not found on PATH (looked for "missing-aider-cmd --yes-always --dark-mode")`
	if res.notice != wantNotice {
		t.Fatalf("notice = %q, want %q", res.notice, wantNotice)
	}
}

func TestAttachCascade_ExistingSessionBypassesLookPath(t *testing.T) {
	sess := uniqueSessionName(t)
	m := baseModel()
	m.tmux = tmux.New("true")
	cleanupSession(t, &m, sess)

	p := project.Project{Name: "p1", Path: t.TempDir()}
	tk := project.Task{Name: "t1", Session: sess, AgentCommand: "missing-binary"}

	resModel, cmd := m.attach(p, tk)
	res := resModel.(model)
	if res.noticeErr {
		t.Fatalf("expected existing session to attach without LookPath error, got notice: %s", res.notice)
	}
	if cmd == nil {
		t.Fatalf("expected non-nil tea.Cmd for attaching existing session")
	}
}

func TestAttachCascade_EmptyGlobalConfigFallback(t *testing.T) {
	sess := uniqueSessionName(t)
	m := baseModel()
	cleanupSession(t, &m, sess)
	m.cfg.ClaudeCommand = ""
	p := project.Project{Name: "p1", Path: t.TempDir(), AgentCommand: ""}
	tk := project.Task{Name: "t1", Session: sess, AgentCommand: ""}

	resModel, _ := m.attach(p, tk)
	res := resModel.(model)
	if !res.noticeErr {
		t.Fatalf("expected noticeErr for empty command")
	}
}

func TestAttachCascade_MultiWordCommandPath(t *testing.T) {
	sess := uniqueSessionName(t)
	m := baseModel()
	cleanupSession(t, &m, sess)
	p := project.Project{Name: "p1", Path: t.TempDir()}
	tk := project.Task{Name: "t1", Session: sess, AgentCommand: "missing-claude-cmd --permission-mode auto"}

	resModel, _ := m.attach(p, tk)
	res := resModel.(model)
	wantNotice := `Agent not found on PATH (looked for "missing-claude-cmd --permission-mode auto")`
	if res.notice != wantNotice {
		t.Fatalf("notice = %q, want %q", res.notice, wantNotice)
	}
}

func TestAttachCascade_AfterAttachRestoreScreen(t *testing.T) {
	sess := uniqueSessionName(t)
	m := baseModel()
	cleanupSession(t, &m, sess)
	m.cfg.ClaudeCommand = "missing-claude-cmd"
	m.screen = scrMission
	p := project.Project{Name: "p1", Path: t.TempDir()}
	tk := project.Task{Name: "t1", Session: sess}

	m.sessions = map[string]tmux.Session{sess: {Attached: true}}

	resModel, _ := m.attach(p, tk)
	res := resModel.(model)
	if res.afterAttach != scrMission {
		t.Fatalf("afterAttach = %v, want scrMission", res.afterAttach)
	}

	res2, _ := res.Update(execDoneMsg{err: nil})
	mBack := res2.(model)
	if mBack.screen != scrMission {
		t.Fatalf("screen after execDoneMsg = %v, want scrMission", mBack.screen)
	}
}

func TestAttachCascade_ExecDoneMsgWithError(t *testing.T) {
	m := baseModel()
	m.screen = scrDashboard

	resModel, _ := m.Update(execDoneMsg{err: fmt.Errorf("tmux disconnected")})
	res := resModel.(model)
	if !res.noticeErr {
		t.Fatalf("expected noticeErr=true after execDoneMsg error")
	}
	if !strings.Contains(res.notice, "session ended with an error: tmux disconnected") {
		t.Fatalf("notice = %q, expected error notice", res.notice)
	}
}

// --- Tier 3: Pairwise Combinations (5 Tests) ------------------------------

func TestPairwise_WorkspaceAgentSelectionAndDisambiguation(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "claude", "opencode")

	dir1 := filepath.Join(t.TempDir(), "path1", "mono")
	dir2 := filepath.Join(t.TempDir(), "path2", "mono")
	_ = os.MkdirAll(dir1, 0o755)
	_ = os.MkdirAll(dir2, 0o755)

	m1 := baseModel()
	m1.screen = scrNewWorkspace
	m1.wsStep = wsStepAgent
	m1.wsDir = dir1
	m1.agentList = []AgentItem{{Name: "Claude Code", Command: "claude"}}
	m1.agentCursor = 0
	m1Res, _ := sendKey(m1, tea.KeyMsg{Type: tea.KeyEnter})
	if m1Res.noticeErr {
		t.Fatalf("ws1 creation error: %s", m1Res.notice)
	}

	m2 := baseModel()
	m2.screen = scrNewWorkspace
	m2.wsStep = wsStepAgent
	m2.wsDir = dir2
	m2.agentList = []AgentItem{{Name: "OpenCode", Command: "opencode"}}
	m2.agentCursor = 0
	m2Res, _ := sendKey(m2, tea.KeyMsg{Type: tea.KeyEnter})
	if m2Res.noticeErr {
		t.Fatalf("ws2 creation error: %s", m2Res.notice)
	}

	projs, _ := project.Load()
	if len(projs) != 2 {
		t.Fatalf("registered projects count = %d, want 2", len(projs))
	}
	if projs[0].Name != "mono" {
		t.Fatalf("projs[0].Name = %q, want mono", projs[0].Name)
	}
	if projs[1].Name != "mono-opencode" {
		t.Fatalf("projs[1].Name = %q, want mono-opencode", projs[1].Name)
	}
}

func TestPairwise_TaskAgentOverrideAndAttachCascade(t *testing.T) {
	sess := uniqueSessionName(t)
	m := baseModel()
	cleanupSession(t, &m, sess)
	p := project.Project{Name: "p1", Path: t.TempDir(), AgentCommand: "missing-proj-cmd"}
	tk := project.Task{Name: "t1", Session: sess, AgentCommand: "missing-task-override --auto"}

	resModel, _ := m.attach(p, tk)
	res := resModel.(model)
	if !strings.Contains(res.notice, "missing-task-override") {
		t.Fatalf("attach cascade notice = %q, expected 'missing-task-override' from task override", res.notice)
	}
}

func TestPairwise_AutoApproveToggleAndTaskCreation(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "claude")

	p := project.Project{Name: "myproj", Path: t.TempDir()}
	_ = project.Register(p)

	m := baseModel()
	m.screen = scrNewTask
	m.formProj = p
	m.newTaskName = "task-with-auto"
	m.newTaskStep = newTaskStepAgent
	m.agentList = []AgentItem{
		{Name: "Claude Code", Command: "claude", AutoApproveFlag: "--permission-mode auto"},
	}
	m.agentCursor = 0
	m.agentAutoApprove = false

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	mRes, cmd := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if mRes.noticeErr {
		t.Fatalf("task creation failed: %s", mRes.notice)
	}

	msg := cmd()
	openMsg := msg.(openTaskMsg)
	wantCmd := "claude --permission-mode auto"
	if openMsg.task.AgentCommand != wantCmd {
		t.Fatalf("openMsg.task.AgentCommand = %q, want %q", openMsg.task.AgentCommand, wantCmd)
	}
}

func TestPairwise_PackSelectionAndAgentSelection(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "claude")

	targetDir := t.TempDir()
	p := pack.Pack{
		Name:        "go-pack",
		Description: "Go Pack",
		Dir:         t.TempDir(),
	}

	m := baseModel()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepAgent
	m.wsDir = targetDir
	m.wsPack = &p
	m.agentList = []AgentItem{{Name: "Claude", Command: "claude"}}
	m.agentCursor = 0

	mRes, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if mRes.noticeErr {
		t.Fatalf("creation failed: %s", mRes.notice)
	}

	projs, _ := project.Load()
	if len(projs) != 1 {
		t.Fatalf("projects len = %d, want 1", len(projs))
	}
	if projs[0].AgentCommand != "claude" {
		t.Fatalf("AgentCommand = %q, want claude", projs[0].AgentCommand)
	}
}

func TestPairwise_WorkspaceDisambiguationAndTaskSessionNaming(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "claude", "opencode")

	dir1 := filepath.Join(t.TempDir(), "a", "app")
	dir2 := filepath.Join(t.TempDir(), "b", "app")
	_ = os.MkdirAll(dir1, 0o755)
	_ = os.MkdirAll(dir2, 0o755)

	m1 := baseModel()
	m1.screen = scrNewWorkspace
	m1.wsStep = wsStepAgent
	m1.wsDir = dir1
	m1.agentList = []AgentItem{{Name: "Claude", Command: "claude"}}
	m1Res, _ := sendKey(m1, tea.KeyMsg{Type: tea.KeyEnter})
	if m1Res.noticeErr {
		t.Fatalf("m1 creation error: %s", m1Res.notice)
	}

	m2 := baseModel()
	m2.screen = scrNewWorkspace
	m2.wsStep = wsStepAgent
	m2.wsDir = dir2
	m2.agentList = []AgentItem{{Name: "OpenCode", Command: "opencode"}}
	m2Res, _ := sendKey(m2, tea.KeyMsg{Type: tea.KeyEnter})
	if m2Res.noticeErr {
		t.Fatalf("m2 creation error: %s", m2Res.notice)
	}

	projs, _ := project.Load()
	if len(projs) < 2 {
		t.Fatalf("expected 2 projects, got %d", len(projs))
	}
	p2 := projs[1]

	mTask := baseModel()
	mTask.screen = scrNewTask
	mTask.formProj = p2
	mTask.newTaskName = "feature-x"
	mTask.newTaskStep = newTaskStepAgent
	mTask.agentList = []AgentItem{{Name: "OpenCode", Command: "opencode"}}

	_, cmd := sendKey(mTask, tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	openMsg := msg.(openTaskMsg)

	wantSession := "crabby_app-opencode_feature-x"
	if openMsg.task.Session != wantSession {
		t.Fatalf("task session = %q, want %q", openMsg.task.Session, wantSession)
	}
}

// --- Tier 4: Application Scenarios (4 Tests) -----------------------------

func TestScenario_CompleteWorkspaceAndMultiTaskLifecycle(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "claude", "opencode")

	targetDir := filepath.Join(t.TempDir(), "monorepo")
	_ = os.MkdirAll(targetDir, 0o755)

	m := baseModel()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepAgent
	m.wsDir = targetDir
	m.agentList = []AgentItem{{Name: "Claude Code", Command: "claude"}}
	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	m.refresh()
	if len(m.rows) != 1 {
		t.Fatalf("expected 1 workspace row, got %d", len(m.rows))
	}
	p := m.rows[0].proj

	mTask1 := baseModel()
	mTask1.screen = scrNewTask
	mTask1.formProj = p
	mTask1.newTaskName = "task-one"
	mTask1.newTaskStep = newTaskStepAgent
	mTask1.agentList = []AgentItem{{Name: "Claude", Command: "claude"}}
	mTask1.agentCursor = len(mTask1.agentList)
	mTask1, _ = sendKey(mTask1, tea.KeyMsg{Type: tea.KeyEnter})

	mTask2 := baseModel()
	mTask2.screen = scrNewTask
	mTask2.formProj = p
	mTask2.newTaskName = "task-two"
	mTask2.newTaskStep = newTaskStepAgent
	mTask2.agentList = []AgentItem{{Name: "OpenCode", Command: "opencode", AutoApproveFlag: "--auto"}}
	mTask2.agentCursor = 0
	mTask2, _ = sendKey(mTask2, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	mTask2, _ = sendKey(mTask2, tea.KeyMsg{Type: tea.KeyEnter})

	projs, _ := project.Load()
	if len(projs[0].Tasks) != 3 {
		t.Fatalf("project tasks count = %d, want 3", len(projs[0].Tasks))
	}
}

func TestScenario_RelinkAndAgentCascade(t *testing.T) {
	setupTestEnv(t)

	oldDir := filepath.Join(t.TempDir(), "old-path")
	newDir := filepath.Join(t.TempDir(), "new-path")
	_ = os.MkdirAll(oldDir, 0o755)
	_ = os.MkdirAll(newDir, 0o755)

	p := project.Project{Name: "myproj", Path: oldDir, AgentCommand: "claude"}
	_ = project.Register(p)

	_ = os.RemoveAll(oldDir)

	m := baseModel()
	m.screen = scrDashboard
	m.refresh()

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.screen != scrRelink {
		t.Fatalf("screen = %v, want scrRelink", m.screen)
	}

	m.input = newTextInput(newDir)
	mRes, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if mRes.screen != scrDashboard {
		t.Fatalf("screen after relink submit = %v, want scrDashboard", mRes.screen)
	}

	projs, _ := project.Load()
	if projs[0].Path != newDir {
		t.Fatalf("updated path = %q, want %q", projs[0].Path, newDir)
	}
}

func TestScenario_ImportWorkspacesAndAgentAssignment(t *testing.T) {
	setupTestEnv(t)

	dir1 := filepath.Join(t.TempDir(), "discovered1")
	dir2 := filepath.Join(t.TempDir(), "discovered2")
	_ = os.MkdirAll(dir1, 0o755)
	_ = os.MkdirAll(dir2, 0o755)

	m := baseModel()
	m.screen = scrImport
	m.importStep = importStepPick
	m.importList = []importcmd.Workspace{
		{Name: "discovered1", Path: dir1},
		{Name: "discovered2", Path: dir2},
	}
	m.importSel = []bool{true, false}
	m.importCursor = 0

	mRes, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if mRes.screen != scrDashboard {
		t.Fatalf("screen after import = %v, want scrDashboard", mRes.screen)
	}

	projs, _ := project.Load()
	if len(projs) != 1 || projs[0].Name != "discovered1" {
		t.Fatalf("imported projects = %v, want [discovered1]", projs)
	}
}

func TestScenario_MissionControlSessionMonitoringAndAttach(t *testing.T) {
	setupTestEnv(t)

	sess := uniqueSessionName(t)
	p := project.Project{Name: "myproj", Path: t.TempDir()}
	tk := project.Task{Name: "active-task", Session: sess}
	p.Tasks = []project.Task{tk}
	_ = project.Register(p)

	m := baseModel()
	m.tmux = tmux.New("true")
	cleanupSession(t, &m, sess)
	m.screen = scrMission
	m.sessions = map[string]tmux.Session{
		sess: {Attached: true, Activity: time.Now()},
	}

	mResModel, _ := m.attach(p, tk)
	mRes := mResModel.(model)

	if mRes.afterAttach != scrMission {
		t.Fatalf("afterAttach = %v, want scrMission", mRes.afterAttach)
	}

	mRestoredModel, _ := mRes.Update(execDoneMsg{err: nil})
	mRestored := mRestoredModel.(model)

	if mRestored.screen != scrMission {
		t.Fatalf("restored screen = %v, want scrMission", mRestored.screen)
	}
}
