package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBranch(t *testing.T) {
	dir := t.TempDir()
	p := Project{Name: "x", Path: dir}

	// Not a git repo.
	if got := p.Branch(); got != "" {
		t.Fatalf("non-git Branch() = %q, want empty", got)
	}

	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHead := func(s string) {
		if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeHead("ref: refs/heads/feature/refunds\n")
	if got := p.Branch(); got != "feature/refunds" {
		t.Fatalf("branch ref Branch() = %q", got)
	}

	writeHead("0123456789abcdef0123456789abcdef01234567\n")
	if got := p.Branch(); got != "0123456" {
		t.Fatalf("detached HEAD Branch() = %q, want short hash", got)
	}

	// A .git file (worktree/submodule) is not a branch ref.
	writeHead("gitdir: /somewhere/else\n")
	if got := p.Branch(); got != "" {
		t.Fatalf("gitdir HEAD Branch() = %q, want empty", got)
	}
}
