#+build linux, darwin
package docker

import "core:c"
import "core:fmt"
import "core:strings"

foreign import libc "system:c"

@(default_calling_convention = "c")
foreign libc {
	@(link_name = "system")
	libc_system :: proc(command: cstring) -> c.int ---
}

// List of shells to try, in order of preference
SHELL_CANDIDATES :: []string{"/bin/bash", "/bin/zsh", "/bin/ash", "/bin/sh"}

// Open an interactive shell in a container using the system default terminal
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
		{"docker exec ", container_id, " test -x ", shell_path, " 2>/dev/null"},
		context.temp_allocator,
	)

	cmd_cstr := strings.clone_to_cstring(cmd, context.temp_allocator)
	result := libc_system(cmd_cstr)

	// Return true if exit code is 0 (shell exists)
	return result == 0
}

// Launch the system terminal with docker exec command
@(private)
launch_terminal_with_docker_exec :: proc(
	container_id: string,
	container_name: string,
	shell: string,
) {
	when ODIN_OS == .Darwin {
		launch_terminal_macos(container_id, container_name, shell)
	} else {
		launch_terminal_linux(container_id, container_name, shell)
	}
}

// macOS: Use osascript to open Terminal.app with the docker exec command
@(private)
launch_terminal_macos :: proc(container_id: string, container_name: string, shell: string) {
	// Use osascript to tell Terminal.app to run the docker exec command
	// We use the short container ID (first 12 chars) for display
	short_id := container_id[:12] if len(container_id) >= 12 else container_id

	// Build the docker exec command
	docker_cmd := strings.concatenate(
		{"docker exec -it ", short_id, " ", shell},
		context.temp_allocator,
	)

	// Build the osascript command
	// Tell Terminal.app to run the command in a new window
	cmd := strings.concatenate(
		{
			"osascript -e 'tell application \"Terminal\" to do script \"",
			docker_cmd,
			"\"' -e 'tell application \"Terminal\" to activate' &",
		},
		context.temp_allocator,
	)

	cmd_cstr := strings.clone_to_cstring(cmd, context.temp_allocator)
	libc_system(cmd_cstr)
}

// Linux: Try common terminal emulators
@(private)
launch_terminal_linux :: proc(container_id: string, container_name: string, shell: string) {
	short_id := container_id[:12] if len(container_id) >= 12 else container_id

	// Build the docker exec command
	docker_cmd := strings.concatenate(
		{"docker exec -it ", short_id, " ", shell},
		context.temp_allocator,
	)

	// Try common terminal emulators in order of preference
	// x-terminal-emulator is a Debian/Ubuntu alternative system
	terminal_commands := []string {
		strings.concatenate({"x-terminal-emulator -e ", docker_cmd, " &"}, context.temp_allocator),
		strings.concatenate({"gnome-terminal -- ", docker_cmd, " &"}, context.temp_allocator),
		strings.concatenate({"konsole -e ", docker_cmd, " &"}, context.temp_allocator),
		strings.concatenate({"xfce4-terminal -e \"", docker_cmd, "\" &"}, context.temp_allocator),
		strings.concatenate({"xterm -e ", docker_cmd, " &"}, context.temp_allocator),
	}

	// Try each terminal until one works
	for term_cmd in terminal_commands {
		// Check if the terminal exists using 'which'
		term_name := get_terminal_name(term_cmd)
		check_cmd := strings.concatenate(
			{"which ", term_name, " >/dev/null 2>&1"},
			context.temp_allocator,
		)
		check_cstr := strings.clone_to_cstring(check_cmd, context.temp_allocator)

		if libc_system(check_cstr) == 0 {
			// Terminal exists, launch it
			cmd_cstr := strings.clone_to_cstring(term_cmd, context.temp_allocator)
			libc_system(cmd_cstr)
			return
		}
	}

	// Fallback: just print an error (no terminal found)
	fmt.eprintf(
		"No terminal emulator found. Please install one of: gnome-terminal, konsole, xfce4-terminal, xterm\n",
	)
}

// Extract terminal name from command for 'which' check
@(private)
get_terminal_name :: proc(cmd: string) -> string {
	// Get the first word (terminal name)
	space_idx := strings.index(cmd, " ")
	if space_idx > 0 {
		return cmd[:space_idx]
	}
	return cmd
}
