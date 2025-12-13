package docker

import (
	"context"
	"fmt"

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

// GetTerminalCommand returns the docker exec command string for opening a shell in the container.
// This is used by the clipboard feature to copy the command without executing it.
func (c *Client) GetTerminalCommand(ctx context.Context, containerID string) (string, error) {
	shell, err := c.GetContainerShell(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to detect shell: %w", err)
	}

	return fmt.Sprintf("docker exec -it %s %s", containerID, shell), nil
}

// OpenTerminal opens a terminal window with a shell session in the specified container.
// It uses the terminal specified in settings.
func (c *Client) OpenTerminal(ctx context.Context, containerID string, terminal *config.Terminal) error {
	if terminal == nil {
		return fmt.Errorf("no terminal configured")
	}

	dockerExecCmd, err := c.GetTerminalCommand(ctx, containerID)
	if err != nil {
		return err
	}

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

// buildTerminalCommand is implemented in platform-specific files:
// - terminal_darwin.go for macOS
// - terminal_linux.go for Linux
// - terminal_windows.go for Windows
// func buildTerminalCommand(ctx context.Context, terminal *config.Terminal, dockerCmd string) (*exec.Cmd, bool)
