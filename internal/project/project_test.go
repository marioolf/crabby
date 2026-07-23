package project

import "testing"

func TestRegisterFindAndUpdate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Empty registry to start.
	got, err := Load()
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty registry, got %d", len(got))
	}

	p := Project{Name: "payments", Path: "/home/x/payments", Session: "crabby_payments"}
	if err := Register(p); err != nil {
		t.Fatalf("Register: %v", err)
	}

	found, err := Find("payments")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if found.Name != p.Name || found.Path != p.Path {
		t.Fatalf("Find returned %+v, want name/path of %+v", found, p)
	}
	// A legacy single-session entry migrates into a default "main" task.
	if len(found.Tasks) != 1 || found.Tasks[0].Name != DefaultTask || found.Tasks[0].Session != "crabby_payments" {
		t.Fatalf("expected migrated default task, got %+v", found.Tasks)
	}

	// Re-registering the same name updates in place, not appends.
	p.Path = "/home/x/moved"
	if err := Register(p); err != nil {
		t.Fatalf("Register update: %v", err)
	}
	all, _ := Load()
	if len(all) != 1 {
		t.Fatalf("expected 1 project after update, got %d", len(all))
	}

	byPath, err := FindByPath("/home/x/moved")
	if err != nil {
		t.Fatalf("FindByPath: %v", err)
	}
	if byPath.Name != "payments" {
		t.Fatalf("FindByPath name = %q", byPath.Name)
	}
}

func TestFindNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := Find("nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAddAndRemoveTask(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := Register(Project{Name: "payments", Path: "/p", Tasks: []Task{{Name: "main", Session: "crabby_payments"}}}); err != nil {
		t.Fatal(err)
	}

	if err := AddTask("payments", Task{Name: "tests", Session: "crabby_payments_tests"}); err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	// Duplicate names are rejected.
	if err := AddTask("payments", Task{Name: "tests", Session: "x"}); err != ErrTaskExists {
		t.Fatalf("AddTask duplicate = %v, want ErrTaskExists", err)
	}

	p, _ := Find("payments")
	if len(p.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %+v", p.Tasks)
	}

	if err := RemoveTask("payments", "tests"); err != nil {
		t.Fatalf("RemoveTask: %v", err)
	}
	p, _ = Find("payments")
	if len(p.Tasks) != 1 || p.Tasks[0].Name != "main" {
		t.Fatalf("after remove expected only main, got %+v", p.Tasks)
	}
}

func TestRemoveLastTaskLeavesWorkspace(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := Register(Project{Name: "w", Path: "/w", Tasks: []Task{{Name: "only", Session: "crabby_w_only"}}}); err != nil {
		t.Fatal(err)
	}
	if err := RemoveTask("w", "only"); err != nil {
		t.Fatalf("RemoveTask: %v", err)
	}
	// The workspace survives and load re-synthesizes a default task.
	p, err := Find("w")
	if err != nil {
		t.Fatalf("workspace gone after removing last task: %v", err)
	}
	if len(p.Tasks) != 1 || p.Tasks[0].Name != DefaultTask {
		t.Fatalf("expected re-synthesized default task, got %+v", p.Tasks)
	}
}

func TestLegacyEntryMigratesToDefaultTask(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Simulate a pre-tasks registry entry (session, no tasks).
	if err := Save([]Project{{Name: "old", Path: "/old", Session: "crabby_old"}}); err != nil {
		t.Fatal(err)
	}
	p, err := Find("old")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tasks) != 1 || p.Tasks[0].Session != "crabby_old" {
		t.Fatalf("legacy entry did not migrate: %+v", p.Tasks)
	}
	// A pre-agents task carries no agent; it must resolve to the default so old
	// installations keep working with no manual migration.
	if p.Tasks[0].Agent != "claude" {
		t.Fatalf("legacy task agent = %q, want claude", p.Tasks[0].Agent)
	}
}

// TestTaskWithoutAgentBackfills covers a task-based registry (already migrated
// to tasks) written before the agent field existed: each task should load with
// the default agent filled in.
func TestTaskWithoutAgentBackfills(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := Save([]Project{{
		Name: "ws", Path: "/ws",
		Tasks: []Task{{Name: "main", Session: "crabby_ws"}, {Name: "tests", Session: "crabby_ws_tests"}},
	}}); err != nil {
		t.Fatal(err)
	}
	p, err := Find("ws")
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range p.Tasks {
		if tk.Agent != "claude" {
			t.Errorf("task %q agent = %q, want claude", tk.Name, tk.Agent)
		}
	}
}

func TestRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := Register(Project{Name: "a", Path: "/a", Session: "crabby_a"}); err != nil {
		t.Fatal(err)
	}
	if err := Register(Project{Name: "b", Path: "/b", Session: "crabby_b"}); err != nil {
		t.Fatal(err)
	}

	if err := Remove("a"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := Find("a"); err != ErrNotFound {
		t.Fatalf("project 'a' still present after Remove")
	}
	if _, err := Find("b"); err != nil {
		t.Fatalf("Remove deleted the wrong project: %v", err)
	}

	if err := Remove("missing"); err != ErrNotFound {
		t.Fatalf("Remove(missing) = %v, want ErrNotFound", err)
	}
}
