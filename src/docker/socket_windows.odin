#+build windows
package docker

import win "core:sys/windows"

DOCKER_SOCKET_PATH :: "\\\\.\\pipe\\docker_engine"

connect_named_pipe :: proc(pipe_path: string) -> (sock: Socket, ok: bool) {
	// Convert string to UTF-16 for Windows API
	wide_path := win.utf8_to_wstring(pipe_path)
	
	handle := win.CreateFileW(
		wide_path,
		win.GENERIC_READ | win.GENERIC_WRITE,
		0,
		nil,
		win.OPEN_EXISTING,
		0,
		nil,
	)
	
	if handle == win.INVALID_HANDLE_VALUE {
		return Socket{}, false
	}
	
	return Socket{handle = cast(rawptr)handle}, true
}

close_socket :: proc(sock: Socket) {
	win.CloseHandle(win.HANDLE(uintptr(sock.handle)))
}

send_data :: proc(sock: Socket, data: []u8) -> (int, bool) {
	bytes_written: win.DWORD
	success := win.WriteFile(
		win.HANDLE(uintptr(sock.handle)),
		raw_data(data),
		cast(win.DWORD)len(data),
		&bytes_written,
		nil,
	)
	if success == false || bytes_written != cast(win.DWORD)len(data) {
		// Connection may be broken, reset it
		reset_connection()
		return int(bytes_written), false
	}
	return int(bytes_written), true
}

receive_data :: proc(sock: Socket, buffer: []u8) -> (int, bool) {
	bytes_read: win.DWORD
	success := win.ReadFile(
		win.HANDLE(uintptr(sock.handle)),
		raw_data(buffer),
		cast(win.DWORD)len(buffer),
		&bytes_read,
		nil,
	)
	if success == false || bytes_read == 0 {
		// Connection may be broken, reset it
		reset_connection()
		return 0, false
	}
	return int(bytes_read), true
}

connect_docker_socket :: proc(socket_path: string) -> (sock: Socket, ok: bool) {
	return connect_named_pipe(socket_path)
}

