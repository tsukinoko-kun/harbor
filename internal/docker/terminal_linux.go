//go:build linux

package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tsukinoko-kun/harbor/internal/config"
)

// isSnapPath checks if the given path is a Snap-installed application.
// Returns true and the snap name if it's a Snap app.
func isSnapPath(terminalPath string) (bool, string) {
	// Resolve symlinks to get the real path
	realPath, err := filepath.EvalSymlinks(terminalPath)
	if err != nil {
		realPath = terminalPath
	}

	// Check if path is under /snap/
	if strings.HasPrefix(realPath, "/snap/") {
		// Extract snap name from path like /snap/<snap-name>/...
		parts := strings.Split(strings.TrimPrefix(realPath, "/snap/"), "/")
		if len(parts) > 0 && parts[0] != "" && parts[0] != "bin" {
			return true, parts[0]
		}
	}

	// Also check if the original path is in /snap/bin/ (common location)
	if strings.HasPrefix(terminalPath, "/snap/bin/") {
		snapName := strings.TrimPrefix(terminalPath, "/snap/bin/")
		// Handle potential suffixed names like "app.name"
		if snapName != "" {
			return true, snapName
		}
	}

	// Check if the symlink target points to a snap
	if realPath != terminalPath && strings.Contains(realPath, "/snap/") {
		// Try to extract snap name from the real path
		if idx := strings.Index(realPath, "/snap/"); idx != -1 {
			remainder := realPath[idx+6:] // len("/snap/") = 6
			parts := strings.Split(remainder, "/")
			if len(parts) > 0 && parts[0] != "" && parts[0] != "bin" {
				return true, parts[0]
			}
		}
	}

	return false, ""
}

// buildSnapCommand creates a command that runs through snap run.
// Uses the pattern: snap run <snap-name> -- <args...>
func buildSnapCommand(ctx context.Context, snapName string, args ...string) *exec.Cmd {
	cmdArgs := []string{"run", snapName, "--"}
	cmdArgs = append(cmdArgs, args...)
	return exec.CommandContext(ctx, "snap", cmdArgs...)
}

// getTerminalArgs returns the arguments for a given terminal to execute a docker command.
// Returns the arguments (without the terminal binary itself).
func getTerminalArgs(terminalName, dockerCmd string) []string {
	switch terminalName {
	case "ghostty":
		return []string{"-e", "sh", "-c", dockerCmd}

	case "kitty":
		return []string{"-e", "sh", "-c", dockerCmd}

	case "alacritty":
		return []string{"-e", "sh", "-c", dockerCmd}

	case "wezterm":
		return []string{"start", "--", "sh", "-c", dockerCmd}

	case "konsole":
		return []string{"-e", "sh", "-c", dockerCmd}

	case "xfce4-terminal":
		return []string{"-e", "sh -c '" + dockerCmd + "'"}

	case "gnome-terminal":
		return []string{"--", "sh", "-c", dockerCmd}

	case "xterm":
		return []string{"-e", "sh", "-c", dockerCmd}

	case "urxvt", "rxvt-unicode":
		return []string{"-e", "sh", "-c", dockerCmd}

	case "terminator":
		return []string{"-e", "sh -c '" + dockerCmd + "'"}

	case "tilix":
		return []string{"-e", "sh -c '" + dockerCmd + "'"}

	default:
		// Fallback: try -e flag which is common for most Linux terminals
		return []string{"-e", "sh", "-c", dockerCmd}
	}
}

// buildTerminalCommand creates the appropriate command for the given terminal on Linux.
// Returns the command and whether to use Run() instead of Start().
func buildTerminalCommand(ctx context.Context, terminal *config.Terminal, dockerCmd string) (*exec.Cmd, bool) {
	args := getTerminalArgs(terminal.Name, dockerCmd)

	fmt.Printf("[DEBUG] buildTerminalCommand: terminal.Name=%s, terminal.Path=%s\n", terminal.Name, terminal.Path)
	fmt.Printf("[DEBUG] args: %v\n", args)

	// Check if this is a Snap-installed terminal
	if isSnap, snapName := isSnapPath(terminal.Path); isSnap {
		fmt.Printf("[DEBUG] Detected snap: %s\n", snapName)
		// Verify snap command exists
		if _, err := exec.LookPath("snap"); err == nil {
			// Also verify the snap is actually installed
			if _, err := os.Stat("/snap/" + snapName); err == nil {
				cmd := buildSnapCommand(ctx, snapName, args...)
				fmt.Printf("[DEBUG] Snap command: %s %v\n", cmd.Path, cmd.Args)
				// Return true to use Run() and capture output for debugging
				return cmd, true
			} else {
				fmt.Printf("[DEBUG] Snap dir not found: /snap/%s\n", snapName)
			}
		} else {
			fmt.Printf("[DEBUG] snap command not found\n")
		}
	} else {
		fmt.Printf("[DEBUG] Not a snap path\n")
	}

	// Regular non-snap terminal
	return exec.CommandContext(ctx, terminal.Path, args...), false
}
