package widgets

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// ListItem represents a clickable list item.
type ListItem struct {
	Clickable   *widget.Clickable
	Hovered     bool
	HoverColor  color.NRGBA
	NormalColor color.NRGBA
}

// NewListItem creates a new list item.
func NewListItem(clickable *widget.Clickable, normal, hover color.NRGBA) ListItem {
	return ListItem{
		Clickable:   clickable,
		HoverColor:  hover,
		NormalColor: normal,
	}
}

// Layout renders the list item with the given content.
func (li *ListItem) Layout(gtx layout.Context, content layout.Widget) layout.Dimensions {
	// Check hover state
	li.Hovered = li.Clickable.Hovered()

	return li.Clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				// Draw background
				bgColor := li.NormalColor
				if li.Hovered {
					bgColor = li.HoverColor
				}

				rect := clip.Rect{Max: gtx.Constraints.Min}
				paint.FillShape(gtx.Ops, bgColor, rect.Op())

				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(content),
		)
	})
}

// ScrollableList wraps a widget.List for scrollable content.
type ScrollableList struct {
	List        widget.List
	ScrollbarBg color.NRGBA
}

// NewScrollableList creates a new scrollable list.
func NewScrollableList() *ScrollableList {
	return &ScrollableList{
		List: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		ScrollbarBg: color.NRGBA{R: 60, G: 60, B: 60, A: 255},
	}
}

// Layout renders the scrollable list with the given elements.
func (sl *ScrollableList) Layout(gtx layout.Context, count int, element func(gtx layout.Context, index int) layout.Dimensions) layout.Dimensions {
	return sl.List.Layout(gtx, count, element)
}

// Divider renders a horizontal divider line.
func Divider(gtx layout.Context, c color.NRGBA, thickness unit.Dp) layout.Dimensions {
	height := gtx.Dp(thickness)

	rect := clip.Rect{
		Max: image.Point{X: gtx.Constraints.Max.X, Y: height},
	}
	paint.FillShape(gtx.Ops, c, rect.Op())

	return layout.Dimensions{
		Size: image.Point{X: gtx.Constraints.Max.X, Y: height},
	}
}
