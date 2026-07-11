// Package config loads Crabby's minimal global configuration.
//
// Configuration lives at ~/.config/crabby/config.yaml and only ever holds a
// couple of keys. To avoid a YAML dependency for such a trivial file we parse
// the handful of "key: value" lines ourselves.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Config holds every setting Crabby understands.
type Config struct {
	ClaudeCommand string
	TmuxBinary    string
	// DetachKey is the single, prefix-free key that returns from a session to
	// Crabby (default "F12"). It uses tmux key names (e.g. "F12", "C-g").
	DetachKey string
}

// Default returns the configuration used when no config file exists.
func Default() Config {
	return Config{
		ClaudeCommand: "claude",
		TmuxBinary:    "tmux",
		DetachKey:     "F12",
	}
}

// Path returns the location of the global config file.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "crabby", "config.yaml"), nil
}

// Load reads the config file, falling back to defaults for any missing key. A
// missing file is not an error: defaults are returned.
func Load() (Config, error) {
	cfg := Default()

	path, err := Path()
	if err != nil {
		return cfg, err
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, value, ok := splitKeyValue(scanner.Text())
		if !ok {
			continue
		}
		switch key {
		case "claude_command":
			cfg.ClaudeCommand = value
		case "tmux_binary":
			cfg.TmuxBinary = value
		case "detach_key":
			cfg.DetachKey = value
		}
	}
	return cfg, scanner.Err()
}

// splitKeyValue parses a single "key: value" line, ignoring blanks and
// comments.
func splitKeyValue(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	value = strings.Trim(value, `"'`)
	return key, value, key != ""
}
