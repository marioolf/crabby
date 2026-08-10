package initcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/marioolf/crabby/internal/project"
)

func TestInitCreatesStructureAndRegisters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", home)

	dir := filepath.Join(home, "work", "payments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := Init(dir, nil, "")
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
	t.Setenv("XDG_DATA_HOME", home)

	dir := filepath.Join(home, "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "# my custom notes\n"
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Init(dir, nil, ""); err != nil {
		t.Fatalf("Init: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if string(got) != custom {
		t.Fatalf("CLAUDE.md was overwritten: %q", got)
	}
}

func TestInitRejectsMissingDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", home)

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := Init(missing, nil, ""); err == nil {
		t.Fatal("expected an error for a non-existent directory")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("Init created a directory that should not exist")
	}
}

func TestInitRejectsClaudeDirectorySymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", home)

	dir := filepath.Join(home, "workspace")
	target := filepath.Join(home, "outside")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, ".claude")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := Init(dir, nil, ""); err == nil {
		t.Fatal("Init accepted a symlinked .claude directory")
	}
	if _, err := os.Stat(filepath.Join(target, "crabby.yaml")); !os.IsNotExist(err) {
		t.Fatalf("Init wrote through .claude symlink: %v", err)
	}
}

func TestInitRejectsCrabbyFileSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", home)

	dir := filepath.Join(home, "workspace")
	target := filepath.Join(home, "outside.txt")
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := []byte("do not overwrite\n")
	if err := os.WriteFile(target, sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, ".claude", "crabby.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := Init(dir, nil, ""); err == nil {
		t.Fatal("Init accepted a symlinked crabby.yaml")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(sentinel) {
		t.Fatalf("symlink target changed to %q", got)
	}
}

func TestUniqueWorkspaceName(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	pathA := filepath.Join(t.TempDir(), "path", "a", "myproj")
	pathB := filepath.Join(t.TempDir(), "path", "b", "myproj")
	pathC := filepath.Join(t.TempDir(), "path", "c", "myproj")
	pathD := filepath.Join(t.TempDir(), "path", "d", "myproj")

	for _, p := range []string{pathA, pathB, pathC, pathD} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Base name `myproj` at `/path/a` -> `myproj`
	resA, err := Init(pathA, nil, "claude")
	if err != nil {
		t.Fatalf("Init pathA failed: %v", err)
	}
	if resA.Project.Name != "myproj" {
		t.Fatalf("resA name = %q, want myproj", resA.Project.Name)
	}

	// 2. Idempotent re-init at `/path/a` with same agent -> returns `myproj`
	resA2, err := Init(pathA, nil, "claude")
	if err != nil {
		t.Fatalf("Init pathA re-init failed: %v", err)
	}
	if resA2.Project.Name != "myproj" {
		t.Fatalf("resA2 re-init name = %q, want myproj", resA2.Project.Name)
	}

	// 3. Base name `myproj` at `/path/b` with agent `opencode` -> `myproj-opencode`
	resB, err := Init(pathB, nil, "opencode")
	if err != nil {
		t.Fatalf("Init pathB failed: %v", err)
	}
	if resB.Project.Name != "myproj-opencode" {
		t.Fatalf("resB name = %q, want myproj-opencode", resB.Project.Name)
	}

	// 4. Base name `myproj` at `/path/c` with agent `opencode` -> `myproj-opencode-3`
	resC, err := Init(pathC, nil, "opencode")
	if err != nil {
		t.Fatalf("Init pathC failed: %v", err)
	}
	if resC.Project.Name != "myproj-opencode-3" {
		t.Fatalf("resC name = %q, want myproj-opencode-3", resC.Project.Name)
	}

	// 5. Base name `myproj` at `/path/d` with empty agent -> `myproj-2`
	resD, err := Init(pathD, nil, "")
	if err != nil {
		t.Fatalf("Init pathD failed: %v", err)
	}
	if resD.Project.Name != "myproj-2" {
		t.Fatalf("resD name = %q, want myproj-2", resD.Project.Name)
	}
}

func TestSanitizeName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"myproj", "myproj"},
		{"My_Project-123", "My_Project-123"},
		{"my project @ 2026!", "myproject2026"},
		{"!!!", "workspace"},
		{"", "workspace"},
	}

	for _, c := range cases {
		got := sanitizeName(c.input)
		if got != c.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// --- R1.4 Workspace Disambiguation Unit Tests (T1 & T2) -------------------

func TestDisambiguation_FirstWorkspaceBaseName(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	dir := filepath.Join(t.TempDir(), "myproject")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := Init(dir, nil, "claude")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if res.Project.Name != "myproject" {
		t.Fatalf("res.Project.Name = %q, want myproject", res.Project.Name)
	}
}

func TestDisambiguation_SecondWorkspaceDifferentAgent(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	dir1 := filepath.Join(t.TempDir(), "path1", "myproject")
	dir2 := filepath.Join(t.TempDir(), "path2", "myproject")
	_ = os.MkdirAll(dir1, 0o755)
	_ = os.MkdirAll(dir2, 0o755)

	_, _ = Init(dir1, nil, "claude")
	res2, err := Init(dir2, nil, "opencode")
	if err != nil {
		t.Fatalf("Init dir2 failed: %v", err)
	}
	if res2.Project.Name != "myproject-opencode" {
		t.Fatalf("res2.Project.Name = %q, want myproject-opencode", res2.Project.Name)
	}
}

func TestDisambiguation_ExactPathAndAgentReuse(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	dir := filepath.Join(t.TempDir(), "myproject")
	_ = os.MkdirAll(dir, 0o755)

	res1, _ := Init(dir, nil, "opencode")
	res2, err := Init(dir, nil, "opencode")
	if err != nil {
		t.Fatalf("re-init failed: %v", err)
	}
	if res2.Project.Name != res1.Project.Name {
		t.Fatalf("re-init name = %q, want %q", res2.Project.Name, res1.Project.Name)
	}
}

func TestDisambiguation_MultipleSameAgentIncrementsIndex(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	dir1 := filepath.Join(t.TempDir(), "path1", "myproject")
	dir2 := filepath.Join(t.TempDir(), "path2", "myproject")
	dir3 := filepath.Join(t.TempDir(), "path3", "myproject")
	_ = os.MkdirAll(dir1, 0o755)
	_ = os.MkdirAll(dir2, 0o755)
	_ = os.MkdirAll(dir3, 0o755)

	_, _ = Init(dir1, nil, "claude")
	_, _ = Init(dir2, nil, "opencode")
	res3, err := Init(dir3, nil, "opencode")
	if err != nil {
		t.Fatalf("Init dir3 failed: %v", err)
	}
	if res3.Project.Name != "myproject-opencode-3" {
		t.Fatalf("res3.Project.Name = %q, want myproject-opencode-3", res3.Project.Name)
	}
}

func TestDisambiguation_EmptyAgentCommandDisambiguation(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	dir1 := filepath.Join(t.TempDir(), "a", "proj")
	dir2 := filepath.Join(t.TempDir(), "b", "proj")
	_ = os.MkdirAll(dir1, 0o755)
	_ = os.MkdirAll(dir2, 0o755)

	_, _ = Init(dir1, nil, "")
	res2, err := Init(dir2, nil, "")
	if err != nil {
		t.Fatalf("Init dir2 failed: %v", err)
	}
	if res2.Project.Name != "proj-2" {
		t.Fatalf("res2.Project.Name = %q, want proj-2", res2.Project.Name)
	}
}

func TestDisambiguation_SpecialCharsInBaseName(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	dir := filepath.Join(t.TempDir(), "my.app#1!")
	_ = os.MkdirAll(dir, 0o755)

	name := uniqueWorkspaceName("my.app#1!", dir, "claude")
	if name != "myapp1" {
		t.Fatalf("uniqueWorkspaceName = %q, want myapp1", name)
	}
}

func TestDisambiguation_SanitizeToEmptyFallback(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	dir := filepath.Join(t.TempDir(), "!!!$$$")
	_ = os.MkdirAll(dir, 0o755)

	name := uniqueWorkspaceName("!!!$$$", dir, "claude")
	if name != "workspace" {
		t.Fatalf("uniqueWorkspaceName = %q, want workspace", name)
	}
}

func TestDisambiguation_ComplexAgentCommandWithFlags(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	dir1 := filepath.Join(t.TempDir(), "a", "myproj")
	dir2 := filepath.Join(t.TempDir(), "b", "myproj")
	_ = os.MkdirAll(dir1, 0o755)
	_ = os.MkdirAll(dir2, 0o755)

	_, _ = Init(dir1, nil, "claude")
	res2, err := Init(dir2, nil, "/usr/local/bin/claude --permission-mode auto --verbose")
	if err != nil {
		t.Fatalf("Init dir2 failed: %v", err)
	}
	if res2.Project.Name != "myproj-claude" {
		t.Fatalf("res2.Project.Name = %q, want myproj-claude", res2.Project.Name)
	}
}

func TestDisambiguation_PathCaseSensitivity(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	dir1 := filepath.Join(t.TempDir(), "path1", "Project")
	dir2 := filepath.Join(t.TempDir(), "path2", "Project")
	_ = os.MkdirAll(dir1, 0o755)
	_ = os.MkdirAll(dir2, 0o755)

	res1, _ := Init(dir1, nil, "claude")
	name := uniqueWorkspaceName("Project", dir2, "claude")
	if name == res1.Project.Name {
		t.Fatalf("expected different name for different path %q vs %q, got same %q", dir1, dir2, name)
	}
}

func TestDisambiguation_LargeCollisionCount(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	// Register base project "myproj"
	p1 := project.Project{
		Name:         "myproj",
		Path:         filepath.Join(base, "myproj"),
		AgentCommand: "claude",
	}
	if err := project.Register(p1); err != nil {
		t.Fatalf("Register p1 failed: %v", err)
	}

	// Register collateral colliding projects myproj-claude, myproj-claude-3 ... myproj-claude-50
	for i := 2; i <= 50; i++ {
		var name string
		if i == 2 {
			name = "myproj-claude"
		} else {
			name = fmt.Sprintf("myproj-claude-%d", i)
		}
		p := project.Project{
			Name:         name,
			Path:         filepath.Join(base, fmt.Sprintf("myproj-dir%d", i)),
			AgentCommand: "claude",
		}
		if err := project.Register(p); err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	targetDir := filepath.Join(base, "myproj-dir51")
	_ = os.MkdirAll(targetDir, 0o755)

	name := uniqueWorkspaceName("myproj", targetDir, "claude")
	if name != "myproj-claude-51" {
		t.Fatalf("uniqueWorkspaceName = %q, want myproj-claude-51", name)
	}
}
