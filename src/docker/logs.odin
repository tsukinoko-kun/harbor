package docker

import "core:fmt"
import "core:strings"
import "core:sync"
import "core:thread"

// Maximum size of the log buffer in bytes (64KB)
LOG_BUFFER_MAX_SIZE :: 64 * 1024

// Number of tail lines to fetch initially
LOG_TAIL_LINES :: 100

// Log stream state
LogStreamState :: struct {
	container_id:   string,
	container_name: string,
	socket:         Socket,
	thread:         ^thread.Thread,
	running:        bool,
	buffer:         LogBuffer,
	mutex:          sync.Mutex,
}

// Circular buffer for log content
LogBuffer :: struct {
	data:      [LOG_BUFFER_MAX_SIZE]u8,
	write_pos: int,
	length:    int, // Actual content length (up to LOG_BUFFER_MAX_SIZE)
}

// Global log stream state
log_stream: LogStreamState

// Flag to track if log content has changed since last check
@(private)
log_content_changed: bool = false

// Check if log stream is active
is_log_stream_active :: proc() -> bool {
	return log_stream.running
}

// Check if log content has changed since last check (clears flag after checking)
has_log_content_changed :: proc() -> bool {
	if log_content_changed {
		log_content_changed = false
		return true
	}
	return false
}

// Get the current container name being logged
get_log_container_name :: proc() -> string {
	return log_stream.container_name
}

// Start streaming logs for a container
start_log_stream :: proc(container_id: string, container_name: string) -> bool {
	// Stop any existing stream
	if log_stream.running {
		stop_log_stream()
	}

	// Create a new dedicated socket for streaming
	sock, sock_ok := create_docker_connection()
	if !sock_ok {
		fmt.eprintf("Failed to create socket for log streaming\n")
		return false
	}

	// Remove timeout for streaming (we want to wait indefinitely for new logs)
	remove_socket_timeout(sock)

	// Initialize state
	log_stream.container_id = strings.clone(container_id)
	log_stream.container_name = strings.clone(container_name)
	log_stream.socket = sock
	log_stream.running = true
	log_stream.buffer = LogBuffer{}

	// Start the streaming thread
	log_stream.thread = thread.create(log_stream_thread_proc)
	if log_stream.thread == nil {
		fmt.eprintf("Failed to create log streaming thread\n")
		close_socket_connection(sock)
		log_stream.running = false
		return false
	}

	thread.start(log_stream.thread)
	return true
}

// Stop the log stream
stop_log_stream :: proc() {
	if !log_stream.running {
		return
	}

	// Signal thread to stop
	log_stream.running = false

	// Close socket to unblock any pending reads
	close_socket_connection(log_stream.socket)

	// Wait for thread to finish
	if log_stream.thread != nil {
		thread.join(log_stream.thread)
		thread.destroy(log_stream.thread)
		log_stream.thread = nil
	}

	// Clean up strings
	delete(log_stream.container_id)
	delete(log_stream.container_name)
	log_stream.container_id = ""
	log_stream.container_name = ""
}

// Get the current log content (thread-safe)
get_log_content :: proc(allocator := context.allocator) -> string {
	sync.mutex_lock(&log_stream.mutex)
	defer sync.mutex_unlock(&log_stream.mutex)

	if log_stream.buffer.length == 0 {
		return ""
	}

	// If buffer hasn't wrapped, simple copy
	if log_stream.buffer.length < LOG_BUFFER_MAX_SIZE {
		result := make([]u8, log_stream.buffer.length, allocator)
		copy(result, log_stream.buffer.data[:log_stream.buffer.length])
		return string(result)
	}

	// Buffer has wrapped - need to reconstruct in order
	result := make([]u8, LOG_BUFFER_MAX_SIZE, allocator)

	// First part: from write_pos to end
	first_part_len := LOG_BUFFER_MAX_SIZE - log_stream.buffer.write_pos
	copy(result[:first_part_len], log_stream.buffer.data[log_stream.buffer.write_pos:])

	// Second part: from start to write_pos
	copy(result[first_part_len:], log_stream.buffer.data[:log_stream.buffer.write_pos])

	return string(result)
}

// Thread procedure for log streaming
@(private)
log_stream_thread_proc :: proc(t: ^thread.Thread) {
	// Build the request
	path := strings.concatenate(
		{
			"/",
			DOCKER_API_VERSION,
			"/containers/",
			log_stream.container_id,
			"/logs?stdout=1&stderr=1&follow=1&tail=",
			int_to_string_simple(LOG_TAIL_LINES),
		},
	)
	defer delete(path)

	request := build_http_request_streaming("GET", path)
	defer delete(request)

	// Send the request
	request_bytes := transmute([]u8)request
	_, send_ok := send_data_on_socket(log_stream.socket, request_bytes)
	if !send_ok {
		fmt.eprintf("Failed to send log request\n")
		log_stream.running = false
		return
	}

	// Read and process response
	buffer: [4096]u8
	header_skipped := false
	is_chunked := false
	partial_data: [dynamic]u8
	defer delete(partial_data)

	for log_stream.running {
		bytes_read, read_ok := receive_data_from_socket(log_stream.socket, buffer[:])
		if !read_ok || bytes_read == 0 {
			// Connection closed or error
			break
		}

		// Accumulate data
		append(&partial_data, ..buffer[:bytes_read])

		// Skip HTTP headers on first successful read
		if !header_skipped {
			header_end := find_header_end(partial_data[:])
			if header_end == -1 {
				// Headers not complete yet, continue reading
				continue
			}

			// Check for chunked transfer encoding
			header_str := string(partial_data[:header_end])
			is_chunked =
				strings.contains(header_str, "Transfer-Encoding: chunked") ||
				strings.contains(header_str, "transfer-encoding: chunked")

			// Remove headers from buffer
			remaining := make([dynamic]u8)
			append(&remaining, ..partial_data[header_end:])
			delete(partial_data)
			partial_data = remaining

			header_skipped = true
		}

		// Process the body data
		if is_chunked {
			process_chunked_log_data(&partial_data)
		} else {
			// Raw multiplexed stream (no chunked encoding)
			process_log_frames(partial_data[:], &partial_data)
			clear(&partial_data)
		}
	}

	log_stream.running = false
}

// Find end of HTTP headers
@(private)
find_header_end :: proc(data: []u8) -> int {
	if len(data) < 4 {
		return -1
	}
	for i in 0 ..= len(data) - 4 {
		if data[i] == '\r' && data[i + 1] == '\n' && data[i + 2] == '\r' && data[i + 3] == '\n' {
			return i + 4
		}
	}
	return -1
}

// Process chunked transfer encoding and extract log frames
@(private)
process_chunked_log_data :: proc(data: ^[dynamic]u8) {
	frame_buffer: [dynamic]u8
	defer delete(frame_buffer)

	offset := 0
	for offset < len(data^) {
		// Find chunk size line (hex number followed by \r\n)
		chunk_line_end := -1
		for i in offset ..< len(data^) - 1 {
			if data[i] == '\r' && data[i + 1] == '\n' {
				chunk_line_end = i
				break
			}
		}

		if chunk_line_end == -1 {
			// Incomplete chunk header, keep remaining data
			break
		}

		// Parse chunk size (hex)
		chunk_size_str := string(data[offset:chunk_line_end])
		chunk_size := parse_hex(chunk_size_str)

		if chunk_size == 0 {
			// End of chunked data
			offset = chunk_line_end + 2
			break
		}

		chunk_start := chunk_line_end + 2
		chunk_end := chunk_start + chunk_size

		if chunk_end + 2 > len(data^) {
			// Incomplete chunk, keep remaining data
			break
		}

		// Extract chunk payload and process as Docker frames
		chunk_payload := data[chunk_start:chunk_end]
		append(&frame_buffer, ..chunk_payload)

		// Skip chunk data and trailing \r\n
		offset = chunk_end + 2
	}

	// Process accumulated frames
	if len(frame_buffer) > 0 {
		process_docker_frames(frame_buffer[:])
	}

	// Keep unprocessed data
	if offset > 0 && offset < len(data^) {
		remaining := make([dynamic]u8)
		append(&remaining, ..data[offset:])
		delete(data^)
		data^ = remaining
	} else if offset >= len(data^) {
		clear(data)
	}
}

// Parse hex string to int
@(private)
parse_hex :: proc(s: string) -> int {
	result := 0
	for c in s {
		result *= 16
		if c >= '0' && c <= '9' {
			result += int(c - '0')
		} else if c >= 'a' && c <= 'f' {
			result += int(c - 'a' + 10)
		} else if c >= 'A' && c <= 'F' {
			result += int(c - 'A' + 10)
		}
	}
	return result
}

// Process Docker multiplexed frames directly (no chunked encoding wrapper)
@(private)
process_docker_frames :: proc(data: []u8) {
	offset := 0
	for offset + 8 <= len(data) {
		// Read frame header
		// Byte 0: stream type (0=stdin, 1=stdout, 2=stderr)
		// Bytes 1-3: padding (zeros)
		// Bytes 4-7: size (big endian)

		frame_size :=
			(int(data[offset + 4]) << 24) |
			(int(data[offset + 5]) << 16) |
			(int(data[offset + 6]) << 8) |
			int(data[offset + 7])

		if offset + 8 + frame_size > len(data) {
			// Incomplete frame
			break
		}

		// Extract payload and append to buffer
		payload := data[offset + 8:offset + 8 + frame_size]
		append_to_log_buffer(payload)

		offset += 8 + frame_size
	}
}

// Process Docker log frames (multiplexed stream format) for raw (non-chunked) responses
@(private)
process_log_frames :: proc(input_data: []u8, buffer: ^[dynamic]u8) {
	// For raw streams, just process the frames directly
	process_docker_frames(input_data)
}

// Append data to the circular log buffer (thread-safe)
@(private)
append_to_log_buffer :: proc(data: []u8) {
	if len(data) == 0 {
		return
	}

	sync.mutex_lock(&log_stream.mutex)
	defer sync.mutex_unlock(&log_stream.mutex)

	buf := &log_stream.buffer

	for i in 0 ..< len(data) {
		buf.data[buf.write_pos] = data[i]
		buf.write_pos = (buf.write_pos + 1) % LOG_BUFFER_MAX_SIZE
		if buf.length < LOG_BUFFER_MAX_SIZE {
			buf.length += 1
		}
	}

	// Mark that log content has changed for UI refresh
	log_content_changed = true
}

// Build HTTP request for streaming (no Connection: close)
@(private)
build_http_request_streaming :: proc(method: string, path: string) -> string {
	return strings.concatenate({method, " ", path, " HTTP/1.1\r\n", "Host: localhost\r\n", "\r\n"})
}

// Simple int to string conversion
@(private)
int_to_string_simple :: proc(n: int) -> string {
	if n == 0 {
		return "0"
	}

	buf: [20]u8
	i := len(buf)
	num := n
	if num < 0 {
		num = -num
	}

	for num > 0 {
		i -= 1
		buf[i] = u8('0' + num % 10)
		num /= 10
	}

	if n < 0 {
		i -= 1
		buf[i] = '-'
	}

	return strings.clone(string(buf[i:]))
}
