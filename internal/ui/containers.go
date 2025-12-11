package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/tsukinoko-kun/harbor/internal/docker"
	"github.com/tsukinoko-kun/harbor/internal/models"
	"github.com/tsukinoko-kun/harbor/internal/ui/widgets"
)

// ContainersView displays the list of containers grouped by project.
type ContainersView struct {
	theme *Theme
	list  widget.List
}

// NewContainersView creates a new containers view.
func NewContainersView(theme *Theme) *ContainersView {
	return &ContainersView{
		theme: theme,
		list: widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
	}
}

// Layout renders the containers view.
func (v *ContainersView) Layout(gtx layout.Context, groups []docker.ContainerGroup) layout.Dimensions {
	if len(groups) == 0 {
		return v.layoutEmpty(gtx)
	}

	// Flatten groups into items for the list
	type listItem struct {
		isHeader  bool
		groupName string
		container docker.Container
	}

	var items []listItem
	for _, group := range groups {
		// Add group header
		groupName := group.Name
		if groupName == "" {
			groupName = "Standalone"
		}
		items = append(items, listItem{isHeader: true, groupName: groupName})

		// Add containers
		for _, c := range group.Containers {
			items = append(items, listItem{isHeader: false, container: c})
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutHeader(gtx, countContainers(groups))
		}),
		// Container list
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Left:  unit.Dp(16),
				Right: unit.Dp(16),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.list.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					item := items[index]
					if item.isHeader {
						return v.layoutGroupHeader(gtx, item.groupName)
					}
					return v.layoutContainer(gtx, item.container)
				})
			})
		}),
	)
}

func (v *ContainersView) layoutHeader(gtx layout.Context, count int) layout.Dimensions {
	return layout.Inset{
		Top:    unit.Dp(20),
		Bottom: unit.Dp(16),
		Left:   unit.Dp(16),
		Right:  unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.H5(v.theme.Material, "Containers")
				title.Color = v.theme.Colors.Text
				return title.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				countStr := intToStr(count)
				label := material.Body2(v.theme.Material, countStr)
				label.Color = v.theme.Colors.TextMuted
				return label.Layout(gtx)
			}),
		)
	})
}

func (v *ContainersView) layoutGroupHeader(gtx layout.Context, name string) layout.Dimensions {
	return layout.Inset{
		Top:    unit.Dp(16),
		Bottom: unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// Background
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				rr := gtx.Dp(unit.Dp(4))
				rect := clip.RRect{
					Rect: image.Rectangle{Max: gtx.Constraints.Min},
					NE:   rr, NW: rr, SE: rr, SW: rr,
				}
				paint.FillShape(gtx.Ops, v.theme.Colors.GroupHeader, rect.Op(gtx.Ops))
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top:    unit.Dp(8),
					Bottom: unit.Dp(8),
					Left:   unit.Dp(12),
					Right:  unit.Dp(12),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(v.theme.Material, name)
					label.Color = v.theme.Colors.TextSecondary
					return label.Layout(gtx)
				})
			}),
		)
	})
}

func (v *ContainersView) layoutContainer(gtx layout.Context, c docker.Container) layout.Dimensions {
	return layout.Inset{
		Top:    unit.Dp(4),
		Bottom: unit.Dp(4),
		Left:   unit.Dp(12),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			// Status indicator
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				state := models.ParseContainerState(c.State)
				indicator := widgets.NewStatusIndicator(state)
				return indicator.Layout(gtx)
			}),
			// Spacing
			layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
			// Container info
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// Name
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Body1(v.theme.Material, c.Name)
						label.Color = v.theme.Colors.Text
						return label.Layout(gtx)
					}),
					// Image and status
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := material.Caption(v.theme.Material, c.Image)
								label.Color = v.theme.Colors.TextMuted
								return label.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := material.Caption(v.theme.Material, "• "+c.Status)
								label.Color = v.theme.Colors.TextMuted
								return label.Layout(gtx)
							}),
						)
					}),
				)
			}),
		)
	})
}

func (v *ContainersView) layoutEmpty(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Body1(v.theme.Material, "No containers found")
		label.Color = v.theme.Colors.TextMuted
		return label.Layout(gtx)
	})
}

func countContainers(groups []docker.ContainerGroup) int {
	count := 0
	for _, g := range groups {
		count += len(g.Containers)
	}
	return count
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
