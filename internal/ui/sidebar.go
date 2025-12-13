package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/tsukinoko-kun/harbor/internal/models"
)

// Sidebar represents the navigation sidebar.
type Sidebar struct {
	theme        *Theme
	onSelect     func(models.View)
	items        []sidebarItem
	settingsItem sidebarItem
	list         widget.List
}

type sidebarItem struct {
	view      models.View
	label     string
	clickable widget.Clickable
}

// NewSidebar creates a new sidebar.
func NewSidebar(theme *Theme, onSelect func(models.View)) *Sidebar {
	return &Sidebar{
		theme:    theme,
		onSelect: onSelect,
		items: []sidebarItem{
			{view: models.ViewContainers, label: "Containers"},
			{view: models.ViewImages, label: "Images"},
			{view: models.ViewVolumes, label: "Volumes"},
			{view: models.ViewNetworks, label: "Networks"},
		},
		settingsItem: sidebarItem{view: models.ViewSettings, label: "Settings"},
		list: widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
	}
}

// Layout renders the sidebar.
func (s *Sidebar) Layout(gtx layout.Context, currentView models.View) layout.Dimensions {
	// Check for clicks on main items
	for i := range s.items {
		if s.items[i].clickable.Clicked(gtx) {
			s.onSelect(s.items[i].view)
		}
	}
	// Check for click on settings
	if s.settingsItem.clickable.Clicked(gtx) {
		s.onSelect(s.settingsItem.view)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Navigation items
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return s.layoutItems(gtx, currentView)
		}),
		// Settings at the bottom
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Left:   unit.Dp(8),
				Right:  unit.Dp(8),
				Bottom: unit.Dp(16),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				isActive := s.settingsItem.view == currentView
				isHovered := s.settingsItem.clickable.Hovered()
				return s.layoutItem(gtx, &s.settingsItem, isActive, isHovered)
			})
		}),
	)
}

func (s *Sidebar) layoutItems(gtx layout.Context, currentView models.View) layout.Dimensions {
	return layout.Inset{
		Top:   unit.Dp(16),
		Left:  unit.Dp(8),
		Right: unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return s.list.Layout(gtx, len(s.items), func(gtx layout.Context, index int) layout.Dimensions {
			item := &s.items[index]
			isActive := item.view == currentView
			isHovered := item.clickable.Hovered()

			return s.layoutItem(gtx, item, isActive, isHovered)
		})
	})
}

func (s *Sidebar) layoutItem(gtx layout.Context, item *sidebarItem, isActive, isHovered bool) layout.Dimensions {
	return item.clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top:    unit.Dp(2),
			Bottom: unit.Dp(2),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// Background
			var bgColor = s.theme.Colors.SidebarBg
			if isActive {
				bgColor = s.theme.Colors.SidebarActive
			} else if isHovered {
				bgColor = s.theme.Colors.SidebarHover
			}

			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					// Rounded rectangle background
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
						Left:   unit.Dp(12),
						Right:  unit.Dp(12),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							// Active indicator
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if isActive {
									size := gtx.Dp(unit.Dp(6))
									circle := clip.Ellipse{
										Min: image.Point{},
										Max: image.Point{X: size, Y: size},
									}
									paint.FillShape(gtx.Ops, s.theme.Colors.Accent, circle.Op(gtx.Ops))
									return layout.Dimensions{Size: image.Point{X: size, Y: size}}
								}
								return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(6)), Y: 0}}
							}),
							// Spacing
							layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
							// Label
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := material.Body1(s.theme.Material, item.label)
								if isActive {
									label.Color = s.theme.Colors.Text
								} else {
									label.Color = s.theme.Colors.TextSecondary
								}
								return label.Layout(gtx)
							}),
						)
					})
				}),
			)
		})
	})
}
