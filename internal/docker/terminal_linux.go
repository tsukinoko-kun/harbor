//go:build linux

package docker

import (
	"context"
	"os/exec"

	"github.com/tsukinoko-kun/harbor/internal/config"
)

// buildTerminalCommand creates the appropriate command for the given terminal on Linux.
// Returns the command and whether to use Run() instead of Start().
func buildTerminalCommand(ctx context.Context, terminal *config.Terminal, dockerCmd string) (*exec.Cmd, bool) {
	switch terminal.Name {
	case "ghostty":
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh", "-c", dockerCmd), false

	case "kitty":
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh", "-c", dockerCmd), false

	case "alacritty":
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh", "-c", dockerCmd), false

	case "wezterm":
		return exec.CommandContext(ctx, terminal.Path, "start", "--", "sh", "-c", dockerCmd), false

	case "konsole":
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh", "-c", dockerCmd), false

	case "xfce4-terminal":
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh -c '"+dockerCmd+"'"), false

	case "gnome-terminal":
		return exec.CommandContext(ctx, terminal.Path, "--", "sh", "-c", dockerCmd), false

	case "xterm":
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh", "-c", dockerCmd), false

	case "urxvt", "rxvt-unicode":
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh", "-c", dockerCmd), false

	case "terminator":
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh -c '"+dockerCmd+"'"), false

	case "tilix":
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh -c '"+dockerCmd+"'"), false

	default:
		// Fallback: try -e flag which is common for most Linux terminals
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh", "-c", dockerCmd), false
	}
}

