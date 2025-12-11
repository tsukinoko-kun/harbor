package ui

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/widget/material"
)

// Colors defines the color palette for the application.
type Colors struct {
	Background      color.NRGBA
	Surface         color.NRGBA
	SidebarBg       color.NRGBA
	SidebarActive   color.NRGBA
	SidebarHover    color.NRGBA
	Text            color.NRGBA
	TextSecondary   color.NRGBA
	TextMuted       color.NRGBA
	Border          color.NRGBA
	StatusRunning   color.NRGBA
	StatusStopped   color.NRGBA
	StatusPaused    color.NRGBA
	StatusCreated   color.NRGBA
	Accent          color.NRGBA
	GroupHeader     color.NRGBA
	ButtonBg        color.NRGBA
	ButtonHover     color.NRGBA
	ButtonDanger    color.NRGBA
	ButtonDangerHov color.NRGBA
}

// DefaultColors returns the default dark color palette.
func DefaultColors() Colors {
	return Colors{
		Background:      rgb(0x1a1a1a),
		Surface:         rgb(0x242424),
		SidebarBg:       rgb(0x1e1e1e),
		SidebarActive:   rgb(0x2d2d2d),
		SidebarHover:    rgb(0x333333),
		Text:            rgb(0xffffff),
		TextSecondary:   rgb(0xb0b0b0),
		TextMuted:       rgb(0x707070),
		Border:          rgb(0x3d3d3d),
		StatusRunning:   rgb(0x4ade80), // Green
		StatusStopped:   rgb(0xf87171), // Red
		StatusPaused:    rgb(0xfbbf24), // Yellow
		StatusCreated:   rgb(0x9ca3af), // Gray
		Accent:          rgb(0x60a5fa), // Blue
		GroupHeader:     rgb(0x2a2a2a),
		ButtonBg:        rgb(0x3d3d3d),
		ButtonHover:     rgb(0x4d4d4d),
		ButtonDanger:    rgb(0xdc2626), // Red
		ButtonDangerHov: rgb(0xef4444), // Lighter red
	}
}

// Theme holds the application theme including colors and material theme.
type Theme struct {
	Material *material.Theme
	Colors   Colors
}

// NewTheme creates a new application theme.
func NewTheme() *Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(defaultFonts()))

	colors := DefaultColors()

	// Configure material theme colors
	th.Bg = colors.Background
	th.Fg = colors.Text
	th.ContrastBg = colors.Accent
	th.ContrastFg = colors.Text

	return &Theme{
		Material: th,
		Colors:   colors,
	}
}

// rgb creates an NRGBA color from a hex value.
func rgb(hex uint32) color.NRGBA {
	return color.NRGBA{
		R: uint8((hex >> 16) & 0xff),
		G: uint8((hex >> 8) & 0xff),
		B: uint8(hex & 0xff),
		A: 0xff,
	}
}

// rgba creates an NRGBA color from a hex value with alpha.
func rgba(hex uint32, alpha uint8) color.NRGBA {
	return color.NRGBA{
		R: uint8((hex >> 16) & 0xff),
		G: uint8((hex >> 8) & 0xff),
		B: uint8(hex & 0xff),
		A: alpha,
	}
}

// defaultFonts returns the default font collection.
func defaultFonts() []font.FontFace {
	return nil // Use system defaults
}
