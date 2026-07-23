// Package initcmd implements `crabby init`: preparing a project so it is
// immediately ready to work with Claude Code, optionally from a reusable pack.
package initcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/marioolf/crabby/internal/agent"
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
func Init(dir string, p *pack.Pack) (Result, error) {
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

	name := filepath.Base(abs)
	proj := project.Project{
		Name: name,
		Path: abs,
		Tasks: []project.Task{
			{Name: project.DefaultTask, Session: session.Name(name), Agent: agent.DefaultID},
		},
	}
	res := Result{Project: proj}

	claudeDir := filepath.Join(abs, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return res, err
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
	meta := fmt.Sprintf("name: %s\npath: %s\nsession: %s\nagent: %s\n", proj.Name, proj.Path, proj.Tasks[0].Session, proj.Tasks[0].Agent)
	if err := os.WriteFile(filepath.Join(claudeDir, "crabby.yaml"), []byte(meta), 0o644); err != nil {
		return res, err
	}

	if err := project.Register(proj); err != nil {
		return res, err
	}
	return res, nil
}

// writeIfMissing writes content to path only if it does not already exist.
func writeIfMissing(path, content string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}
