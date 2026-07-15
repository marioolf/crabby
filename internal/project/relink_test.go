package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRelinkUpdatesPathAndName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Register a workspace with a task, then move its folder.
	oldDir := filepath.Join(home, "old")
	newDir := filepath.Join(home, "renamed")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := Project{
		Name:  "old",
		Path:  oldDir,
		Tasks: []Task{{Name: DefaultTask, Session: "crabby_old"}},
	}
	if err := Register(p); err != nil {
		t.Fatal(err)
	}

	if err := Relink("old", newDir); err != nil {
		t.Fatalf("Relink: %v", err)
	}

	got, err := Find("renamed")
	if err != nil {
		t.Fatalf("Find renamed: %v", err)
	}
	if got.Path != newDir {
		t.Fatalf("path = %q, want %q", got.Path, newDir)
	}
	// The task (and its stored session) must survive the relink.
	if len(got.Tasks) != 1 || got.Tasks[0].Session != "crabby_old" {
		t.Fatalf("tasks not preserved: %+v", got.Tasks)
	}
	// The old name is gone.
	if _, err := Find("old"); err != ErrNotFound {
		t.Fatalf("old name still resolves (err %v)", err)
	}
}

func TestRelinkRejectsMissingDirAndNameClash(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	a := filepath.Join(home, "a")
	b := filepath.Join(home, "b")
	for _, d := range []string{a, b} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_ = Register(Project{Name: "a", Path: a, Tasks: []Task{{Name: DefaultTask, Session: "crabby_a"}}})
	_ = Register(Project{Name: "b", Path: b, Tasks: []Task{{Name: DefaultTask, Session: "crabby_b"}}})

	// Relinking to a non-existent directory fails.
	if err := Relink("a", filepath.Join(home, "nope")); err == nil {
		t.Fatal("expected error relinking to a missing directory")
	}
	// Relinking "a" onto b's folder would collide with the existing "b".
	if err := Relink("a", b); err == nil {
		t.Fatal("expected error on name clash")
	}
}
