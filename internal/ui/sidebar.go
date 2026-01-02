package ui

import (
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"strings"
	"time"

	"gio.tools/icons"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/Masterminds/semver/v3"
	"github.com/tsukinoko-kun/harbor/internal/env"
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
	icon      *widget.Icon
	clickable widget.Clickable
}

func checkBuildxDAPSupport() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use env.GetDockerPath() to find docker even when launched from Finder on macOS
	// where PATH may not include the docker binary location
	dockerPath := env.GetDockerPath()
	cmd := exec.CommandContext(ctx, dockerPath, "buildx", "version")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("docker at %q: %w", dockerPath, err)
	}

	versionStr := string(out)
	versionStrParts := strings.Split(versionStr, " ")
	minVersion, err := semver.NewConstraint(">=0.29.0")
	if err != nil {
		panic(err)
	}
	for _, part := range versionStrParts {
		if v, ok := strings.CutPrefix(part, "v"); ok {
			sv, err := semver.NewVersion(v)
			if err != nil {
				continue
			}
			if minVersion.Check(sv) {
				return true, nil
			}
		} else {
			sv, err := semver.NewVersion(part)
			if err != nil {
				continue
			}
			if minVersion.Check(sv) {
				return true, nil
			}
		}
	}
	return false, nil
}

// NewSidebar creates a new sidebar.
func NewSidebar(theme *Theme, onSelect func(models.View)) *Sidebar {
	items := []sidebarItem{
		{view: models.ViewContainers, icon: icons.ActionViewModule},
		{view: models.ViewImages, icon: icons.ImageImage},
		{view: models.ViewVolumes, icon: icons.DeviceStorage},
		{view: models.ViewNetworks, icon: icons.ActionSettingsEthernet},
	}
	if ok, err := checkBuildxDAPSupport(); ok {
		items = append(items, sidebarItem{view: models.ViewDebug, icon: icons.ActionBugReport})
	} else {
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error checking buildx support: %v\n", err)
		}
		_, _ = fmt.Fprintf(os.Stderr, "Warning: debugger is not available because buildx >= 0.29.0 is required\n")
	}
	return &Sidebar{
		theme:        theme,
		onSelect:     onSelect,
		items:        items,
		settingsItem: sidebarItem{view: models.ViewSettings, icon: icons.ActionSettings},
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
		if isHovered {
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return layout.Inset{
			Top:    unit.Dp(2),
			Bottom: unit.Dp(2),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// Background: selected = dark indigo (no hover change), hover = gray, default = sidebar bg
			var bgColor = s.theme.Colors.SidebarBg
			if isActive {
				bgColor = s.theme.Colors.SidebarSelectedBg
			} else if isHovered {
				bgColor = s.theme.Colors.SidebarHover
			}

			// Icon color: indigo when selected, gray otherwise
			iconColor := s.theme.Colors.SidebarIconDefault
			if isActive {
				iconColor = s.theme.Colors.SidebarIconActive
			}

			return layout.Stack{Alignment: layout.Center}.Layout(gtx,
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
						Left:   unit.Dp(10),
						Right:  unit.Dp(10),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						// Render centered icon
						iconSize := gtx.Dp(unit.Dp(24))
						gtx.Constraints.Min = image.Point{X: iconSize, Y: iconSize}
						gtx.Constraints.Max = gtx.Constraints.Min
						return item.icon.Layout(gtx, iconColor)
					})
				}),
			)
		})
	})
}
