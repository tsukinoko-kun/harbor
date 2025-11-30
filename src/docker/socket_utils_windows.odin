#+build windows
package docker

import win "core:sys/windows"

// Create a new dedicated socket connection to Docker
// This is separate from the global connection used for regular API calls
create_docker_connection :: proc() -> (sock: Socket, ok: bool) {
	return connect_docker_socket(DOCKER_SOCKET_PATH)
}

// Send data on a specific socket without affecting global connection state
send_data_on_socket :: proc(sock: Socket, data: []u8) -> (int, bool) {
	bytes_written: win.DWORD
	success := win.WriteFile(
		win.HANDLE(uintptr(sock.handle)),
		raw_data(data),
		cast(win.DWORD)len(data),
		&bytes_written,
		nil,
	)
	if success == false {
		return int(bytes_written), false
	}
	return int(bytes_written), true
}

// Receive data from a specific socket without affecting global connection state
// Returns bytes read, and whether the read was successful (0 bytes with ok=true means EOF)
receive_data_from_socket :: proc(sock: Socket, buffer: []u8) -> (int, bool) {
	bytes_read: win.DWORD
	success := win.ReadFile(
		win.HANDLE(uintptr(sock.handle)),
		raw_data(buffer),
		cast(win.DWORD)len(buffer),
		&bytes_read,
		nil,
	)
	if success == false {
		return 0, false
	}
	return int(bytes_read), true
}

// Close a specific socket connection
close_socket_connection :: proc(sock: Socket) {
	close_socket(sock)
}

// Remove socket timeout (for streaming connections that need to wait indefinitely)
// Windows named pipes don't have the same timeout mechanism as Unix sockets
remove_socket_timeout :: proc(sock: Socket) -> bool {
	return true
}
