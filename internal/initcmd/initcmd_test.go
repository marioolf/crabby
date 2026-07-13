package initcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marioolf/crabby/internal/project"
)

func TestInitCreatesStructureAndRegisters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, "work", "payments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := Init(dir, nil)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if res.Project.Name != "payments" {
		t.Fatalf("name = %q, want payments", res.Project.Name)
	}
	// A fresh workspace starts with a single default task on the legacy session.
	if len(res.Project.Tasks) != 1 || res.Project.Tasks[0].Session != "crabby_payments" {
		t.Fatalf("tasks = %+v, want one default task on crabby_payments", res.Project.Tasks)
	}

	for _, f := range []string{
		filepath.Join(dir, ".claude", "crabby.yaml"),
		filepath.Join(dir, "CLAUDE.md"),
	} {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("expected %s to exist: %v", f, err)
		}
	}

	if _, err := project.Find("payments"); err != nil {
		t.Fatalf("project not registered: %v", err)
	}
}

func TestInitDoesNotClobberExistingClaudeMD(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "# my custom notes\n"
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Init(dir, nil); err != nil {
		t.Fatalf("Init: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if string(got) != custom {
		t.Fatalf("CLAUDE.md was overwritten: %q", got)
	}
}

func TestInitRejectsMissingDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := Init(missing, nil); err == nil {
		t.Fatal("expected an error for a non-existent directory")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("Init created a directory that should not exist")
	}
}
