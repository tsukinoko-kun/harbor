package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/tsukinoko-kun/harbor/internal/config"
)

// SettingsView displays the application settings.
type SettingsView struct {
	theme           *Theme
	settings        *config.Settings
	list            widget.List
	terminalButtons []widget.Clickable
}

// NewSettingsView creates a new settings view.
func NewSettingsView(theme *Theme, settings *config.Settings) *SettingsView {
	return &SettingsView{
		theme:    theme,
		settings: settings,
		list: widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		terminalButtons: make([]widget.Clickable, len(settings.Terminals)),
	}
}

// Layout renders the settings view.
func (v *SettingsView) Layout(gtx layout.Context) layout.Dimensions {
	// Handle terminal selection clicks
	for i := range v.settings.Terminals {
		if i < len(v.terminalButtons) && v.terminalButtons[i].Clicked(gtx) {
			v.settings.SelectedTerminal = v.settings.Terminals[i].Name
			// Save settings asynchronously
			go func() {
				_ = v.settings.Save()
			}()
		}
	}

	// Ensure we have enough buttons for terminals
	if len(v.terminalButtons) < len(v.settings.Terminals) {
		v.terminalButtons = make([]widget.Clickable, len(v.settings.Terminals))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutHeader(gtx)
		}),
		// Settings content
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Left:  unit.Dp(16),
				Right: unit.Dp(16),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutContent(gtx)
			})
		}),
	)
}

func (v *SettingsView) layoutHeader(gtx layout.Context) layout.Dimensions {
	return layout.Inset{
		Top:    unit.Dp(20),
		Bottom: unit.Dp(16),
		Left:   unit.Dp(16),
		Right:  unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		title := material.H5(v.theme.Material, "Settings")
		title.Color = v.theme.Colors.Text
		return title.Layout(gtx)
	})
}

func (v *SettingsView) layoutContent(gtx layout.Context) layout.Dimensions {
	return v.list.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return v.layoutTerminalSection(gtx)
	})
}

func (v *SettingsView) layoutTerminalSection(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Section header
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.H6(v.theme.Material, "Terminal")
				label.Color = v.theme.Colors.Text
				return label.Layout(gtx)
			})
		}),
		// Description
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Body2(v.theme.Material, "Select the terminal emulator to use when opening container shells.")
				label.Color = v.theme.Colors.TextMuted
				return label.Layout(gtx)
			})
		}),
		// Terminal options (always has at least the clipboard option)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				v.layoutTerminalOptions(gtx)...,
			)
		}),
	)
}

func (v *SettingsView) layoutTerminalOptions(gtx layout.Context) []layout.FlexChild {
	children := make([]layout.FlexChild, len(v.settings.Terminals))

	for i, terminal := range v.settings.Terminals {
		idx := i
		term := terminal
		isSelected := term.Name == v.settings.SelectedTerminal

		children[idx] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutTerminalOption(gtx, &v.terminalButtons[idx], term, isSelected)
		})
	}

	return children
}

func (v *SettingsView) layoutTerminalOption(gtx layout.Context, clickable *widget.Clickable, terminal config.Terminal, isSelected bool) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					bgColor := v.theme.Colors.CardBg
					if isSelected {
						bgColor = v.theme.Colors.SelectedBg
					} else if clickable.Hovered() {
						bgColor = v.theme.Colors.ButtonHover
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
						Left:   unit.Dp(16),
						Right:  unit.Dp(16),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							// Radio indicator
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return v.layoutRadio(gtx, isSelected)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
							// Terminal info
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									// Terminal name
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										label := material.Body1(v.theme.Material, terminal.Name)
										label.Color = v.theme.Colors.Text
										return label.Layout(gtx)
									}),
									// Terminal path or description
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										description := terminal.Path
										if terminal.IsCopyToClipboard() {
											description = "Copies command to clipboard"
										}
										label := material.Caption(v.theme.Material, description)
										label.Color = v.theme.Colors.TextMuted
										return label.Layout(gtx)
									}),
								)
							}),
						)
					})
				}),
			)
		})
	})
}

func (v *SettingsView) layoutRadio(gtx layout.Context, isSelected bool) layout.Dimensions {
	size := gtx.Dp(unit.Dp(18))
	outerRadius := size / 2
	innerRadius := size / 4

	// Draw outer circle
	outerColor := v.theme.Colors.TextMuted
	if isSelected {
		outerColor = v.theme.Colors.Primary
	}

	// Outer circle
	outerRect := clip.Ellipse{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: size, Y: size},
	}
	paint.FillShape(gtx.Ops, outerColor, outerRect.Op(gtx.Ops))

	// Inner background (to create ring effect)
	innerBgSize := size - gtx.Dp(unit.Dp(2))*2
	innerBgOffset := gtx.Dp(unit.Dp(2))
	innerBgRect := clip.Ellipse{
		Min: image.Point{X: innerBgOffset, Y: innerBgOffset},
		Max: image.Point{X: innerBgOffset + innerBgSize, Y: innerBgOffset + innerBgSize},
	}
	paint.FillShape(gtx.Ops, v.theme.Colors.CardBg, innerBgRect.Op(gtx.Ops))

	// Inner filled circle when selected
	if isSelected {
		innerOffset := outerRadius - innerRadius
		innerRect := clip.Ellipse{
			Min: image.Point{X: innerOffset, Y: innerOffset},
			Max: image.Point{X: innerOffset + innerRadius*2, Y: innerOffset + innerRadius*2},
		}
		paint.FillShape(gtx.Ops, v.theme.Colors.Primary, innerRect.Op(gtx.Ops))
	}

	return layout.Dimensions{Size: image.Point{X: size, Y: size}}
}

