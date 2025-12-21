package ui

import (
	"image"
	"image/color"
	"log"
	"path/filepath"

	"gio.tools/icons"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/sqweek/dialog"

	"github.com/tsukinoko-kun/harbor/internal/dap"
)

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
	stepOverButton  widget.Clickable
	continueButton  widget.Clickable
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
	// Handle button clicks
	if v.stepOverButton.Clicked(gtx) {
		if err := dap.Client.SendNext(); err != nil {
			log.Printf("Failed to step over: %v", err)
		}
	}
	if v.continueButton.Clicked(gtx) {
		if err := dap.Client.SendContinue(); err != nil {
			log.Printf("Failed to continue: %v", err)
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Status message
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				var statusText string
				if dap.Client.Stopped {
					statusText = "Debugger paused"
				} else {
					statusText = "Debugger running..."
				}
				label := material.Body1(v.theme.Material, statusText)
				label.Color = v.theme.Colors.Text
				return label.Layout(gtx)
			})
		}),
		// Dockerfile info
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				params := dap.Client.Params
				label := material.Body2(v.theme.Material, "Dockerfile: "+params.Dockerfile)
				label.Color = v.theme.Colors.TextMuted
				return label.Layout(gtx)
			})
		}),
		// Context info
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				params := dap.Client.Params
				label := material.Body2(v.theme.Material, "Context: "+params.Context)
				label.Color = v.theme.Colors.TextMuted
				return label.Layout(gtx)
			})
		}),
		// Debug control buttons
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutDebugButtons(gtx)
			})
		}),
	)
}

// layoutDebugButtons renders the step over and continue buttons
func (v *DebugView) layoutDebugButtons(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceStart}.Layout(gtx,
		// Step Over button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutDebugButton(gtx, &v.stepOverButton, "Step Over", dap.Client.Stopped)
		}),
		// Spacer
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{}
			})
		}),
		// Continue button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutDebugButton(gtx, &v.continueButton, "Continue", dap.Client.Stopped)
		}),
	)
}

// layoutDebugButton renders a single debug control button
func (v *DebugView) layoutDebugButton(gtx layout.Context, clickable *widget.Clickable, text string, enabled bool) layout.Dimensions {
	return clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if enabled && clickable.Hovered() {
			pointer.CursorPointer.Add(gtx.Ops)
		}

		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				var bgColor color.NRGBA
				if !enabled {
					// Disabled state - muted color
					bgColor = v.theme.Colors.CardBg
				} else if clickable.Hovered() {
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
					Top:    unit.Dp(10),
					Bottom: unit.Dp(10),
					Left:   unit.Dp(16),
					Right:  unit.Dp(16),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(v.theme.Material, text)
					if enabled {
						label.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
					} else {
						label.Color = v.theme.Colors.TextMuted
					}
					return label.Layout(gtx)
				})
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
