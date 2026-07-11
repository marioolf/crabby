// Package pack implements Crabby's local project packs.
//
// A pack is just a directory of files that get copied into a project during
// `crabby init`. No plugins, no scripting, no variables, no remote sources —
// only folders. Packs live in ~/.config/crabby/packs/<name>/.
package pack

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// manifest is the file every pack must contain.
const manifest = "pack.yaml"

// Pack is a validated pack on disk.
type Pack struct {
	Name        string
	Description string
	Author      string
	Version     string
	Dir         string // absolute path to the pack directory
}

// Dir returns the directory where packs live.
func Dir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "crabby", "packs"), nil
}

// List returns every valid pack, sorted by name. Directories without a valid
// pack.yaml are skipped. A missing packs directory yields no packs (not an
// error).
func List() ([]Pack, error) {
	root, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var packs []Pack
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := Load(filepath.Join(root, e.Name()))
		if err != nil {
			continue // ignore non-pack directories
		}
		packs = append(packs, p)
	}
	sort.Slice(packs, func(i, j int) bool { return packs[i].Name < packs[j].Name })
	return packs, nil
}

// Load reads and validates a single pack directory.
func Load(dir string) (Pack, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Pack{}, err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return Pack{}, fmt.Errorf("pack directory not found: %s", abs)
	}

	data, err := os.ReadFile(filepath.Join(abs, manifest))
	if err != nil {
		return Pack{}, fmt.Errorf("%s has no %s", filepath.Base(abs), manifest)
	}

	p := parseManifest(data)
	p.Dir = abs
	if p.Name == "" {
		return Pack{}, fmt.Errorf("%s: %s is missing a 'name'", filepath.Base(abs), manifest)
	}
	return p, nil
}

// Apply copies the pack's files into projectDir. Existing files are never
// overwritten — they are reported in skipped instead. The manifest itself
// (pack.yaml) is not copied.
func Apply(p Pack, projectDir string) (copied, skipped []string, err error) {
	walkErr := filepath.WalkDir(p.Dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(p.Dir, path)
		if err != nil {
			return err
		}
		if rel == "." || rel == manifest {
			return nil
		}

		dest := filepath.Join(projectDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		if _, statErr := os.Stat(dest); statErr == nil {
			skipped = append(skipped, rel)
			return nil
		}
		if copyErr := copyFile(path, dest); copyErr != nil {
			return copyErr
		}
		copied = append(copied, rel)
		return nil
	})
	return copied, skipped, walkErr
}

func copyFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// parseManifest reads the handful of "key: value" lines we understand and
// ignores everything else (forward compatibility).
func parseManifest(data []byte) Pack {
	var p Pack
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.Trim(strings.TrimSpace(line[idx+1:]), `"'`)
		switch key {
		case "name":
			p.Name = val
		case "description":
			p.Description = val
		case "author":
			p.Author = val
		case "version":
			p.Version = val
		}
	}
	return p
}
