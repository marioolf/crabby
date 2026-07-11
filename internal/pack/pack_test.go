package pack

import (
	"os"
	"path/filepath"
	"testing"
)

func writePack(t *testing.T, dir, manifestBody string, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if manifestBody != "" {
		if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(manifestBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadValidatesManifest(t *testing.T) {
	base := t.TempDir()

	// Missing pack.yaml.
	noManifest := filepath.Join(base, "empty")
	if err := os.MkdirAll(noManifest, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(noManifest); err == nil {
		t.Fatal("expected error for missing pack.yaml")
	}

	// Manifest without a name.
	noName := filepath.Join(base, "noname")
	writePack(t, noName, "description: x\n", nil)
	if _, err := Load(noName); err == nil {
		t.Fatal("expected error for missing name")
	}

	// Valid.
	ok := filepath.Join(base, "go")
	writePack(t, ok, "name: go\ndescription: Go pack\nfuture_field: ignored\n", nil)
	p, err := Load(ok)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Name != "go" || p.Description != "Go pack" {
		t.Fatalf("parsed pack = %+v", p)
	}
}

func TestApplyCopiesAndNeverOverwrites(t *testing.T) {
	base := t.TempDir()
	packDir := filepath.Join(base, "pack")
	writePack(t, packDir, "name: simple\n", map[string]string{
		"CLAUDE.md":           "pack version\n",
		".claude/settings.md": "settings\n",
	})
	p, err := Load(packDir)
	if err != nil {
		t.Fatal(err)
	}

	proj := filepath.Join(base, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-existing file must be preserved.
	if err := os.WriteFile(filepath.Join(proj, "CLAUDE.md"), []byte("MINE\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	copied, skipped, err := Apply(p, proj)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got, _ := os.ReadFile(filepath.Join(proj, "CLAUDE.md")); string(got) != "MINE\n" {
		t.Fatalf("existing CLAUDE.md was overwritten: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(proj, ".claude", "settings.md")); string(got) != "settings\n" {
		t.Fatalf("nested file not copied: %q", got)
	}
	if _, err := os.Stat(filepath.Join(proj, "pack.yaml")); !os.IsNotExist(err) {
		t.Fatal("pack.yaml must not be copied into the project")
	}

	assertContains(t, "copied", copied, ".claude/settings.md")
	assertContains(t, "skipped", skipped, "CLAUDE.md")
}

func assertContains(t *testing.T, label string, list []string, want string) {
	t.Helper()
	for _, s := range list {
		if s == want {
			return
		}
	}
	t.Fatalf("%s %v does not contain %q", label, list, want)
}
