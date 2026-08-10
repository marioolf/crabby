// Package importcmd discovers existing AI Agent projects so they can be
// adopted into Crabby without re-initializing them.
//
// A folder qualifies simply by containing a CLAUDE.md (or claude.md) — a git
// repository is not required (git only ever supplied the branch shown for
// context). Discovery walks a directory tree once, pruning dependency folders
// and stopping at the first context file on any branch, so a project's own
// subdirectory context files are never imported as separate workspaces.
//
// Discover scans a single root the caller names; DiscoverDefault scans the
// machine's usual development locations across WSL and the mounted Windows
// drives, so the user can adopt everything without typing a path.
package importcmd

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/marioolf/crabby/internal/project"
)

// Workspace is a discovered AI Agent project, ready to be imported.
type Workspace struct {
	Name     string // directory base name
	Path     string // absolute path
	Branch   string // current git branch, or "" when the folder is not a git repo
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
	"build":        true,
	"dist":         true,
	"__pycache__":  true,
	".tox":         true,
	".mypy_cache":  true,
	".next":        true,
	".cache":       true,
	".gradle":      true,
}

// contextFiles are the names that mark a Agent workspace, matched
// case-sensitively against a small set rather than statting the whole directory.
var contextFiles = []string{"CLAUDE.md", "claude.md", "Agent.md"}

// Discover walks root and returns every folder containing a context file found
// beneath it, sorted by name. Already-registered projects are flagged rather
// than omitted, so the selector can show them as imported. Unreadable
// directories are skipped silently rather than aborting the scan.
func Discover(root string) ([]Workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	found := walkRoot(abs, -1, registeredPaths())
	sort.Slice(found, func(i, j int) bool { return found[i].Name < found[j].Name })
	return found, nil
}

// DiscoverDefault scans the machine's usual development locations — the WSL home
// and the common project folders under each mounted Windows user — and returns
// every workspace found, de-duplicated and sorted. Roots are scanned in parallel
// because the Windows mounts are comparatively slow, and each is depth-capped so
// a stray deep tree can't stall the scan.
func DiscoverDefault() ([]Workspace, error) {
	registered := registeredPaths()
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		out   []Workspace
		seen  = map[string]bool{}
		roots = DefaultRoots()
	)
	for _, root := range roots {
		root := root
		depth := 8
		if strings.HasPrefix(root, "/mnt/") {
			depth = 6 // the Windows mounts are slow; keep those walks shallow
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			ws := walkRoot(root, depth, registered)
			mu.Lock()
			for _, w := range ws {
				if !seen[w.Path] {
					seen[w.Path] = true
					out = append(out, w)
				}
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

// DefaultRoots is the list of directories DiscoverDefault scans: the WSL home,
// plus the common development folders under each Windows user on every mounted
// drive. Only directories that actually exist are returned.
func DefaultRoots() []string {
	var roots []string
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, home)
	}
	for _, drive := range []string{"c", "d", "e"} {
		users := filepath.Join("/mnt", drive, "Users")
		entries, err := os.ReadDir(users)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || skipWinUser(e.Name()) {
				continue
			}
			userHome := filepath.Join(users, e.Name())
			for _, sub := range winDevSubdirs {
				p := filepath.Join(userHome, sub)
				if isDir(p) {
					roots = append(roots, p)
				}
			}
		}
	}
	return dedupe(roots)
}

// winDevSubdirs are the folders under a Windows user home where projects usually
// live. Scanning these (rather than the whole home) keeps the slow /mnt walk
// targeted and quick.
var winDevSubdirs = []string{
	"Desktop", "Documents", "source", "source/repos", "repos",
	"projects", "Projects", "dev", "code", "git",
	"OneDrive/Desktop", "OneDrive/Documents",
}

func skipWinUser(name string) bool {
	switch name {
	case "Public", "Default", "Default User", "All Users":
		return true
	}
	return false
}

// validateWorkspaceName ensures the workspace name contains only safe characters.
func validateWorkspaceName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// walkRoot walks one root, returning the workspaces beneath it. maxDepth < 0
// means no limit; otherwise directories deeper than maxDepth below root are not
// descended into.
func walkRoot(root string, maxDepth int, registered map[string]bool) []Workspace {
	var found []Workspace
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry — keep scanning the rest
		}
		if !d.IsDir() {
			return nil
		}
		// Prune dependency folders and hidden directories (never the root the
		// caller named). Skipping dot-directories keeps config and cache trees —
		// ~/.claude, ~/.config, ~/.cache and the like — out of the results, which
		// otherwise surface Crabby's own packs and the global Agent config.
		if path != root && (ignoredDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
			return fs.SkipDir
		}
		if maxDepth >= 0 && depthUnder(root, path) > maxDepth {
			return fs.SkipDir
		}
		if !hasContextFile(path) {
			return nil
		}
		// A context file marks a project. Record it (with its git branch if it
		// happens to be a repo) and stop descending, so nested context files
		// deeper in the project aren't imported as separate workspaces.
		name := filepath.Base(path)
		if !validateWorkspaceName(name) {
			return fs.SkipDir
		}

		var branch string
		func() {
			defer func() { recover() }()
			branch = project.Project{Path: path}.Branch()
		}()

		found = append(found, Workspace{
			Name:     name,
			Path:     path,
			Branch:   branch,
			Imported: registered[path],
		})
		return fs.SkipDir
	})
	return found
}

// depthUnder reports how many directory levels path sits below root.
func depthUnder(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	return strings.Count(rel, string(os.PathSeparator)) + 1
}

func hasContextFile(dir string) bool {
	for _, name := range contextFiles {
		if fileExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

// registeredPaths returns the set of paths already registered in Crabby, so
// discovery can flag what is already imported.
func registeredPaths() map[string]bool {
	registered := map[string]bool{}
	if projects, err := project.Load(); err == nil {
		for _, p := range projects {
			registered[p.Path] = true
		}
	}
	return registered
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
