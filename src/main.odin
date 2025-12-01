package main

import "core:time"

import "docker"
import "gui"

import rl "vendor:raylib"

REFRESH_INTERVAL :: 3 * time.Second
IDLE_SLEEP_TIME :: 0.030 // 30ms sleep when idle (~33 FPS max polling rate)

main :: proc() {
	// Initialize GUI
	gui.init()
	defer gui.shutdown()

	// Fetch containers at startup
	containers, _ := docker.get_all_containers()
	last_refresh := time.now()

	// Main loop
	for !rl.WindowShouldClose() {
		// Poll for input events - Raylib needs this to update input state
		rl.PollInputEvents()

		// Check for input events that require redraw
		gui.check_events()

		// Refresh containers periodically when window is focused
		if rl.IsWindowFocused() {
			elapsed := time.diff(last_refresh, time.now())
			if elapsed >= REFRESH_INTERVAL {
				new_containers, ok := docker.get_all_containers()
				if ok {
					containers = new_containers
				}
				last_refresh = time.now()
				gui.mark_dirty() // Container data changed
			}
		}

		// Check if log content has changed
		if docker.has_log_content_changed() {
			gui.mark_dirty()
		}

		// Only render if something changed
		if gui.is_dirty() {
			gui.render(containers)
			gui.clear_dirty()
		} else {
			// Sleep to reduce CPU usage when idle
			rl.WaitTime(IDLE_SLEEP_TIME)
		}
	}
}
