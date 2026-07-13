package importcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marioolf/crabby/internal/project"
)

// repo creates dir with an optional .git and CLAUDE.md, mirroring what Discover
// looks for.
func repo(t *testing.T, dir string, git, claude bool) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if git {
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if claude {
		if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# notes\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func names(ws []Workspace) map[string]bool {
	out := map[string]bool{}
	for _, w := range ws {
		out[w.Name] = true
	}
	return out
}

func TestDiscoverFindsFoldersWithClaudeMD(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()

	repo(t, filepath.Join(root, "payments"), true, true)   // git + CLAUDE.md
	repo(t, filepath.Join(root, "frontend"), true, true)   // git + CLAUDE.md
	repo(t, filepath.Join(root, "no-git"), false, true)    // CLAUDE.md, no git — still qualifies
	repo(t, filepath.Join(root, "no-claude"), true, false) // git only — does not qualify

	found, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	got := names(found)
	if len(got) != 3 || !got["frontend"] || !got["payments"] || !got["no-git"] {
		t.Fatalf("got %v, want frontend+payments+no-git", got)
	}
	// Results are sorted by name.
	if found[0].Name != "frontend" || found[1].Name != "no-git" || found[2].Name != "payments" {
		t.Fatalf("not sorted: %v", found)
	}
}

func TestDiscoverPrunesDepsAndNestedWorkspaces(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()

	// A qualifying project that itself contains a dependency folder with a
	// CLAUDE.md and a nested project — neither should surface, because scanning
	// stops at the outer CLAUDE.md.
	outer := filepath.Join(root, "outer")
	repo(t, outer, true, true)
	repo(t, filepath.Join(outer, "node_modules", "dep"), false, true)
	repo(t, filepath.Join(outer, "packages", "inner"), true, true)

	found, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(found) != 1 || found[0].Name != "outer" {
		t.Fatalf("got %v, want outer only (nested workspaces ignored)", names(found))
	}
}

func TestDiscoverBranchEmptyForNonGitFolder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	repo(t, filepath.Join(root, "notes"), false, true) // CLAUDE.md, no git

	found, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(found) != 1 || found[0].Branch != "" {
		t.Fatalf("got %v, want one workspace with empty branch", found)
	}
}

func TestDiscoverFlagsAlreadyImported(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()

	payments := filepath.Join(root, "payments")
	repo(t, payments, true, true)
	repo(t, filepath.Join(root, "frontend"), true, true)

	if err := project.Register(project.Project{Name: "payments", Path: payments, Session: "crabby_payments"}); err != nil {
		t.Fatal(err)
	}

	found, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	for _, w := range found {
		want := w.Name == "payments"
		if w.Imported != want {
			t.Fatalf("%s: Imported = %v, want %v", w.Name, w.Imported, want)
		}
	}
}

func TestDiscoverOnASingleFolder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	repo(t, root, false, true) // the root itself, no git

	found, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(found) != 1 || found[0].Path != mustAbs(t, root) {
		t.Fatalf("got %v, want the root folder itself", found)
	}
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
