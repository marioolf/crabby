// Package importcmd discovers existing Claude Code repositories so they can be
// adopted into Crabby without re-initializing them.
//
// A repository qualifies when it is a git repository (has a .git) and already
// contains a CLAUDE.md. Discovery walks a directory tree once, pruning
// dependency folders and stopping at each git repository so nested repositories
// inside an imported one are never double-counted.
package importcmd

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/marioolf/crabby/internal/project"
)

// Workspace is a discovered Claude Code repository, ready to be imported.
type Workspace struct {
	Name     string // directory base name
	Path     string // absolute path
	Branch   string // current git branch, or "" if it cannot be read
	Imported bool   // already registered in Crabby
}

// ignoredDirs are never descended into: version-control internals and the
// obvious dependency/build folders. Keeping the walk out of these is what makes
// scanning large collections fast.
var ignoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".venv":        true,
	"venv":         true,
	"vendor":       true,
	"target":       true,
	"__pycache__":  true,
	".tox":         true,
	".mypy_cache":  true,
}

// Discover walks root and returns every Claude workspace found beneath it,
// sorted by name. Already-registered projects are flagged rather than omitted,
// so the selector can show them as imported. Unreadable directories are skipped
// silently rather than aborting the scan.
func Discover(root string) ([]Workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	// Registered paths, so we can mark what is already imported.
	registered := map[string]bool{}
	if projects, err := project.Load(); err == nil {
		for _, p := range projects {
			registered[p.Path] = true
		}
	}

	var found []Workspace
	walkErr := filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry — keep scanning the rest
		}
		if !d.IsDir() {
			return nil
		}
		// Prune dependency folders, but never the root the user asked for.
		if path != abs && ignoredDirs[d.Name()] {
			return fs.SkipDir
		}
		if !isGitRepo(path) {
			return nil
		}
		// A git repository is one unit: record it if it has a CLAUDE.md, then
		// stop descending so nested repositories inside it are ignored.
		if fileExists(filepath.Join(path, "CLAUDE.md")) {
			found = append(found, Workspace{
				Name:     filepath.Base(path),
				Path:     path,
				Branch:   project.Project{Path: path}.Branch(),
				Imported: registered[path],
			})
		}
		return fs.SkipDir
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(found, func(i, j int) bool { return found[i].Name < found[j].Name })
	return found, nil
}

// isGitRepo reports whether dir contains a .git entry (a directory for a normal
// clone, or a file for worktrees and submodules).
func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
