#+build windows
package docker

import "core:c"
import "core:strings"

foreign import msvcrt "system:msvcrt.lib"

@(default_calling_convention = "c")
foreign msvcrt {
	@(link_name = "system")
	libc_system :: proc(command: cstring) -> c.int ---
}

// List of shells to try, in order of preference
SHELL_CANDIDATES :: []string{"/bin/bash", "/bin/zsh", "/bin/ash", "/bin/sh"}

// Open an interactive shell in a container using Windows Terminal
open_container_shell :: proc(container_id: string, container_name: string) {
	// Detect available shell in the container
	shell := detect_container_shell(container_id)

	// Build and execute the terminal command
	launch_terminal_with_docker_exec(container_id, container_name, shell)
}

// Detect the best available shell in a container
// Returns the path to the shell, or /bin/sh as fallback
detect_container_shell :: proc(container_id: string) -> string {
	// Try each shell candidate
	for shell in SHELL_CANDIDATES {
		if check_shell_exists(container_id, shell) {
			return shell
		}
	}

	// Fallback to /bin/sh
	return "/bin/sh"
}

// Check if a shell exists in the container by running a quick test
@(private)
check_shell_exists :: proc(container_id: string, shell_path: string) -> bool {
	// Build docker exec command to test if shell exists
	// Use test -x to check if the file exists and is executable
	cmd := strings.concatenate(
		{"docker exec ", container_id, " test -x ", shell_path, " >NUL 2>&1"},
		context.temp_allocator,
	)

	cmd_cstr := strings.clone_to_cstring(cmd, context.temp_allocator)
	result := libc_system(cmd_cstr)

	// Return true if exit code is 0 (shell exists)
	return result == 0
}

// Launch Windows Terminal with docker exec command
@(private)
launch_terminal_with_docker_exec :: proc(
	container_id: string,
	container_name: string,
	shell: string,
) {
	short_id := container_id[:12] if len(container_id) >= 12 else container_id

	// Build the docker exec command for Windows Terminal
	// wt new-tab opens a new tab in Windows Terminal
	// cmd /c is used to run the docker command
	cmd := strings.concatenate(
		{"start wt new-tab cmd /c docker exec -it ", short_id, " ", shell},
		context.temp_allocator,
	)

	cmd_cstr := strings.clone_to_cstring(cmd, context.temp_allocator)
	libc_system(cmd_cstr)
}
