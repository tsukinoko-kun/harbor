#+build linux, darwin
package docker

import "core:c"

foreign import libc "system:c"

@(default_calling_convention="c")
foreign libc {
	socket :: proc(domain: c.int, type: c.int, protocol: c.int) -> c.int ---
	connect :: proc(sockfd: c.int, addr: rawptr, addrlen: c.uint) -> c.int ---
	send :: proc(sockfd: c.int, buf: rawptr, len: c.size_t, flags: c.int) -> c.ssize_t ---
	recv :: proc(sockfd: c.int, buf: rawptr, len: c.size_t, flags: c.int) -> c.ssize_t ---
	close :: proc(fd: c.int) -> c.int ---
	setsockopt :: proc(sockfd: c.int, level: c.int, optname: c.int, optval: rawptr, optlen: c.uint) -> c.int ---
}

AF_UNIX :: 1
SOCK_STREAM :: 1
SOL_SOCKET :: 1

when ODIN_OS == .Darwin {
	// macOS uses different socket option constants
	SO_RCVTIMEO :: 4102
	SO_SNDTIMEO :: 4101
} else {
	// Linux uses these values
	SO_RCVTIMEO :: 20
	SO_SNDTIMEO :: 21
}

SOCKADDR_UN :: struct {
	sun_family: c.ushort,
	sun_path: [108]u8,
}

TIMEVAL :: struct {
	tv_sec: c.long,
	tv_usec: c.long,
}

DOCKER_SOCKET_PATH :: "/var/run/docker.sock"

connect_docker_socket :: proc(socket_path: string) -> (sock: Socket, ok: bool) {
	return connect_unix_socket(socket_path)
}

connect_unix_socket :: proc(socket_path: string) -> (sock: Socket, ok: bool) {
	sockfd := socket(AF_UNIX, SOCK_STREAM, 0)
	if sockfd < 0 {
		return Socket{}, false
	}
	
	// Set socket timeouts to prevent indefinite blocking
	// 5 second timeout for receive and send operations
	// If this fails, we continue anyway - timeout is a safety feature
	timeout: TIMEVAL
	timeout.tv_sec = 5
	timeout.tv_usec = 0
	timeout_len := cast(c.uint)size_of(TIMEVAL)
	
	// Try to set receive timeout (non-fatal if it fails)
	setsockopt(sockfd, SOL_SOCKET, SO_RCVTIMEO, &timeout, timeout_len)
	
	// Try to set send timeout (non-fatal if it fails)
	setsockopt(sockfd, SOL_SOCKET, SO_SNDTIMEO, &timeout, timeout_len)
	
	addr := create_socket_address(socket_path)
	addr_len := calculate_address_length(socket_path)
	
	if connect(sockfd, &addr, cast(c.uint)addr_len) < 0 {
		close(sockfd)
		return Socket{}, false
	}
	
	return Socket{handle = cast(rawptr)uintptr(sockfd)}, true
}

create_socket_address :: proc(socket_path: string) -> SOCKADDR_UN {
	addr: SOCKADDR_UN
	addr.sun_family = AF_UNIX
	copy(addr.sun_path[:], socket_path)
	
	path_len := len(socket_path)
	if path_len < 107 {
		addr.sun_path[path_len] = 0
	}
	
	return addr
}

calculate_address_length :: proc(socket_path: string) -> int {
	path_len := len(socket_path)
	addr_len := 2 + path_len + 1 // sun_family + path + null terminator
	if path_len >= 108 {
		addr_len = size_of(SOCKADDR_UN)
	}
	return addr_len
}

close_socket :: proc(sock: Socket) {
	close(cast(c.int)uintptr(sock.handle))
}

send_data :: proc(sock: Socket, data: []u8) -> (int, bool) {
	sent := send(cast(c.int)uintptr(sock.handle), raw_data(data), cast(c.size_t)len(data), 0)
	if sent != cast(c.ssize_t)len(data) {
		// Connection may be broken, reset it
		reset_connection()
		return int(sent), false
	}
	return int(sent), true
}

receive_data :: proc(sock: Socket, buffer: []u8) -> (int, bool) {
	read := recv(cast(c.int)uintptr(sock.handle), raw_data(buffer), cast(c.size_t)len(buffer), 0)
	if read <= 0 {
		// Timeout or connection broken - reset it
		// Note: recv returns 0 on connection close, negative on error/timeout
		reset_connection()
		return 0, false
	}
	return int(read), true
}
