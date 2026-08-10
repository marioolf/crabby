package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/marioolf/crabby/internal/importcmd"
	"github.com/marioolf/crabby/internal/project"
)

// --- Tier 1: Feature Coverage (R2 Integration Coverage) --------------------

func TestTeatest_Dashboard_Navigation(t *testing.T) {
	h := newTestModelHarness(t)
	projDir := filepath.Join(h.HomeDir, "proj-nav")
	setupTestWorkspace(t, projDir)

	sendKeyRune(h.Tm, 'r')
	waitForOutput(t, h.Tm, "proj-nav")

	sendTab(h.Tm)
	time.Sleep(50 * time.Millisecond)

	sendShiftTab(h.Tm)
	time.Sleep(50 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard, got %v", finalM.screen)
	}
	if finalM.focus != paneWorkspaces {
		t.Fatalf("expected paneWorkspaces, got %v", finalM.focus)
	}
}

func TestTeatest_NewWorkspace_WizardFlow(t *testing.T) {
	h := newTestModelHarness(t)
	projDir := filepath.Join(h.HomeDir, "new-ws-target")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}

	sendKeyRune(h.Tm, 'n')
	waitForOutput(t, h.Tm, "New workspace")

	h.Tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.Tm.Type(projDir)
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard, got %v", finalM.screen)
	}
	if len(finalM.rows) != 1 {
		t.Fatalf("expected 1 workspace registered, got %d", len(finalM.rows))
	}
	if finalM.rows[0].proj.Name != "new-ws-target" {
		t.Fatalf("expected project name new-ws-target, got %s", finalM.rows[0].proj.Name)
	}
}

func TestTeatest_Import_DiscoveryAndPick(t *testing.T) {
	h := newTestModelHarness(t)
	dummyDir := filepath.Join(h.HomeDir, "discovered-proj")
	if err := os.MkdirAll(dummyDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(dummyDir, "CLAUDE.md"), []byte("# project"), 0o644)

	sendKeyRune(h.Tm, 'i')
	time.Sleep(100 * time.Millisecond)

	h.Tm.Send(discoverDoneMsg{
		found: []importcmd.Workspace{{Name: "discovered-proj", Path: dummyDir, Branch: "main"}},
		err:   nil,
	})
	time.Sleep(100 * time.Millisecond)

	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after import, got %v", finalM.screen)
	}
	if len(finalM.rows) != 1 {
		t.Fatalf("expected 1 workspace imported, got %d", len(finalM.rows))
	}
}

func TestTeatest_Relink_WorkspacePath(t *testing.T) {
	h := newTestModelHarness(t)
	oldDir := filepath.Join(h.HomeDir, "old-path")
	newDir := filepath.Join(h.HomeDir, "relinked-path")
	setupTestWorkspace(t, oldDir)
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sendKeyRune(h.Tm, 'r')
	waitForOutput(t, h.Tm, "old-path")

	sendKeyRune(h.Tm, 'e')
	waitForOutput(t, h.Tm, "Point")

	h.Tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.Tm.Type(newDir)
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after relink, got %v", finalM.screen)
	}
	if len(finalM.rows) > 0 && finalM.rows[0].proj.Path != newDir {
		t.Fatalf("expected relinked path %s, got %s", newDir, finalM.rows[0].proj.Path)
	}
}

func TestTeatest_NewTask_WizardAndAgentSelect(t *testing.T) {
	h := newTestModelHarness(t)
	projDir := filepath.Join(h.HomeDir, "task-proj")
	setupTestWorkspace(t, projDir)

	sendKeyRune(h.Tm, 'r')
	waitForOutput(t, h.Tm, "task-proj")

	sendKeyRune(h.Tm, 't')
	waitForOutput(t, h.Tm, "New task")

	h.Tm.Type("feature-auth")
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	sendEnter(h.Tm)
	waitForOutput(t, h.Tm, "Agent not found")

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard, got %v", finalM.screen)
	}
	projs, err := project.Load()
	if err != nil || len(projs) == 0 {
		t.Fatalf("failed to load projects: %v", err)
	}
	foundTask := false
	for _, tk := range projs[0].Tasks {
		if tk.Name == "feature-auth" {
			foundTask = true
			break
		}
	}
	if !foundTask {
		t.Fatalf("expected task 'feature-auth' to be created in project")
	}
}

func TestTeatest_Packs_CRUDOperations(t *testing.T) {
	h := newTestModelHarness(t)

	sendKeyRune(h.Tm, 'p')
	waitForOutput(t, h.Tm, "Packs")

	sendKeyRune(h.Tm, 'n')
	h.Tm.Type("custom-pack")
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	sendKeyRune(h.Tm, 'c')
	h.Tm.Type("custom-pack-copy")
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	sendKeyRune(h.Tm, 'd')
	sendKeyRune(h.Tm, 'y')
	time.Sleep(100 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrPacks {
		t.Fatalf("expected scrPacks, got %v", finalM.screen)
	}
}

func TestTeatest_Settings_DiagnosticsAndAbout(t *testing.T) {
	h := newTestModelHarness(t)

	sendKeyRune(h.Tm, 's')
	waitForOutput(t, h.Tm, "Settings")

	sendDown(h.Tm)
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	sendEsc(h.Tm)
	time.Sleep(50 * time.Millisecond)

	sendDown(h.Tm)
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.settingsPane != settingsAbout {
		t.Fatalf("expected settingsAbout pane, got %v", finalM.settingsPane)
	}
}

func TestTeatest_Help_ViewAndReturn(t *testing.T) {
	h := newTestModelHarness(t)

	sendKeyRune(h.Tm, '?')
	waitForOutput(t, h.Tm, "Help")

	sendEsc(h.Tm)
	time.Sleep(50 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after help, got %v", finalM.screen)
	}
}

func TestTeatest_MissionControl_GridAndCardSelection(t *testing.T) {
	h := newTestModelHarness(t)
	projDir := filepath.Join(h.HomeDir, "mc-proj")
	p := setupTestWorkspace(t, projDir)
	_, _ = createTask(p, "task-1", "")

	sendKeyRune(h.Tm, 'r')
	sendF12(h.Tm)
	waitForOutput(t, h.Tm, "Mission Control")

	sendKeyRune(h.Tm, 'l')
	sendKeyRune(h.Tm, 'h')
	sendKeyRune(h.Tm, 'j')
	sendKeyRune(h.Tm, 'k')

	sendF12(h.Tm)
	time.Sleep(50 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after F12 toggle, got %v", finalM.screen)
	}
}

// --- Tier 2: Boundary & Corner Cases ---------------------------------------

func TestTeatest_NewWorkspace_InvalidPath(t *testing.T) {
	h := newTestModelHarness(t)

	sendKeyRune(h.Tm, 'n')
	waitForOutput(t, h.Tm, "New workspace")

	h.Tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.Tm.Type("/proc/sys/crabby_invalid_dir/path")
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	finalM := h.Finish(t)
	if !finalM.noticeErr && finalM.wsStep != wsStepDir {
		t.Fatalf("expected noticeErr or wsStepDir on invalid path, noticeErr=%v, wsStep=%v", finalM.noticeErr, finalM.wsStep)
	}
}

func TestTeatest_NewTask_InvalidNameAndCollision(t *testing.T) {
	h := newTestModelHarness(t)
	projDir := filepath.Join(h.HomeDir, "task-invalid-proj")
	setupTestWorkspace(t, projDir)

	sendKeyRune(h.Tm, 'r')
	sendKeyRune(h.Tm, 't')
	waitForOutput(t, h.Tm, "New task")

	h.Tm.Type("invalid task!")
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	h.Tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.Tm.Type("main")
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	finalM := h.Finish(t)
	if !finalM.noticeErr {
		t.Fatalf("expected noticeErr on invalid or duplicate task name")
	}
}

func TestTeatest_Wizard_EscCancellationAtAllSteps(t *testing.T) {
	h := newTestModelHarness(t)

	sendKeyRune(h.Tm, 'n')
	sendEsc(h.Tm)
	time.Sleep(50 * time.Millisecond)

	sendKeyRune(h.Tm, 'p')
	sendKeyRune(h.Tm, 'n')
	sendEsc(h.Tm)
	time.Sleep(50 * time.Millisecond)
	sendEsc(h.Tm)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after ESC cancellations, got %v", finalM.screen)
	}
}

func TestTeatest_Dashboard_EmptyStateAndMissingDir(t *testing.T) {
	h := newTestModelHarness(t)
	waitForOutput(t, h.Tm, "No workspaces yet.")

	missingDir := filepath.Join(h.HomeDir, "temp-missing-ws")
	setupTestWorkspace(t, missingDir)
	if err := os.RemoveAll(missingDir); err != nil {
		t.Fatalf("failed to remove dir: %v", err)
	}

	sendKeyRune(h.Tm, 'r')
	time.Sleep(100 * time.Millisecond)

	sendKeyRune(h.Tm, 'o')
	time.Sleep(50 * time.Millisecond)

	finalM := h.Finish(t)
	if len(finalM.rows) > 0 && !finalM.rows[0].missing {
		t.Fatalf("expected row.missing to be true for deleted directory")
	}
}

func TestTeatest_TerminalResize_BoundaryDimensions(t *testing.T) {
	h := newTestModelHarness(t)

	sizes := []tea.WindowSizeMsg{
		{Width: 80, Height: 24},
		{Width: 200, Height: 60},
		{Width: 40, Height: 10},
	}

	for _, sz := range sizes {
		h.Tm.Send(sz)
		time.Sleep(50 * time.Millisecond)
	}

	finalM := h.Finish(t)
	view := finalM.View()
	if view == "" {
		t.Fatalf("expected non-empty view after terminal resizes")
	}
}

func TestTeatest_AgentSelection_NoInstalledAgentsFallback(t *testing.T) {
	h := newTestModelHarness(t)
	projDir := filepath.Join(h.HomeDir, "no-agent-ws")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sendKeyRune(h.Tm, 'n')
	waitForOutput(t, h.Tm, "New workspace")

	h.Tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.Tm.Type(projDir)
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after agent fallback, got %v", finalM.screen)
	}
}

func TestTeatest_Workspace_AgentSelectionWizard(t *testing.T) {
	h := newTestModelHarness(t)
	setupMockAgentBinary(t, h.HomeDir, "claude")

	projDir := filepath.Join(h.HomeDir, "agent-wizard-ws")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sendKeyRune(h.Tm, 'n')
	waitForOutput(t, h.Tm, "New workspace")

	h.Tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.Tm.Type(projDir)
	sendEnter(h.Tm)
	waitForOutput(t, h.Tm, "Choose an AI Agent")

	// Toggle auto-approve flag using space
	sendSpace(h.Tm)
	time.Sleep(50 * time.Millisecond)

	// Press enter to create workspace with selected agent
	sendEnter(h.Tm)
	waitForOutput(t, h.Tm, "Created workspace")

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard, got %v", finalM.screen)
	}
	if len(finalM.rows) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(finalM.rows))
	}
	if finalM.rows[0].proj.AgentCommand != "claude --permission-mode auto" {
		t.Fatalf("expected AgentCommand 'claude --permission-mode auto', got %q", finalM.rows[0].proj.AgentCommand)
	}
}

func TestTeatest_Task_AgentSelectionWizard(t *testing.T) {
	h := newTestModelHarness(t)
	setupMockAgentBinary(t, h.HomeDir, "claude")

	projDir := filepath.Join(h.HomeDir, "task-agent-wizard-ws")
	p := setupTestWorkspace(t, projDir)

	sendKeyRune(h.Tm, 'r')
	waitForOutput(t, h.Tm, p.Name)

	sendKeyRune(h.Tm, 't')
	waitForOutput(t, h.Tm, "New task")

	h.Tm.Type("feature-billing")
	sendEnter(h.Tm)
	waitForOutput(t, h.Tm, "Choose an AI Agent")

	// Toggle auto-approve flag using space
	sendSpace(h.Tm)
	time.Sleep(50 * time.Millisecond)

	// Submit task creation
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard, got %v", finalM.screen)
	}

	projs, err := project.Load()
	if err != nil || len(projs) == 0 {
		t.Fatalf("failed to load projects: %v", err)
	}
	var createdTask *project.Task
	for _, tk := range projs[0].Tasks {
		if tk.Name == "feature-billing" {
			tCopy := tk
			createdTask = &tCopy
			break
		}
	}
	if createdTask == nil {
		t.Fatalf("expected task 'feature-billing' in project")
	}
	if createdTask.AgentCommand != "claude --permission-mode auto" {
		t.Fatalf("expected task AgentCommand 'claude --permission-mode auto', got %q", createdTask.AgentCommand)
	}
}

func TestTeatest_Import_InteractiveCheckboxAndUnimport(t *testing.T) {
	h := newTestModelHarness(t)

	importedDir := filepath.Join(h.HomeDir, "already-imported-ws")
	p := setupTestWorkspace(t, importedDir)

	unimportedDir := filepath.Join(h.HomeDir, "unimported-ws")
	if err := os.MkdirAll(unimportedDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	_ = os.WriteFile(filepath.Join(unimportedDir, "CLAUDE.md"), []byte("# project"), 0o644)

	sendKeyRune(h.Tm, 'i')
	time.Sleep(100 * time.Millisecond)

	// Send discoverDoneMsg with 1 imported and 1 unimported workspace
	h.Tm.Send(discoverDoneMsg{
		found: []importcmd.Workspace{
			{Name: p.Name, Path: p.Path, Imported: true},
			{Name: "unimported-ws", Path: unimportedDir, Imported: false, Branch: "main"},
		},
	})
	time.Sleep(100 * time.Millisecond)

	// Test bulk deselect 'n'
	sendKeyRune(h.Tm, 'n')
	time.Sleep(50 * time.Millisecond)

	// Test bulk select 'a'
	sendKeyRune(h.Tm, 'a')
	time.Sleep(50 * time.Millisecond)

	// Test space key toggle on second row
	sendDown(h.Tm)
	sendSpace(h.Tm)
	time.Sleep(50 * time.Millisecond)

	// Test un-importing 'd' on first row (imported)
	sendUp(h.Tm)
	sendKeyRune(h.Tm, 'd')
	time.Sleep(100 * time.Millisecond)

	// Test rescan 'r'
	sendKeyRune(h.Tm, 'r')
	time.Sleep(100 * time.Millisecond)

	// Complete import
	h.Tm.Send(discoverDoneMsg{
		found: []importcmd.Workspace{
			{Name: "unimported-ws", Path: unimportedDir, Imported: false},
		},
	})
	time.Sleep(100 * time.Millisecond)
	sendKeyRune(h.Tm, 'i')
	time.Sleep(100 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after import completion, got %v", finalM.screen)
	}
}

// --- Tier 3: Pairwise Combinations ------------------------------------------

func TestTeatest_Pairwise_ScreenRoutingMatrix(t *testing.T) {
	h := newTestModelHarness(t)

	sendKeyRune(h.Tm, 'p')
	waitForOutput(t, h.Tm, "Packs")

	sendEsc(h.Tm)
	sendKeyRune(h.Tm, 's')
	waitForOutput(t, h.Tm, "Settings")

	sendEsc(h.Tm)
	sendKeyRune(h.Tm, '?')
	waitForOutput(t, h.Tm, "Help")

	sendEsc(h.Tm)
	sendF12(h.Tm)
	waitForOutput(t, h.Tm, "Mission Control")

	sendF12(h.Tm)
	time.Sleep(50 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after routing matrix, got %v", finalM.screen)
	}
}

func TestTeatest_Pairwise_F12MissionControlToggleFromNestedScreens(t *testing.T) {
	h := newTestModelHarness(t)

	sendKeyRune(h.Tm, 'p')
	sendF12(h.Tm)
	sendF12(h.Tm)
	sendEsc(h.Tm)

	sendKeyRune(h.Tm, 's')
	sendF12(h.Tm)
	sendF12(h.Tm)
	sendEsc(h.Tm)

	sendKeyRune(h.Tm, '?')
	sendF12(h.Tm)
	sendF12(h.Tm)
	sendEsc(h.Tm)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after nested F12 toggles, got %v", finalM.screen)
	}
}

func TestTeatest_Pairwise_PaneFocusAndCursorMovement(t *testing.T) {
	h := newTestModelHarness(t)
	p1 := setupTestWorkspace(t, filepath.Join(h.HomeDir, "pw-ws1"))
	p2 := setupTestWorkspace(t, filepath.Join(h.HomeDir, "pw-ws2"))
	_, _ = createTask(p1, "t1-1", "")
	_, _ = createTask(p1, "t1-2", "")
	_, _ = createTask(p2, "t2-1", "")

	sendKeyRune(h.Tm, 'r')
	time.Sleep(50 * time.Millisecond)

	sendDown(h.Tm)
	sendTab(h.Tm)
	sendDown(h.Tm)
	sendUp(h.Tm)
	sendShiftTab(h.Tm)
	sendUp(h.Tm)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard, got %v", finalM.screen)
	}
}

func TestTeatest_Pairwise_MultiStepWizardBackAndForth(t *testing.T) {
	h := newTestModelHarness(t)
	projDir := filepath.Join(h.HomeDir, "wizard-backforth")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	sendKeyRune(h.Tm, 'n')
	waitForOutput(t, h.Tm, "New workspace")

	h.Tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.Tm.Type(projDir)
	sendEnter(h.Tm)
	time.Sleep(50 * time.Millisecond)

	sendEsc(h.Tm)
	time.Sleep(50 * time.Millisecond)

	sendEsc(h.Tm)
	time.Sleep(50 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard, got %v", finalM.screen)
	}
}

func TestTeatest_Pairwise_ConfirmDialogActions(t *testing.T) {
	h := newTestModelHarness(t)
	p := setupTestWorkspace(t, filepath.Join(h.HomeDir, "confirm-proj"))
	_, _ = createTask(p, "task-del", "")

	sendKeyRune(h.Tm, 'r')
	time.Sleep(50 * time.Millisecond)

	sendKeyRune(h.Tm, 'x')
	sendKeyRune(h.Tm, 'n')
	time.Sleep(50 * time.Millisecond)

	sendTab(h.Tm)
	sendKeyRune(h.Tm, 'd')
	sendKeyRune(h.Tm, 'n')
	time.Sleep(50 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.confirm != confirmNone {
		t.Fatalf("expected confirmNone after dialogue cancellations, got %v", finalM.confirm)
	}
}

// --- Tier 4: Real-World Workloads -------------------------------------------

func TestTeatest_Workload_FullProjectLifecycle(t *testing.T) {
	h := newTestModelHarness(t)

	wsDir := filepath.Join(h.HomeDir, "lifecycle-backend")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	sendKeyRune(h.Tm, 'n')
	waitForOutput(t, h.Tm, "New workspace")
	h.Tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.Tm.Type(wsDir)
	waitForOutput(t, h.Tm, "lifecycle-backend")
	time.Sleep(50 * time.Millisecond)

	// wsStepDir -> wsStepPack
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	// wsStepPack -> wsStepAgent
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	// wsStepAgent -> createWorkspace -> scrDashboard
	sendEnter(h.Tm)
	waitForOutput(t, h.Tm, "Created workspace")

	// Task creation
	sendKeyRune(h.Tm, 't')
	waitForOutput(t, h.Tm, "New task")
	h.Tm.Type("auth")
	waitForOutput(t, h.Tm, "auth")
	time.Sleep(50 * time.Millisecond)

	// newTaskStepName -> newTaskStepAgent
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	// newTaskStepAgent -> finishNewTask -> attach -> execDoneMsg -> scrDashboard
	sendEnter(h.Tm)
	waitForOutput(t, h.Tm, "Agent not found")

	// F12 Mission Control toggle back and forth
	sendF12(h.Tm)
	waitForOutput(t, h.Tm, "Mission Control")
	sendF12(h.Tm)
	waitForOutput(t, h.Tm, "Workspaces")
	time.Sleep(100 * time.Millisecond)

	// Delete task (focus task column via Tab)
	sendTab(h.Tm)
	time.Sleep(50 * time.Millisecond)
	sendKeyRune(h.Tm, 'd')
	waitForOutput(t, h.Tm, "Remove task")
	sendKeyRune(h.Tm, 'y')
	time.Sleep(100 * time.Millisecond)

	// Delete workspace (focus workspace column via Shift+Tab)
	sendShiftTab(h.Tm)
	time.Sleep(50 * time.Millisecond)
	sendKeyRune(h.Tm, 'd')
	waitForOutput(t, h.Tm, "Remove workspace")
	sendKeyRune(h.Tm, 'y')
	time.Sleep(100 * time.Millisecond)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard after project lifecycle, got %v", finalM.screen)
	}
}

func TestTeatest_Workload_PackManagementAndRelinkFlow(t *testing.T) {
	h := newTestModelHarness(t)
	wsDir := filepath.Join(h.HomeDir, "workload-ws")
	setupTestWorkspace(t, wsDir)

	sendKeyRune(h.Tm, 'p')
	waitForOutput(t, h.Tm, "Packs")

	sendKeyRune(h.Tm, 'n')
	h.Tm.Type("go-microservice")
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	sendKeyRune(h.Tm, 'c')
	h.Tm.Type("go-microservice-v2")
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	sendEsc(h.Tm)

	sendKeyRune(h.Tm, 'r')
	sendKeyRune(h.Tm, 'e')
	waitForOutput(t, h.Tm, "Point")

	newPath := filepath.Join(h.HomeDir, "workload-ws-moved")
	if err := os.MkdirAll(newPath, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	h.Tm.Send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.Tm.Type(newPath)
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)

	sendKeyRune(h.Tm, 's')
	waitForOutput(t, h.Tm, "Settings")
	sendDown(h.Tm)
	sendEnter(h.Tm)
	time.Sleep(100 * time.Millisecond)
	sendEsc(h.Tm)
	sendEsc(h.Tm)

	finalM := h.Finish(t)
	if finalM.screen != scrDashboard {
		t.Fatalf("expected scrDashboard at end of workload test, got %v", finalM.screen)
	}
}
