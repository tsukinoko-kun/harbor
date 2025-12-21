package terminal

import "image/color"

// ColorType represents the type of color specification.
type ColorType uint8

const (
	// ColorDefault indicates the default terminal color should be used.
	ColorDefault ColorType = iota
	// ColorIndexed indicates an indexed color (0-255).
	ColorIndexed
	// ColorRGB indicates a 24-bit RGB color.
	ColorRGB
)

// Color represents a terminal color that can be default, indexed (8/16/256), or RGB.
type Color struct {
	Type  ColorType
	Index uint8 // For ColorIndexed: 0-255
	R, G, B uint8 // For ColorRGB
}

// DefaultColor returns a Color representing the default terminal color.
func DefaultColor() Color {
	return Color{Type: ColorDefault}
}

// IndexedColor returns a Color for an indexed palette color (0-255).
func IndexedColor(index uint8) Color {
	return Color{Type: ColorIndexed, Index: index}
}

// RGBColor returns a Color for a 24-bit RGB color.
func RGBColor(r, g, b uint8) Color {
	return Color{Type: ColorRGB, R: r, G: g, B: b}
}

// Standard 8 colors (indices 0-7)
const (
	ColorBlack   uint8 = 0
	ColorRed     uint8 = 1
	ColorGreen   uint8 = 2
	ColorYellow  uint8 = 3
	ColorBlue    uint8 = 4
	ColorMagenta uint8 = 5
	ColorCyan    uint8 = 6
	ColorWhite   uint8 = 7
)

// Bright colors are indices 8-15
const (
	ColorBrightBlack   uint8 = 8
	ColorBrightRed     uint8 = 9
	ColorBrightGreen   uint8 = 10
	ColorBrightYellow  uint8 = 11
	ColorBrightBlue    uint8 = 12
	ColorBrightMagenta uint8 = 13
	ColorBrightCyan    uint8 = 14
	ColorBrightWhite   uint8 = 15
)

// palette16 contains the standard 16-color ANSI palette.
// These colors are commonly used in terminals.
var palette16 = [16]color.NRGBA{
	// Standard colors (0-7)
	{R: 0, G: 0, B: 0, A: 255},       // Black
	{R: 205, G: 49, B: 49, A: 255},   // Red
	{R: 13, G: 188, B: 121, A: 255},  // Green
	{R: 229, G: 229, B: 16, A: 255},  // Yellow
	{R: 36, G: 114, B: 200, A: 255},  // Blue
	{R: 188, G: 63, B: 188, A: 255},  // Magenta
	{R: 17, G: 168, B: 205, A: 255},  // Cyan
	{R: 229, G: 229, B: 229, A: 255}, // White
	// Bright colors (8-15)
	{R: 102, G: 102, B: 102, A: 255}, // Bright Black (Gray)
	{R: 241, G: 76, B: 76, A: 255},   // Bright Red
	{R: 35, G: 209, B: 139, A: 255},  // Bright Green
	{R: 245, G: 245, B: 67, A: 255},  // Bright Yellow
	{R: 59, G: 142, B: 234, A: 255},  // Bright Blue
	{R: 214, G: 112, B: 214, A: 255}, // Bright Magenta
	{R: 41, G: 184, B: 219, A: 255},  // Bright Cyan
	{R: 255, G: 255, B: 255, A: 255}, // Bright White
}

// palette256 is lazily initialized on first use.
var palette256 [256]color.NRGBA
var palette256Initialized bool

// initPalette256 initializes the 256-color palette.
// Colors 0-15: standard 16 colors
// Colors 16-231: 6x6x6 color cube
// Colors 232-255: grayscale ramp
func initPalette256() {
	if palette256Initialized {
		return
	}

	// Copy standard 16 colors
	copy(palette256[:16], palette16[:])

	// 6x6x6 color cube (216 colors, indices 16-231)
	// Each component has 6 levels: 0, 95, 135, 175, 215, 255
	levels := [6]uint8{0, 95, 135, 175, 215, 255}
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				index := 16 + r*36 + g*6 + b
				palette256[index] = color.NRGBA{
					R: levels[r],
					G: levels[g],
					B: levels[b],
					A: 255,
				}
			}
		}
	}

	// Grayscale ramp (24 colors, indices 232-255)
	// Values: 8, 18, 28, ..., 238
	for i := 0; i < 24; i++ {
		gray := uint8(8 + i*10)
		palette256[232+i] = color.NRGBA{R: gray, G: gray, B: gray, A: 255}
	}

	palette256Initialized = true
}

// ToNRGBA converts a Color to an NRGBA value for rendering.
// For ColorDefault, it returns the provided default color.
func (c Color) ToNRGBA(defaultColor color.NRGBA) color.NRGBA {
	switch c.Type {
	case ColorDefault:
		return defaultColor
	case ColorIndexed:
		initPalette256()
		return palette256[c.Index]
	case ColorRGB:
		return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255}
	default:
		return defaultColor
	}
}

// IsDefault returns true if this is the default color.
func (c Color) IsDefault() bool {
	return c.Type == ColorDefault
}

// Equal returns true if two colors are equal.
func (c Color) Equal(other Color) bool {
	if c.Type != other.Type {
		return false
	}
	switch c.Type {
	case ColorDefault:
		return true
	case ColorIndexed:
		return c.Index == other.Index
	case ColorRGB:
		return c.R == other.R && c.G == other.G && c.B == other.B
	default:
		return false
	}
}

// GetPalette16Color returns the NRGBA for a 16-color palette index.
func GetPalette16Color(index uint8) color.NRGBA {
	if index > 15 {
		index = 15
	}
	return palette16[index]
}

// GetPalette256Color returns the NRGBA for a 256-color palette index.
func GetPalette256Color(index uint8) color.NRGBA {
	initPalette256()
	return palette256[index]
}

