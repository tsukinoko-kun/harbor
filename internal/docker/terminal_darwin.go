//go:build darwin

package docker

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/tsukinoko-kun/harbor/internal/config"
)

// buildTerminalCommand creates the appropriate command for the given terminal on macOS.
// Returns the command and whether to use Run() instead of Start().
func buildTerminalCommand(ctx context.Context, terminal *config.Terminal, dockerCmd string) (*exec.Cmd, bool) {
	switch terminal.Name {
	case "ghostty":
		// On macOS, Ghostty CLI launching is not supported, use 'open' instead
		return exec.CommandContext(ctx, "open", "-na", "Ghostty", "--args", "-e", "sh", "-c", dockerCmd), true

	case "kitty":
		// On macOS, kitty also needs special handling
		return exec.CommandContext(ctx, "open", "-na", "kitty", "--args", "-e", "sh", "-c", dockerCmd), true

	case "alacritty":
		// On macOS, use 'open' for Alacritty as well
		return exec.CommandContext(ctx, "open", "-na", "Alacritty", "--args", "-e", "sh", "-c", dockerCmd), true

	case "wezterm":
		// On macOS, use 'open' for WezTerm
		return exec.CommandContext(ctx, "open", "-na", "WezTerm", "--args", "start", "--", "sh", "-c", dockerCmd), true

	case "Terminal.app":
		// macOS: Use osascript to open Terminal.app
		escapedCmd := escapeAppleScript(dockerCmd)
		script := fmt.Sprintf(`
			tell application "Terminal"
				activate
				do script "%s"
			end tell
		`, escapedCmd)
		return exec.CommandContext(ctx, "osascript", "-e", script), true

	case "iTerm", "iTerm2", "iTerm.app":
		// macOS: Use osascript to open iTerm
		escapedCmd := escapeAppleScript(dockerCmd)
		script := fmt.Sprintf(`
			tell application "iTerm"
				activate
				create window with default profile command "%s"
			end tell
		`, escapedCmd)
		return exec.CommandContext(ctx, "osascript", "-e", script), true

	default:
		// Fallback: try using 'open' with -a flag for .app bundles, or direct execution
		return exec.CommandContext(ctx, "open", "-na", terminal.Name, "--args", "-e", "sh", "-c", dockerCmd), true
	}
}

// escapeAppleScript escapes a string for use in AppleScript.
func escapeAppleScript(s string) string {
	// Escape backslashes first, then quotes
	result := ""
	for _, c := range s {
		switch c {
		case '\\':
			result += "\\\\"
		case '"':
			result += "\\\""
		default:
			result += string(c)
		}
	}
	return result
}
