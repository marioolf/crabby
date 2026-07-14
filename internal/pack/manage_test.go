package pack

import (
	"os"
	"path/filepath"
	"testing"
)

// withPacksDir points the packs directory at a temp dir for the duration of a
// test, so Create/Duplicate/Delete never touch the real ~/.config.
func withPacksDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	root, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCreateWritesManifestAndStarter(t *testing.T) {
	withPacksDir(t)

	p, err := Create("go-api", "A Go API pack")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.Name != "go-api" || p.Description != "A Go API pack" {
		t.Fatalf("created pack = %+v", p)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "CLAUDE.md")); err != nil {
		t.Fatalf("starter CLAUDE.md missing: %v", err)
	}

	// A second create with the same name must fail rather than clobber.
	if _, err := Create("go-api", "again"); err == nil {
		t.Fatal("expected error creating a duplicate name")
	}

	// Invalid names are rejected.
	if _, err := Create("bad name", ""); err == nil {
		t.Fatal("expected error for an invalid name")
	}
}

func TestDuplicateCopiesFilesAndRenames(t *testing.T) {
	withPacksDir(t)

	src, err := Create("base", "Base pack")
	if err != nil {
		t.Fatal(err)
	}
	// Add an extra payload file to prove the whole tree is copied.
	if err := os.MkdirAll(filepath.Join(src.Dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src.Dir, ".claude", "settings.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dup, err := Duplicate(src, "copy")
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if dup.Name != "copy" {
		t.Fatalf("duplicate name = %q, want copy", dup.Name)
	}
	if _, err := os.Stat(filepath.Join(dup.Dir, ".claude", "settings.md")); err != nil {
		t.Fatalf("nested payload not duplicated: %v", err)
	}
	// The manifest of the copy must carry the new name.
	reloaded, err := Load(dup.Dir)
	if err != nil || reloaded.Name != "copy" {
		t.Fatalf("reloaded copy = %+v (err %v)", reloaded, err)
	}
}

func TestDeleteRemovesOnlyPackDirs(t *testing.T) {
	withPacksDir(t)

	p, err := Create("gone", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Delete(p); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(p.Dir); !os.IsNotExist(err) {
		t.Fatal("pack directory still present after Delete")
	}

	// Delete must refuse a directory outside the packs root.
	outside := Pack{Name: "evil", Dir: t.TempDir()}
	if err := Delete(outside); err == nil {
		t.Fatal("expected Delete to refuse a non-pack directory")
	}
}
