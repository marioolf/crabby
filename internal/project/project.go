// Package project manages the registry of Crabby projects.
//
// The registry is a single JSON file at
// ~/.local/share/crabby/projects.json. No database, JSON is enough.
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

// Task is one AI Agent session inside a workspace. A workspace may hold
// several, each an independent session sharing the same project directory.
type Task struct {
	Name         string    `json:"name"`
	Session      string    `json:"session"`
	Created      time.Time `json:"created,omitempty"`
	AgentCommand string    `json:"agent_command,omitempty"`
}

// Project is a registered workspace. It owns one or more tasks; the legacy
// Session field is kept only so older registries migrate cleanly.
type Project struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Session      string `json:"session,omitempty"` // legacy single session (pre-tasks)
	Tasks        []Task `json:"tasks,omitempty"`
	AgentCommand string `json:"agent_command,omitempty"`
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

	return loadPath(path)
}

func loadPath(path string) ([]Project, error) {
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
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}

	return withRegistryLock(path, func() error {
		return saveLocked(path, projects)
	})
}

func withRegistryLock(path string, fn func() error) error {
	dir := filepath.Dir(path)
	lockPath := filepath.Join(dir, ".lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lockFile.Close()
	if err := os.Chmod(lockPath, 0o600); err != nil {
		return err
	}
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	return fn()
}

func saveLocked(path string, projects []Project) error {
	dir := filepath.Dir(path)
	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".projects.json-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		_ = tmp.Close()
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTemp = false
	return os.Chmod(path, 0o600)
}

func updateRegistry(mutator func([]Project) ([]Project, error)) error {
	path, err := dbPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	return withRegistryLock(path, func() error {
		projects, err := loadPath(path)
		if err != nil {
			return err
		}
		projects, err = mutator(projects)
		if err != nil {
			return err
		}
		return saveLocked(path, projects)
	})
}

// Register adds a project or updates the existing entry with the same name.
func Register(p Project) error {
	return updateRegistry(func(projects []Project) ([]Project, error) {
		if err := validateProjectSessions(projects, p); err != nil {
			return nil, err
		}
		for i := range projects {
			if projects[i].Name == p.Name {
				projects[i] = p
				return projects, nil
			}
		}
		return append(projects, p), nil
	})
}

func validateProjectSessions(projects []Project, p Project) error {
	for _, other := range projects {
		if other.Name == p.Name {
			continue
		}
		for _, t := range p.Tasks {
			for _, ot := range other.Tasks {
				if t.Session == ot.Session {
					return fmt.Errorf("session %q collides with workspace %q", t.Session, other.Name)
				}
			}
			if other.Session != "" && t.Session == other.Session {
				return fmt.Errorf("session %q collides with workspace %q", t.Session, other.Name)
			}
		}
		if p.Session != "" {
			for _, ot := range other.Tasks {
				if p.Session == ot.Session {
					return fmt.Errorf("session %q collides with workspace %q", p.Session, other.Name)
				}
			}
			if other.Session != "" && p.Session == other.Session {
				return fmt.Errorf("session %q collides with workspace %q", p.Session, other.Name)
			}
		}
	}
	return nil
}

// validateWorkspaceName ensures the workspace name contains only safe characters.
func validateWorkspaceName(name string) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return fmt.Errorf("invalid character %q in name", r)
		}
	}
	return nil
}

// Relink points an existing workspace at a new directory — for when its folder
// has been moved or renamed — updating both its path and its name (to the new
// folder's base name). Its tasks are kept as they are: their tmux session names
// are stored, so the running sessions still match after the rename. Returns
// ErrNotFound if oldName isn't registered, or an error if newPath isn't an
// existing directory or its name is already taken by another workspace.
func Relink(oldName, newPath string) error {
	abs, err := filepath.Abs(newPath)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(abs); statErr != nil || !info.IsDir() {
		return fmt.Errorf("%q is not a directory", newPath)
	}
	newName := filepath.Base(abs)
	if err := validateWorkspaceName(newName); err != nil {
		return err
	}

	return updateRegistry(func(projects []Project) ([]Project, error) {
		idx := -1
		for i := range projects {
			if projects[i].Name == oldName {
				idx = i
			} else if projects[i].Name == newName {
				return nil, fmt.Errorf("a workspace named %q already exists", newName)
			}
		}
		if idx < 0 {
			return nil, ErrNotFound
		}
		projects[idx].Name = newName
		projects[idx].Path = abs
		return projects, nil
	})
}

// Remove deletes a project from the registry. It does not touch any files on
// disk. Returns ErrNotFound if no such project is registered.
func Remove(name string) error {
	return updateRegistry(func(projects []Project) ([]Project, error) {
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
			return nil, ErrNotFound
		}
		return out, nil
	})
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
	return updateRegistry(func(projects []Project) ([]Project, error) {
		for _, p := range projects {
			if p.Name == workspace {
				continue
			}
			for _, ot := range p.Tasks {
				if t.Session == ot.Session {
					return nil, fmt.Errorf("session %q collides with workspace %q", t.Session, p.Name)
				}
			}
			if p.Session != "" && t.Session == p.Session {
				return nil, fmt.Errorf("session %q collides with workspace %q", t.Session, p.Name)
			}
		}
		for i := range projects {
			if projects[i].Name != workspace {
				continue
			}
			if _, exists := projects[i].Task(t.Name); exists {
				return nil, ErrTaskExists
			}
			projects[i].Tasks = append(projects[i].Tasks, t)
			return projects, nil
		}
		return nil, ErrNotFound
	})
}

// RemoveTask forgets a task. Removing the last task leaves the workspace itself
// in place, ready to accept new tasks.
func RemoveTask(workspace, taskName string) error {
	return updateRegistry(func(projects []Project) ([]Project, error) {
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
				return nil, fmt.Errorf("task %q not found in %q", taskName, workspace)
			}
			// Drop the legacy field so a fully task-managed workspace stops
			// resurrecting the old default on the next load.
			projects[i].Tasks = kept
			projects[i].Session = ""
			return projects, nil
		}
		return nil, ErrNotFound
	})
}

// Branch returns the current git branch for display, or "" if the project is
// not a git repository. It reads .git/HEAD directly — Crabby does not run git
// or perform any git operations; this is purely informational.
func (p Project) Branch() string {
	headPath := filepath.Join(p.Path, ".git", "HEAD")
	info, err := os.Lstat(headPath)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return ""
	}
	f, err := os.Open(headPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, 8192))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(data))

	var ref string
	if r, ok := strings.CutPrefix(head, "ref: refs/heads/"); ok {
		ref = r
	} else if len(head) >= 7 && isHex(head) {
		ref = head[:7]
	} else {
		return ""
	}

	for _, r := range ref {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '/' || r == '-') {
			return ""
		}
	}
	return ref
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
