// Package initcmd implements `crabby init`: preparing a project so it is
// immediately ready to work with Claude Code.
package initcmd

import (
	"fmt"
	"os"
	"path/filepath"

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

// Result reports what Init did, so the caller can print a friendly summary.
type Result struct {
	Project project.Project
	// Created lists the files/directories that were newly created.
	Created []string
}

// Init prepares the project rooted at dir: it creates .claude/crabby.yaml and
// CLAUDE.md, then registers the project.
func Init(dir string) (Result, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Result{}, err
	}

	name := filepath.Base(abs)
	p := project.Project{
		Name:    name,
		Path:    abs,
		Session: session.Name(name),
	}
	res := Result{Project: p}

	claudeDir := filepath.Join(abs, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return res, err
	}

	// .claude/crabby.yaml — project metadata.
	metaPath := filepath.Join(claudeDir, "crabby.yaml")
	meta := fmt.Sprintf("name: %s\npath: %s\nsession: %s\n", p.Name, p.Path, p.Session)
	if created, err := writeIfMissing(metaPath, meta); err != nil {
		return res, err
	} else if created {
		res.Created = append(res.Created, metaPath)
	}

	// CLAUDE.md — the small, user-customized starting point.
	claudePath := filepath.Join(abs, "CLAUDE.md")
	if created, err := writeIfMissing(claudePath, claudeMD); err != nil {
		return res, err
	} else if created {
		res.Created = append(res.Created, claudePath)
	}

	if err := project.Register(p); err != nil {
		return res, err
	}
	return res, nil
}

// writeIfMissing writes content to path only if it does not already exist, so
// re-running init never clobbers a customized file.
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
