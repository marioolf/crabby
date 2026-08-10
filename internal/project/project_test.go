package project

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

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

func TestRegistryPermissionsArePrivate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Save([]Project{{Name: "private", Path: "/private", Session: "crabby_private"}}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".local", "share", "crabby")
	for _, path := range []string{dir, filepath.Join(dir, "projects.json"), filepath.Join(dir, ".lock")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if got := info.Mode().Perm(); got != map[string]os.FileMode{
			dir:                                 0o700,
			filepath.Join(dir, "projects.json"): 0o600,
			filepath.Join(dir, ".lock"):         0o600,
		}[path] {
			t.Fatalf("permissions for %s = %o, want %o", path, got, map[string]os.FileMode{
				dir:                                 0o700,
				filepath.Join(dir, "projects.json"): 0o600,
				filepath.Join(dir, ".lock"):         0o600,
			}[path])
		}
	}

	if err := os.Chmod(filepath.Join(dir, "projects.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save([]Project{{Name: "private", Path: "/private", Session: "crabby_private"}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "projects.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("existing registry permissions = %o, want 600", got)
	}
}

func TestConcurrentRegistryUpdatesArePreserved(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const count = 8
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- Register(Project{
				Name:    "workspace-" + string(rune('a'+i)),
				Path:    "/workspace/" + string(rune('a'+i)),
				Session: "crabby_workspace_" + string(rune('a'+i)),
			})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	projects, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != count {
		t.Fatalf("concurrent Register preserved %d projects, want %d", len(projects), count)
	}
}

func TestRegisterRejectsSessionCollisionAcrossWorkspaces(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := Register(Project{Name: "foo_bar", Path: "/foo_bar", Session: "crabby_foo_bar"}); err != nil {
		t.Fatal(err)
	}
	err := Register(Project{
		Name:  "foo",
		Path:  "/foo",
		Tasks: []Task{{Name: "bar", Session: "crabby_foo_bar"}},
	})
	if err == nil {
		t.Fatal("Register accepted a colliding session")
	}
}
