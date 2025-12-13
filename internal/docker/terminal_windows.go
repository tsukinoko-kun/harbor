//go:build windows

package docker

import (
	"context"
	"os/exec"

	"github.com/tsukinoko-kun/harbor/internal/config"
)

// buildTerminalCommand creates the appropriate command for the given terminal on Windows.
// Returns the command and whether to use Run() instead of Start().
func buildTerminalCommand(ctx context.Context, terminal *config.Terminal, dockerCmd string) (*exec.Cmd, bool) {
	switch terminal.Name {
	case "wt", "Windows Terminal":
		// Windows Terminal
		return exec.CommandContext(ctx, terminal.Path, "cmd", "/k", dockerCmd), false

	case "cmd", "Command Prompt":
		// Windows CMD
		return exec.CommandContext(ctx, terminal.Path, "/c", "start", "cmd", "/k", dockerCmd), false

	case "powershell", "PowerShell":
		// PowerShell
		return exec.CommandContext(ctx, terminal.Path, "-NoExit", "-Command", dockerCmd), false

	case "pwsh", "PowerShell Core":
		// PowerShell Core
		return exec.CommandContext(ctx, terminal.Path, "-NoExit", "-Command", dockerCmd), false

	case "alacritty":
		return exec.CommandContext(ctx, terminal.Path, "-e", "cmd", "/k", dockerCmd), false

	case "wezterm":
		return exec.CommandContext(ctx, terminal.Path, "start", "--", "cmd", "/k", dockerCmd), false

	case "kitty":
		return exec.CommandContext(ctx, terminal.Path, "-e", "cmd", "/k", dockerCmd), false

	default:
		// Fallback: use cmd to start the docker command
		return exec.CommandContext(ctx, "cmd", "/c", "start", "cmd", "/k", dockerCmd), false
	}
}


