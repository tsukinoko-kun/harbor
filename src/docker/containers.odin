package docker

import "core:fmt"
import "core:strings"
import "core:encoding/json"
import "core:mem"

DOCKER_API_VERSION :: "v1.51"

get_all_containers :: proc() -> (containers: []ContainerSummary, ok: bool) {
	response, response_ok := send_docker_request("/containers/json?all=true")
	if !response_ok {
		return nil, false
	}
	
	json_body, json_ok := extract_json_from_response(response)
	if !json_ok {
		return nil, false
	}
	
	return parse_containers_json(json_body)
}

send_docker_request :: proc(path: string) -> (response: string, ok: bool) {
	full_path := strings.concatenate({"/", DOCKER_API_VERSION, path})
	request := build_http_request("GET", full_path)
	return send_http_request(request)
}

send_http_request :: proc(request: string) -> (response: string, ok: bool) {
	// Try with existing connection, retry once if connection is broken
	for attempt in 0..<2 {
		sock, connect_ok := ensure_connection()
		if !connect_ok {
			if attempt == 0 {
				// First attempt failed, reset and try again
				reset_connection()
				continue
			}
			fmt.eprintf("Failed to establish connection to Docker socket\n")
			return "", false
		}
		
		// Don't close the socket - we keep it open for reuse
		request_bytes := transmute([]u8)request
		_, send_ok := send_data(sock, request_bytes)
		if !send_ok {
			if attempt == 0 {
				// Connection broken, reset and retry
				reset_connection()
				continue
			}
			fmt.eprintf("Failed to send request\n")
			return "", false
		}
		
		response_builder := strings.builder_make()
		defer strings.builder_destroy(&response_builder)
		
		buffer: [8192]u8
		total_read := 0
		
		for {
			bytes_read, recv_ok := receive_data(sock, buffer[:])
			if !recv_ok || bytes_read == 0 {
				break
			}
			total_read += bytes_read
			strings.write_bytes(&response_builder, buffer[:bytes_read])
		}
		
		if total_read == 0 {
			if attempt == 0 {
				// Connection broken, reset and retry
				reset_connection()
				continue
			}
			fmt.eprintf("No data received from Docker socket\n")
			return "", false
		}
		
		return strings.to_string(response_builder), true
	}
	
	return "", false
}

parse_containers_json :: proc(json_body: string) -> (containers: []ContainerSummary, ok: bool) {
	json_bytes := make([]u8, len(json_body))
	copy(json_bytes, json_body)
	
	unmarshaled_containers: []ContainerSummary
	unmarshal_err := json.unmarshal(json_bytes, &unmarshaled_containers)
	if unmarshal_err == nil {
		return unmarshaled_containers, true
	}
	
	return parse_containers_json_manual(json_bytes)
}

parse_containers_json_manual :: proc(json_bytes: []u8) -> (containers: []ContainerSummary, ok: bool) {
	json_data, parse_err := json.parse(json_bytes, json.Specification.JSON, true)
	if parse_err != nil {
		fmt.eprintf("Failed to parse JSON: %v\n", parse_err)
		return nil, false
	}
	defer json.destroy_value(json_data)
	
	containers_array, is_array := json_data.(json.Array)
	if !is_array {
		fmt.eprintf("Expected array, got: %v\n", json_data)
		return nil, false
	}
	
	result := make([]ContainerSummary, len(containers_array))
	
	for container, i in containers_array {
		container_obj, is_object := container.(json.Object)
		if !is_object {
			continue
		}
		
		extract_container_fields(&result[i], container_obj)
	}
	
	return result, true
}

extract_container_fields :: proc(container: ^ContainerSummary, obj: json.Object) {
	if id_val, ok := obj["Id"]; ok {
		if id_str, is_str := id_val.(string); is_str {
			container.Id = id_str
		}
	}
	
	if names_val, ok := obj["Names"]; ok {
		if names_arr, is_arr := names_val.(json.Array); is_arr {
			container.Names = make([]string, len(names_arr))
			for name, j in names_arr {
				if name_str, is_str := name.(string); is_str {
					container.Names[j] = name_str
				}
			}
		}
	}
	
	if image_val, ok := obj["Image"]; ok {
		if image_str, is_str := image_val.(string); is_str {
			container.Image = image_str
		}
	}
	
	if state_val, ok := obj["State"]; ok {
		if state_str, is_str := state_val.(string); is_str {
			container.State = state_str
		}
	}
	
	if status_val, ok := obj["Status"]; ok {
		if status_str, is_str := status_val.(string); is_str {
			container.Status = status_str
		}
	}
	
	if labels_val, ok := obj["Labels"]; ok {
		if labels_obj, is_obj := labels_val.(json.Object); is_obj {
			container.Labels = make(map[string]string)
			for key, val in labels_obj {
				if val_str, is_str := val.(string); is_str {
					container.Labels[key] = val_str
				}
			}
		}
	}
}

