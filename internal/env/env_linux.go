//go:build linux

package env

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// LoadShellEnvironment loads environment variables from the user's default shell.
// On Linux, when launching from desktop launchers (AppImage, Flatpak, etc.),
// the shell environment may not be inherited. This function spawns the user's
// shell as a login shell to capture the environment variables.
//
// Returns a map of environment variables. Returns an empty map on error.
func LoadShellEnvironment() map[string]string {
	result := make(map[string]string)

	// Get the user's default shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash" // Linux default
	}

	// Create a context with timeout to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run the shell as a login shell to load profile, then print environment
	// -l: login shell (loads .bash_profile, .bashrc, etc.)
	// -c: execute command
	cmd := exec.CommandContext(ctx, shell, "-l", "-c", "printenv")

	// Don't inherit current environment - we want only what the shell sets up
	cmd.Env = []string{
		"HOME=" + os.Getenv("HOME"),
		"USER=" + os.Getenv("USER"),
		"SHELL=" + shell,
		"TERM=xterm-256color",
	}

	output, err := cmd.Output()
	if err != nil {
		return result
	}

	// Parse the output line by line
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, "="); idx > 0 {
			key := line[:idx]
			value := line[idx+1:]
			result[key] = value
		}
	}

	return result
}
