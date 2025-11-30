package main

import "core:fmt"
import "core:time"

import "docker"
import "gui"

import rl "vendor:raylib"

REFRESH_INTERVAL :: 3 * time.Second

main :: proc() {
	// Initialize GUI
	gui.init()
	defer gui.shutdown()

	// Fetch containers at startup
	containers, _ := docker.get_all_containers()
	last_refresh := time.now()

	// Main loop
	for !rl.WindowShouldClose() {
		// Refresh containers periodically when window is focused
		if rl.IsWindowFocused() {
			elapsed := time.diff(last_refresh, time.now())
			if elapsed >= REFRESH_INTERVAL {
				new_containers, ok := docker.get_all_containers()
				if ok {
					containers = new_containers
				}
				last_refresh = time.now()
			}
		}

		gui.render(containers)
	}
}
