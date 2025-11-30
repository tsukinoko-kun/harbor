package docker

import "core:strings"

build_http_request :: proc(method: string, path: string) -> string {
	return strings.concatenate(
		{
			method,
			" ",
			path,
			" HTTP/1.1\r\n",
			"Host: localhost\r\n",
			"Connection: close\r\n",
			"\r\n",
		},
	)
}

extract_json_from_response :: proc(response: string) -> (json_body: string, ok: bool) {
	json_start := find_body_start(response)
	if json_start == -1 {
		return "", false
	}

	body_start := response[json_start:]
	json_body = skip_chunked_encoding_header(body_start)
	json_body = find_json_start(json_body)
	json_body = find_json_end(json_body)

	return json_body, true
}

find_body_start :: proc(response: string) -> int {
	json_start := strings.index(response, "\r\n\r\n")
	if json_start != -1 {
		return json_start + 4
	}

	json_start = strings.index(response, "\n\n")
	if json_start != -1 {
		return json_start + 2
	}

	return -1
}

skip_chunked_encoding_header :: proc(body: string) -> string {
	chunk_end := strings.index(body, "\r\n")
	if chunk_end == -1 {
		return body
	}

	potential_chunk_size := body[:chunk_end]
	if is_hex_number(potential_chunk_size) && len(potential_chunk_size) > 0 {
		return body[chunk_end + 2:]
	}

	return body
}

is_hex_number :: proc(s: string) -> bool {
	for c in s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

find_json_start :: proc(body: string) -> string {
	for i in 0 ..< len(body) {
		if body[i] == '[' || body[i] == '{' {
			return body[i:]
		}
	}
	return body
}

find_json_end :: proc(json_body: string) -> string {
	bracket_count := 0
	in_string := false
	escape_next := false

	for i in 0 ..< len(json_body) {
		if escape_next {
			escape_next = false
			continue
		}

		c := json_body[i]
		if c == '\\' {
			escape_next = true
			continue
		}

		if c == '"' {
			in_string = !in_string
			continue
		}

		if in_string {
			continue
		}

		if c == '[' || c == '{' {
			bracket_count += 1
		} else if c == ']' || c == '}' {
			bracket_count -= 1
			if bracket_count == 0 {
				return json_body[:i + 1]
			}
		}
	}

	return json_body
}
