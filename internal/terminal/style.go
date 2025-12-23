package terminal

// Style represents the visual attributes of a terminal cell.
type Style struct {
	Fg            Color
	Bg            Color
	Bold          bool
	Italic        bool
	Underline     bool
	Strikethrough bool
	Dim           bool
	Blink         bool
	Reverse       bool
	Hidden        bool
}

// DefaultStyle returns a Style with default colors and no attributes.
func DefaultStyle() Style {
	return Style{
		Fg: DefaultColor(),
		Bg: DefaultColor(),
	}
}

// Equal returns true if two styles are equal.
func (s Style) Equal(other Style) bool {
	return s.Fg.Equal(other.Fg) &&
		s.Bg.Equal(other.Bg) &&
		s.Bold == other.Bold &&
		s.Italic == other.Italic &&
		s.Underline == other.Underline &&
		s.Strikethrough == other.Strikethrough &&
		s.Dim == other.Dim &&
		s.Blink == other.Blink &&
		s.Reverse == other.Reverse &&
		s.Hidden == other.Hidden
}

// WithFg returns a copy of the style with the foreground color set.
func (s Style) WithFg(c Color) Style {
	s.Fg = c
	return s
}

// WithBg returns a copy of the style with the background color set.
func (s Style) WithBg(c Color) Style {
	s.Bg = c
	return s
}

// WithBold returns a copy of the style with bold set.
func (s Style) WithBold(b bool) Style {
	s.Bold = b
	return s
}

// WithItalic returns a copy of the style with italic set.
func (s Style) WithItalic(b bool) Style {
	s.Italic = b
	return s
}

// WithUnderline returns a copy of the style with underline set.
func (s Style) WithUnderline(b bool) Style {
	s.Underline = b
	return s
}

// WithStrikethrough returns a copy of the style with strikethrough set.
func (s Style) WithStrikethrough(b bool) Style {
	s.Strikethrough = b
	return s
}

// ApplySGR applies a Select Graphic Rendition (SGR) parameter to the style.
// SGR parameters come from CSI sequences like "\x1b[1;31m".
// Returns a new Style with the parameter applied.
func (s Style) ApplySGR(param int) Style {
	switch {
	case param == 0:
		// Reset all attributes
		return DefaultStyle()

	case param == 1:
		s.Bold = true
	case param == 2:
		s.Dim = true
	case param == 3:
		s.Italic = true
	case param == 4:
		s.Underline = true
	case param == 5 || param == 6:
		s.Blink = true
	case param == 7:
		s.Reverse = true
	case param == 8:
		s.Hidden = true
	case param == 9:
		s.Strikethrough = true

	case param == 21:
		// Double underline (treat as underline)
		s.Underline = true
	case param == 22:
		// Normal intensity (not bold, not dim)
		s.Bold = false
		s.Dim = false
	case param == 23:
		s.Italic = false
	case param == 24:
		s.Underline = false
	case param == 25:
		s.Blink = false
	case param == 27:
		s.Reverse = false
	case param == 28:
		s.Hidden = false
	case param == 29:
		s.Strikethrough = false

	// Standard foreground colors (30-37)
	case param >= 30 && param <= 37:
		s.Fg = IndexedColor(uint8(param - 30))

	// Default foreground (39)
	case param == 39:
		s.Fg = DefaultColor()

	// Standard background colors (40-47)
	case param >= 40 && param <= 47:
		s.Bg = IndexedColor(uint8(param - 40))

	// Default background (49)
	case param == 49:
		s.Bg = DefaultColor()

	// Bright foreground colors (90-97)
	case param >= 90 && param <= 97:
		s.Fg = IndexedColor(uint8(param - 90 + 8))

	// Bright background colors (100-107)
	case param >= 100 && param <= 107:
		s.Bg = IndexedColor(uint8(param - 100 + 8))
	}

	return s
}

// ApplySGRParams applies multiple SGR parameters, handling extended color sequences.
// Extended colors use formats like:
//   - 38;5;N for 256-color foreground
//   - 48;5;N for 256-color background
//   - 38;2;R;G;B for 24-bit foreground
//   - 48;2;R;G;B for 24-bit background
func (s Style) ApplySGRParams(params []int) Style {
	i := 0
	for i < len(params) {
		param := params[i]

		// Check for extended color sequences
		if param == 38 && i+1 < len(params) {
			// Extended foreground color
			if params[i+1] == 5 && i+2 < len(params) {
				// 256-color: 38;5;N
				s.Fg = IndexedColor(uint8(params[i+2]))
				i += 3
				continue
			} else if params[i+1] == 2 && i+4 < len(params) {
				// 24-bit: 38;2;R;G;B
				r := clampByte(params[i+2])
				g := clampByte(params[i+3])
				b := clampByte(params[i+4])
				s.Fg = RGBColor(r, g, b)
				i += 5
				continue
			}
		} else if param == 48 && i+1 < len(params) {
			// Extended background color
			if params[i+1] == 5 && i+2 < len(params) {
				// 256-color: 48;5;N
				s.Bg = IndexedColor(uint8(params[i+2]))
				i += 3
				continue
			} else if params[i+1] == 2 && i+4 < len(params) {
				// 24-bit: 48;2;R;G;B
				r := clampByte(params[i+2])
				g := clampByte(params[i+3])
				b := clampByte(params[i+4])
				s.Bg = RGBColor(r, g, b)
				i += 5
				continue
			}
		}

		// Regular SGR parameter
		s = s.ApplySGR(param)
		i++
	}

	return s
}

// clampByte clamps an int to the range [0, 255].
func clampByte(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
