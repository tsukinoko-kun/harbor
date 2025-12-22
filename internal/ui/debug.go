package ui

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"path/filepath"

	"gio.tools/icons"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/sqweek/dialog"

	ts "github.com/tree-sitter/go-tree-sitter"
	"github.com/tsukinoko-kun/harbor/internal/config"
	"github.com/tsukinoko-kun/harbor/internal/dap"
	"github.com/tsukinoko-kun/harbor/internal/terminal"
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

// buildArgRow holds state for a single key-value build argument.
type buildArgRow struct {
	keyEditor   widget.Editor
	valueEditor widget.Editor
	removeBtn   widget.Clickable
}

// DebugView displays debug information and controls.
type DebugView struct {
	theme    *Theme
	settings *config.Settings

	// Form state for start phase
	dockerfileEditor       widget.Editor
	dockerfileBrowseButton widget.Clickable
	contextEditor          widget.Editor
	contextBrowseButton    widget.Clickable
	cachingEnabled         widget.Bool
	pullImages             widget.Bool
	buildArgs              []buildArgRow
	addBuildArgBtn         widget.Clickable
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

	// Breakpoint state
	breakpoints    map[int]bool       // Map of line numbers (1-indexed) to breakpoint state
	lineClickables []widget.Clickable // Clickable areas for each line (for toggling breakpoints)

	// Log panel state
	logList   widget.List   // Scrollable list for log entries
	logEditor widget.Editor // Read-only editor for selectable log text

	// Console state
	consoleEditor widget.Editor // Single-line input for commands
	consoleList   widget.List   // Scrollable list for command history

	// Scroll tracking
	lastScrolledLine int // Track the last line we scrolled to
}

// NewDebugView creates a new debug view.
func NewDebugView(theme *Theme, settings *config.Settings) *DebugView {
	return &DebugView{
		theme:    theme,
		settings: settings,
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
		logList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
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
		breakpoints: make(map[int]bool),
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
	// Check if form is valid (no duplicate build arg keys)
	hasDuplicates := v.hasDuplicateBuildArgKeys()

	// Handle start button click (only if no duplicates)
	if v.startButton.Clicked(gtx) && !hasDuplicates {
		params := dap.DebugParams{
			Dockerfile:    v.dockerfileEditor.Text(),
			Context:       v.contextEditor.Text(),
			PS1:           v.settings.DebugPS1,
			EnableCaching: v.cachingEnabled.Value,
			PullImages:    v.pullImages.Value,
			BuildArgs:     v.collectBuildArgs(),
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
		// Enable caching checkbox
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutCheckbox(gtx, &v.cachingEnabled, "Enable caching", "Cached layers are skipped, making those lines not debuggable.")
			})
		}),
		// Pull images checkbox
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutCheckbox(gtx, &v.pullImages, "Pull images", "")
			})
		}),
		// Build arguments
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutBuildArgsList(gtx)
			})
		}),
		// Start button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutStartButton(gtx, hasDuplicates)
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

// layoutStartButton renders the start debugger button.
// If disabled is true, the button appears grayed out.
func (v *DebugView) layoutStartButton(gtx layout.Context, disabled bool) layout.Dimensions {
	return v.startButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if !disabled && v.startButton.Hovered() {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				var bgColor color.NRGBA
				if disabled {
					bgColor = v.theme.Colors.ButtonBg
				} else if v.startButton.Hovered() {
					bgColor = lightenColor(v.theme.Colors.Primary, 0.1)
				} else {
					bgColor = v.theme.Colors.Primary
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
					if disabled {
						label.Color = v.theme.Colors.TextMuted
					} else {
						label.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
					}
					return label.Layout(gtx)
				})
			}),
		)
	})
}

// layoutCheckbox renders a styled checkbox with a label and optional warning text.
// Warning text is gray by default and yellow when checkbox is checked.
func (v *DebugView) layoutCheckbox(gtx layout.Context, checkbox *widget.Bool, label string, warning string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Checkbox with label
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return checkbox.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if checkbox.Hovered() {
					pointer.CursorPointer.Add(gtx.Ops)
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					// Checkbox box
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						size := gtx.Dp(unit.Dp(18))
						rr := gtx.Dp(unit.Dp(3))

						// Draw checkbox border
						borderColor := v.theme.Colors.Border
						if checkbox.Value {
							borderColor = v.theme.Colors.Primary
						}

						rect := clip.RRect{
							Rect: image.Rectangle{Max: image.Point{X: size, Y: size}},
							NE:   rr, NW: rr, SE: rr, SW: rr,
						}
						paint.FillShape(gtx.Ops, borderColor, rect.Op(gtx.Ops))

						// Inner fill
						borderWidth := gtx.Dp(unit.Dp(2))
						innerRect := clip.RRect{
							Rect: image.Rectangle{
								Min: image.Point{X: borderWidth, Y: borderWidth},
								Max: image.Point{X: size - borderWidth, Y: size - borderWidth},
							},
							NE: rr - 1, NW: rr - 1, SE: rr - 1, SW: rr - 1,
						}
						innerColor := v.theme.Colors.CardBg
						if checkbox.Hovered() && !checkbox.Value {
							innerColor = v.theme.Colors.ButtonHover
						}
						if checkbox.Value {
							innerColor = v.theme.Colors.Primary
						}
						paint.FillShape(gtx.Ops, innerColor, innerRect.Op(gtx.Ops))

						// Draw checkmark if checked
						if checkbox.Value {
							v.drawCheckmark(gtx, size)
						}

						return layout.Dimensions{Size: image.Point{X: size, Y: size}}
					}),
					// Label
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							l := material.Body2(v.theme.Material, label)
							l.Color = v.theme.Colors.Text
							return l.Layout(gtx)
						})
					}),
				)
			})
		}),
		// Warning text (if provided) - gray by default, yellow when checked
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if warning == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: unit.Dp(26), Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(v.theme.Material, warning)
				if checkbox.Value {
					l.Color = v.theme.Colors.Warning
				} else {
					l.Color = v.theme.Colors.TextMuted
				}
				return l.Layout(gtx)
			})
		}),
	)
}

// drawCheckmark draws a checkmark icon inside the checkbox.
func (v *DebugView) drawCheckmark(gtx layout.Context, boxSize int) {
	// Draw a simple checkmark using lines
	// The checkmark consists of two line segments forming a "✓" shape
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	strokeWidth := gtx.Dp(unit.Dp(2))

	// Calculate checkmark points relative to box size
	// Short leg: from bottom-left going down-right
	// Long leg: from bottom going up-right
	padding := boxSize / 5

	// Points for the checkmark (adjusted for visual centering)
	x1 := padding + 1           // Start of short leg
	y1 := boxSize/2 + 1         // Start Y
	x2 := boxSize/2 - 1         // Corner point X
	y2 := boxSize - padding - 2 // Corner point Y (bottom of check)
	x3 := boxSize - padding - 1 // End of long leg X
	y3 := padding + 2           // End of long leg Y (top)

	// Draw short leg (bottom-left to corner)
	drawLine(gtx.Ops, white, strokeWidth, x1, y1, x2, y2)
	// Draw long leg (corner to top-right)
	drawLine(gtx.Ops, white, strokeWidth, x2, y2, x3, y3)
}

// drawLine draws a line between two points using a rotated rectangle.
func drawLine(ops *op.Ops, c color.NRGBA, width, x1, y1, x2, y2 int) {
	// Calculate line length and angle
	dx := float32(x2 - x1)
	dy := float32(y2 - y1)
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if length < 1 {
		return
	}

	// Create a thin rectangle and rotate it
	halfWidth := float32(width) / 2

	// Build the path for a line segment
	var path clip.Path
	path.Begin(ops)
	path.MoveTo(f32.Point{X: float32(x1), Y: float32(y1) - halfWidth})
	path.LineTo(f32.Point{X: float32(x2), Y: float32(y2) - halfWidth})
	path.LineTo(f32.Point{X: float32(x2), Y: float32(y2) + halfWidth})
	path.LineTo(f32.Point{X: float32(x1), Y: float32(y1) + halfWidth})
	path.Close()

	paint.FillShape(ops, c, clip.Outline{Path: path.End()}.Op())
}

// layoutBuildArgsList renders the dynamic list of build arguments with add/remove buttons.
func (v *DebugView) layoutBuildArgsList(gtx layout.Context) layout.Dimensions {
	// Handle add button click
	if v.addBuildArgBtn.Clicked(gtx) {
		v.buildArgs = append(v.buildArgs, buildArgRow{
			keyEditor: widget.Editor{
				SingleLine: true,
				Submit:     false,
			},
			valueEditor: widget.Editor{
				SingleLine: true,
				Submit:     false,
			},
		})
	}

	// Handle remove button clicks (iterate in reverse to handle removal safely)
	for i := len(v.buildArgs) - 1; i >= 0; i-- {
		if v.buildArgs[i].removeBtn.Clicked(gtx) {
			v.buildArgs = append(v.buildArgs[:i], v.buildArgs[i+1:]...)
		}
	}

	// Detect duplicate keys
	duplicates := v.findDuplicateBuildArgKeys()

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Label
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(v.theme.Material, "Build Arguments")
				l.Color = v.theme.Colors.Text
				return l.Layout(gtx)
			})
		}),
		// Duplicate warning
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(duplicates) == 0 {
				return layout.Dimensions{}
			}
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(v.theme.Material, "Duplicate keys are not allowed")
				l.Color = v.theme.Colors.ButtonDanger
				return l.Layout(gtx)
			})
		}),
		// Build arg rows
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(v.buildArgs) == 0 {
				return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Caption(v.theme.Material, "No build arguments defined")
					l.Color = v.theme.Colors.TextMuted
					return l.Layout(gtx)
				})
			}

			// Render each row
			children := make([]layout.FlexChild, len(v.buildArgs))
			for i := range v.buildArgs {
				idx := i // Capture for closure
				isDuplicate := duplicates[v.buildArgs[idx].keyEditor.Text()]
				children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return v.layoutBuildArgRow(gtx, idx, isDuplicate)
					})
				})
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}),
		// Add button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutAddButton(gtx)
		}),
	)
}

// findDuplicateBuildArgKeys returns a map of keys that appear more than once.
func (v *DebugView) findDuplicateBuildArgKeys() map[string]bool {
	keyCounts := make(map[string]int)
	for _, row := range v.buildArgs {
		key := row.keyEditor.Text()
		if key != "" {
			keyCounts[key]++
		}
	}

	duplicates := make(map[string]bool)
	for key, count := range keyCounts {
		if count > 1 {
			duplicates[key] = true
		}
	}
	return duplicates
}

// hasDuplicateBuildArgKeys returns true if there are any duplicate build arg keys.
func (v *DebugView) hasDuplicateBuildArgKeys() bool {
	return len(v.findDuplicateBuildArgKeys()) > 0
}

// layoutBuildArgRow renders a single build argument row with key, value, and remove button.
func (v *DebugView) layoutBuildArgRow(gtx layout.Context, index int, isDuplicate bool) layout.Dimensions {
	row := &v.buildArgs[index]

	// Fixed dimensions for consistent layout
	keyWidth := gtx.Dp(unit.Dp(100))
	valueWidth := gtx.Dp(unit.Dp(250))
	buttonSize := gtx.Dp(unit.Dp(28))
	rowHeight := gtx.Dp(unit.Dp(36))
	gap := gtx.Dp(unit.Dp(8))

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		// Key field (fixed size)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{X: keyWidth, Y: rowHeight}
			gtx.Constraints.Max = gtx.Constraints.Min
			return v.layoutSmallTextInput(gtx, "Key", &row.keyEditor, isDuplicate)
		}),
		// Gap
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Point{X: gap}}
		}),
		// Value field (fixed size, wider)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{X: valueWidth, Y: rowHeight}
			gtx.Constraints.Max = gtx.Constraints.Min
			return v.layoutSmallTextInput(gtx, "Value", &row.valueEditor, false)
		}),
		// Gap
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Point{X: gap}}
		}),
		// Remove button (fixed size)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{X: buttonSize, Y: buttonSize}
			gtx.Constraints.Max = gtx.Constraints.Min
			return v.layoutRemoveButton(gtx, &row.removeBtn)
		}),
	)
}

// layoutSmallTextInput renders a smaller text input suitable for build args.
// If isError is true, the border is drawn in red to indicate an error.
// Returns exactly the size specified in constraints (fixed size).
func (v *DebugView) layoutSmallTextInput(gtx layout.Context, hint string, editor *widget.Editor, isError bool) layout.Dimensions {
	// Use the exact size from constraints
	size := gtx.Constraints.Min

	rr := gtx.Dp(unit.Dp(4))

	// Border - red if error, normal otherwise
	borderWidth := gtx.Dp(unit.Dp(1))
	borderColor := v.theme.Colors.Border
	if isError {
		borderColor = v.theme.Colors.ButtonDanger
		borderWidth = gtx.Dp(unit.Dp(2))
	}

	// Draw border
	borderRect := clip.RRect{
		Rect: image.Rectangle{Max: size},
		NE:   rr, NW: rr, SE: rr, SW: rr,
	}
	paint.FillShape(gtx.Ops, borderColor, borderRect.Op(gtx.Ops))

	// Draw inner fill
	innerRect := clip.RRect{
		Rect: image.Rectangle{
			Min: image.Point{X: borderWidth, Y: borderWidth},
			Max: image.Point{X: size.X - borderWidth, Y: size.Y - borderWidth},
		},
		NE: rr - borderWidth, NW: rr - borderWidth, SE: rr - borderWidth, SW: rr - borderWidth,
	}
	paint.FillShape(gtx.Ops, v.theme.Colors.CardBg, innerRect.Op(gtx.Ops))

	// Layout editor with padding inside the fixed size
	padding := gtx.Dp(unit.Dp(8))
	editorGtx := gtx
	editorGtx.Constraints.Min = image.Point{X: size.X - 2*padding, Y: size.Y - 2*padding}
	editorGtx.Constraints.Max = editorGtx.Constraints.Min

	// Offset for padding
	defer op.Offset(image.Point{X: padding, Y: padding}).Push(gtx.Ops).Pop()

	e := material.Editor(v.theme.Material, editor, hint)
	e.Color = v.theme.Colors.Text
	e.HintColor = v.theme.Colors.TextMuted
	e.Layout(editorGtx)

	return layout.Dimensions{Size: size}
}

// layoutAddButton renders the add button for build arguments.
func (v *DebugView) layoutAddButton(gtx layout.Context) layout.Dimensions {
	return v.addBuildArgBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if v.addBuildArgBtn.Hovered() {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			// Plus icon
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				iconSize := gtx.Dp(unit.Dp(16))
				gtx.Constraints.Min = image.Point{X: iconSize, Y: iconSize}
				gtx.Constraints.Max = gtx.Constraints.Min
				iconColor := v.theme.Colors.Primary
				if v.addBuildArgBtn.Hovered() {
					iconColor = lightenColor(iconColor, 0.2)
				}
				return icons.ContentAdd.Layout(gtx, iconColor)
			}),
			// Label
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(v.theme.Material, "Add")
					l.Color = v.theme.Colors.Primary
					if v.addBuildArgBtn.Hovered() {
						l.Color = lightenColor(l.Color, 0.2)
					}
					return l.Layout(gtx)
				})
			}),
		)
	})
}

// layoutRemoveButton renders the remove button (X) for a build argument row.
// Returns exactly the size specified in constraints (fixed size).
func (v *DebugView) layoutRemoveButton(gtx layout.Context, clickable *widget.Clickable) layout.Dimensions {
	// Use the exact size from constraints
	size := gtx.Constraints.Min

	return clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if clickable.Hovered() {
			pointer.CursorPointer.Add(gtx.Ops)
		}

		bgColor := v.theme.Colors.CardBg
		if clickable.Hovered() {
			bgColor = v.theme.Colors.ButtonDanger
		}

		rr := gtx.Dp(unit.Dp(4))
		rect := clip.RRect{
			Rect: image.Rectangle{Max: size},
			NE:   rr, NW: rr, SE: rr, SW: rr,
		}
		paint.FillShape(gtx.Ops, bgColor, rect.Op(gtx.Ops))

		// Center the icon
		iconSize := gtx.Dp(unit.Dp(16))
		iconOffset := image.Point{
			X: (size.X - iconSize) / 2,
			Y: (size.Y - iconSize) / 2,
		}
		defer op.Offset(iconOffset).Push(gtx.Ops).Pop()

		iconGtx := gtx
		iconGtx.Constraints.Min = image.Point{X: iconSize, Y: iconSize}
		iconGtx.Constraints.Max = iconGtx.Constraints.Min

		iconColor := v.theme.Colors.TextMuted
		if clickable.Hovered() {
			iconColor = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		}
		icons.ContentClear.Layout(iconGtx, iconColor)

		return layout.Dimensions{Size: size}
	})
}

// collectBuildArgs gathers non-empty build arguments into a map.
func (v *DebugView) collectBuildArgs() map[string]string {
	result := make(map[string]string)
	for _, row := range v.buildArgs {
		key := row.keyEditor.Text()
		value := row.valueEditor.Text()
		if key != "" {
			result[key] = value
		}
	}
	return result
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
		v.resetState()
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
		// Main content area: Dockerfile viewer + Log/Console (horizontal split)
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
				// Log + Console (right side, vertically split)
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						// Log panel (top, ~40%)
						layout.Flexed(0.4, func(gtx layout.Context) layout.Dimensions {
							return v.layoutLogPanel(gtx)
						}),
						// Spacer between log and console
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Dimensions{}
							})
						}),
						// Console (bottom, ~60%)
						layout.Flexed(0.6, func(gtx layout.Context) layout.Dimensions {
							return v.layoutConsole(gtx)
						}),
					)
				}),
			)
		}),
	)
}

// resetState clears all debug view state, preparing for a new session or file.
func (v *DebugView) resetState() {
	v.dockerfileLines = nil
	v.dockerfilePath = ""
	v.dockerfileHighlights = nil
	v.lineClickables = nil
	v.breakpoints = make(map[int]bool)
	v.lastScrolledLine = 0
}

// loadDockerfile reads the Dockerfile content into dockerfileLines.
// It caches the result and only reloads if the path changes.
func (v *DebugView) loadDockerfile(path string) {
	if path == v.dockerfilePath && len(v.dockerfileLines) > 0 {
		return // Already loaded
	}

	v.resetState()
	v.dockerfilePath = path

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

	// Initialize clickables for each line (for breakpoint toggling)
	v.lineClickables = make([]widget.Clickable, len(v.dockerfileLines))
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

						// Get the clickable for this line (for breakpoint toggling)
						var clickable *widget.Clickable
						if index < len(v.lineClickables) {
							clickable = &v.lineClickables[index]
						}

						// Check if this line has a breakpoint
						hasBreakpoint := v.breakpoints[lineNum]

						return v.layoutDockerfileLine(gtx, lineNum, tokens, isInRange, isRangeStart, hasBreakpoint, clickable, markerWidth, lineNumWidth)
					})
				})
			})
		}),
	)
}

// layoutDockerfileLine renders a single line of the Dockerfile with syntax highlighting.
// isInRange indicates if this line is within the current execution range.
// isRangeStart indicates if this is the first line of the range (shows play arrow).
// hasBreakpoint indicates if this line has a breakpoint set.
// clickable is used for click detection on the gutter area to toggle breakpoints.
func (v *DebugView) layoutDockerfileLine(gtx layout.Context, lineNum int, tokens []highlightToken, isInRange, isRangeStart, hasBreakpoint bool, clickable *widget.Clickable, markerWidth, lineNumWidth int) layout.Dimensions {
	lineHeight := gtx.Dp(unit.Dp(22))

	// Handle breakpoint toggle clicks
	if clickable != nil && clickable.Clicked(gtx) {
		// Toggle breakpoint for this line
		if v.breakpoints[lineNum] {
			delete(v.breakpoints, lineNum)
		} else {
			v.breakpoints[lineNum] = true
		}
		// Send updated breakpoints to DAP server
		v.sendBreakpointsToDAP()
	}

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
				// Gutter area (breakpoint dot + play arrow) - clickable for toggling breakpoints
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gutterWidth := markerWidth + lineNumWidth
					gtx.Constraints.Min.X = gutterWidth
					gtx.Constraints.Max.X = gutterWidth
					gtx.Constraints.Min.Y = lineHeight
					gtx.Constraints.Max.Y = lineHeight

					// Wrap gutter in clickable area
					gutterLayout := func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							// Marker area (breakpoint dot and/or play arrow)
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = markerWidth
								gtx.Constraints.Max.X = markerWidth
								gtx.Constraints.Min.Y = lineHeight
								gtx.Constraints.Max.Y = lineHeight

								return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									// Draw breakpoint dot if set
									if hasBreakpoint {
										dotSize := gtx.Dp(unit.Dp(10))
										center := image.Point{X: dotSize / 2, Y: dotSize / 2}
										radius := dotSize / 2

										// Draw filled circle for breakpoint
										circle := clip.Ellipse{
											Min: image.Point{X: center.X - radius, Y: center.Y - radius},
											Max: image.Point{X: center.X + radius, Y: center.Y + radius},
										}
										paint.FillShape(gtx.Ops, v.theme.Colors.Breakpoint, circle.Op(gtx.Ops))
										return layout.Dimensions{Size: image.Point{X: dotSize, Y: dotSize}}
									}

									// Show play arrow for current execution line (only if no breakpoint)
									if isRangeStart {
										iconSize := gtx.Dp(unit.Dp(16))
										gtx.Constraints.Min = image.Point{X: iconSize, Y: iconSize}
										gtx.Constraints.Max = gtx.Constraints.Min
										return icons.AVPlayArrow.Layout(gtx, v.theme.Colors.Primary)
									}

									return layout.Dimensions{Size: image.Point{X: markerWidth, Y: lineHeight}}
								})
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
						)
					}

					// If we have a clickable, wrap the gutter in it
					if clickable != nil {
						return clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							// Show pointer cursor on hover
							if clickable.Hovered() {
								pointer.CursorPointer.Add(gtx.Ops)
							}
							return gutterLayout(gtx)
						})
					}
					return gutterLayout(gtx)
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

// sendBreakpointsToDAP sends the current breakpoints to the DAP server.
func (v *DebugView) sendBreakpointsToDAP() {
	if dap.Client == nil || v.dockerfilePath == "" {
		return
	}

	// Collect all breakpoint line numbers
	lines := make([]int, 0, len(v.breakpoints))
	for lineNum, enabled := range v.breakpoints {
		if enabled {
			lines = append(lines, lineNum)
		}
	}

	// Send to DAP server (fire and forget, errors are logged internally)
	go func() {
		if err := dap.Client.SendSetBreakpoints(v.dockerfilePath, lines); err != nil {
			log.Printf("Failed to send breakpoints: %v", err)
		}
	}()
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

// layoutConsoleHistory renders the console output as a terminal-style view with ANSI colors.
func (v *DebugView) layoutConsoleHistory(gtx layout.Context) layout.Dimensions {
	if dap.Client == nil || dap.Client.Console == nil {
		return layout.Dimensions{}
	}

	numLines := dap.Client.Console.Lines()
	if numLines == 0 {
		// Show placeholder when empty
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Body2(v.theme.Material, "Type a command below...")
			label.Color = v.theme.Colors.TextMuted
			return label.Layout(gtx)
		})
	}

	// Auto-scroll to bottom
	v.consoleList.Position.BeforeEnd = false

	return material.List(v.theme.Material, &v.consoleList).Layout(gtx, numLines, func(gtx layout.Context, index int) layout.Dimensions {
		spans := dap.Client.Console.GetLineSpans(index)
		return v.layoutTerminalLine(gtx, spans)
	})
}

// layoutTerminalLine renders a single line of terminal output with styled spans.
func (v *DebugView) layoutTerminalLine(gtx layout.Context, spans []terminal.StyledSpan) layout.Dimensions {
	if len(spans) == 0 {
		// Empty line - return minimal height
		lineHeight := gtx.Dp(unit.Dp(18))
		return layout.Dimensions{Size: image.Point{Y: lineHeight}}
	}

	// Build layout children for each styled span
	children := make([]layout.FlexChild, len(spans))
	for i, span := range spans {
		span := span // capture for closure
		children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Body2(v.theme.Material, span.Text)
			label.Font.Typeface = v.theme.MonoTypeface // Use monospace font
			label.Color = v.styleToColor(span.Style)
			if span.Style.Bold {
				label.Font.Weight = font.Bold
			}
			if span.Style.Italic {
				label.Font.Style = font.Italic
			}
			return label.Layout(gtx)
		})
	}

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

// styleToColor converts a terminal.Style to a color.NRGBA for rendering.
func (v *DebugView) styleToColor(style terminal.Style) color.NRGBA {
	// Default colors for the terminal
	defaultFg := v.theme.Colors.TextSecondary
	defaultBg := color.NRGBA{A: 0} // Transparent

	fg := style.Fg.ToNRGBA(defaultFg)
	bg := style.Bg.ToNRGBA(defaultBg)

	// Handle reverse video
	if style.Reverse {
		fg, bg = bg, fg
		if bg.A == 0 {
			bg = v.theme.Colors.CardBg
		}
	}

	// Handle dim (reduce brightness)
	if style.Dim {
		fg.R = fg.R / 2
		fg.G = fg.G / 2
		fg.B = fg.B / 2
	}

	// Hidden text
	if style.Hidden {
		fg = bg
	}

	return fg
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
						label.Font.Typeface = v.theme.MonoTypeface
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
						e.Font.Typeface = v.theme.MonoTypeface
						e.Color = v.theme.Colors.Text
						e.HintColor = v.theme.Colors.TextMuted
						return e.Layout(gtx)
					})
				}),
			)
		}),
	)
}

// layoutLogPanel renders the log panel showing DAP events and debug messages.
func (v *DebugView) layoutLogPanel(gtx layout.Context) layout.Dimensions {
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
					// Log header
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							label := material.Body2(v.theme.Material, "Log")
							label.Color = v.theme.Colors.TextMuted
							return label.Layout(gtx)
						})
					}),
					// Log content (takes remaining space)
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return v.layoutLogContent(gtx)
					}),
				)
			})
		}),
	)
}

// layoutLogContent renders the scrollable log content with selectable text.
func (v *DebugView) layoutLogContent(gtx layout.Context) layout.Dimensions {
	if dap.Client == nil {
		return layout.Dimensions{}
	}

	logs := dap.Client.GetLogs()
	if len(logs) == 0 {
		// Show placeholder when empty
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Body2(v.theme.Material, "No log messages yet...")
			label.Color = v.theme.Colors.TextMuted
			return label.Layout(gtx)
		})
	}

	// Build the full log text for selectable display
	logText := ""
	for i, line := range logs {
		if i > 0 {
			logText += "\n"
		}
		logText += line
	}

	// Update editor content if it changed
	currentText := v.logEditor.Text()
	if currentText != logText {
		v.logEditor.SetText(logText)
		// Move cursor to end for auto-scroll effect
		v.logEditor.SetCaret(len(logText), len(logText))
	}

	// Use material.Editor for styled text with selection support
	editor := material.Editor(v.theme.Material, &v.logEditor, "")
	editor.Font.Typeface = v.theme.MonoTypeface
	editor.Color = v.theme.Colors.TextSecondary
	editor.SelectionColor = v.theme.Colors.SelectedBg
	return editor.Layout(gtx)
}
