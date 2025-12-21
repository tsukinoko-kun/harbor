package ui

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"path/filepath"

	"gio.tools/icons"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/sqweek/dialog"

	ts "github.com/tree-sitter/go-tree-sitter"
	"github.com/tsukinoko-kun/harbor/internal/dap"
	ts_dockerfile "github.com/tsukinoko-kun/harbor/internal/treesitter/dockerfile"
)

var (
	tsDockerfileLanguage *ts.Language
	tsDockerfileQuery    *ts.Query
)

func init() {
	tsDockerfileLanguage = ts.NewLanguage(ts_dockerfile.Language())

	var err *ts.QueryError = nil
	tsDockerfileQuery, err = ts.NewQuery(tsDockerfileLanguage, ts_dockerfile.QueryHighlights)
	if err != nil {
		panic(err)
	}
}

// highlightToken represents a syntax-highlighted token within a line.
type highlightToken struct {
	Start   int    // byte offset within the line
	End     int    // byte offset within the line (exclusive)
	Text    string // the actual text content
	Capture string // capture name like "keyword", "string", "comment"
}

// parseDockerfileHighlights parses Dockerfile content and returns syntax highlight tokens per line.
func parseDockerfileHighlights(content []byte, lines []string) [][]highlightToken {
	parser := ts.NewParser()
	defer parser.Close()
	parser.SetLanguage(tsDockerfileLanguage)

	tree := parser.Parse(content, nil)
	defer tree.Close()

	cursor := ts.NewQueryCursor()
	defer cursor.Close()

	captures := cursor.Captures(tsDockerfileQuery, tree.RootNode(), content)

	// Build a map of line -> captures
	lineCaptures := make(map[int][]highlightToken)

	for match, captureIndex := captures.Next(); match != nil; match, captureIndex = captures.Next() {
		capture := match.Captures[captureIndex]
		captureName := tsDockerfileQuery.CaptureNames()[capture.Index]

		// Skip "none" captures
		if captureName == "none" {
			continue
		}

		node := capture.Node
		startPoint := node.StartPosition()
		endPoint := node.EndPosition()
		startByte := node.StartByte()
		endByte := node.EndByte()

		// Handle single-line captures
		if startPoint.Row == endPoint.Row {
			lineNum := int(startPoint.Row)
			token := highlightToken{
				Start:   int(startPoint.Column),
				End:     int(endPoint.Column),
				Text:    string(content[startByte:endByte]),
				Capture: captureName,
			}
			lineCaptures[lineNum] = append(lineCaptures[lineNum], token)
		} else {
			// Handle multi-line captures (e.g., heredocs)
			for row := startPoint.Row; row <= endPoint.Row; row++ {
				lineNum := int(row)
				if lineNum >= len(lines) {
					continue
				}
				line := lines[lineNum]

				var tokenStart, tokenEnd int
				if row == startPoint.Row {
					tokenStart = int(startPoint.Column)
					tokenEnd = len(line)
				} else if row == endPoint.Row {
					tokenStart = 0
					tokenEnd = int(endPoint.Column)
				} else {
					tokenStart = 0
					tokenEnd = len(line)
				}

				if tokenEnd > len(line) {
					tokenEnd = len(line)
				}
				if tokenStart > tokenEnd {
					tokenStart = tokenEnd
				}

				token := highlightToken{
					Start:   tokenStart,
					End:     tokenEnd,
					Text:    line[tokenStart:tokenEnd],
					Capture: captureName,
				}
				lineCaptures[lineNum] = append(lineCaptures[lineNum], token)
			}
		}
	}

	// Convert to slice and fill gaps with default tokens
	result := make([][]highlightToken, len(lines))
	for i, line := range lines {
		captures := lineCaptures[i]

		// Sort captures by start position
		sortTokens(captures)

		// Merge overlapping/contained captures (later captures win)
		captures = mergeTokens(captures, line)

		// Fill gaps with default tokens
		result[i] = fillGaps(captures, line)
	}

	return result
}

// sortTokens sorts tokens by start position.
func sortTokens(tokens []highlightToken) {
	for i := 1; i < len(tokens); i++ {
		for j := i; j > 0 && tokens[j].Start < tokens[j-1].Start; j-- {
			tokens[j], tokens[j-1] = tokens[j-1], tokens[j]
		}
	}
}

// mergeTokens handles overlapping tokens by keeping the most specific (smallest) ones.
func mergeTokens(tokens []highlightToken, line string) []highlightToken {
	if len(tokens) == 0 {
		return tokens
	}

	// Create a character-level map of which capture applies
	charCapture := make([]string, len(line))
	for i := range charCapture {
		charCapture[i] = ""
	}

	// Apply captures - later captures in the query order take precedence
	for _, token := range tokens {
		for i := token.Start; i < token.End && i < len(charCapture); i++ {
			charCapture[i] = token.Capture
		}
	}

	// Reconstruct tokens from the character map
	var result []highlightToken
	i := 0
	for i < len(line) {
		capture := charCapture[i]
		start := i

		// Find the end of this capture span
		for i < len(line) && charCapture[i] == capture {
			i++
		}

		if capture != "" {
			result = append(result, highlightToken{
				Start:   start,
				End:     i,
				Text:    line[start:i],
				Capture: capture,
			})
		}
	}

	return result
}

// fillGaps adds default tokens for unhighlighted text.
func fillGaps(tokens []highlightToken, line string) []highlightToken {
	if len(line) == 0 {
		return nil
	}

	var result []highlightToken
	pos := 0

	for _, token := range tokens {
		// Add gap before this token if needed
		if token.Start > pos {
			result = append(result, highlightToken{
				Start:   pos,
				End:     token.Start,
				Text:    line[pos:token.Start],
				Capture: "", // default/unhighlighted
			})
		}
		result = append(result, token)
		pos = token.End
	}

	// Add trailing gap if needed
	if pos < len(line) {
		result = append(result, highlightToken{
			Start:   pos,
			End:     len(line),
			Text:    line[pos:],
			Capture: "", // default/unhighlighted
		})
	}

	return result
}

// DebugView displays debug information and controls.
type DebugView struct {
	theme *Theme

	// Form state for start phase
	dockerfileEditor       widget.Editor
	dockerfileBrowseButton widget.Clickable
	contextEditor          widget.Editor
	contextBrowseButton    widget.Clickable
	startButton            widget.Clickable

	// Debug control buttons
	stepOverButton widget.Clickable
	stepInButton   widget.Clickable
	stepOutButton  widget.Clickable
	continueButton widget.Clickable
	stopButton     widget.Clickable

	// Dockerfile viewer state
	dockerfileLines      []string           // Cached lines from the Dockerfile
	dockerfilePath       string             // Path to the currently loaded Dockerfile
	dockerfileList       widget.List        // Scrollable list for Dockerfile lines (vertical)
	dockerfileHScroll    widget.List        // Horizontal scroll for wide content
	dockerfileHighlights [][]highlightToken // Syntax highlights per line

	// Console state
	consoleEditor widget.Editor // Single-line input for commands
	consoleList   widget.List   // Scrollable list for command history

	// Scroll tracking
	lastScrolledLine int // Track the last line we scrolled to
}

// NewDebugView creates a new debug view.
func NewDebugView(theme *Theme) *DebugView {
	return &DebugView{
		theme: theme,
		dockerfileEditor: widget.Editor{
			SingleLine: true,
			Submit:     true,
		},
		contextEditor: widget.Editor{
			SingleLine: true,
			Submit:     true,
		},
		dockerfileList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		dockerfileHScroll: widget.List{
			List: layout.List{
				Axis: layout.Horizontal,
			},
		},
		consoleEditor: widget.Editor{
			SingleLine: true,
			Submit:     true,
		},
		consoleList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	}
}

// Layout renders the debug view.
func (v *DebugView) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutHeader(gtx)
		}),
		// Content - depends on whether debugger is running
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Left:  unit.Dp(16),
				Right: unit.Dp(16),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if dap.Client != nil {
					return v.layoutRunning(gtx)
				}
				return v.layoutStartForm(gtx)
			})
		}),
	)
}

func (v *DebugView) layoutHeader(gtx layout.Context) layout.Dimensions {
	return layout.Inset{
		Top:    unit.Dp(20),
		Bottom: unit.Dp(16),
		Left:   unit.Dp(16),
		Right:  unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		title := material.H5(v.theme.Material, "Debug")
		title.Color = v.theme.Colors.Text
		return title.Layout(gtx)
	})
}

// layoutStartForm shows the form to configure and start the debugger
func (v *DebugView) layoutStartForm(gtx layout.Context) layout.Dimensions {
	// Handle start button click
	if v.startButton.Clicked(gtx) {
		params := dap.DebugParams{
			Dockerfile: v.dockerfileEditor.Text(),
			Context:    v.contextEditor.Text(),
		}
		client, err := dap.NewClient(params)
		if err != nil {
			// TODO: Show error in UI
			log.Printf("Failed to start debugger: %v", err)
		} else {
			dap.Client = client
		}
	}

	// Handle dockerfile browse button click
	if v.dockerfileBrowseButton.Clicked(gtx) {
		go func() {
			filename, err := dialog.File().Title("Select Dockerfile").Load()
			if err == nil {
				v.dockerfileEditor.SetText(filename)
				if v.contextEditor.Text() == "" {
					v.contextEditor.SetText(filepath.Dir(filename))
				}
			}
		}()
	}

	// Handle context browse button click
	if v.contextBrowseButton.Clicked(gtx) {
		go func() {
			directory, err := dialog.Directory().Title("Select Build Context").Browse()
			if err == nil {
				v.contextEditor.SetText(directory)
			}
		}()
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Description
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Body1(v.theme.Material, "Configure the debugger to step through your Dockerfile build process.")
				label.Color = v.theme.Colors.TextMuted
				return label.Layout(gtx)
			})
		}),
		// Dockerfile input
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutFormFieldWithBrowse(gtx, "Dockerfile", "Path to Dockerfile (e.g., ./Dockerfile)", &v.dockerfileEditor, &v.dockerfileBrowseButton, icons.EditorInsertDriveFile)
		}),
		// Context input
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutFormFieldWithBrowse(gtx, "Build Context", "Context directory (e.g., .)", &v.contextEditor, &v.contextBrowseButton, icons.FileFolderOpen)
			})
		}),
		// Start button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutStartButton(gtx)
			})
		}),
	)
}

// layoutFormFieldWithBrowse renders a labeled text input field with a browse button
func (v *DebugView) layoutFormFieldWithBrowse(gtx layout.Context, label, hint string, editor *widget.Editor, browseButton *widget.Clickable, icon *widget.Icon) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Label
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(v.theme.Material, label)
				l.Color = v.theme.Colors.Text
				return l.Layout(gtx)
			})
		}),
		// Input field with browse button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// Browse button
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return v.layoutIconButton(gtx, browseButton, icon)
					})
				}),
				// Input field
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return v.layoutTextInput(gtx, hint, editor)
				}),
			)
		}),
	)
}

// layoutIconButton renders an icon button for browse actions
func (v *DebugView) layoutIconButton(gtx layout.Context, clickable *widget.Clickable, icon *widget.Icon) layout.Dimensions {
	return clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if clickable.Hovered() {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		size := gtx.Dp(unit.Dp(40))

		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				bgColor := v.theme.Colors.CardBg
				if clickable.Hovered() {
					bgColor = v.theme.Colors.ButtonHover
				}

				rr := gtx.Dp(unit.Dp(6))
				rect := clip.RRect{
					Rect: image.Rectangle{Max: image.Point{X: size, Y: size}},
					NE:   rr, NW: rr, SE: rr, SW: rr,
				}
				paint.FillShape(gtx.Ops, bgColor, rect.Op(gtx.Ops))

				// Border
				borderWidth := gtx.Dp(unit.Dp(1))
				borderRect := clip.RRect{
					Rect: image.Rectangle{Max: image.Point{X: size, Y: size}},
					NE:   rr, NW: rr, SE: rr, SW: rr,
				}
				paint.FillShape(gtx.Ops, v.theme.Colors.Border, borderRect.Op(gtx.Ops))

				// Inner fill
				innerRect := clip.RRect{
					Rect: image.Rectangle{
						Min: image.Point{X: borderWidth, Y: borderWidth},
						Max: image.Point{X: size - borderWidth, Y: size - borderWidth},
					},
					NE: rr - borderWidth, NW: rr - borderWidth, SE: rr - borderWidth, SW: rr - borderWidth,
				}
				innerColor := v.theme.Colors.CardBg
				if clickable.Hovered() {
					innerColor = v.theme.Colors.ButtonHover
				}
				paint.FillShape(gtx.Ops, innerColor, innerRect.Op(gtx.Ops))

				return layout.Dimensions{Size: image.Point{X: size, Y: size}}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				iconSize := gtx.Dp(unit.Dp(20))
				gtx.Constraints.Min = image.Point{X: iconSize, Y: iconSize}
				gtx.Constraints.Max = gtx.Constraints.Min
				return icon.Layout(gtx, v.theme.Colors.Text)
			}),
		)
	})
}

// layoutTextInput renders a styled text input
func (v *DebugView) layoutTextInput(gtx layout.Context, hint string, editor *widget.Editor) layout.Dimensions {
	// Card background with border
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			rr := gtx.Dp(unit.Dp(6))
			rect := clip.RRect{
				Rect: image.Rectangle{Max: gtx.Constraints.Min},
				NE:   rr, NW: rr, SE: rr, SW: rr,
			}
			paint.FillShape(gtx.Ops, v.theme.Colors.CardBg, rect.Op(gtx.Ops))

			// Border
			borderWidth := gtx.Dp(unit.Dp(1))
			borderRect := clip.RRect{
				Rect: image.Rectangle{Max: gtx.Constraints.Min},
				NE:   rr, NW: rr, SE: rr, SW: rr,
			}
			paint.FillShape(gtx.Ops, v.theme.Colors.Border, borderRect.Op(gtx.Ops))

			// Inner fill (slightly smaller to show border)
			innerRect := clip.RRect{
				Rect: image.Rectangle{
					Min: image.Point{X: borderWidth, Y: borderWidth},
					Max: image.Point{X: gtx.Constraints.Min.X - borderWidth, Y: gtx.Constraints.Min.Y - borderWidth},
				},
				NE: rr - borderWidth, NW: rr - borderWidth, SE: rr - borderWidth, SW: rr - borderWidth,
			}
			paint.FillShape(gtx.Ops, v.theme.Colors.CardBg, innerRect.Op(gtx.Ops))

			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(12),
				Bottom: unit.Dp(12),
				Left:   unit.Dp(12),
				Right:  unit.Dp(12),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				e := material.Editor(v.theme.Material, editor, hint)
				e.Color = v.theme.Colors.Text
				e.HintColor = v.theme.Colors.TextMuted
				return e.Layout(gtx)
			})
		}),
	)
}

// layoutStartButton renders the start debugger button
func (v *DebugView) layoutStartButton(gtx layout.Context) layout.Dimensions {
	return v.startButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if v.startButton.Hovered() {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				bgColor := v.theme.Colors.Primary
				if v.startButton.Hovered() {
					// Slightly lighter on hover
					bgColor = lightenColor(bgColor, 0.1)
				}

				rr := gtx.Dp(unit.Dp(6))
				rect := clip.RRect{
					Rect: image.Rectangle{Max: gtx.Constraints.Min},
					NE:   rr, NW: rr, SE: rr, SW: rr,
				}
				paint.FillShape(gtx.Ops, bgColor, rect.Op(gtx.Ops))
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top:    unit.Dp(12),
					Bottom: unit.Dp(12),
					Left:   unit.Dp(24),
					Right:  unit.Dp(24),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.Body1(v.theme.Material, "Start Debugger")
					label.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
					return label.Layout(gtx)
				})
			}),
		)
	})
}

// layoutRunning shows the running debugger view
func (v *DebugView) layoutRunning(gtx layout.Context) layout.Dimensions {
	// Guard against nil client (can happen if stop button was just clicked)
	if dap.Client == nil {
		return layout.Dimensions{}
	}

	// Handle button clicks
	if v.stepOverButton.Clicked(gtx) {
		if err := dap.Client.SendNext(); err != nil {
			log.Printf("Failed to step over: %v", err)
		}
	}
	if v.stepInButton.Clicked(gtx) {
		if err := dap.Client.SendStepIn(); err != nil {
			log.Printf("Failed to step in: %v", err)
		}
	}
	if v.stepOutButton.Clicked(gtx) {
		if err := dap.Client.SendStepOut(); err != nil {
			log.Printf("Failed to step out: %v", err)
		}
	}
	if v.continueButton.Clicked(gtx) {
		if err := dap.Client.SendContinue(); err != nil {
			log.Printf("Failed to continue: %v", err)
		}
	}
	if v.stopButton.Clicked(gtx) {
		log.Printf("Stopping debugger...")
		dap.Client.Close()
		dap.Client = nil
		// Clear cached Dockerfile content so it reloads on next debug session
		v.dockerfileLines = nil
		v.dockerfilePath = ""
		v.dockerfileHighlights = nil
		v.lastScrolledLine = 0
		// Return early to avoid accessing the now-nil client
		return layout.Dimensions{}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Status message and current line info
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				var statusText string
				if dap.Client.Stopped {
					statusText = fmt.Sprintf("Paused at line %d", dap.Client.CurrentLine)
				} else {
					statusText = "Running..."
				}
				label := material.Body1(v.theme.Material, statusText)
				label.Color = v.theme.Colors.Text
				return label.Layout(gtx)
			})
		}),
		// Debug control buttons
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutDebugButtons(gtx)
			})
		}),
		// Main content area: Dockerfile viewer + Console (horizontal split)
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				// Dockerfile viewer (left side)
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return v.layoutDockerfile(gtx)
				}),
				// Spacer between panels
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{}
					})
				}),
				// Console (right side)
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return v.layoutConsole(gtx)
				}),
			)
		}),
	)
}

// loadDockerfile reads the Dockerfile content into dockerfileLines.
// It caches the result and only reloads if the path changes.
func (v *DebugView) loadDockerfile(path string) {
	if path == v.dockerfilePath && len(v.dockerfileLines) > 0 {
		return // Already loaded
	}

	v.dockerfilePath = path
	v.dockerfileLines = nil
	v.dockerfileHighlights = nil

	// Read the entire file content
	content, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Failed to read Dockerfile: %v", err)
		return
	}

	// Split into lines
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		v.dockerfileLines = append(v.dockerfileLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Failed to scan Dockerfile: %v", err)
		return
	}

	// Parse syntax highlights
	v.dockerfileHighlights = parseDockerfileHighlights(content, v.dockerfileLines)
}

// layoutDockerfile renders the Dockerfile content with line numbers and execution marker.
func (v *DebugView) layoutDockerfile(gtx layout.Context) layout.Dimensions {
	if dap.Client == nil {
		return layout.Dimensions{}
	}

	// Load the Dockerfile if not already loaded
	v.loadDockerfile(dap.Client.Params.Dockerfile)

	if len(v.dockerfileLines) == 0 {
		// Show error message if Dockerfile couldn't be loaded
		label := material.Body2(v.theme.Material, "Could not load Dockerfile")
		label.Color = v.theme.Colors.TextMuted
		return label.Layout(gtx)
	}

	currentLine := dap.Client.CurrentLine
	currentEndLine := dap.Client.CurrentEndLine
	isStopped := dap.Client.Stopped

	// Determine the effective end line for range highlighting
	// If EndLine is 0 (not provided), treat it as single-line (same as start)
	effectiveEndLine := currentEndLine
	if effectiveEndLine == 0 {
		effectiveEndLine = currentLine
	}

	// Auto-scroll to current execution line when it changes
	if isStopped && currentLine != v.lastScrolledLine && currentLine > 0 {
		// Scroll with some context lines above (show ~3 lines before current line)
		targetLine := currentLine - 4 // 1-indexed to 0-indexed, minus context
		if targetLine < 0 {
			targetLine = 0
		}
		v.dockerfileList.Position.First = targetLine
		v.dockerfileList.Position.Offset = 0
		v.lastScrolledLine = currentLine
	}

	// Calculate width needed for line numbers (based on total lines)
	lineNumWidth := gtx.Dp(unit.Dp(40))
	markerWidth := gtx.Dp(unit.Dp(24))

	// Render in a card-like container
	return layout.Stack{}.Layout(gtx,
		// Background
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			rr := gtx.Dp(unit.Dp(6))
			rect := clip.RRect{
				Rect: image.Rectangle{Max: gtx.Constraints.Max},
				NE:   rr, NW: rr, SE: rr, SW: rr,
			}
			paint.FillShape(gtx.Ops, v.theme.Colors.CardBg, rect.Op(gtx.Ops))
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		// Content with horizontal scroll
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(8),
				Bottom: unit.Dp(8),
				Left:   unit.Dp(4),
				Right:  unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				// Horizontal scroll wrapper using layout.List directly for better control
				return v.dockerfileHScroll.List.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
					// Vertical list of lines - let it size naturally
					return material.List(v.theme.Material, &v.dockerfileList).Layout(gtx, len(v.dockerfileLines), func(gtx layout.Context, index int) layout.Dimensions {
						lineNum := index + 1 // 1-indexed

						// Determine if this line is in the execution range
						isInRange := isStopped && lineNum >= currentLine && lineNum <= effectiveEndLine
						isRangeStart := isStopped && lineNum == currentLine

						// Get highlights for this line (if available)
						var tokens []highlightToken
						if index < len(v.dockerfileHighlights) {
							tokens = v.dockerfileHighlights[index]
						}

						return v.layoutDockerfileLine(gtx, lineNum, tokens, isInRange, isRangeStart, markerWidth, lineNumWidth)
					})
				})
			})
		}),
	)
}

// layoutDockerfileLine renders a single line of the Dockerfile with syntax highlighting.
// isInRange indicates if this line is within the current execution range.
// isRangeStart indicates if this is the first line of the range (shows play arrow).
func (v *DebugView) layoutDockerfileLine(gtx layout.Context, lineNum int, tokens []highlightToken, isInRange, isRangeStart bool, markerWidth, lineNumWidth int) layout.Dimensions {
	lineHeight := gtx.Dp(unit.Dp(22))

	// Wrap content in a Stack to draw background highlight for lines in range
	return layout.Stack{}.Layout(gtx,
		// Background highlight for lines in execution range
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if isInRange {
				// Subtle highlight color based on primary color with low opacity
				highlightColor := color.NRGBA{
					R: v.theme.Colors.Primary.R,
					G: v.theme.Colors.Primary.G,
					B: v.theme.Colors.Primary.B,
					A: 30, // Low opacity for subtle highlight
				}
				rect := clip.Rect{Max: image.Point{X: gtx.Constraints.Max.X, Y: lineHeight}}
				paint.FillShape(gtx.Ops, highlightColor, rect.Op())
			}
			return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Max.X, Y: lineHeight}}
		}),
		// Line content
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// Marker area (play arrow for range start)
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = markerWidth
					gtx.Constraints.Max.X = markerWidth
					gtx.Constraints.Min.Y = lineHeight
					gtx.Constraints.Max.Y = lineHeight

					if isRangeStart {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							iconSize := gtx.Dp(unit.Dp(16))
							gtx.Constraints.Min = image.Point{X: iconSize, Y: iconSize}
							gtx.Constraints.Max = gtx.Constraints.Min
							return icons.AVPlayArrow.Layout(gtx, v.theme.Colors.Primary)
						})
					}
					return layout.Dimensions{Size: image.Point{X: markerWidth, Y: lineHeight}}
				}),
				// Line number
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = lineNumWidth
					gtx.Constraints.Max.X = lineNumWidth
					gtx.Constraints.Min.Y = lineHeight
					gtx.Constraints.Max.Y = lineHeight

					return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							label := material.Body2(v.theme.Material, fmt.Sprintf("%d", lineNum))
							if isInRange {
								label.Color = v.theme.Colors.Text // Brighter for lines in range
							} else {
								label.Color = v.theme.Colors.TextMuted
							}
							return label.Layout(gtx)
						})
					})
				}),
				// Separator
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						// Draw vertical line separator
						width := gtx.Dp(unit.Dp(1))
						rect := clip.Rect{Max: image.Point{X: width, Y: lineHeight}}
						paint.FillShape(gtx.Ops, v.theme.Colors.Border, rect.Op())
						return layout.Dimensions{Size: image.Point{X: width, Y: lineHeight}}
					})
				}),
				// Dockerfile content with syntax highlighting
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.Y = lineHeight
					gtx.Constraints.Max.Y = lineHeight
					gtx.Constraints.Min.X = 0 // Don't force minimum width

					return v.layoutHighlightedContent(gtx, tokens, isInRange)
				}),
			)
		}),
	)
}

// layoutHighlightedContent renders syntax-highlighted tokens as a horizontal layout.
func (v *DebugView) layoutHighlightedContent(gtx layout.Context, tokens []highlightToken, isInRange bool) layout.Dimensions {
	if len(tokens) == 0 {
		return layout.Dimensions{}
	}

	// Build layout children for each token
	children := make([]layout.FlexChild, len(tokens))
	for i, token := range tokens {
		token := token // capture for closure
		children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Body2(v.theme.Material, token.Text)
			label.Font.Weight = font.Medium

			// Get color based on capture type
			tokenColor := v.getCaptureColor(token.Capture)

			// If this line is in the execution range, boost the brightness slightly
			if isInRange {
				tokenColor = brightenColor(tokenColor, 0.15)
			}

			label.Color = tokenColor
			return label.Layout(gtx)
		})
	}

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

// getCaptureColor returns the appropriate color for a syntax capture name.
func (v *DebugView) getCaptureColor(capture string) color.NRGBA {
	switch capture {
	case "keyword":
		return v.theme.Syntax.Keyword
	case "string":
		return v.theme.Syntax.String
	case "comment":
		return v.theme.Syntax.Comment
	case "operator":
		return v.theme.Syntax.Operator
	case "constant":
		return v.theme.Syntax.Constant
	case "punctuation.special":
		return v.theme.Syntax.Punctuation
	default:
		return v.theme.Syntax.Default
	}
}

// brightenColor increases the brightness of a color by the given factor (0-1).
func brightenColor(c color.NRGBA, factor float32) color.NRGBA {
	return color.NRGBA{
		R: uint8(min(255, float32(c.R)*(1+factor))),
		G: uint8(min(255, float32(c.G)*(1+factor))),
		B: uint8(min(255, float32(c.B)*(1+factor))),
		A: c.A,
	}
}

// layoutDebugButtons renders the debug control buttons
func (v *DebugView) layoutDebugButtons(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceStart}.Layout(gtx,
		// Continue button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutDebugIconButton(gtx, &v.continueButton, icons.AVPlayArrow, dap.Client.Stopped)
		}),
		// Spacer
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{}
			})
		}),
		// Step Over button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutDebugIconButton(gtx, &v.stepOverButton, icons.AVSkipNext, dap.Client.Stopped)
		}),
		// Spacer
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{}
			})
		}),
		// Step In button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutDebugIconButton(gtx, &v.stepInButton, icons.NavigationArrowDownward, dap.Client.Stopped)
		}),
		// Spacer
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{}
			})
		}),
		// Step Out button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutDebugIconButton(gtx, &v.stepOutButton, icons.NavigationArrowUpward, dap.Client.Stopped)
		}),
		// Spacer
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{}
			})
		}),
		// Stop button (always enabled when debugger is running)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutStopButton(gtx, &v.stopButton, icons.AVStop)
		}),
	)
}

// layoutStopButton renders the stop button (gray normally, red on hover)
func (v *DebugView) layoutStopButton(gtx layout.Context, clickable *widget.Clickable, icon *widget.Icon) layout.Dimensions {
	return clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if clickable.Hovered() {
			pointer.CursorPointer.Add(gtx.Ops)
		}

		size := gtx.Dp(unit.Dp(36))
		isHovered := clickable.Hovered()

		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				var bgColor color.NRGBA
				if isHovered {
					bgColor = v.theme.Colors.ButtonDanger
				} else {
					bgColor = v.theme.Colors.ButtonBg
				}

				rr := gtx.Dp(unit.Dp(6))
				rect := clip.RRect{
					Rect: image.Rectangle{Max: image.Point{X: size, Y: size}},
					NE:   rr, NW: rr, SE: rr, SW: rr,
				}
				paint.FillShape(gtx.Ops, bgColor, rect.Op(gtx.Ops))
				return layout.Dimensions{Size: image.Point{X: size, Y: size}}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				iconSize := gtx.Dp(unit.Dp(20))
				gtx.Constraints.Min = image.Point{X: iconSize, Y: iconSize}
				gtx.Constraints.Max = gtx.Constraints.Min

				var iconColor color.NRGBA
				if isHovered {
					iconColor = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
				} else {
					iconColor = v.theme.Colors.Text
				}
				return icon.Layout(gtx, iconColor)
			}),
		)
	})
}

// layoutDebugIconButton renders a single debug control button with an icon
func (v *DebugView) layoutDebugIconButton(gtx layout.Context, clickable *widget.Clickable, icon *widget.Icon, enabled bool) layout.Dimensions {
	return clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if enabled && clickable.Hovered() {
			pointer.CursorPointer.Add(gtx.Ops)
		}

		size := gtx.Dp(unit.Dp(36))

		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				var bgColor color.NRGBA
				if !enabled {
					// Disabled state - darker muted color
					bgColor = v.theme.Colors.CardBg
				} else if clickable.Hovered() {
					bgColor = v.theme.Colors.ButtonHover
				} else {
					bgColor = v.theme.Colors.ButtonBg
				}

				rr := gtx.Dp(unit.Dp(6))
				rect := clip.RRect{
					Rect: image.Rectangle{Max: image.Point{X: size, Y: size}},
					NE:   rr, NW: rr, SE: rr, SW: rr,
				}
				paint.FillShape(gtx.Ops, bgColor, rect.Op(gtx.Ops))
				return layout.Dimensions{Size: image.Point{X: size, Y: size}}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				iconSize := gtx.Dp(unit.Dp(20))
				gtx.Constraints.Min = image.Point{X: iconSize, Y: iconSize}
				gtx.Constraints.Max = gtx.Constraints.Min

				var iconColor color.NRGBA
				if enabled {
					iconColor = v.theme.Colors.Text
				} else {
					iconColor = v.theme.Colors.TextMuted
				}
				return icon.Layout(gtx, iconColor)
			}),
		)
	})
}

// lightenColor adjusts a color to be lighter by the given factor (0-1)
func lightenColor(c color.NRGBA, factor float32) color.NRGBA {
	return color.NRGBA{
		R: uint8(float32(c.R) + (255-float32(c.R))*factor),
		G: uint8(float32(c.G) + (255-float32(c.G))*factor),
		B: uint8(float32(c.B) + (255-float32(c.B))*factor),
		A: c.A,
	}
}

// layoutConsole renders the REPL-style console panel.
func (v *DebugView) layoutConsole(gtx layout.Context) layout.Dimensions {
	// Handle console input submission
	for {
		event, ok := v.consoleEditor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			text := v.consoleEditor.Text()
			if text != "" && dap.Client != nil && !dap.Client.EvaluatePending {
				if err := dap.Client.SendEvaluate(text); err != nil {
					log.Printf("Failed to send evaluate: %v", err)
				}
				v.consoleEditor.SetText("")
			}
		}
	}

	// Render in a card-like container
	return layout.Stack{}.Layout(gtx,
		// Background
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			rr := gtx.Dp(unit.Dp(6))
			rect := clip.RRect{
				Rect: image.Rectangle{Max: gtx.Constraints.Max},
				NE:   rr, NW: rr, SE: rr, SW: rr,
			}
			paint.FillShape(gtx.Ops, v.theme.Colors.CardBg, rect.Op(gtx.Ops))
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		// Content
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(8),
				Bottom: unit.Dp(8),
				Left:   unit.Dp(8),
				Right:  unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// Console header
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							label := material.Body2(v.theme.Material, "Console")
							label.Color = v.theme.Colors.TextMuted
							return label.Layout(gtx)
						})
					}),
					// Command history (takes remaining space)
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return v.layoutConsoleHistory(gtx)
					}),
					// Input box at bottom
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return v.layoutConsoleInput(gtx)
						})
					}),
				)
			})
		}),
	)
}

// layoutConsoleHistory renders the console output as a terminal-style view.
func (v *DebugView) layoutConsoleHistory(gtx layout.Context) layout.Dimensions {
	if dap.Client == nil {
		return layout.Dimensions{}
	}

	output := dap.Client.ConsoleOutput.Text
	if output == "" {
		// Show placeholder when empty
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Body2(v.theme.Material, "Type a command below...")
			label.Color = v.theme.Colors.TextMuted
			return label.Layout(gtx)
		})
	}

	// Auto-scroll to bottom
	v.consoleList.Position.BeforeEnd = false

	// Split output into lines for rendering
	lines := splitLines(output)

	return material.List(v.theme.Material, &v.consoleList).Layout(gtx, len(lines), func(gtx layout.Context, index int) layout.Dimensions {
		label := material.Body2(v.theme.Material, lines[index])
		label.Color = v.theme.Colors.TextSecondary
		return label.Layout(gtx)
	})
}

// splitLines splits a string into lines, preserving empty lines.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	// Add the last line if there's remaining content
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// layoutConsoleInput renders the input box for entering commands.
func (v *DebugView) layoutConsoleInput(gtx layout.Context) layout.Dimensions {
	isPending := dap.Client != nil && dap.Client.EvaluatePending

	return layout.Stack{}.Layout(gtx,
		// Background
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			rr := gtx.Dp(unit.Dp(4))
			bgColor := v.theme.Colors.Background
			if isPending {
				bgColor = v.theme.Colors.SidebarBg
			}
			rect := clip.RRect{
				Rect: image.Rectangle{Max: gtx.Constraints.Min},
				NE:   rr, NW: rr, SE: rr, SW: rr,
			}
			paint.FillShape(gtx.Ops, bgColor, rect.Op(gtx.Ops))

			// Border
			borderWidth := gtx.Dp(unit.Dp(1))
			borderRect := clip.RRect{
				Rect: image.Rectangle{Max: gtx.Constraints.Min},
				NE:   rr, NW: rr, SE: rr, SW: rr,
			}
			paint.FillShape(gtx.Ops, v.theme.Colors.Border, borderRect.Op(gtx.Ops))

			// Inner fill
			innerRect := clip.RRect{
				Rect: image.Rectangle{
					Min: image.Point{X: borderWidth, Y: borderWidth},
					Max: image.Point{X: gtx.Constraints.Min.X - borderWidth, Y: gtx.Constraints.Min.Y - borderWidth},
				},
				NE: rr - borderWidth, NW: rr - borderWidth, SE: rr - borderWidth, SW: rr - borderWidth,
			}
			paint.FillShape(gtx.Ops, bgColor, innerRect.Op(gtx.Ops))

			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		// Content
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// Prompt
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := material.Body2(v.theme.Material, ">")
						label.Color = v.theme.Colors.Primary
						label.Font.Weight = font.Bold
						return label.Layout(gtx)
					})
				}),
				// Input editor
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{
						Top:    unit.Dp(8),
						Bottom: unit.Dp(8),
						Left:   unit.Dp(4),
						Right:  unit.Dp(8),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						hint := "Enter command..."
						if isPending {
							hint = "Waiting for response..."
						}
						e := material.Editor(v.theme.Material, &v.consoleEditor, hint)
						e.Color = v.theme.Colors.Text
						e.HintColor = v.theme.Colors.TextMuted
						return e.Layout(gtx)
					})
				}),
			)
		}),
	)
}
