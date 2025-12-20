package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// DebugView displays debug information.
type DebugView struct {
	theme *Theme
}

// NewDebugView creates a new debug view.
func NewDebugView(theme *Theme) *DebugView {
	return &DebugView{
		theme: theme,
	}
}

// Layout renders the debug view.
func (v *DebugView) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutHeader(gtx)
		}),
		// Content
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Left:  unit.Dp(16),
				Right: unit.Dp(16),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Body1(v.theme.Material, "debug")
				label.Color = v.theme.Colors.Text
				return label.Layout(gtx)
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
