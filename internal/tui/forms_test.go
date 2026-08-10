package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/marioolf/crabby/internal/pack"
	"github.com/marioolf/crabby/internal/project"
)

func setupMockPATH(t *testing.T, agents ...string) string {
	dir := t.TempDir()
	for _, name := range agents {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	origPATH := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPATH)
	return dir
}

func setupTestEnv(t *testing.T) string {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	return dir
}

func TestDirCandidatesAndCompletion(t *testing.T) {
	setupTestEnv(t)
	base := t.TempDir()
	for _, d := range []string{"alpha", "alps", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "afile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Listing a directory: only sub-directories, hidden ones excluded.
	if _, got := dirCandidates(base + string(os.PathSeparator)); !reflect.DeepEqual(got, []string{"alpha", "alps"}) {
		t.Fatalf("children = %v, want [alpha alps]", got)
	}

	// A prefix filters, and a dot prefix reveals hidden directories.
	if _, got := dirCandidates(filepath.Join(base, "alph")); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("prefix alph = %v, want [alpha]", got)
	}
	if _, got := dirCandidates(filepath.Join(base, ".hid")); !reflect.DeepEqual(got, []string{".hidden"}) {
		t.Fatalf("prefix .hid = %v, want [.hidden]", got)
	}

	// Completion: a single match gains a trailing separator to drill further.
	if got, want := completePath(filepath.Join(base, "alph")), filepath.Join(base, "alpha")+string(os.PathSeparator); got != want {
		t.Fatalf("complete alph = %q, want %q", got, want)
	}
	// Several matches extend to the longest common prefix only.
	if got, want := completePath(base+string(os.PathSeparator)), filepath.Join(base, "alp"); got != want {
		t.Fatalf("complete (all) = %q, want %q", got, want)
	}
}

// --- R1.2 Workspace Agent Selection Unit Tests (T1 & T2) -------------------

func TestWorkspaceAgent_StepTransitionHappyPath(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "claude", "opencode")

	m := baseModel()
	res, _ := m.startNewWorkspace()
	m = res.(model)

	targetDir := t.TempDir()
	m.input = newTextInput(targetDir)
	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.wsStep != wsStepAgent {
		t.Fatalf("expected wsStepAgent, got step %v", m.wsStep)
	}

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after workspace creation, got screen %v", m.screen)
	}
	if m.noticeErr {
		t.Fatalf("workspace creation error notice: %s", m.notice)
	}
}

func TestWorkspaceAgent_AutoApproveToggleSpace(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "claude")

	m := baseModel()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepAgent
	m.wsDir = t.TempDir()
	m.agentList = []AgentItem{
		{Name: "Claude Code", Command: "claude", AutoApproveFlag: "--permission-mode auto"},
	}
	m.agentCursor = 0
	m.agentAutoApprove = false

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if !m.agentAutoApprove {
		t.Fatalf("expected agentAutoApprove to be true after Space key")
	}

	mRes, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if mRes.noticeErr {
		t.Fatalf("creation failed: %s", mRes.notice)
	}

	projs, err := project.Load()
	if err != nil || len(projs) == 0 {
		t.Fatalf("failed loading projects: %v", err)
	}
	wantCmd := "claude --permission-mode auto"
	lastProj := projs[len(projs)-1]
	if lastProj.AgentCommand != wantCmd {
		t.Fatalf("project AgentCommand = %q, want %q", lastProj.AgentCommand, wantCmd)
	}
}

func TestWorkspaceAgent_SelectDefaultGlobalAgent(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "claude")

	m := baseModel()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepAgent
	m.wsDir = t.TempDir()
	m.agentList = []AgentItem{
		{Name: "Claude Code", Command: "claude", AutoApproveFlag: "--permission-mode auto"},
	}
	m.agentCursor = len(m.agentList) // Index for "Default global agent"
	m.agentAutoApprove = true

	mRes, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if mRes.noticeErr {
		t.Fatalf("workspace creation failed: %s", mRes.notice)
	}

	projs, _ := project.Load()
	if len(projs) == 0 {
		t.Fatalf("no project registered")
	}
	lastProj := projs[len(projs)-1]
	if lastProj.AgentCommand != "" {
		t.Fatalf("AgentCommand = %q, want empty string", lastProj.AgentCommand)
	}
}

func TestWorkspaceAgent_NoPacksBypassesPackStep(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "claude")

	m := baseModel()
	res, _ := m.startNewWorkspace()
	m = res.(model)
	m.packList = nil // No packs available

	targetDir := t.TempDir()
	m.input = newTextInput(targetDir)
	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.wsStep != wsStepAgent {
		t.Fatalf("expected step wsStepAgent when 0 packs available, got %v", m.wsStep)
	}
}

func TestWorkspaceAgent_EscNavigatesBackward(t *testing.T) {
	setupTestEnv(t)
	m := baseModel()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepAgent
	m.agentList = []AgentItem{{Name: "Claude", Command: "claude"}}

	// 1. With multiple packs -> Esc goes to wsStepPack
	m.packList = []pack.Pack{{Name: "pack1"}, {Name: "pack2"}}
	mEscPack, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	if mEscPack.wsStep != wsStepPack {
		t.Fatalf("expected wsStepPack after Esc, got %v", mEscPack.wsStep)
	}

	// 2. With 0 packs -> Esc goes to wsStepDir
	m.packList = nil
	mEscDir, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	if mEscDir.wsStep != wsStepDir {
		t.Fatalf("expected wsStepDir after Esc, got %v", mEscDir.wsStep)
	}
}

func TestWorkspaceAgent_NoAgentsInstalledFallback(t *testing.T) {
	setupTestEnv(t)
	t.Setenv("PATH", t.TempDir()) // Empty PATH -> 0 agents installed

	m := baseModel()
	res, _ := m.startNewWorkspace()
	m = res.(model)

	targetDir := t.TempDir()
	m.input = newTextInput(targetDir)
	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after bypassing agent step with 0 installed agents, got screen %v, step %v", m.screen, m.wsStep)
	}
	if m.noticeErr {
		t.Fatalf("workspace creation error: %s", m.notice)
	}
}

func TestWorkspaceAgent_SymlinkDirectoryHandling(t *testing.T) {
	setupTestEnv(t)

	targetDir := t.TempDir()
	realDir := t.TempDir()
	symlinkPath := filepath.Join(targetDir, ".claude")
	if err := os.Symlink(realDir, symlinkPath); err != nil {
		t.Fatal(err)
	}

	m := baseModel()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepAgent
	m.wsDir = targetDir

	mRes, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !mRes.noticeErr {
		t.Fatalf("expected notice error when .claude is a symlink")
	}
	if !strings.Contains(mRes.notice, "symlink") {
		t.Fatalf("notice = %q, expected symlink error message", mRes.notice)
	}
}

func TestWorkspaceAgent_SpecialCharsInDirPath(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "claude")

	targetDir := filepath.Join(t.TempDir(), "space test [v1] 🔥")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	m := baseModel()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepAgent
	m.wsDir = targetDir
	m.agentList = []AgentItem{{Name: "Claude", Command: "claude"}}

	mRes, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if mRes.noticeErr {
		t.Fatalf("workspace creation failed for path with special chars: %s", mRes.notice)
	}

	projs, _ := project.Load()
	found := false
	for _, p := range projs {
		if p.Path == targetDir {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("target path %q not found in registered projects: %v", targetDir, projs)
	}
}

func TestWorkspaceAgent_AgentListCursorWrapAround(t *testing.T) {
	setupTestEnv(t)
	m := baseModel()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepAgent
	m.agentList = []AgentItem{
		{Name: "Claude", Command: "claude"},
		{Name: "AGY", Command: "agy"},
	}
	m.agentCursor = 0

	for i := 0; i < 10; i++ {
		m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.agentCursor != len(m.agentList) {
		t.Fatalf("agentCursor = %d, want %d", m.agentCursor, len(m.agentList))
	}
}

func TestWorkspaceAgent_CreateWorkspaceFailureReset(t *testing.T) {
	setupTestEnv(t)

	m := baseModel()
	m.screen = scrNewWorkspace
	m.wsStep = wsStepAgent
	m.wsDir = filepath.Join(t.TempDir(), "non-existent-dir")

	mRes, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !mRes.noticeErr {
		t.Fatalf("expected noticeErr when creating workspace in non-existent directory")
	}
	if mRes.wsStep != wsStepDir {
		t.Fatalf("expected wsStep reset to wsStepDir after failure, got %v", mRes.wsStep)
	}
}

// --- R1.3 Task Agent Selection Unit Tests (T1 & T2) -----------------------

func TestTaskAgent_StepTransitionHappyPath(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "claude")

	p := project.Project{Name: "myproj", Path: t.TempDir()}
	_ = project.Register(p)

	m := baseModel()
	res, _ := m.startNewTask(p)
	m = res.(model)

	m, _ = typeString(m, "feature-auth")
	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.newTaskStep != newTaskStepAgent {
		t.Fatalf("expected newTaskStepAgent, got step %v", m.newTaskStep)
	}

	mRes, cmd := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if mRes.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after task creation, got %v", mRes.screen)
	}
	if cmd == nil {
		t.Fatalf("expected openTaskMsg command returned")
	}
}

func TestTaskAgent_AgentWithAutoApproveFlag(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "agy")

	p := project.Project{Name: "myproj", Path: t.TempDir()}
	_ = project.Register(p)

	m := baseModel()
	m.screen = scrNewTask
	m.formProj = p
	m.newTaskName = "db-migration"
	m.newTaskStep = newTaskStepAgent
	m.agentList = []AgentItem{
		{Name: "Antigravity CLI", Command: "agy", AutoApproveFlag: "--dangerously-skip-permissions"},
	}
	m.agentCursor = 0

	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	mRes, cmd := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	if mRes.noticeErr {
		t.Fatalf("task creation failed: %s", mRes.notice)
	}
	msg := cmd()
	openMsg := msg.(openTaskMsg)
	want := "agy --dangerously-skip-permissions"
	if openMsg.task.AgentCommand != want {
		t.Fatalf("task.AgentCommand = %q, want %q", openMsg.task.AgentCommand, want)
	}
}

func TestTaskAgent_SelectDefaultGlobalAgent(t *testing.T) {
	setupTestEnv(t)

	p := project.Project{Name: "myproj", Path: t.TempDir()}
	_ = project.Register(p)

	m := baseModel()
	m.screen = scrNewTask
	m.formProj = p
	m.newTaskName = "clean-code"
	m.newTaskStep = newTaskStepAgent
	m.agentList = []AgentItem{
		{Name: "Claude Code", Command: "claude"},
	}
	m.agentCursor = len(m.agentList)

	mRes, cmd := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if mRes.noticeErr {
		t.Fatalf("task creation failed: %s", mRes.notice)
	}
	msg := cmd()
	openMsg := msg.(openTaskMsg)
	if openMsg.task.AgentCommand != "" {
		t.Fatalf("task.AgentCommand = %q, want empty string", openMsg.task.AgentCommand)
	}
}

func TestTaskAgent_EscFromAgentReturnsToName(t *testing.T) {
	setupTestEnv(t)
	p := project.Project{Name: "myproj", Path: t.TempDir()}

	m := baseModel()
	m.screen = scrNewTask
	m.formProj = p
	m.newTaskName = "my-feature"
	m.newTaskStep = newTaskStepAgent
	m.input = newTextInput("my-feature")

	mEsc, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	if mEsc.newTaskStep != newTaskStepName {
		t.Fatalf("expected newTaskStepName after Esc in agent step, got %v", mEsc.newTaskStep)
	}
	if mEsc.input.Value() != "my-feature" {
		t.Fatalf("typed input = %q, want my-feature", mEsc.input.Value())
	}
}

func TestTaskAgent_ValidationFailureStaysOnNameStep(t *testing.T) {
	setupTestEnv(t)
	p := project.Project{Name: "myproj", Path: t.TempDir()}

	m := baseModel()
	res, _ := m.startNewTask(p)
	m = res.(model)

	m, _ = typeString(m, "invalid/task@name")
	mRes, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	if !mRes.noticeErr {
		t.Fatalf("expected notice error for invalid task name")
	}
	if mRes.newTaskStep != newTaskStepName {
		t.Fatalf("expected step to remain newTaskStepName, got %v", mRes.newTaskStep)
	}
}

func TestTaskAgent_DuplicateTaskNameRejection(t *testing.T) {
	setupTestEnv(t)

	p := project.Project{
		Name: "myproj",
		Path: t.TempDir(),
		Tasks: []project.Task{
			{Name: "existing-task", Session: "crabby_myproj_existing-task"},
		},
	}
	_ = project.Register(p)

	m := baseModel()
	m.screen = scrNewTask
	m.formProj = p
	m.newTaskName = "existing-task"
	m.newTaskStep = newTaskStepAgent

	mRes, _ := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !mRes.noticeErr {
		t.Fatalf("expected noticeErr when creating duplicate task name")
	}
	if !strings.Contains(mRes.notice, "already exists") {
		t.Fatalf("notice = %q, expected 'already exists'", mRes.notice)
	}
	if mRes.newTaskStep != newTaskStepName {
		t.Fatalf("expected reset to newTaskStepName, got %v", mRes.newTaskStep)
	}
}

func TestTaskAgent_SpecialCharactersInTaskName(t *testing.T) {
	setupTestEnv(t)
	invalidInputs := []string{"feat/login", "task@1", "my task", "task#name", "task!"}
	for _, input := range invalidInputs {
		t.Run("invalid_"+input, func(t *testing.T) {
			if err := validateTaskName(input); err == nil {
				t.Fatalf("expected validateTaskName(%q) to return error, got nil", input)
			}
		})
	}
}

func TestTaskAgent_MaxLengthTaskName(t *testing.T) {
	setupTestEnv(t)
	longName := strings.Repeat("a", 256)
	if err := validateTaskName(longName); err != nil {
		t.Fatalf("expected 256-character valid name to pass validation, got err: %v", err)
	}
}

func TestTaskAgent_AgentAutoApproveToggleReset(t *testing.T) {
	setupTestEnv(t)
	setupMockPATH(t, "claude")
	p := project.Project{Name: "myproj", Path: t.TempDir()}

	m := baseModel()
	m.agentAutoApprove = true
	res, _ := m.startNewTask(p)
	mRes := res.(model)

	if mRes.agentAutoApprove {
		t.Fatalf("expected agentAutoApprove to be reset to false when starting new task")
	}
}

func TestTaskAgent_ProjectWithNoExistingTasks(t *testing.T) {
	setupTestEnv(t)

	p := project.Project{Name: "myproj", Path: t.TempDir(), Tasks: nil}
	_ = project.Register(p)

	m := baseModel()
	m.screen = scrNewTask
	m.formProj = p
	m.newTaskName = "first-task"
	m.newTaskStep = newTaskStepAgent

	mRes, cmd := sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if mRes.noticeErr {
		t.Fatalf("createTask failed: %s", mRes.notice)
	}
	if cmd == nil {
		t.Fatalf("expected openTaskMsg command")
	}
}
