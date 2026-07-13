// Package project manages the registry of Crabby projects.
//
// The registry is a single JSON file at
// ~/.local/share/crabby/projects.json. No database, JSON is enough.
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotFound is returned when a project cannot be located in the registry.
var ErrNotFound = errors.New("project not found")

// ErrTaskExists is returned when a task name is already taken in a workspace.
var ErrTaskExists = errors.New("a task with that name already exists")

// DefaultTask is the implicit task every workspace starts with, so a workspace
// with a single session looks and behaves exactly like it did before tasks
// existed. Its session keeps the legacy "crabby_<workspace>" name.
const DefaultTask = "main"

// Task is one Claude Code session inside a workspace. A workspace may hold
// several, each an independent session sharing the same project directory.
type Task struct {
	Name    string    `json:"name"`
	Session string    `json:"session"`
	Created time.Time `json:"created,omitempty"`
}

// Project is a registered workspace. It owns one or more tasks; the legacy
// Session field is kept only so older registries migrate cleanly.
type Project struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Session string `json:"session,omitempty"` // legacy single session (pre-tasks)
	Tasks   []Task `json:"tasks,omitempty"`
}

// withTasks fills in the implicit default task for a workspace that has none,
// mapping it onto the legacy session name so existing sessions are still
// recognised. This runs on load, so callers always see at least one task.
func (p Project) withTasks() Project {
	if len(p.Tasks) > 0 {
		return p
	}
	legacy := p.Session
	if legacy == "" {
		legacy = "crabby_" + p.Name
	}
	p.Tasks = []Task{{Name: DefaultTask, Session: legacy}}
	return p
}

// Task returns the named task within the workspace.
func (p Project) Task(name string) (Task, bool) {
	for _, t := range p.Tasks {
		if t.Name == name {
			return t, true
		}
	}
	return Task{}, false
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
	for i := range projects {
		projects[i] = projects[i].withTasks()
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

// Remove deletes a project from the registry. It does not touch any files on
// disk. Returns ErrNotFound if no such project is registered.
func Remove(name string) error {
	projects, err := Load()
	if err != nil {
		return err
	}
	out := make([]Project, 0, len(projects))
	found := false
	for _, p := range projects {
		if p.Name == name {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		return ErrNotFound
	}
	return Save(out)
}

// Find returns the project with the given name.
func Find(name string) (Project, error) {
	projects, err := Load()
	if err != nil {
		return Project{}, err
	}
	for _, p := range projects {
		if p.Name == name {
			return p.withTasks(), nil
		}
	}
	return Project{}, ErrNotFound
}

// AddTask registers a new task in a workspace. It fails if the workspace is
// unknown or the task name is already taken.
func AddTask(workspace string, t Task) error {
	projects, err := Load()
	if err != nil {
		return err
	}
	for i := range projects {
		if projects[i].Name != workspace {
			continue
		}
		if _, exists := projects[i].Task(t.Name); exists {
			return ErrTaskExists
		}
		projects[i].Tasks = append(projects[i].Tasks, t)
		return Save(projects)
	}
	return ErrNotFound
}

// RemoveTask forgets a task. Removing the last task leaves the workspace itself
// in place, ready to accept new tasks.
func RemoveTask(workspace, taskName string) error {
	projects, err := Load()
	if err != nil {
		return err
	}
	for i := range projects {
		if projects[i].Name != workspace {
			continue
		}
		kept := projects[i].Tasks[:0]
		found := false
		for _, t := range projects[i].Tasks {
			if t.Name == taskName {
				found = true
				continue
			}
			kept = append(kept, t)
		}
		if !found {
			return fmt.Errorf("task %q not found in %q", taskName, workspace)
		}
		// Drop the legacy field so a fully task-managed workspace stops
		// resurrecting the old default on the next load.
		projects[i].Tasks = kept
		projects[i].Session = ""
		return Save(projects)
	}
	return ErrNotFound
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
