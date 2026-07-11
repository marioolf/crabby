// Package project manages the registry of Crabby projects.
//
// The registry is a single JSON file at
// ~/.local/share/crabby/projects.json. No database, JSON is enough.
package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is returned when a project cannot be located in the registry.
var ErrNotFound = errors.New("project not found")

// Project is a single registered project.
type Project struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Session string `json:"session"`
}

// dbPath returns the location of the registry file.
func dbPath() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".local", "share", "crabby", "projects.json"), nil
}

// Load returns every registered project. A missing registry yields an empty
// slice rather than an error.
func Load() ([]Project, error) {
	path, err := dbPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Project{}, nil
		}
		return nil, err
	}

	var projects []Project
	if len(data) == 0 {
		return []Project{}, nil
	}
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// Save writes the whole registry back to disk, creating the directory if
// needed.
func Save(projects []Project) error {
	path, err := dbPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// Register adds a project or updates the existing entry with the same name.
func Register(p Project) error {
	projects, err := Load()
	if err != nil {
		return err
	}

	for i := range projects {
		if projects[i].Name == p.Name {
			projects[i] = p
			return Save(projects)
		}
	}

	projects = append(projects, p)
	return Save(projects)
}

// Find returns the project with the given name.
func Find(name string) (Project, error) {
	projects, err := Load()
	if err != nil {
		return Project{}, err
	}
	for _, p := range projects {
		if p.Name == name {
			return p, nil
		}
	}
	return Project{}, ErrNotFound
}

// Branch returns the current git branch for display, or "" if the project is
// not a git repository. It reads .git/HEAD directly — Crabby does not run git
// or perform any git operations; this is purely informational.
func (p Project) Branch() string {
	data, err := os.ReadFile(filepath.Join(p.Path, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(data))
	if ref, ok := strings.CutPrefix(head, "ref: refs/heads/"); ok {
		return ref
	}
	// Detached HEAD: show a short commit hash if it looks like one.
	if len(head) >= 7 && isHex(head) {
		return head[:7]
	}
	return ""
}

func isHex(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// FindByPath returns the project registered at the given directory.
func FindByPath(path string) (Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Project{}, err
	}
	projects, err := Load()
	if err != nil {
		return Project{}, err
	}
	for _, p := range projects {
		if p.Path == abs {
			return p, nil
		}
	}
	return Project{}, ErrNotFound
}
