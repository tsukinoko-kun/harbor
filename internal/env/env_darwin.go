//go:build darwin

package env

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// dockerDesktopBinPaths contains paths where Docker Desktop installs its binaries.
// These paths contain docker-credential-osxkeychain and other helpers.
var dockerDesktopBinPaths = []string{
	"/Applications/Docker.app/Contents/Resources/bin",
	"/usr/local/bin",
	"/opt/homebrew/bin",
}

// LoadShellEnvironment loads environment variables from the user's default shell.
// On macOS, when launching from Finder, the shell environment (including PATH)
// is not inherited. This function spawns the user's shell as a login shell
// to capture the environment variables that would normally be set.
//
// Returns a map of environment variables. Returns an empty map on error.
func LoadShellEnvironment() map[string]string {
	result := make(map[string]string)

	// Get the user's default shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh" // macOS default
	}

	// Create a context with timeout to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run the shell as a login shell to load profile, then print environment
	// -l: login shell (loads .zprofile, .zshrc, etc.)
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

	// Ensure Docker Desktop bin paths are in PATH for credential helpers
	// (docker-credential-osxkeychain, etc.)
	if path, ok := result["PATH"]; ok {
		result["PATH"] = ensureDockerPaths(path)
	}

	return result
}

// ensureDockerPaths adds Docker Desktop bin directories to PATH if they exist
// and aren't already included. This ensures docker-credential-osxkeychain
// and other Docker helpers can be found.
func ensureDockerPaths(currentPath string) string {
	pathParts := strings.Split(currentPath, ":")
	pathSet := make(map[string]bool)
	for _, p := range pathParts {
		pathSet[p] = true
	}

	// Add Docker paths that exist and aren't already in PATH
	var additions []string
	for _, dockerPath := range dockerDesktopBinPaths {
		if !pathSet[dockerPath] {
			// Check if the directory exists
			if info, err := os.Stat(dockerPath); err == nil && info.IsDir() {
				additions = append(additions, dockerPath)
			}
		}
	}

	if len(additions) > 0 {
		return strings.Join(additions, ":") + ":" + currentPath
	}
	return currentPath
}
