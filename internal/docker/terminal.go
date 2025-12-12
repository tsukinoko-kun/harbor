package docker

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/docker/docker/api/types/container"
	"github.com/tsukinoko-kun/harbor/internal/config"
)

// linuxShells are the shells to try for Linux containers, in order of preference.
var linuxShells = []string{"/bin/bash", "/bin/sh"}

// windowsShells are the shells to try for Windows containers, in order of preference.
var windowsShells = []string{"powershell.exe", "cmd.exe"}

// IsWindowsContainer checks if a container is a Windows container.
func (c *Client) IsWindowsContainer(ctx context.Context, containerID string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}

	// Check the OS field in the container's platform
	if info.Config != nil {
		// The Image's OS is stored in the image config, but we can also check
		// the platform from the container info
		return info.Platform == "windows", nil
	}

	return false, nil
}

// GetContainerShell detects the available shell in a container.
// For Linux containers, it tries /bin/bash first, then /bin/sh.
// For Windows containers, it tries powershell.exe first, then cmd.exe.
func (c *Client) GetContainerShell(ctx context.Context, containerID string) (string, error) {
	isWindows, err := c.IsWindowsContainer(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to detect container OS: %w", err)
	}

	shells := linuxShells
	if isWindows {
		shells = windowsShells
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, shell := range shells {
		// Try to create an exec instance to test if the shell exists
		execConfig := container.ExecOptions{
			Cmd:          []string{shell, "-c", "exit 0"},
			AttachStdout: false,
			AttachStderr: false,
		}

		// For Windows shells, use different test command
		if isWindows {
			if shell == "powershell.exe" {
				execConfig.Cmd = []string{shell, "-Command", "exit 0"}
			} else {
				execConfig.Cmd = []string{shell, "/c", "exit 0"}
			}
		}

		execID, err := c.cli.ContainerExecCreate(ctx, containerID, execConfig)
		if err != nil {
			continue
		}

		// Start the exec to verify it works
		err = c.cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{})
		if err != nil {
			continue
		}

		// Shell exists and works
		return shell, nil
	}

	// Default fallback
	if isWindows {
		return "cmd.exe", nil
	}
	return "/bin/sh", nil
}

// OpenTerminal opens a terminal window with a shell session in the specified container.
// It uses the terminal specified in settings.
func (c *Client) OpenTerminal(ctx context.Context, containerID string, terminal *config.Terminal) error {
	if terminal == nil {
		return fmt.Errorf("no terminal configured")
	}

	shell, err := c.GetContainerShell(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to detect shell: %w", err)
	}

	dockerExecCmd := fmt.Sprintf("docker exec -it %s %s", containerID, shell)

	cmd, useRun := buildTerminalCommand(ctx, terminal, dockerExecCmd)

	if useRun {
		// Run and wait for completion to catch errors
		output, err := cmd.CombinedOutput()
		if err != nil {
			if len(output) > 0 {
				return fmt.Errorf("failed to open terminal: %w: %s", err, string(output))
			}
			return fmt.Errorf("failed to open terminal: %w", err)
		}
	} else {
		// Start the command (don't wait for it to complete)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to open terminal: %w", err)
		}
	}

	return nil
}

// buildTerminalCommand creates the appropriate command for the given terminal.
// Returns the command and whether to use Run() instead of Start().
func buildTerminalCommand(ctx context.Context, terminal *config.Terminal, dockerCmd string) (*exec.Cmd, bool) {
	switch terminal.Name {
	case "ghostty":
		// On macOS, Ghostty CLI launching is not supported, use 'open' instead
		if runtime.GOOS == "darwin" {
			return exec.CommandContext(ctx, "open", "-na", "Ghostty", "--args", "-e", "sh", "-c", dockerCmd), true
		}
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh", "-c", dockerCmd), false

	case "kitty":
		// On macOS, kitty also needs special handling
		if runtime.GOOS == "darwin" {
			return exec.CommandContext(ctx, "open", "-na", "kitty", "--args", "-e", "sh", "-c", dockerCmd), true
		}
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh", "-c", dockerCmd), false

	case "alacritty":
		// On macOS, use 'open' for Alacritty as well
		if runtime.GOOS == "darwin" {
			return exec.CommandContext(ctx, "open", "-na", "Alacritty", "--args", "-e", "sh", "-c", dockerCmd), true
		}
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh", "-c", dockerCmd), false

	case "wezterm":
		// On macOS, use 'open' for WezTerm
		if runtime.GOOS == "darwin" {
			return exec.CommandContext(ctx, "open", "-na", "WezTerm", "--args", "start", "--", "sh", "-c", dockerCmd), true
		}
		return exec.CommandContext(ctx, terminal.Path, "start", "--", "sh", "-c", dockerCmd), false

	case "konsole", "xfce4-terminal":
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh", "-c", dockerCmd), false

	case "gnome-terminal":
		return exec.CommandContext(ctx, terminal.Path, "--", "sh", "-c", dockerCmd), false

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

	case "wt":
		// Windows Terminal
		return exec.CommandContext(ctx, terminal.Path, "cmd", "/k", dockerCmd), false

	case "cmd":
		// Windows CMD
		return exec.CommandContext(ctx, terminal.Path, "/c", "start", "cmd", "/k", dockerCmd), false

	default:
		// Fallback: try -e flag which is common
		return exec.CommandContext(ctx, terminal.Path, "-e", "sh", "-c", dockerCmd), false
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
