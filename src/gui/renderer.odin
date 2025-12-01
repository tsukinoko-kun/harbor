package gui

import "base:runtime"
import "core:math"
import "core:strings"
import "core:unicode/utf8"

import clay "../../vendor/clay-odin"
import rl "vendor:raylib"

import "../docker"

// Font constants
FONT_ID_BODY :: 0
FONT_ID_MONO :: 1
FONT_SIZE_BODY :: 16
FONT_SIZE_HEADER :: 24
FONT_SIZE_LOG :: 13

// Colors
COLOR_BACKGROUND :: clay.Color{30, 30, 30, 255}
COLOR_HEADER :: clay.Color{45, 45, 45, 255}
COLOR_CARD :: clay.Color{50, 50, 50, 255}
COLOR_CARD_HOVER :: clay.Color{60, 60, 60, 255}
COLOR_TEXT :: clay.Color{240, 240, 240, 255}
COLOR_TEXT_SECONDARY :: clay.Color{180, 180, 180, 255}
COLOR_STATUS_RUNNING :: clay.Color{76, 175, 80, 255}
COLOR_STATUS_STOPPED :: clay.Color{244, 67, 54, 255}
COLOR_STATUS_OTHER :: clay.Color{255, 193, 7, 255}
COLOR_BUTTON :: clay.Color{70, 130, 180, 255}
COLOR_BUTTON_HOVER :: clay.Color{90, 150, 200, 255}
COLOR_OVERLAY :: clay.Color{0, 0, 0, 180}
COLOR_LOG_BACKGROUND :: clay.Color{25, 25, 25, 255}
COLOR_SELECTION :: clay.Color{70, 130, 180, 128} // Semi-transparent blue for selection

// Embedded font data
@(private)
FONT_DATA :: #load("../../assets/fonts/Roboto.ttf", []u8)

@(private)
FONT_DATA_MONO :: #load("../../assets/fonts/JetBrainsMono.ttf", []u8)

// Embedded icon data (PNG format for Raylib compatibility)
@(private)
ICON_CHEVRON_DOWN :: #load("../../assets/icons/chevron-down.png", []u8)

@(private)
ICON_CHEVRON_RIGHT :: #load("../../assets/icons/chevron-right.png", []u8)

@(private)
ICON_PARAGRAPH :: #load("../../assets/icons/paragraph.png", []u8)

@(private)
ICON_X :: #load("../../assets/icons/x.png", []u8)

@(private)
ICON_TERMINAL :: #load("../../assets/icons/terminal.png", []u8)

@(private)
ICON_PLAY :: #load("../../assets/icons/play.png", []u8)

@(private)
ICON_STOP :: #load("../../assets/icons/stop.png", []u8)

// Icon textures struct
Icons :: struct {
	chevron_down:  rl.Texture2D,
	chevron_right: rl.Texture2D,
	paragraph:     rl.Texture2D,
	x:             rl.Texture2D,
	terminal:      rl.Texture2D,
	play:          rl.Texture2D,
	stop:          rl.Texture2D,
}

@(private)
icons: Icons

Raylib_Font :: struct {
	fontId: u16,
	font:   rl.Font,
}

@(private)
raylib_fonts := [dynamic]Raylib_Font{}

@(private)
clay_arena: clay.Arena

@(private)
clay_memory: []u8

// Project collapse state (true = collapsed, false/absent = expanded)
@(private)
collapsed_projects: map[string]bool

// Log text selection state
@(private)
log_selection_start: int = -1 // -1 means no selection

@(private)
log_selection_end: int = -1

@(private)
log_is_selecting: bool = false

// Cached log text area bounds (set during render, used for selection)
@(private)
log_text_bounds: clay.BoundingBox

// Cached monospace character width for selection calculations
@(private)
mono_char_width: f32 = 0

// Dirty state tracking for performance optimization
@(private)
needs_redraw: bool = true

@(private)
last_mouse_pos: rl.Vector2

@(private)
last_window_width: i32 = 0

@(private)
last_window_height: i32 = 0

@(private)
last_window_focused: bool = true

// Label key for Docker Compose project
COMPOSE_PROJECT_LABEL :: "com.docker.compose.project"

clay_color_to_rl_color :: proc(color: clay.Color) -> rl.Color {
	return {u8(color.r), u8(color.g), u8(color.b), u8(color.a)}
}

measure_text :: proc "c" (
	text: clay.StringSlice,
	config: ^clay.TextElementConfig,
	userData: rawptr,
) -> clay.Dimensions {
	context = runtime.default_context()

	line_width: f32 = 0

	font := raylib_fonts[config.fontId].font
	text_str := string(text.chars[:text.length])
	grapheme_count, _, _ := utf8.grapheme_count(text_str)
	for letter, byte_idx in text_str {
		glyph_index := rl.GetGlyphIndex(font, letter)
		glyph := font.glyphs[glyph_index]
		if glyph.advanceX != 0 {
			line_width += f32(glyph.advanceX)
		} else {
			line_width += font.recs[glyph_index].width + f32(font.glyphs[glyph_index].offsetX)
		}
	}
	scaleFactor := f32(config.fontSize) / f32(font.baseSize)
	total_spacing := f32(grapheme_count) * f32(config.letterSpacing)
	return {width = line_width * scaleFactor + total_spacing, height = f32(config.fontSize)}
}

error_handler :: proc "c" (errorData: clay.ErrorData) {
	context = runtime.default_context()
	// Log errors for debugging
}

// Load an icon from embedded PNG data and return a texture
@(private)
load_icon :: proc(data: []u8) -> rl.Texture2D {
	image := rl.LoadImageFromMemory(".png", raw_data(data), i32(len(data)))
	texture := rl.LoadTextureFromImage(image)
	rl.SetTextureFilter(texture, .BILINEAR)
	rl.UnloadImage(image)
	return texture
}

init :: proc() {
	// Initialize Raylib window
	rl.SetConfigFlags({.WINDOW_RESIZABLE, .MSAA_4X_HINT})
	rl.InitWindow(1024, 768, "Harbor - Docker Manager")
	// Note: We don't use SetTargetFPS - we control frame timing via dirty state

	// Initialize Clay
	min_memory := clay.MinMemorySize()
	clay_memory = make([]u8, min_memory)
	clay_arena = clay.CreateArenaWithCapacityAndMemory(len(clay_memory), raw_data(clay_memory))

	clay.Initialize(
		clay_arena,
		{f32(rl.GetScreenWidth()), f32(rl.GetScreenHeight())},
		{handler = error_handler, userData = nil},
	)

	// Load body font from embedded data
	font := rl.LoadFontFromMemory(".ttf", raw_data(FONT_DATA), i32(len(FONT_DATA)), 32, nil, 0)
	rl.SetTextureFilter(font.texture, .BILINEAR)
	append(&raylib_fonts, Raylib_Font{fontId = FONT_ID_BODY, font = font})

	// Load monospace font for logs
	mono_font := rl.LoadFontFromMemory(
		".ttf",
		raw_data(FONT_DATA_MONO),
		i32(len(FONT_DATA_MONO)),
		32,
		nil,
		0,
	)
	rl.SetTextureFilter(mono_font.texture, .BILINEAR)
	append(&raylib_fonts, Raylib_Font{fontId = FONT_ID_MONO, font = mono_font})

	// Set Clay text measurement function
	clay.SetMeasureTextFunction(measure_text, nil)

	// Load icons
	icons.chevron_down = load_icon(ICON_CHEVRON_DOWN)
	icons.chevron_right = load_icon(ICON_CHEVRON_RIGHT)
	icons.paragraph = load_icon(ICON_PARAGRAPH)
	icons.x = load_icon(ICON_X)
	icons.terminal = load_icon(ICON_TERMINAL)
	icons.play = load_icon(ICON_PLAY)
	icons.stop = load_icon(ICON_STOP)
}

shutdown :: proc() {
	// Stop any active log stream
	docker.stop_log_stream()

	// Unload icons
	rl.UnloadTexture(icons.chevron_down)
	rl.UnloadTexture(icons.chevron_right)
	rl.UnloadTexture(icons.paragraph)
	rl.UnloadTexture(icons.x)
	rl.UnloadTexture(icons.terminal)
	rl.UnloadTexture(icons.play)
	rl.UnloadTexture(icons.stop)

	for rf in raylib_fonts {
		rl.UnloadFont(rf.font)
	}
	delete(raylib_fonts)
	delete(clay_memory)
	rl.CloseWindow()
}

// Mark the UI as needing a redraw
mark_dirty :: proc() {
	needs_redraw = true
}

// Check if the UI needs to be redrawn
is_dirty :: proc() -> bool {
	return needs_redraw
}

// Clear the dirty flag after rendering
clear_dirty :: proc() {
	needs_redraw = false
}

// Check for input events and window changes that require a redraw
// Call this every iteration of the main loop
check_events :: proc() {
	// Check for window resize
	current_width := rl.GetScreenWidth()
	current_height := rl.GetScreenHeight()
	if current_width != last_window_width || current_height != last_window_height {
		last_window_width = current_width
		last_window_height = current_height
		needs_redraw = true
	}

	// Check for window focus change
	current_focused := rl.IsWindowFocused()
	if current_focused != last_window_focused {
		last_window_focused = current_focused
		needs_redraw = true
	}

	// Check for mouse movement (with threshold to avoid micro-movements)
	current_mouse := rl.GetMousePosition()
	mouse_delta_x := current_mouse.x - last_mouse_pos.x
	mouse_delta_y := current_mouse.y - last_mouse_pos.y
	if mouse_delta_x * mouse_delta_x + mouse_delta_y * mouse_delta_y > 1.0 {
		last_mouse_pos = current_mouse
		needs_redraw = true
	}

	// Check for mouse button events
	if rl.IsMouseButtonPressed(.LEFT) ||
	   rl.IsMouseButtonPressed(.RIGHT) ||
	   rl.IsMouseButtonPressed(.MIDDLE) ||
	   rl.IsMouseButtonReleased(.LEFT) ||
	   rl.IsMouseButtonReleased(.RIGHT) ||
	   rl.IsMouseButtonReleased(.MIDDLE) {
		needs_redraw = true
	}

	// Check for mouse wheel scroll
	scroll := rl.GetMouseWheelMoveV()
	if scroll.x != 0 || scroll.y != 0 {
		needs_redraw = true
	}

	// Check for any key press (using GetKeyPressed which returns 0 if no key)
	if rl.GetKeyPressed() != .KEY_NULL {
		needs_redraw = true
	}

	// Check for specific modifier keys that GetKeyPressed might miss
	if rl.IsKeyPressed(.ESCAPE) ||
	   rl.IsKeyPressed(.LEFT_CONTROL) ||
	   rl.IsKeyPressed(.RIGHT_CONTROL) ||
	   rl.IsKeyPressed(.LEFT_SUPER) ||
	   rl.IsKeyPressed(.RIGHT_SUPER) {
		needs_redraw = true
	}
}

update :: proc() {
	// Update Clay with current window size
	clay.SetLayoutDimensions({f32(rl.GetScreenWidth()), f32(rl.GetScreenHeight())})

	// Update pointer state
	mouse_pos := rl.GetMousePosition()
	clay.SetPointerState({mouse_pos.x, mouse_pos.y}, rl.IsMouseButtonDown(.LEFT))

	// Update scroll - disable drag scrolling when log overlay is active to allow text selection
	scroll_delta := rl.GetMouseWheelMoveV()
	enable_drag := !docker.is_log_stream_active() // Disable drag when log overlay is shown
	clay.UpdateScrollContainers(
		enable_drag,
		{scroll_delta.x, scroll_delta.y * 10},
		rl.GetFrameTime(),
	)
}

render :: proc(containers: []docker.ContainerSummary) {
	update()

	// Handle log overlay close on Escape
	if docker.is_log_stream_active() && rl.IsKeyPressed(.ESCAPE) {
		docker.stop_log_stream()
	}

	// Group containers by project using temp allocator
	ungrouped := make([dynamic]docker.ContainerSummary, context.temp_allocator)
	groups := make(map[string][dynamic]docker.ContainerSummary, allocator = context.temp_allocator)
	project_order := make([dynamic]string, context.temp_allocator)

	for container in containers {
		project := get_container_project(container)
		if project == "" {
			append(&ungrouped, container)
		} else {
			if project not_in groups {
				groups[project] = make([dynamic]docker.ContainerSummary, context.temp_allocator)
				append(&project_order, project)
			}
			append(&groups[project], container)
		}
	}

	clay.BeginLayout()

	// Main container
	if clay.UI()(
	{
		layout = {
			sizing = {clay.SizingGrow({}), clay.SizingGrow({})},
			layoutDirection = .TopToBottom,
		},
		backgroundColor = COLOR_BACKGROUND,
	},
	) {
		// Content area with scroll
		if clay.UI(clay.GetElementId(clay.MakeString("ScrollContainer")))(
		{
			layout = {
				sizing = {clay.SizingGrow({}), clay.SizingGrow({})},
				padding = {16, 16, 16, 16},
				childGap = 8,
				layoutDirection = .TopToBottom,
			},
			clip = {vertical = true, childOffset = clay.GetScrollOffset()},
		},
		) {
			container_index: u32 = 0

			// Render ungrouped containers first (no header)
			for container in ungrouped {
				render_container_card(container, container_index)
				container_index += 1
			}

			// Render project groups
			for project in project_order {
				project_containers := groups[project]
				render_project_group(project, project_containers[:], &container_index)
			}

			if len(containers) == 0 {
				clay.Text(
					"No containers found",
					clay.TextConfig(
						{
							textColor = COLOR_TEXT_SECONDARY,
							fontId = FONT_ID_BODY,
							fontSize = FONT_SIZE_BODY,
						},
					),
				)
			}
		}

		// Render log overlay if active
		if docker.is_log_stream_active() {
			render_log_overlay()
		}
	}

	render_commands := clay.EndLayout()

	rl.BeginDrawing()
	rl.ClearBackground(clay_color_to_rl_color(COLOR_BACKGROUND))
	clay_raylib_render(&render_commands)

	// Draw selection rectangles on top of Clay rendering (if log overlay is active)
	if docker.is_log_stream_active() {
		draw_log_selection()
	}

	rl.EndDrawing()
}

@(private)
render_container_card :: proc(container: docker.ContainerSummary, index: u32) {
	// Get container name
	name := get_container_name(container)
	status_color := get_status_color(container.State)

	// Pre-compute element IDs for hover detection
	element_id := clay.GetElementIdWithIndex(clay.MakeString("ContainerCard"), index)
	action_container_id := clay.GetElementIdWithIndex(clay.MakeString("CardActions"), index)
	logs_btn_id := clay.GetElementIdWithIndex(clay.MakeString("LogsBtn"), index)
	terminal_btn_id := clay.GetElementIdWithIndex(clay.MakeString("TerminalBtn"), index)
	startstop_btn_id := clay.GetElementIdWithIndex(clay.MakeString("StartStopBtn"), index)

	// Check hover state - include floating action elements to prevent flickering
	card_hovered :=
		clay.PointerOver(element_id) ||
		clay.PointerOver(action_container_id) ||
		clay.PointerOver(logs_btn_id) ||
		clay.PointerOver(terminal_btn_id) ||
		clay.PointerOver(startstop_btn_id)
	card_color := COLOR_CARD
	if card_hovered {
		card_color = COLOR_CARD_HOVER
	}

	if clay.UI(element_id)(
	{
		layout = {
			sizing = {clay.SizingGrow({}), clay.SizingFit({})},
			padding = {16, 16, 12, 12},
			childGap = 8,
			layoutDirection = .TopToBottom,
		},
		backgroundColor = card_color,
		cornerRadius = clay.CornerRadiusAll(8),
	},
	) {
		// Container name
		clay.TextDynamic(
			name,
			clay.TextConfig(
				{textColor = COLOR_TEXT, fontId = FONT_ID_BODY, fontSize = FONT_SIZE_BODY},
			),
		)

		// Container details row
		if clay.UI()(
		{layout = {sizing = {clay.SizingGrow({}), clay.SizingFit({})}, childGap = 16}},
		) {
			// Image
			clay.TextDynamic(
				container.Image,
				clay.TextConfig(
					{
						textColor = COLOR_TEXT_SECONDARY,
						fontId = FONT_ID_BODY,
						fontSize = FONT_SIZE_BODY - 2,
					},
				),
			)

			// Status
			clay.TextDynamic(
				container.Status != "" ? container.Status : container.State,
				clay.TextConfig(
					{
						textColor = status_color,
						fontId = FONT_ID_BODY,
						fontSize = FONT_SIZE_BODY - 2,
					},
				),
			)
		}

		// Floating action buttons (only visible on hover)
		if card_hovered {
			if clay.UI(action_container_id)(
			{
				layout = {
					sizing = {clay.SizingFit({}), clay.SizingFit({})},
					padding = {8, 16, 0, 0},
					childGap = 4,
					childAlignment = {y = .Center},
				},
				floating = {
					attachTo = .Parent,
					attachment = {element = .RightCenter, parent = .RightCenter},
				},
			},
			) {
				// Start/Stop button
				is_running := container.State == "running"
				startstop_btn_color := is_running ? COLOR_STATUS_STOPPED : COLOR_STATUS_RUNNING
				if clay.PointerOver(startstop_btn_id) {
					startstop_btn_color =
						is_running ? clay.Color{255, 100, 100, 255} : clay.Color{100, 200, 100, 255}

					// Handle click
					if rl.IsMouseButtonPressed(.LEFT) {
						if is_running {
							docker.stop_container(container.Id)
						} else {
							docker.start_container(container.Id)
						}
					}
				}

				if clay.UI(startstop_btn_id)(
				{
					layout = {
						sizing = {clay.SizingFixed(28), clay.SizingFixed(28)},
						padding = {6, 6, 6, 6},
						childAlignment = {x = .Center, y = .Center},
					},
					backgroundColor = startstop_btn_color,
					cornerRadius = clay.CornerRadiusAll(4),
				},
				) {
					// Play or Stop icon
					icon_texture := is_running ? &icons.stop : &icons.play
					if clay.UI()(
					{
						layout = {sizing = {clay.SizingFixed(16), clay.SizingFixed(16)}},
						image = {imageData = icon_texture},
					},
					) {}
				}

				// Logs button
				logs_btn_color := COLOR_BUTTON
				if clay.PointerOver(logs_btn_id) {
					logs_btn_color = COLOR_BUTTON_HOVER

					// Handle click
					if rl.IsMouseButtonPressed(.LEFT) {
						docker.start_log_stream(container.Id, name)
					}
				}

				if clay.UI(logs_btn_id)(
				{
					layout = {
						sizing = {clay.SizingFixed(28), clay.SizingFixed(28)},
						padding = {6, 6, 6, 6},
						childAlignment = {x = .Center, y = .Center},
					},
					backgroundColor = logs_btn_color,
					cornerRadius = clay.CornerRadiusAll(4),
				},
				) {
					// Paragraph icon for logs
					if clay.UI()(
					{
						layout = {sizing = {clay.SizingFixed(16), clay.SizingFixed(16)}},
						image = {imageData = &icons.paragraph},
					},
					) {}
				}

				// Terminal button (only for running containers)
				if container.State == "running" {
					terminal_btn_color := COLOR_BUTTON
					if clay.PointerOver(terminal_btn_id) {
						terminal_btn_color = COLOR_BUTTON_HOVER

						// Handle click
						if rl.IsMouseButtonPressed(.LEFT) {
							docker.open_container_shell(container.Id, name)
						}
					}

					if clay.UI(terminal_btn_id)(
					{
						layout = {
							sizing = {clay.SizingFixed(28), clay.SizingFixed(28)},
							padding = {6, 6, 6, 6},
							childAlignment = {x = .Center, y = .Center},
						},
						backgroundColor = terminal_btn_color,
						cornerRadius = clay.CornerRadiusAll(4),
					},
					) {
						// Terminal icon
						if clay.UI()(
						{
							layout = {sizing = {clay.SizingFixed(16), clay.SizingFixed(16)}},
							image = {imageData = &icons.terminal},
						},
						) {}
					}
				}
			}
		}
	}
}

@(private)
get_container_name :: proc(container: docker.ContainerSummary) -> string {
	if len(container.Names) > 0 {
		name := container.Names[0]
		if strings.has_prefix(name, "/") {
			return name[1:]
		}
		return name
	}
	if len(container.Id) >= 12 {
		return container.Id[:12]
	}
	return container.Id
}

@(private)
get_status_color :: proc(state: string) -> clay.Color {
	switch state {
	case "running":
		return COLOR_STATUS_RUNNING
	case "exited", "dead":
		return COLOR_STATUS_STOPPED
	case:
		return COLOR_STATUS_OTHER
	}
}

@(private)
get_container_project :: proc(container: docker.ContainerSummary) -> string {
	if container.Labels == nil {
		return ""
	}
	return container.Labels[COMPOSE_PROJECT_LABEL] or_else ""
}

@(private)
render_project_group :: proc(
	project: string,
	containers: []docker.ContainerSummary,
	container_index: ^u32,
) {
	is_collapsed := collapsed_projects[project] or_else false

	// Determine if project is running (any container running)
	project_is_running := false
	for container in containers {
		if container.State == "running" {
			project_is_running = true
			break
		}
	}

	// Pre-compute element IDs
	header_id := clay.GetElementId(clay.MakeString(project))
	project_startstop_id := clay.GetElementId(
		clay.MakeString(strings.concatenate({project, "_startstop"}, context.temp_allocator)),
	)

	// Project group container
	if clay.UI()(
	{
		layout = {
			sizing = {clay.SizingGrow({}), clay.SizingFit({})},
			childGap = 4,
			layoutDirection = .TopToBottom,
		},
	},
	) {
		// Clickable header
		header_color := COLOR_HEADER
		if clay.PointerOver(header_id) || clay.PointerOver(project_startstop_id) {
			header_color = COLOR_CARD_HOVER
		}

		// Handle click to toggle collapse - but not if clicking the start/stop button
		if clay.PointerOver(header_id) &&
		   !clay.PointerOver(project_startstop_id) &&
		   rl.IsMouseButtonPressed(.LEFT) {
			collapsed_projects[project] = !is_collapsed
			is_collapsed = !is_collapsed
		}

		if clay.UI(header_id)(
		{
			layout = {
				sizing = {clay.SizingGrow({}), clay.SizingFit({})},
				padding = {12, 12, 8, 8},
				childGap = 8,
			},
			backgroundColor = header_color,
			cornerRadius = clay.CornerRadiusAll(6),
		},
		) {
			// Collapse indicator icon
			icon_texture := is_collapsed ? &icons.chevron_right : &icons.chevron_down
			if clay.UI()(
			{
				layout = {sizing = {clay.SizingFixed(16), clay.SizingFixed(16)}},
				image = {imageData = icon_texture},
			},
			) {}

			// Project name
			clay.TextDynamic(
				project,
				clay.TextConfig(
					{textColor = COLOR_TEXT, fontId = FONT_ID_BODY, fontSize = FONT_SIZE_BODY},
				),
			)

			// Container count
			count_str := strings.concatenate(
				{"(", int_to_string(len(containers)), ")"},
				context.temp_allocator,
			)
			clay.TextDynamic(
				count_str,
				clay.TextConfig(
					{
						textColor = COLOR_TEXT_SECONDARY,
						fontId = FONT_ID_BODY,
						fontSize = FONT_SIZE_BODY - 2,
					},
				),
			)

			// Floating start/stop button for project
			project_btn_color := project_is_running ? COLOR_STATUS_STOPPED : COLOR_STATUS_RUNNING
			if clay.PointerOver(project_startstop_id) {
				project_btn_color =
					project_is_running ? clay.Color{255, 100, 100, 255} : clay.Color{100, 200, 100, 255}

				// Handle click - start or stop all containers in project
				if rl.IsMouseButtonPressed(.LEFT) {
					for container in containers {
						if project_is_running {
							docker.stop_container(container.Id)
						} else {
							docker.start_container(container.Id)
						}
					}
				}
			}

			if clay.UI(project_startstop_id)(
			{
				layout = {
					sizing = {clay.SizingFixed(24), clay.SizingFixed(24)},
					padding = {4, 4, 4, 4},
					childAlignment = {x = .Center, y = .Center},
				},
				backgroundColor = project_btn_color,
				cornerRadius = clay.CornerRadiusAll(4),
				floating = {
					attachTo = .Parent,
					attachment = {element = .RightCenter, parent = .RightCenter},
					offset = {-8, 0},
				},
			},
			) {
				// Play or Stop icon
				project_icon := project_is_running ? &icons.stop : &icons.play
				if clay.UI()(
				{
					layout = {sizing = {clay.SizingFixed(14), clay.SizingFixed(14)}},
					image = {imageData = project_icon},
				},
				) {}
			}
		}

		// Render containers if not collapsed
		if !is_collapsed {
			// Container list with left padding for indentation
			if clay.UI()(
			{
				layout = {
					sizing = {clay.SizingGrow({}), clay.SizingFit({})},
					padding = {left = 16},
					childGap = 4,
					layoutDirection = .TopToBottom,
				},
			},
			) {
				for container in containers {
					render_container_card(container, container_index^)
					container_index^ += 1
				}
			}
		} else {
			// Still increment indices even when collapsed to maintain consistency
			container_index^ += u32(len(containers))
		}
	}
}

@(private)
int_to_string :: proc(n: int, allocator := context.temp_allocator) -> string {
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

	return strings.clone(string(buf[i:]), allocator)
}

// Raylib rendering functions
@(private)
clay_raylib_render :: proc(
	render_commands: ^clay.ClayArray(clay.RenderCommand),
	allocator := context.temp_allocator,
) {
	for i in 0 ..< render_commands.length {
		render_command := clay.RenderCommandArray_Get(render_commands, i)
		bounds := render_command.boundingBox
		switch render_command.commandType {
		case .None:
		// None
		case .Text:
			config := render_command.renderData.text
			text := string(config.stringContents.chars[:config.stringContents.length])
			cstr_text := strings.clone_to_cstring(text, allocator)
			font := raylib_fonts[config.fontId].font
			rl.DrawTextEx(
				font,
				cstr_text,
				{bounds.x, bounds.y},
				f32(config.fontSize),
				f32(config.letterSpacing),
				clay_color_to_rl_color(config.textColor),
			)
		case .Image:
			config := render_command.renderData.image
			tint := config.backgroundColor
			if tint == 0 {
				tint = {255, 255, 255, 255}
			}
			imageTexture := (^rl.Texture2D)(config.imageData)
			rl.DrawTextureEx(
				imageTexture^,
				{bounds.x, bounds.y},
				0,
				bounds.width / f32(imageTexture.width),
				clay_color_to_rl_color(tint),
			)
		case .ScissorStart:
			rl.BeginScissorMode(
				i32(math.round(bounds.x)),
				i32(math.round(bounds.y)),
				i32(math.round(bounds.width)),
				i32(math.round(bounds.height)),
			)
		case .ScissorEnd:
			rl.EndScissorMode()
		case .Rectangle:
			config := render_command.renderData.rectangle
			if config.cornerRadius.topLeft > 0 {
				radius: f32 = (config.cornerRadius.topLeft * 2) / min(bounds.width, bounds.height)
				draw_rect_rounded(
					bounds.x,
					bounds.y,
					bounds.width,
					bounds.height,
					radius,
					config.backgroundColor,
				)
			} else {
				draw_rect(bounds.x, bounds.y, bounds.width, bounds.height, config.backgroundColor)
			}
		case .Border:
			config := render_command.renderData.border
			// Left border
			if config.width.left > 0 {
				draw_rect(
					bounds.x,
					bounds.y + config.cornerRadius.topLeft,
					f32(config.width.left),
					bounds.height - config.cornerRadius.topLeft - config.cornerRadius.bottomLeft,
					config.color,
				)
			}
			// Right border
			if config.width.right > 0 {
				draw_rect(
					bounds.x + bounds.width - f32(config.width.right),
					bounds.y + config.cornerRadius.topRight,
					f32(config.width.right),
					bounds.height - config.cornerRadius.topRight - config.cornerRadius.bottomRight,
					config.color,
				)
			}
			// Top border
			if config.width.top > 0 {
				draw_rect(
					bounds.x + config.cornerRadius.topLeft,
					bounds.y,
					bounds.width - config.cornerRadius.topLeft - config.cornerRadius.topRight,
					f32(config.width.top),
					config.color,
				)
			}
			// Bottom border
			if config.width.bottom > 0 {
				draw_rect(
					bounds.x + config.cornerRadius.bottomLeft,
					bounds.y + bounds.height - f32(config.width.bottom),
					bounds.width -
					config.cornerRadius.bottomLeft -
					config.cornerRadius.bottomRight,
					f32(config.width.bottom),
					config.color,
				)
			}
			// Rounded Borders
			if config.cornerRadius.topLeft > 0 {
				draw_arc(
					bounds.x + config.cornerRadius.topLeft,
					bounds.y + config.cornerRadius.topLeft,
					config.cornerRadius.topLeft - f32(config.width.top),
					config.cornerRadius.topLeft,
					180,
					270,
					config.color,
				)
			}
			if config.cornerRadius.topRight > 0 {
				draw_arc(
					bounds.x + bounds.width - config.cornerRadius.topRight,
					bounds.y + config.cornerRadius.topRight,
					config.cornerRadius.topRight - f32(config.width.top),
					config.cornerRadius.topRight,
					270,
					360,
					config.color,
				)
			}
			if config.cornerRadius.bottomLeft > 0 {
				draw_arc(
					bounds.x + config.cornerRadius.bottomLeft,
					bounds.y + bounds.height - config.cornerRadius.bottomLeft,
					config.cornerRadius.bottomLeft - f32(config.width.top),
					config.cornerRadius.bottomLeft,
					90,
					180,
					config.color,
				)
			}
			if config.cornerRadius.bottomRight > 0 {
				draw_arc(
					bounds.x + bounds.width - config.cornerRadius.bottomRight,
					bounds.y + bounds.height - config.cornerRadius.bottomRight,
					config.cornerRadius.bottomRight - f32(config.width.bottom),
					config.cornerRadius.bottomRight,
					0.1,
					90,
					config.color,
				)
			}
		case clay.RenderCommandType.Custom:
		// Implement custom element rendering here
		}
	}
}

@(private = "file")
draw_arc :: proc(
	x, y: f32,
	inner_rad, outer_rad: f32,
	start_angle, end_angle: f32,
	color: clay.Color,
) {
	rl.DrawRing(
		{math.round(x), math.round(y)},
		math.round(inner_rad),
		outer_rad,
		start_angle,
		end_angle,
		10,
		clay_color_to_rl_color(color),
	)
}

@(private = "file")
draw_rect :: proc(x, y, w, h: f32, color: clay.Color) {
	rl.DrawRectangle(
		i32(math.round(x)),
		i32(math.round(y)),
		i32(math.round(w)),
		i32(math.round(h)),
		clay_color_to_rl_color(color),
	)
}

@(private = "file")
draw_rect_rounded :: proc(x, y, w, h: f32, radius: f32, color: clay.Color) {
	rl.DrawRectangleRounded({x, y, w, h}, radius, 8, clay_color_to_rl_color(color))
}

// Get the monospace character width (cached for performance)
@(private)
get_mono_char_width :: proc() -> f32 {
	if mono_char_width == 0 {
		// Calculate character width for monospace font
		font := raylib_fonts[FONT_ID_MONO].font
		scale_factor := f32(FONT_SIZE_LOG) / f32(font.baseSize)
		// Use 'M' as reference character for monospace width
		glyph_index := rl.GetGlyphIndex(font, 'M')
		glyph := font.glyphs[glyph_index]
		if glyph.advanceX != 0 {
			mono_char_width = f32(glyph.advanceX) * scale_factor
		} else {
			mono_char_width = font.recs[glyph_index].width * scale_factor
		}
	}
	return mono_char_width
}

// Calculate character index from mouse position relative to text area
@(private)
get_char_index_at_position :: proc(
	mouse_x, mouse_y: f32,
	text: string,
	text_bounds: clay.BoundingBox,
	scroll_offset: clay.Vector2,
) -> int {
	if len(text) == 0 {
		return 0
	}

	char_width := get_mono_char_width()
	line_height := f32(FONT_SIZE_LOG)

	// Calculate relative position within text bounds, accounting for scroll
	// scroll_offset.y is negative when scrolled down, so we subtract it to get the actual text position
	rel_x := mouse_x - text_bounds.x
	rel_y := mouse_y - text_bounds.y - scroll_offset.y

	// Clamp to valid range
	if rel_x < 0 {rel_x = 0}
	if rel_y < 0 {rel_y = 0}

	// Split text into lines to calculate position
	lines := strings.split(text, "\n", allocator = context.temp_allocator)

	// Calculate which line we're on
	line_index := int(rel_y / line_height)
	if line_index < 0 {line_index = 0}
	if line_index >= len(lines) {line_index = len(lines) - 1}

	// Calculate character position within the line
	char_in_line := int(rel_x / char_width)
	if char_in_line < 0 {char_in_line = 0}

	// Calculate absolute character index
	char_index := 0
	for i in 0 ..< line_index {
		char_index += len(lines[i]) + 1 // +1 for newline
	}

	// Add position within current line
	if line_index < len(lines) {
		line_len := len(lines[line_index])
		if char_in_line > line_len {
			char_in_line = line_len
		}
		char_index += char_in_line
	}

	// Clamp to text length
	if char_index > len(text) {
		char_index = len(text)
	}

	return char_index
}

// Clear text selection
@(private)
clear_log_selection :: proc() {
	log_selection_start = -1
	log_selection_end = -1
	log_is_selecting = false
}

// Get selected text (returns empty string if no selection)
@(private)
get_selected_log_text :: proc(text: string) -> string {
	if log_selection_start < 0 || log_selection_end < 0 {
		return ""
	}

	start := min(log_selection_start, log_selection_end)
	end := max(log_selection_start, log_selection_end)

	if start >= len(text) || end > len(text) || start == end {
		return ""
	}

	return text[start:end]
}

// Render the log overlay
@(private)
render_log_overlay :: proc() {
	screen_width := f32(rl.GetScreenWidth())
	screen_height := f32(rl.GetScreenHeight())

	// Get log content early for selection handling
	log_content := docker.get_log_content(context.temp_allocator)

	// Handle copy shortcut (Cmd+C on macOS, Ctrl+C on others)
	copy_pressed :=
		(rl.IsKeyDown(.LEFT_SUPER) ||
			rl.IsKeyDown(.RIGHT_SUPER) ||
			rl.IsKeyDown(.LEFT_CONTROL) ||
			rl.IsKeyDown(.RIGHT_CONTROL)) &&
		rl.IsKeyPressed(.C)

	if copy_pressed && log_selection_start >= 0 && log_selection_end >= 0 {
		selected_text := get_selected_log_text(log_content)
		if len(selected_text) > 0 {
			cstr := strings.clone_to_cstring(selected_text, context.temp_allocator)
			rl.SetClipboardText(cstr)
		}
	}

	// Semi-transparent overlay background (clicking closes the overlay)
	overlay_id := clay.GetElementId(clay.MakeString("LogOverlayBg"))

	// Check if clicking outside the log panel (on the overlay background)
	if clay.PointerOver(overlay_id) && rl.IsMouseButtonPressed(.LEFT) {
		// Check if we're not over the log panel itself
		panel_id := clay.GetElementId(clay.MakeString("LogPanel"))
		if !clay.PointerOver(panel_id) {
			clear_log_selection()
			docker.stop_log_stream()
			return
		}
	}

	if clay.UI(overlay_id)(
	{
		layout = {
			sizing = {clay.SizingGrow({}), clay.SizingGrow({})},
			childAlignment = {x = .Center, y = .Center},
		},
		floating = {attachTo = .Root, zIndex = 100},
		backgroundColor = COLOR_OVERLAY,
	},
	) {
		// Log panel
		panel_id := clay.GetElementId(clay.MakeString("LogPanel"))
		panel_width := screen_width * 0.8
		panel_height := screen_height * 0.8

		if clay.UI(panel_id)(
		{
			layout = {
				sizing = {clay.SizingFixed(panel_width), clay.SizingFixed(panel_height)},
				padding = {16, 16, 16, 16},
				childGap = 8,
				layoutDirection = .TopToBottom,
			},
			backgroundColor = COLOR_LOG_BACKGROUND,
			cornerRadius = clay.CornerRadiusAll(8),
		},
		) {
			// Header with container name and close button
			if clay.UI()(
			{
				layout = {
					sizing = {clay.SizingGrow({}), clay.SizingFit({})},
					childAlignment = {y = .Center},
				},
			},
			) {
				// Title
				container_name := docker.get_log_container_name()
				title := strings.concatenate({"Logs: ", container_name}, context.temp_allocator)
				clay.TextDynamic(
					title,
					clay.TextConfig(
						{textColor = COLOR_TEXT, fontId = FONT_ID_BODY, fontSize = FONT_SIZE_BODY},
					),
				)

				// Spacer
				if clay.UI()({layout = {sizing = {clay.SizingGrow({}), clay.SizingFit({})}}}) {}

				// Close button
				close_btn_id := clay.GetElementId(clay.MakeString("LogCloseBtn"))
				close_btn_color := COLOR_CARD
				if clay.PointerOver(close_btn_id) {
					close_btn_color = COLOR_CARD_HOVER
					if rl.IsMouseButtonPressed(.LEFT) {
						clear_log_selection()
						docker.stop_log_stream()
					}
				}

				if clay.UI(close_btn_id)(
				{
					layout = {
						sizing = {clay.SizingFixed(28), clay.SizingFixed(28)},
						padding = {6, 6, 6, 6},
						childAlignment = {x = .Center, y = .Center},
					},
					backgroundColor = close_btn_color,
					cornerRadius = clay.CornerRadiusAll(4),
				},
				) {
					// X icon for close button
					if clay.UI()(
					{
						layout = {sizing = {clay.SizingFixed(16), clay.SizingFixed(16)}},
						image = {imageData = &icons.x},
					},
					) {}
				}
			}

			// Log content area with scroll
			log_scroll_id := clay.GetElementId(clay.MakeString("LogScrollContainer"))

			if clay.UI(log_scroll_id)(
			{
				layout = {
					sizing = {clay.SizingGrow({}), clay.SizingGrow({})},
					padding = {8, 8, 8, 8},
					layoutDirection = .TopToBottom,
				},
				backgroundColor = COLOR_CARD,
				cornerRadius = clay.CornerRadiusAll(4),
				clip = {vertical = true, childOffset = clay.GetScrollOffset()},
			},
			) {
				// Handle mouse selection within the log text area
				log_text_id := clay.GetElementId(clay.MakeString("LogTextContent"))
				mouse_over_log := clay.PointerOver(log_scroll_id)
				mouse_pos := rl.GetMousePosition()

				// Get scroll offset for this specific container using GetScrollContainerData
				scroll_container_data := clay.GetScrollContainerData(log_scroll_id)
				scroll_offset: clay.Vector2 = {0, 0}
				if scroll_container_data.found && scroll_container_data.scrollPosition != nil {
					scroll_offset = scroll_container_data.scrollPosition^
				}

				// Get the element data to know the bounds
				scroll_data := clay.GetElementData(log_scroll_id)
				if scroll_data.found {
					log_text_bounds = scroll_data.boundingBox
					// Adjust for padding
					log_text_bounds.x += 8
					log_text_bounds.y += 8
					log_text_bounds.width -= 16
					log_text_bounds.height -= 16
				}

				if mouse_over_log && len(log_content) > 0 {
					// Handle mouse press - start selection
					if rl.IsMouseButtonPressed(.LEFT) {
						log_selection_start = get_char_index_at_position(
							mouse_pos.x,
							mouse_pos.y,
							log_content,
							log_text_bounds,
							scroll_offset,
						)
						log_selection_end = log_selection_start
						log_is_selecting = true
					}

					// Handle mouse drag - update selection end
					if log_is_selecting && rl.IsMouseButtonDown(.LEFT) {
						log_selection_end = get_char_index_at_position(
							mouse_pos.x,
							mouse_pos.y,
							log_content,
							log_text_bounds,
							scroll_offset,
						)
					}
				}

				// Handle mouse release - end selection
				if rl.IsMouseButtonReleased(.LEFT) {
					log_is_selecting = false
				}

				if len(log_content) > 0 {
					// Render log text with selection
					render_log_text_with_selection(log_content, log_text_id)
				} else {
					clay.Text(
						"Waiting for logs...",
						clay.TextConfig(
							{
								textColor = COLOR_TEXT_SECONDARY,
								fontId = FONT_ID_MONO,
								fontSize = FONT_SIZE_LOG,
							},
						),
					)
				}
			}
		}
	}
}

// Render log text with selection highlighting
@(private)
render_log_text_with_selection :: proc(text: string, element_id: clay.ElementId) {
	// Just render the text - selection highlighting is done separately via draw_log_selection
	clay.TextDynamic(
		text,
		clay.TextConfig(
			{
				textColor = COLOR_TEXT,
				fontId = FONT_ID_MONO,
				fontSize = FONT_SIZE_LOG,
				wrapMode = .Newlines,
			},
		),
	)
}

// Draw selection rectangles for log text (called after Clay render)
@(private)
draw_log_selection :: proc() {
	// Check if there's a valid selection
	if log_selection_start < 0 ||
	   log_selection_end < 0 ||
	   log_selection_start == log_selection_end {
		return
	}

	// Get log content
	log_content := docker.get_log_content(context.temp_allocator)
	if len(log_content) == 0 {
		return
	}

	// Calculate selection bounds
	start := min(log_selection_start, log_selection_end)
	end := max(log_selection_start, log_selection_end)

	// Clamp to valid range
	if start < 0 {start = 0}
	if end > len(log_content) {end = len(log_content)}
	if start >= end {
		return
	}

	// Get character dimensions
	char_width := get_mono_char_width()
	line_height := f32(FONT_SIZE_LOG)

	// Get scroll offset for the log scroll container
	log_scroll_id := clay.GetElementId(clay.MakeString("LogScrollContainer"))
	scroll_container_data := clay.GetScrollContainerData(log_scroll_id)
	scroll_offset: clay.Vector2 = {0, 0}
	if scroll_container_data.found && scroll_container_data.scrollPosition != nil {
		scroll_offset = scroll_container_data.scrollPosition^
	}

	// Calculate text bounds with scroll offset applied
	text_x := log_text_bounds.x
	text_y := log_text_bounds.y + scroll_offset.y

	// Split text into lines
	lines := strings.split(log_content, "\n", allocator = context.temp_allocator)

	// Find which lines are selected
	current_char := 0
	selection_color := clay_color_to_rl_color(COLOR_SELECTION)

	for line, line_idx in lines {
		line_start := current_char
		line_end := current_char + len(line)

		// Check if this line overlaps with selection
		if line_end >= start && line_start < end {
			// Calculate selection within this line
			sel_start_in_line := max(0, start - line_start)
			sel_end_in_line := min(len(line), end - line_start)

			// Calculate rectangle position
			rect_x := text_x + f32(sel_start_in_line) * char_width
			rect_y := text_y + f32(line_idx) * line_height
			rect_width := f32(sel_end_in_line - sel_start_in_line) * char_width
			rect_height := line_height

			// Only draw if within visible bounds (basic culling)
			if rect_y + rect_height >= log_text_bounds.y &&
			   rect_y <= log_text_bounds.y + log_text_bounds.height {
				// Clip to text bounds
				if rect_y < log_text_bounds.y {
					clip_amount := log_text_bounds.y - rect_y
					rect_y = log_text_bounds.y
					rect_height -= clip_amount
				}
				if rect_y + rect_height > log_text_bounds.y + log_text_bounds.height {
					rect_height = log_text_bounds.y + log_text_bounds.height - rect_y
				}

				if rect_height > 0 {
					rl.DrawRectangle(
						i32(rect_x),
						i32(rect_y),
						i32(rect_width),
						i32(rect_height),
						selection_color,
					)
				}
			}
		}

		current_char = line_end + 1 // +1 for newline
	}
}
