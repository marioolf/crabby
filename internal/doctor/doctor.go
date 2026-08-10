// Package doctor verifies that the environment Crabby depends on is present.
package doctor

import (
	"os"
	"os/exec"
	"strings"

	"github.com/marioolf/crabby/internal/config"
)

// Check is a single environment probe and its result.
type Check struct {
	Name string
	OK   bool
	Note string
}

// Run performs every environment check and returns the results in display
// order.
func Run(cfg config.Config) []Check {
	return []Check{
		checkWSL(),
		checkUbuntu(),
		checkBinary("tmux installed", cfg.TmuxBinary),
		checkBinary("Agent installed", cfg.ClaudeCommand),
		checkCrabby(),
	}
}

// checkWSL reports whether we appear to be running inside WSL.
func checkWSL() Check {
	c := Check{Name: "WSL installed"}
	data, err := os.ReadFile("/proc/version")
	if err == nil && strings.Contains(strings.ToLower(string(data)), "microsoft") {
		c.OK = true
		return c
	}
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		c.OK = true
		return c
	}
	c.Note = "not running inside WSL"
	return c
}

// checkUbuntu reports whether the distro looks like Ubuntu.
func checkUbuntu() Check {
	c := Check{Name: "Ubuntu available"}
	if distro := os.Getenv("WSL_DISTRO_NAME"); strings.Contains(strings.ToLower(distro), "ubuntu") {
		c.OK = true
		return c
	}
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		if strings.Contains(strings.ToLower(string(data)), "ubuntu") {
			c.OK = true
			return c
		}
	}
	c.Note = "Ubuntu not detected"
	return c
}

// checkBinary reports whether a binary exists on PATH.
func checkBinary(name, binary string) Check {
	c := Check{Name: name}
	if _, err := exec.LookPath(binary); err == nil {
		c.OK = true
		return c
	}
	c.Note = binary + " not found on PATH"
	return c
}

// checkCrabby reports whether the crabby binary itself is on PATH.
func checkCrabby() Check {
	c := Check{Name: "crabby installed"}
	if _, err := exec.LookPath("crabby"); err == nil {
		c.OK = true
		return c
	}
	c.Note = "crabby not found on PATH (add it to use the ps/attach workflow)"
	return c
}
