package terminal

import "testing"

func TestDefaultStyle(t *testing.T) {
	s := DefaultStyle()
	if !s.Fg.IsDefault() {
		t.Error("DefaultStyle().Fg should be default")
	}
	if !s.Bg.IsDefault() {
		t.Error("DefaultStyle().Bg should be default")
	}
	if s.Bold || s.Italic || s.Underline || s.Strikethrough {
		t.Error("DefaultStyle() should have no attributes set")
	}
}

func TestStyleEqual(t *testing.T) {
	s1 := DefaultStyle()
	s2 := DefaultStyle()
	if !s1.Equal(s2) {
		t.Error("Two default styles should be equal")
	}

	s3 := s1.WithBold(true)
	if s1.Equal(s3) {
		t.Error("Default style should not equal bold style")
	}

	s4 := s3.WithBold(true)
	if !s3.Equal(s4) {
		t.Error("Two bold styles should be equal")
	}
}

func TestStyleWithMethods(t *testing.T) {
	s := DefaultStyle()

	s1 := s.WithFg(IndexedColor(ColorRed))
	if s1.Fg.Index != ColorRed {
		t.Error("WithFg should set foreground color")
	}
	if !s.Fg.IsDefault() {
		t.Error("Original style should not be modified")
	}

	s2 := s.WithBg(IndexedColor(ColorBlue))
	if s2.Bg.Index != ColorBlue {
		t.Error("WithBg should set background color")
	}

	s3 := s.WithBold(true)
	if !s3.Bold {
		t.Error("WithBold(true) should set bold")
	}

	s4 := s.WithItalic(true)
	if !s4.Italic {
		t.Error("WithItalic(true) should set italic")
	}

	s5 := s.WithUnderline(true)
	if !s5.Underline {
		t.Error("WithUnderline(true) should set underline")
	}

	s6 := s.WithStrikethrough(true)
	if !s6.Strikethrough {
		t.Error("WithStrikethrough(true) should set strikethrough")
	}
}

func TestApplySGR_Reset(t *testing.T) {
	s := DefaultStyle().
		WithBold(true).
		WithItalic(true).
		WithFg(IndexedColor(ColorRed))

	s = s.ApplySGR(0)
	if !s.Equal(DefaultStyle()) {
		t.Error("SGR 0 should reset to default style")
	}
}

func TestApplySGR_Attributes(t *testing.T) {
	tests := []struct {
		param int
		check func(Style) bool
		name  string
	}{
		{1, func(s Style) bool { return s.Bold }, "bold"},
		{2, func(s Style) bool { return s.Dim }, "dim"},
		{3, func(s Style) bool { return s.Italic }, "italic"},
		{4, func(s Style) bool { return s.Underline }, "underline"},
		{5, func(s Style) bool { return s.Blink }, "blink slow"},
		{6, func(s Style) bool { return s.Blink }, "blink rapid"},
		{7, func(s Style) bool { return s.Reverse }, "reverse"},
		{8, func(s Style) bool { return s.Hidden }, "hidden"},
		{9, func(s Style) bool { return s.Strikethrough }, "strikethrough"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := DefaultStyle().ApplySGR(tt.param)
			if !tt.check(s) {
				t.Errorf("SGR %d should set %s", tt.param, tt.name)
			}
		})
	}
}

func TestApplySGR_DisableAttributes(t *testing.T) {
	tests := []struct {
		enableParam  int
		disableParam int
		check        func(Style) bool
		name         string
	}{
		{1, 22, func(s Style) bool { return !s.Bold }, "bold off (22)"},
		{2, 22, func(s Style) bool { return !s.Dim }, "dim off (22)"},
		{3, 23, func(s Style) bool { return !s.Italic }, "italic off"},
		{4, 24, func(s Style) bool { return !s.Underline }, "underline off"},
		{5, 25, func(s Style) bool { return !s.Blink }, "blink off"},
		{7, 27, func(s Style) bool { return !s.Reverse }, "reverse off"},
		{8, 28, func(s Style) bool { return !s.Hidden }, "hidden off"},
		{9, 29, func(s Style) bool { return !s.Strikethrough }, "strikethrough off"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := DefaultStyle().ApplySGR(tt.enableParam).ApplySGR(tt.disableParam)
			if !tt.check(s) {
				t.Errorf("SGR %d after SGR %d should disable %s", tt.disableParam, tt.enableParam, tt.name)
			}
		})
	}
}

func TestApplySGR_StandardForeground(t *testing.T) {
	for i := 0; i < 8; i++ {
		s := DefaultStyle().ApplySGR(30 + i)
		if s.Fg.Type != ColorIndexed || s.Fg.Index != uint8(i) {
			t.Errorf("SGR %d should set fg to indexed color %d, got %+v", 30+i, i, s.Fg)
		}
	}
}

func TestApplySGR_StandardBackground(t *testing.T) {
	for i := 0; i < 8; i++ {
		s := DefaultStyle().ApplySGR(40 + i)
		if s.Bg.Type != ColorIndexed || s.Bg.Index != uint8(i) {
			t.Errorf("SGR %d should set bg to indexed color %d, got %+v", 40+i, i, s.Bg)
		}
	}
}

func TestApplySGR_BrightForeground(t *testing.T) {
	for i := 0; i < 8; i++ {
		s := DefaultStyle().ApplySGR(90 + i)
		expectedIndex := uint8(i + 8) // Bright colors are 8-15
		if s.Fg.Type != ColorIndexed || s.Fg.Index != expectedIndex {
			t.Errorf("SGR %d should set fg to indexed color %d, got %+v", 90+i, expectedIndex, s.Fg)
		}
	}
}

func TestApplySGR_BrightBackground(t *testing.T) {
	for i := 0; i < 8; i++ {
		s := DefaultStyle().ApplySGR(100 + i)
		expectedIndex := uint8(i + 8)
		if s.Bg.Type != ColorIndexed || s.Bg.Index != expectedIndex {
			t.Errorf("SGR %d should set bg to indexed color %d, got %+v", 100+i, expectedIndex, s.Bg)
		}
	}
}

func TestApplySGR_DefaultColors(t *testing.T) {
	s := DefaultStyle().
		WithFg(IndexedColor(ColorRed)).
		WithBg(IndexedColor(ColorBlue))

	s = s.ApplySGR(39)
	if !s.Fg.IsDefault() {
		t.Error("SGR 39 should reset fg to default")
	}
	if s.Bg.IsDefault() {
		t.Error("SGR 39 should not affect bg")
	}

	s = s.ApplySGR(49)
	if !s.Bg.IsDefault() {
		t.Error("SGR 49 should reset bg to default")
	}
}

func TestApplySGRParams_256Color(t *testing.T) {
	// Foreground 256-color
	s := DefaultStyle().ApplySGRParams([]int{38, 5, 196})
	if s.Fg.Type != ColorIndexed || s.Fg.Index != 196 {
		t.Errorf("38;5;196 should set fg to indexed 196, got %+v", s.Fg)
	}

	// Background 256-color
	s = DefaultStyle().ApplySGRParams([]int{48, 5, 21})
	if s.Bg.Type != ColorIndexed || s.Bg.Index != 21 {
		t.Errorf("48;5;21 should set bg to indexed 21, got %+v", s.Bg)
	}
}

func TestApplySGRParams_24BitColor(t *testing.T) {
	// Foreground 24-bit
	s := DefaultStyle().ApplySGRParams([]int{38, 2, 128, 64, 32})
	if s.Fg.Type != ColorRGB || s.Fg.R != 128 || s.Fg.G != 64 || s.Fg.B != 32 {
		t.Errorf("38;2;128;64;32 should set fg to RGB(128,64,32), got %+v", s.Fg)
	}

	// Background 24-bit
	s = DefaultStyle().ApplySGRParams([]int{48, 2, 255, 128, 0})
	if s.Bg.Type != ColorRGB || s.Bg.R != 255 || s.Bg.G != 128 || s.Bg.B != 0 {
		t.Errorf("48;2;255;128;0 should set bg to RGB(255,128,0), got %+v", s.Bg)
	}
}

func TestApplySGRParams_Combined(t *testing.T) {
	// "\x1b[1;31m" - bold red
	s := DefaultStyle().ApplySGRParams([]int{1, 31})
	if !s.Bold {
		t.Error("1;31 should set bold")
	}
	if s.Fg.Type != ColorIndexed || s.Fg.Index != ColorRed {
		t.Errorf("1;31 should set fg to red, got %+v", s.Fg)
	}

	// "\x1b[0;38;2;100;150;200;48;5;232m" - reset, fg RGB, bg 256
	s = DefaultStyle().ApplySGRParams([]int{0, 38, 2, 100, 150, 200, 48, 5, 232})
	if s.Fg.Type != ColorRGB || s.Fg.R != 100 || s.Fg.G != 150 || s.Fg.B != 200 {
		t.Errorf("Expected fg RGB(100,150,200), got %+v", s.Fg)
	}
	if s.Bg.Type != ColorIndexed || s.Bg.Index != 232 {
		t.Errorf("Expected bg indexed 232, got %+v", s.Bg)
	}
}

func TestApplySGRParams_Empty(t *testing.T) {
	s := DefaultStyle().WithBold(true)
	s = s.ApplySGRParams([]int{})
	if !s.Bold {
		t.Error("Empty params should not change style")
	}
}

func TestApplySGRParams_IncompleteExtended(t *testing.T) {
	// Incomplete 256-color sequence (missing color index)
	s := DefaultStyle().ApplySGRParams([]int{38, 5})
	// Should not crash, fg should remain default
	if !s.Fg.IsDefault() {
		t.Error("Incomplete 38;5 should not change fg")
	}

	// Incomplete 24-bit sequence
	s = DefaultStyle().ApplySGRParams([]int{38, 2, 100, 150})
	if !s.Fg.IsDefault() {
		t.Error("Incomplete 38;2;r;g should not change fg")
	}
}

func TestClampByte(t *testing.T) {
	tests := []struct {
		input int
		want  uint8
	}{
		{-10, 0},
		{0, 0},
		{128, 128},
		{255, 255},
		{300, 255},
	}

	for _, tt := range tests {
		got := clampByte(tt.input)
		if got != tt.want {
			t.Errorf("clampByte(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

