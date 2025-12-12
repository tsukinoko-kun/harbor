package ui

import (
	"bufio"
	"context"
	"encoding/binary"
	"image"
	"io"
	"strings"
	"sync"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/tsukinoko-kun/harbor/internal/docker"
)

// LogsWindow represents a window for displaying container logs.
type LogsWindow struct {
	window        *app.Window
	theme         *Theme
	docker        *docker.Client
	containerID   string
	containerName string

	// Log content
	mu         sync.RWMutex
	logContent strings.Builder
	editor     widget.Editor
	list       widget.List

	// Control
	cancel context.CancelFunc
	closed bool
}

// NewLogsWindow creates and runs a new logs window for a container.
func NewLogsWindow(theme *Theme, dockerClient *docker.Client, containerID, containerName string) {
	lw := &LogsWindow{
		theme:         theme,
		docker:        dockerClient,
		containerID:   containerID,
		containerName: containerName,
		editor: widget.Editor{
			ReadOnly:   true,
			SingleLine: false,
		},
		list: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	}

	go lw.run()
}

func (lw *LogsWindow) run() {
	lw.window = new(app.Window)
	lw.window.Option(
		app.Title("Logs: "+lw.containerName),
		app.Size(unit.Dp(800), unit.Dp(600)),
		app.MinSize(unit.Dp(400), unit.Dp(300)),
	)

	// Start streaming logs
	ctx, cancel := context.WithCancel(context.Background())
	lw.cancel = cancel
	go lw.streamLogs(ctx)

	// Run the event loop
	var ops op.Ops
	for {
		switch e := lw.window.Event().(type) {
		case app.DestroyEvent:
			lw.closed = true
			if lw.cancel != nil {
				lw.cancel()
			}
			return
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			lw.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (lw *LogsWindow) streamLogs(ctx context.Context) {
	reader, err := lw.docker.ContainerLogs(ctx, lw.containerID, true)
	if err != nil {
		lw.appendLog("Error fetching logs: " + err.Error() + "\n")
		return
	}
	defer reader.Close()

	// Docker logs have an 8-byte header for multiplexed streams
	// Format: [1 byte stream type][3 bytes padding][4 bytes size (big-endian)]
	header := make([]byte, 8)
	bufReader := bufio.NewReader(reader)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read the 8-byte header
		_, err := io.ReadFull(bufReader, header)
		if err != nil {
			if err == io.EOF || ctx.Err() != nil {
				return
			}
			// Try reading as raw text (some containers don't have multiplexed output)
			lw.readRawLogs(bufReader, ctx)
			return
		}

		// Parse the size from header bytes 4-7
		size := binary.BigEndian.Uint32(header[4:8])
		if size == 0 {
			continue
		}

		// Read the log message
		msg := make([]byte, size)
		_, err = io.ReadFull(bufReader, msg)
		if err != nil {
			if err == io.EOF || ctx.Err() != nil {
				return
			}
			continue
		}

		lw.appendLog(string(msg))
	}
}

func (lw *LogsWindow) readRawLogs(reader *bufio.Reader, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if line != "" {
			lw.appendLog(line)
		}
		if err != nil {
			return
		}
	}
}

func (lw *LogsWindow) appendLog(msg string) {
	lw.mu.Lock()
	lw.logContent.WriteString(msg)
	lw.mu.Unlock()

	if lw.window != nil && !lw.closed {
		lw.window.Invalidate()
	}
}

func (lw *LogsWindow) layout(gtx layout.Context) layout.Dimensions {
	// Fill background
	paint.FillShape(gtx.Ops, lw.theme.Colors.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())

	return layout.Inset{
		Top:    unit.Dp(8),
		Bottom: unit.Dp(8),
		Left:   unit.Dp(12),
		Right:  unit.Dp(12),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return lw.layoutLogs(gtx)
	})
}

func (lw *LogsWindow) layoutLogs(gtx layout.Context) layout.Dimensions {
	lw.mu.RLock()
	content := lw.logContent.String()
	lw.mu.RUnlock()

	if content == "" {
		content = "Waiting for logs..."
	}

	// Update editor content if it changed
	currentText := lw.editor.Text()
	if currentText != content {
		lw.editor.SetText(content)
		// Move cursor to end for auto-scroll effect
		lw.editor.SetCaret(len(content), len(content))
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			// Background for the log area
			rr := gtx.Dp(unit.Dp(4))
			rect := clip.RRect{
				Rect: image.Rectangle{Max: gtx.Constraints.Max},
				NE:   rr, NW: rr, SE: rr, SW: rr,
			}
			paint.FillShape(gtx.Ops, lw.theme.Colors.Surface, rect.Op(gtx.Ops))
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(8),
				Bottom: unit.Dp(8),
				Left:   unit.Dp(8),
				Right:  unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				// Use material.Editor for styled text editing
				editor := material.Editor(lw.theme.Material, &lw.editor, "")
				editor.Color = lw.theme.Colors.Text
				editor.SelectionColor = lw.theme.Colors.SelectedBg
				return editor.Layout(gtx)
			})
		}),
	)
}
