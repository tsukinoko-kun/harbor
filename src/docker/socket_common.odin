package docker

Socket :: struct {
	handle: rawptr, // c.int on Unix, HANDLE on Windows
}

// Global connection state
global_connection: Socket
connection_established: bool = false

get_connection :: proc() -> (Socket, bool) {
	// Check if connection exists and is valid
	if connection_established {
		// Test if connection is still alive by attempting a small operation
		// For now, we'll just return it and let the caller handle errors
		return global_connection, true
	}
	
	// Establish new connection
	socket_path := DOCKER_SOCKET_PATH
	new_sock, connect_ok := connect_docker_socket(socket_path)
	if !connect_ok {
		return Socket{}, false
	}
	
	global_connection = new_sock
	connection_established = true
	return new_sock, true
}

ensure_connection :: proc() -> (Socket, bool) {
	conn, conn_ok := get_connection()
	if !conn_ok {
		return Socket{}, false
	}
	
	// If we get here and connection_established is true, return it
	// Errors will be handled by the caller when they try to use it
	return conn, true
}

reset_connection :: proc() {
	if connection_established {
		close_socket(global_connection)
		connection_established = false
		global_connection = Socket{}
	}
}

