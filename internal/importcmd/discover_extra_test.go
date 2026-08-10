package importcmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverMatchesLowercaseClaude(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()

	mk := func(name, file string) {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte("# notes\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("upper", "CLAUDE.md")
	mk("lower", "claude.md")
	mk("title", "Agent.md")

	found, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(found) != 3 {
		t.Fatalf("got %d workspaces, want 3 (CLAUDE.md/claude.md/Agent.md)", len(found))
	}
}

func TestWalkRootDepthCap(t *testing.T) {
	root := t.TempDir()

	// A project three levels below the root.
	deep := filepath.Join(root, "a", "b", "c", "proj")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "CLAUDE.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A shallow cap must not reach it; a generous cap must.
	if got := walkRoot(root, 2, map[string]bool{}); len(got) != 0 {
		t.Fatalf("depth 2 found %d, want 0 (project is 4 levels deep)", len(got))
	}
	if got := walkRoot(root, 6, map[string]bool{}); len(got) != 1 {
		t.Fatalf("depth 6 found %d, want 1", len(got))
	}
}
