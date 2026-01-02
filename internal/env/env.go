package env

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// commonDockerPaths contains the most common Docker installation paths.
var commonDockerPaths = []string{
	"/usr/local/bin/docker",
	"/opt/homebrew/bin/docker",
	"/usr/bin/docker",
	"/snap/bin/docker",
}

// commonDockerPathsWindows contains common Docker paths on Windows.
var commonDockerPathsWindows = []string{
	`C:\Program Files\Docker\Docker\resources\bin\docker.exe`,
	`C:\ProgramData\DockerDesktop\version-bin\docker.exe`,
}

// commonDockerBinDirs contains directories where Docker binaries and helpers
// (like docker-credential-osxkeychain) are typically installed.
var commonDockerBinDirs = []string{
	"/Applications/Docker.app/Contents/Resources/bin", // macOS Docker Desktop
	"/usr/local/bin",
	"/opt/homebrew/bin",
	"/usr/bin",
	"/snap/bin",
}

// GetDockerPath returns the path to the docker executable.
// It first checks if docker is in PATH, then searches common installation locations.
// Returns "docker" as fallback if not found (relying on PATH).
func GetDockerPath() string {
	// First, try to find docker in PATH
	if path, err := exec.LookPath("docker"); err == nil {
		return path
	}

	// Search common paths based on OS
	var paths []string
	if runtime.GOOS == "windows" {
		paths = commonDockerPathsWindows
	} else {
		paths = commonDockerPaths
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Fallback to "docker" and hope it's in PATH
	return "docker"
}

// EnsureDockerInPath checks if Docker's bin directories are in PATH and adds
// them if they're missing. This ensures docker-credential-* helpers and other
// Docker tools can be found. This is especially important on macOS when
// launching from Finder.
func EnsureDockerInPath() {
	if runtime.GOOS == "windows" {
		// Windows doesn't typically have this issue
		return
	}

	currentPath := os.Getenv("PATH")
	pathParts := strings.Split(currentPath, ":")
	pathSet := make(map[string]bool)
	for _, p := range pathParts {
		pathSet[p] = true
	}

	// Add Docker directories that exist and aren't already in PATH
	var additions []string
	for _, dir := range commonDockerBinDirs {
		if !pathSet[dir] {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				additions = append(additions, dir)
			}
		}
	}

	if len(additions) > 0 {
		newPath := strings.Join(additions, ":") + ":" + currentPath
		os.Setenv("PATH", newPath)
	}
}
