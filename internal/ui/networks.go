package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/tsukinoko-kun/harbor/internal/docker"
)

// NetworksView displays the list of Docker networks.
type NetworksView struct {
	theme *Theme
	list  widget.List
}

// NewNetworksView creates a new networks view.
func NewNetworksView(theme *Theme) *NetworksView {
	return &NetworksView{
		theme: theme,
		list: widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
	}
}

// Layout renders the networks view.
func (v *NetworksView) Layout(gtx layout.Context, networks []docker.Network) layout.Dimensions {
	if len(networks) == 0 {
		return v.layoutEmpty(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutHeader(gtx, len(networks))
		}),
		// Network list
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Left:  unit.Dp(16),
				Right: unit.Dp(16),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.list.Layout(gtx, len(networks), func(gtx layout.Context, index int) layout.Dimensions {
					return v.layoutNetwork(gtx, networks[index])
				})
			})
		}),
	)
}

func (v *NetworksView) layoutHeader(gtx layout.Context, count int) layout.Dimensions {
	return layout.Inset{
		Top:    unit.Dp(20),
		Bottom: unit.Dp(16),
		Left:   unit.Dp(16),
		Right:  unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.H5(v.theme.Material, "Networks")
				title.Color = v.theme.Colors.Text
				return title.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Body2(v.theme.Material, intToStr(count))
				label.Color = v.theme.Colors.TextMuted
				return label.Layout(gtx)
			}),
		)
	})
}

func (v *NetworksView) layoutNetwork(gtx layout.Context, net docker.Network) layout.Dimensions {
	return layout.Inset{
		Top:    unit.Dp(8),
		Bottom: unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Name
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Body1(v.theme.Material, net.Name)
				label.Color = v.theme.Colors.Text
				return label.Layout(gtx)
			}),
			// Driver and scope
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Caption(v.theme.Material, net.ID)
						label.Color = v.theme.Colors.TextMuted
						return label.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Caption(v.theme.Material, net.Driver)
						label.Color = v.theme.Colors.TextMuted
						return label.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Caption(v.theme.Material, net.Scope)
						label.Color = v.theme.Colors.TextMuted
						return label.Layout(gtx)
					}),
				)
			}),
		)
	})
}

func (v *NetworksView) layoutEmpty(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Body1(v.theme.Material, "No networks found")
		label.Color = v.theme.Colors.TextMuted
		return label.Layout(gtx)
	})
}
