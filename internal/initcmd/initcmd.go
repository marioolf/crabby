// Package initcmd implements `crabby init`: preparing a project so it is
// immediately ready to work with AI Agent, optionally from a reusable pack.
package initcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/marioolf/crabby/internal/pack"
	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/session"
)

const claudeMD = `# Project

Describe the project.

# Architecture

Describe the architecture.

# Coding conventions

Describe conventions.

# Important notes

Write important information here.
`

// Result reports what Init did so the caller can print a friendly summary.
type Result struct {
	Project project.Project
	PackID  string   // the pack used, or "" for the built-in default
	Created []string // files newly created (relative paths)
	Skipped []string // files left untouched because they already existed
}

// Init prepares the project rooted at dir. When p is non-nil its files are
// copied in (never overwriting existing ones); otherwise a minimal default
// CLAUDE.md is created. Either way, Crabby's own .claude/crabby.yaml is written
// and the project is registered.
func Init(dir string, p *pack.Pack, agentCmd string) (Result, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Result{}, err
	}

	// Only initialize a directory that already exists — never conjure one from a
	// mistyped path.
	if info, err := os.Stat(abs); err != nil {
		return Result{}, fmt.Errorf("cannot initialize %q: %w", dir, err)
	} else if !info.IsDir() {
		return Result{}, fmt.Errorf("%q is not a directory", dir)
	}

	baseName := filepath.Base(abs)
	name := uniqueWorkspaceName(baseName, abs, agentCmd)
	if err := validateWorkspaceName(name); err != nil {
		return Result{}, err
	}
	proj := project.Project{
		Name: name,
		Path: abs,
		Tasks: []project.Task{
			{
				Name:         project.DefaultTask,
				Session:      session.Name(name),
				AgentCommand: agentCmd,
			},
		},
		AgentCommand: agentCmd,
	}
	res := Result{Project: proj}

	claudeDir := filepath.Join(abs, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return res, err
	}
	if info, err := os.Lstat(claudeDir); err != nil {
		return res, err
	} else if info.Mode()&os.ModeSymlink != 0 {
		return res, fmt.Errorf(".claude is a symlink")
	}

	if p != nil {
		copied, skipped, err := pack.Apply(*p, abs)
		if err != nil {
			return res, fmt.Errorf("applying pack %q: %w", p.Name, err)
		}
		res.PackID = p.Name
		res.Created = append(res.Created, copied...)
		res.Skipped = append(res.Skipped, skipped...)
	} else {
		if created, err := writeIfMissing(filepath.Join(abs, "CLAUDE.md"), claudeMD); err != nil {
			return res, err
		} else if created {
			res.Created = append(res.Created, "CLAUDE.md")
		} else {
			res.Skipped = append(res.Skipped, "CLAUDE.md")
		}
	}

	// Crabby owns .claude/crabby.yaml (project metadata for the registry), so it
	// is always written with the correct values regardless of the pack.
	meta := fmt.Sprintf("name: %s\npath: %s\nsession: %s\nagent_command: %s\n", proj.Name, proj.Path, proj.Tasks[0].Session, proj.AgentCommand)
	crabbyPath := filepath.Join(claudeDir, "crabby.yaml")
	if info, err := os.Lstat(crabbyPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return res, fmt.Errorf("crabby.yaml is a symlink")
	}

	f, err := openNoFollow(crabbyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return res, err
	}
	if _, err := f.Write([]byte(meta)); err != nil {
		f.Close()
		return res, err
	}
	if err := f.Close(); err != nil {
		return res, err
	}

	if err := project.Register(proj); err != nil {
		return res, err
	}
	return res, nil
}

// writeIfMissing writes content to path only if it does not already exist.
func writeIfMissing(path, content string) (bool, error) {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("%q is a symlink", path)
		}
		return false, nil // File exists
	} else if !os.IsNotExist(err) {
		return false, err
	}

	f, err := openNoFollow(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return false, err
	}
	if _, err := f.Write([]byte(content)); err != nil {
		f.Close()
		return false, err
	}
	if err := f.Close(); err != nil {
		return false, err
	}
	return true, nil
}

// openNoFollow opens the final path component relative to a directory descriptor.
// This keeps a checked directory from being replaced by a symlink before the
// file is opened.
func openNoFollow(path string, flags int, perm os.FileMode) (*os.File, error) {
	parent := filepath.Dir(path)
	base := filepath.Base(path)
	dirFD, err := syscall.Open(parent, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(dirFD)

	fd, err := syscall.Openat(dirFD, base, flags|syscall.O_NOFOLLOW, uint32(perm.Perm()))
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

// validateWorkspaceName ensures the workspace name contains only safe characters.
func validateWorkspaceName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return fmt.Errorf("invalid character %q in name", r)
		}
	}
	return nil
}

// uniqueWorkspaceName ensures the workspace name is unique in the registry.
// If the exact same path and agent already exist, it returns the existing name
// so the workspace is updated rather than duplicated.
func uniqueWorkspaceName(baseName, path, agentCmd string) string {
	projects, err := project.Load()
	if err != nil {
		return sanitizeName(baseName)
	}

	for _, p := range projects {
		if p.Path == path && p.AgentCommand == agentCmd {
			return p.Name
		}
	}

	agentBin := ""
	if agentCmd != "" {
		fields := strings.Fields(agentCmd)
		if len(fields) > 0 {
			agentBin = filepath.Base(fields[0])
		}
	}

	name := sanitizeName(baseName)
	suffix := ""

	for i := 1; ; i++ {
		if i == 2 && agentBin != "" {
			suffix = "-" + sanitizeName(agentBin)
		} else if i > 2 && agentBin != "" {
			suffix = fmt.Sprintf("-%s-%d", sanitizeName(agentBin), i)
		} else if i > 1 {
			suffix = fmt.Sprintf("-%d", i)
		}

		candidate := name + suffix

		collision := false
		for _, p := range projects {
			if p.Name == candidate {
				collision = true
				break
			}
		}
		if !collision {
			return candidate
		}
	}
}

// sanitizeName strips out unsafe characters from the generated workspace name.
func sanitizeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "workspace"
	}
	return b.String()
}
