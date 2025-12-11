package ui

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/tsukinoko-kun/harbor/internal/docker"
)

// ImagesView displays the list of Docker images.
type ImagesView struct {
	theme *Theme
	list  widget.List
}

// NewImagesView creates a new images view.
func NewImagesView(theme *Theme) *ImagesView {
	return &ImagesView{
		theme: theme,
		list: widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
	}
}

// Layout renders the images view.
func (v *ImagesView) Layout(gtx layout.Context, images []docker.Image) layout.Dimensions {
	if len(images) == 0 {
		return v.layoutEmpty(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutHeader(gtx, len(images))
		}),
		// Image list
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Left:  unit.Dp(16),
				Right: unit.Dp(16),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.list.Layout(gtx, len(images), func(gtx layout.Context, index int) layout.Dimensions {
					return v.layoutImage(gtx, images[index])
				})
			})
		}),
	)
}

func (v *ImagesView) layoutHeader(gtx layout.Context, count int) layout.Dimensions {
	return layout.Inset{
		Top:    unit.Dp(20),
		Bottom: unit.Dp(16),
		Left:   unit.Dp(16),
		Right:  unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.H5(v.theme.Material, "Images")
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

func (v *ImagesView) layoutImage(gtx layout.Context, img docker.Image) layout.Dimensions {
	return layout.Inset{
		Top:    unit.Dp(8),
		Bottom: unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			// Image info
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// Tags
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						tagStr := strings.Join(img.Tags, ", ")
						label := material.Body1(v.theme.Material, tagStr)
						label.Color = v.theme.Colors.Text
						return label.Layout(gtx)
					}),
					// ID and size
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := material.Caption(v.theme.Material, img.ID)
								label.Color = v.theme.Colors.TextMuted
								return label.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								sizeStr := docker.FormatSize(img.Size)
								label := material.Caption(v.theme.Material, sizeStr)
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

func (v *ImagesView) layoutEmpty(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Body1(v.theme.Material, "No images found")
		label.Color = v.theme.Colors.TextMuted
		return label.Layout(gtx)
	})
}
