#+build linux, darwin
package docker

import "core:c"

// Create a new dedicated socket connection to Docker
// This is separate from the global connection used for regular API calls
create_docker_connection :: proc() -> (sock: Socket, ok: bool) {
	return connect_docker_socket(DOCKER_SOCKET_PATH)
}

// Send data on a specific socket without affecting global connection state
send_data_on_socket :: proc(sock: Socket, data: []u8) -> (int, bool) {
	sent := send(cast(c.int)uintptr(sock.handle), raw_data(data), cast(c.size_t)len(data), 0)
	if sent < 0 {
		return 0, false
	}
	return int(sent), true
}

// Receive data from a specific socket without affecting global connection state
// Returns bytes read, and whether the read was successful (0 bytes with ok=true means EOF)
receive_data_from_socket :: proc(sock: Socket, buffer: []u8) -> (int, bool) {
	read := recv(cast(c.int)uintptr(sock.handle), raw_data(buffer), cast(c.size_t)len(buffer), 0)
	if read < 0 {
		return 0, false
	}
	return int(read), true
}

// Close a specific socket connection
close_socket_connection :: proc(sock: Socket) {
	close_socket(sock)
}

// Remove socket timeout (for streaming connections that need to wait indefinitely)
remove_socket_timeout :: proc(sock: Socket) -> bool {
	// Set timeout to 0 which means no timeout
	timeout: TIMEVAL
	timeout.tv_sec = 0
	timeout.tv_usec = 0
	timeout_len := cast(c.uint)size_of(TIMEVAL)

	result1 := setsockopt(
		cast(c.int)uintptr(sock.handle),
		SOL_SOCKET,
		SO_RCVTIMEO,
		&timeout,
		timeout_len,
	)
	result2 := setsockopt(
		cast(c.int)uintptr(sock.handle),
		SOL_SOCKET,
		SO_SNDTIMEO,
		&timeout,
		timeout_len,
	)

	return result1 >= 0 && result2 >= 0
}
