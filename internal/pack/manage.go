package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// starterCLAUDE is the CLAUDE.md a freshly created pack ships until the user
// edits it. It mirrors the built-in default so a new pack is useful at once.
const starterCLAUDE = `# Project

Describe the project.

# Architecture

Describe the architecture.

# Coding conventions

Describe conventions.

# Important notes

Write important information here.
`

// validateName keeps a pack name safe to use as a directory and readable in the
// selector — the same rule Crabby applies to task names.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("a pack name is required")
	}
	for _, r := range name {
		ok := r == '-' || r == '_' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return fmt.Errorf("pack names may only contain letters, digits, '-' and '_' (got %q)", name)
		}
	}
	return nil
}

// dirFor returns the on-disk directory a pack of the given name lives in.
func dirFor(name string) (string, error) {
	root, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

// manifestBody renders a pack.yaml from its fields, writing only the keys that
// have a value so the file stays as small as the pack needs it to be.
func manifestBody(p Pack) string {
	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\n", p.Name)
	if p.Description != "" {
		fmt.Fprintf(&b, "description: %s\n", p.Description)
	}
	if p.Author != "" {
		fmt.Fprintf(&b, "author: %s\n", p.Author)
	}
	if p.Version != "" {
		fmt.Fprintf(&b, "version: %s\n", p.Version)
	}
	return b.String()
}

// Create makes a new pack directory with a manifest and a starter CLAUDE.md,
// then returns the loaded pack. It fails if a pack with that name already
// exists, so an accidental second create never clobbers work.
func Create(name, description string) (Pack, error) {
	if err := validateName(name); err != nil {
		return Pack{}, err
	}
	dir, err := dirFor(name)
	if err != nil {
		return Pack{}, err
	}
	if _, err := os.Stat(dir); err == nil {
		return Pack{}, fmt.Errorf("a pack named %q already exists", name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Pack{}, err
	}
	p := Pack{Name: name, Description: description, Version: "0.1.0"}
	if err := os.WriteFile(filepath.Join(dir, manifest), []byte(manifestBody(p)), 0o644); err != nil {
		return Pack{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(starterCLAUDE), 0o644); err != nil {
		return Pack{}, err
	}
	return Load(dir)
}

// Duplicate copies an existing pack to a new name and returns the copy. Every
// payload file is carried over; only the manifest's name is changed so the copy
// is a true starting point rather than a second pack claiming the same name.
func Duplicate(src Pack, newName string) (Pack, error) {
	if err := validateName(newName); err != nil {
		return Pack{}, err
	}
	dest, err := dirFor(newName)
	if err != nil {
		return Pack{}, err
	}
	if _, err := os.Stat(dest); err == nil {
		return Pack{}, fmt.Errorf("a pack named %q already exists", newName)
	}
	if err := copyTree(src.Dir, dest); err != nil {
		return Pack{}, err
	}
	// Rewrite the manifest so the copy owns its new name while keeping the rest.
	clone := src
	clone.Name = newName
	if err := os.WriteFile(filepath.Join(dest, manifest), []byte(manifestBody(clone)), 0o644); err != nil {
		return Pack{}, err
	}
	return Load(dest)
}

// Delete removes a pack's directory. As a safety measure it refuses to touch
// anything that is not inside the packs directory.
func Delete(p Pack) error {
	root, err := Dir()
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, p.Dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("refusing to delete %q: not a pack directory", p.Dir)
	}
	return os.RemoveAll(p.Dir)
}

// copyTree recursively copies the directory at src to dest, preserving the
// layout. It reuses copyFile for the leaves.
func copyTree(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}
