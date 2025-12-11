package widgets

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/tsukinoko-kun/harbor/internal/models"
)

// StatusIndicator displays a colored circle indicating container state.
type StatusIndicator struct {
	State  models.ContainerState
	Size   unit.Dp
	Colors StatusColors
}

// StatusColors defines the colors for each container state.
type StatusColors struct {
	Running    color.NRGBA
	Stopped    color.NRGBA
	Paused     color.NRGBA
	Restarting color.NRGBA
	Created    color.NRGBA
	Removing   color.NRGBA
	Unknown    color.NRGBA
}

// DefaultStatusColors returns the default status indicator colors.
func DefaultStatusColors() StatusColors {
	return StatusColors{
		Running:    color.NRGBA{R: 74, G: 222, B: 128, A: 255},  // Green
		Stopped:    color.NRGBA{R: 248, G: 113, B: 113, A: 255}, // Red
		Paused:     color.NRGBA{R: 251, G: 191, B: 36, A: 255},  // Yellow
		Restarting: color.NRGBA{R: 251, G: 191, B: 36, A: 255},  // Yellow
		Created:    color.NRGBA{R: 156, G: 163, B: 175, A: 255}, // Gray
		Removing:   color.NRGBA{R: 156, G: 163, B: 175, A: 255}, // Gray
		Unknown:    color.NRGBA{R: 156, G: 163, B: 175, A: 255}, // Gray
	}
}

// NewStatusIndicator creates a new status indicator with default colors.
func NewStatusIndicator(state models.ContainerState) StatusIndicator {
	return StatusIndicator{
		State:  state,
		Size:   unit.Dp(8),
		Colors: DefaultStatusColors(),
	}
}

// Layout renders the status indicator.
func (s StatusIndicator) Layout(gtx layout.Context) layout.Dimensions {
	size := gtx.Dp(s.Size)

	// Get color based on state
	c := s.colorForState()

	// Draw filled circle
	defer clip.Ellipse{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: size, Y: size},
	}.Op(gtx.Ops).Push(gtx.Ops).Pop()

	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	return layout.Dimensions{
		Size: image.Point{X: size, Y: size},
	}
}

func (s StatusIndicator) colorForState() color.NRGBA {
	switch s.State {
	case models.StateRunning:
		return s.Colors.Running
	case models.StateStopped:
		return s.Colors.Stopped
	case models.StatePaused:
		return s.Colors.Paused
	case models.StateRestarting:
		return s.Colors.Restarting
	case models.StateCreated:
		return s.Colors.Created
	case models.StateRemoving:
		return s.Colors.Removing
	default:
		return s.Colors.Unknown
	}
}
