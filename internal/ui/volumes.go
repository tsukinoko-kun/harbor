package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/tsukinoko-kun/harbor/internal/docker"
)

// VolumesView displays the list of Docker volumes.
type VolumesView struct {
	theme *Theme
	list  widget.List
}

// NewVolumesView creates a new volumes view.
func NewVolumesView(theme *Theme) *VolumesView {
	return &VolumesView{
		theme: theme,
		list: widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
	}
}

// Layout renders the volumes view.
func (v *VolumesView) Layout(gtx layout.Context, volumes []docker.Volume) layout.Dimensions {
	if len(volumes) == 0 {
		return v.layoutEmpty(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutHeader(gtx, len(volumes))
		}),
		// Volume list
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Left:  unit.Dp(16),
				Right: unit.Dp(16),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.list.Layout(gtx, len(volumes), func(gtx layout.Context, index int) layout.Dimensions {
					return v.layoutVolume(gtx, volumes[index])
				})
			})
		}),
	)
}

func (v *VolumesView) layoutHeader(gtx layout.Context, count int) layout.Dimensions {
	return layout.Inset{
		Top:    unit.Dp(20),
		Bottom: unit.Dp(16),
		Left:   unit.Dp(16),
		Right:  unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.H5(v.theme.Material, "Volumes")
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

func (v *VolumesView) layoutVolume(gtx layout.Context, vol docker.Volume) layout.Dimensions {
	return layout.Inset{
		Top:    unit.Dp(8),
		Bottom: unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Name
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Body1(v.theme.Material, vol.Name)
				label.Color = v.theme.Colors.Text
				return label.Layout(gtx)
			}),
			// Driver and mountpoint
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Caption(v.theme.Material, "Driver: "+vol.Driver)
						label.Color = v.theme.Colors.TextMuted
						return label.Layout(gtx)
					}),
				)
			}),
			// Mountpoint (truncated)
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				mountpoint := vol.Mountpoint
				if len(mountpoint) > 60 {
					mountpoint = "..." + mountpoint[len(mountpoint)-57:]
				}
				label := material.Caption(v.theme.Material, mountpoint)
				label.Color = v.theme.Colors.TextMuted
				return label.Layout(gtx)
			}),
		)
	})
}

func (v *VolumesView) layoutEmpty(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Body1(v.theme.Material, "No volumes found")
		label.Color = v.theme.Colors.TextMuted
		return label.Layout(gtx)
	})
}
