package docker

import "core:c"
import "core:os"

when ODIN_OS == .Darwin || ODIN_OS == .Linux {
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
}

when ODIN_OS == .Windows {
	foreign import "system:kernel32.lib"
	
	@(default_calling_convention="std")
	foreign kernel32 {
		CreateFileW :: proc(
			lpFileName: ^u16,
			dwDesiredAccess: u32,
			dwShareMode: u32,
			lpSecurityAttributes: rawptr,
			dwCreationDisposition: u32,
			dwFlagsAndAttributes: u32,
			hTemplateFile: rawptr,
		) -> os.Handle ---
		WriteFile :: proc(
			hFile: os.Handle,
			lpBuffer: rawptr,
			nNumberOfBytesToWrite: u32,
			lpNumberOfBytesWritten: ^u32,
			lpOverlapped: rawptr,
		) -> bool ---
		ReadFile :: proc(
			hFile: os.Handle,
			lpBuffer: rawptr,
			nNumberOfBytesToRead: u32,
			lpNumberOfBytesRead: ^u32,
			lpOverlapped: rawptr,
		) -> bool ---
		CloseHandle :: proc(hObject: os.Handle) -> bool ---
	}
	
	GENERIC_READ  :: 0x80000000
	GENERIC_WRITE :: 0x40000000
	OPEN_EXISTING :: 3
}

Socket :: struct {
	handle: rawptr, // c.int on Unix, os.Handle on Windows
}

// Global connection state
global_connection: Socket
connection_established: bool = false

when ODIN_OS == .Darwin || ODIN_OS == .Linux {
	DOCKER_SOCKET_PATH :: "/var/run/docker.sock"
} else when ODIN_OS == .Windows {
	DOCKER_SOCKET_PATH :: "\\\\.\\pipe\\docker_engine"
}

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

connect_docker_socket :: proc(socket_path: string) -> (sock: Socket, ok: bool) {
	when ODIN_OS == .Darwin || ODIN_OS == .Linux {
		return connect_unix_socket(socket_path)
	} else when ODIN_OS == .Windows {
		return connect_named_pipe(socket_path)
	} else {
		return Socket{}, false
	}
}

connect_unix_socket :: proc(socket_path: string) -> (sock: Socket, ok: bool) {
	when ODIN_OS == .Darwin || ODIN_OS == .Linux {
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
	} else {
		return Socket{}, false
	}
}

connect_named_pipe :: proc(pipe_path: string) -> (sock: Socket, ok: bool) {
	when ODIN_OS == .Windows {
		// Convert string to UTF-16 for Windows API
		wide_path := os.utf8_to_wstring(pipe_path)
		defer delete(wide_path)
		
		handle := CreateFileW(
			raw_data(wide_path),
			GENERIC_READ | GENERIC_WRITE,
			0,
			nil,
			OPEN_EXISTING,
			0,
			nil,
		)
		
		if handle == os.INVALID_HANDLE {
			return Socket{}, false
		}
		
		return Socket{handle = cast(rawptr)handle}, true
	} else {
		return Socket{}, false
	}
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
	when ODIN_OS == .Darwin || ODIN_OS == .Linux {
		close(cast(c.int)uintptr(sock.handle))
	} else when ODIN_OS == .Windows {
		CloseHandle(cast(os.Handle)sock.handle)
	}
}

send_data :: proc(sock: Socket, data: []u8) -> (int, bool) {
	when ODIN_OS == .Darwin || ODIN_OS == .Linux {
		sent := send(cast(c.int)uintptr(sock.handle), raw_data(data), cast(c.size_t)len(data), 0)
		if sent != cast(c.ssize_t)len(data) {
			// Connection may be broken, reset it
			reset_connection()
			return int(sent), false
		}
		return int(sent), true
	} else when ODIN_OS == .Windows {
		bytes_written: u32
		success := WriteFile(
			cast(os.Handle)sock.handle,
			raw_data(data),
			cast(u32)len(data),
			&bytes_written,
			nil,
		)
		if !success || bytes_written != cast(u32)len(data) {
			// Connection may be broken, reset it
			reset_connection()
			return int(bytes_written), false
		}
		return int(bytes_written), true
	} else {
		return 0, false
	}
}

receive_data :: proc(sock: Socket, buffer: []u8) -> (int, bool) {
	when ODIN_OS == .Darwin || ODIN_OS == .Linux {
		read := recv(cast(c.int)uintptr(sock.handle), raw_data(buffer), cast(c.size_t)len(buffer), 0)
		if read <= 0 {
			// Timeout or connection broken - reset it
			// Note: recv returns 0 on connection close, negative on error/timeout
			reset_connection()
			return 0, false
		}
		return int(read), true
	} else when ODIN_OS == .Windows {
		bytes_read: u32
		success := ReadFile(
			cast(os.Handle)sock.handle,
			raw_data(buffer),
			cast(u32)len(buffer),
			&bytes_read,
			nil,
		)
		if !success || bytes_read == 0 {
			// Connection may be broken, reset it
			reset_connection()
			return 0, false
		}
		return int(bytes_read), true
	} else {
		return 0, false
	}
}

