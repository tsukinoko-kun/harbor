package docker

import (
	"context"
	"fmt"
	"time"

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
	cli := c.cli
	c.mu.RUnlock()

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

		execID, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
		if err != nil {
			// Shell likely doesn't exist
			continue
		}

		// Start the exec to verify the shell exists and works
		// The error from ContainerExecStart will tell us if the shell binary is missing
		err = cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{
			Detach: false,
		})
		if err != nil {
			// Shell doesn't exist or failed to start
			continue
		}

		// Give the exec a moment to complete
		time.Sleep(50 * time.Millisecond)

		// Inspect the exec to check if it completed successfully
		inspect, err := cli.ContainerExecInspect(ctx, execID.ID)
		if err != nil {
			// Couldn't inspect, try next shell
			continue
		}

		// If still running, wait a bit more
		if inspect.Running {
			time.Sleep(100 * time.Millisecond)
			inspect, err = cli.ContainerExecInspect(ctx, execID.ID)
			if err != nil {
				continue
			}
		}

		// Only accept shell if it completed successfully (exit code 0)
		if inspect.ExitCode == 0 && !inspect.Running {
			// Shell exists and works
			return shell, nil
		}
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
