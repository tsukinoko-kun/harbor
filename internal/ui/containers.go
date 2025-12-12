package ui

import (
	"context"
	"image"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/tsukinoko-kun/harbor/internal/config"
	"github.com/tsukinoko-kun/harbor/internal/docker"
	"github.com/tsukinoko-kun/harbor/internal/models"
	"github.com/tsukinoko-kun/harbor/internal/ui/widgets"
)

// containerRowButtons holds the button states for a container row.
type containerRowButtons struct {
	delete     widget.Clickable
	toggle     widget.Clickable
	terminal   widget.Clickable
	logs       widget.Clickable
	processing bool // true when an action is in progress
}

// projectRowButtons holds the button states for a project row.
type projectRowButtons struct {
	delete     widget.Clickable
	toggle     widget.Clickable
	processing bool // true when an action is in progress
}

// ContainersView displays the list of containers grouped by project.
type ContainersView struct {
	theme            *Theme
	docker           *docker.Client
	settings         *config.Settings
	list             widget.List
	containerButtons map[string]*containerRowButtons
	projectButtons   map[string]*projectRowButtons

	// Error notification
	errorMu      sync.RWMutex
	errorMessage string
	errorDismiss widget.Clickable
}

// NewContainersView creates a new containers view.
func NewContainersView(theme *Theme, dockerClient *docker.Client, settings *config.Settings) *ContainersView {
	return &ContainersView{
		theme:            theme,
		docker:           dockerClient,
		settings:         settings,
		list:             widget.List{List: layout.List{Axis: layout.Vertical}},
		containerButtons: make(map[string]*containerRowButtons),
		projectButtons:   make(map[string]*projectRowButtons),
	}
}

// getContainerButtons returns or creates button state for a container.
func (v *ContainersView) getContainerButtons(containerID string) *containerRowButtons {
	if btns, ok := v.containerButtons[containerID]; ok {
		return btns
	}
	btns := &containerRowButtons{}
	v.containerButtons[containerID] = btns
	return btns
}

// getProjectButtons returns or creates button state for a project.
func (v *ContainersView) getProjectButtons(projectName string) *projectRowButtons {
	if btns, ok := v.projectButtons[projectName]; ok {
		return btns
	}
	btns := &projectRowButtons{}
	v.projectButtons[projectName] = btns
	return btns
}

// setError sets an error message to display.
func (v *ContainersView) setError(msg string) {
	v.errorMu.Lock()
	defer v.errorMu.Unlock()
	v.errorMessage = msg
}

// clearError clears the error message.
func (v *ContainersView) clearError() {
	v.errorMu.Lock()
	defer v.errorMu.Unlock()
	v.errorMessage = ""
}

// getError returns the current error message.
func (v *ContainersView) getError() string {
	v.errorMu.RLock()
	defer v.errorMu.RUnlock()
	return v.errorMessage
}

// Layout renders the containers view.
func (v *ContainersView) Layout(gtx layout.Context, groups []docker.ContainerGroup) layout.Dimensions {
	// Handle error dismiss button click
	if v.errorDismiss.Clicked(gtx) {
		v.clearError()
	}

	if len(groups) == 0 {
		return v.layoutEmpty(gtx)
	}

	// Flatten groups into items for the list
	type listItem struct {
		isHeader  bool
		group     docker.ContainerGroup
		container docker.Container
	}

	var items []listItem
	for _, group := range groups {
		// Add group header (only for non-standalone groups)
		if group.Name != "" {
			items = append(items, listItem{isHeader: true, group: group})
		}

		// Add containers
		for _, c := range group.Containers {
			items = append(items, listItem{isHeader: false, container: c})
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Error notification (if any)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutError(gtx)
		}),
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
						return v.layoutGroupHeader(gtx, item.group)
					}
					return v.layoutContainer(gtx, item.container)
				})
			})
		}),
	)
}

func (v *ContainersView) layoutError(gtx layout.Context) layout.Dimensions {
	errMsg := v.getError()
	if errMsg == "" {
		return layout.Dimensions{}
	}

	return layout.Inset{
		Top:   unit.Dp(8),
		Left:  unit.Dp(16),
		Right: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				rr := gtx.Dp(unit.Dp(6))
				rect := clip.RRect{
					Rect: image.Rectangle{Max: gtx.Constraints.Min},
					NE:   rr, NW: rr, SE: rr, SW: rr,
				}
				paint.FillShape(gtx.Ops, v.theme.Colors.ErrorBg, rect.Op(gtx.Ops))
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
						// Error message
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							label := material.Body2(v.theme.Material, errMsg)
							label.Color = v.theme.Colors.ErrorText
							return label.Layout(gtx)
						}),
						// Dismiss button
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return v.errorDismiss.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								label := material.Body2(v.theme.Material, "✕")
								label.Color = v.theme.Colors.ErrorText
								return label.Layout(gtx)
							})
						}),
					)
				})
			}),
		)
	})
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

func (v *ContainersView) layoutGroupHeader(gtx layout.Context, group docker.ContainerGroup) layout.Dimensions {
	btns := v.getProjectButtons(group.Name)

	// Handle button clicks (only if not processing)
	if !btns.processing {
		if btns.toggle.Clicked(gtx) {
			btns.processing = true
			projectName := group.Name
			if isGroupRunning(group) {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					_ = v.docker.StopProject(ctx, projectName)
					btns.processing = false
				}()
			} else {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					_ = v.docker.StartProject(ctx, projectName)
					btns.processing = false
				}()
			}
		}
		if btns.delete.Clicked(gtx) {
			btns.processing = true
			projectName := group.Name
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_ = v.docker.RemoveProject(ctx, projectName)
				btns.processing = false
			}()
		}
	}

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
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						// Project name
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							label := material.Body2(v.theme.Material, group.Name)
							label.Color = v.theme.Colors.TextSecondary
							return label.Layout(gtx)
						}),
						// Start/Stop button
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return v.layoutButton(gtx, &btns.toggle, v.toggleLabel(isGroupRunning(group)), false, btns.processing)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						// Delete button
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return v.layoutButton(gtx, &btns.delete, "Delete", true, btns.processing)
						}),
					)
				})
			}),
		)
	})
}

// isGroupRunning returns true if any container in the group is running.
func isGroupRunning(group docker.ContainerGroup) bool {
	for _, c := range group.Containers {
		if c.State == "running" {
			return true
		}
	}
	return false
}

func (v *ContainersView) layoutContainer(gtx layout.Context, c docker.Container) layout.Dimensions {
	btns := v.getContainerButtons(c.ID)
	isRunning := c.State == "running"

	// Handle button clicks (only if not processing)
	if !btns.processing {
		if btns.toggle.Clicked(gtx) {
			btns.processing = true
			containerID := c.ID
			if isRunning {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					_ = v.docker.StopContainer(ctx, containerID)
					btns.processing = false
				}()
			} else {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					_ = v.docker.StartContainer(ctx, containerID)
					btns.processing = false
				}()
			}
		}
		if btns.delete.Clicked(gtx) {
			btns.processing = true
			containerID := c.ID
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_ = v.docker.RemoveContainer(ctx, containerID)
				btns.processing = false
			}()
		}
		if btns.terminal.Clicked(gtx) {
			btns.processing = true
			containerID := c.ID
			terminal := v.settings.GetSelectedTerminal()
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := v.docker.OpenTerminal(ctx, containerID, terminal); err != nil {
					v.setError("Terminal error: " + err.Error())
				}
				btns.processing = false
			}()
		}
		if btns.logs.Clicked(gtx) {
			containerID := c.ID
			containerName := c.Name
			NewLogsWindow(v.theme, v.docker, containerID, containerName)
		}
	}

	return layout.Inset{
		Top:    unit.Dp(4),
		Bottom: unit.Dp(4),
		Left:   unit.Dp(12),
		Right:  unit.Dp(12),
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
			// Buttons (right-aligned): Logs, Terminal, Start/Stop, Delete
			// Logs button (available for all containers)
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.layoutButton(gtx, &btns.logs, "Logs", false, false)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			// Terminal button (only shown when running)
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !isRunning {
					return layout.Dimensions{}
				}
				return v.layoutButton(gtx, &btns.terminal, "Terminal", false, btns.processing)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !isRunning {
					return layout.Dimensions{}
				}
				return layout.Spacer{Width: unit.Dp(8)}.Layout(gtx)
			}),
			// Start/Stop button
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.layoutButton(gtx, &btns.toggle, v.toggleLabel(isRunning), false, btns.processing)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			// Delete button
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.layoutButton(gtx, &btns.delete, "Delete", true, btns.processing)
			}),
		)
	})
}

// toggleLabel returns the appropriate label for a start/stop button.
func (v *ContainersView) toggleLabel(isRunning bool) string {
	if isRunning {
		return "Stop"
	}
	return "Start"
}

// layoutButton renders a small action button.
func (v *ContainersView) layoutButton(gtx layout.Context, clickable *widget.Clickable, label string, isDanger bool, disabled bool) layout.Dimensions {
	// When disabled, don't process clicks
	if disabled {
		return v.layoutButtonContent(gtx, label, isDanger, disabled, false)
	}

	return clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return v.layoutButtonContent(gtx, label, isDanger, disabled, clickable.Hovered())
	})
}

// layoutButtonContent renders the button content with appropriate styling.
func (v *ContainersView) layoutButtonContent(gtx layout.Context, label string, isDanger bool, disabled bool, hovered bool) layout.Dimensions {
	// Determine colors
	bgColor := v.theme.Colors.ButtonBg
	textColor := v.theme.Colors.Text

	if disabled {
		textColor = v.theme.Colors.TextMuted
	} else if hovered {
		if isDanger {
			bgColor = v.theme.Colors.ButtonDangerHov
		} else {
			bgColor = v.theme.Colors.ButtonHover
		}
	} else if isDanger {
		bgColor = v.theme.Colors.ButtonDanger
	}

	// Show "..." when disabled (processing)
	displayLabel := label
	if disabled {
		displayLabel = label + "..."
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			rr := gtx.Dp(unit.Dp(4))
			rect := clip.RRect{
				Rect: image.Rectangle{Max: gtx.Constraints.Min},
				NE:   rr, NW: rr, SE: rr, SW: rr,
			}
			paint.FillShape(gtx.Ops, bgColor, rect.Op(gtx.Ops))
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(4),
				Bottom: unit.Dp(4),
				Left:   unit.Dp(8),
				Right:  unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(v.theme.Material, displayLabel)
				lbl.Color = textColor
				return lbl.Layout(gtx)
			})
		}),
	)
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
