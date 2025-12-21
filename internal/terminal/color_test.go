package terminal

import (
	"image/color"
	"testing"
)

func TestDefaultColor(t *testing.T) {
	c := DefaultColor()
	if c.Type != ColorDefault {
		t.Errorf("DefaultColor().Type = %v, want ColorDefault", c.Type)
	}
	if !c.IsDefault() {
		t.Error("DefaultColor().IsDefault() = false, want true")
	}
}

func TestIndexedColor(t *testing.T) {
	tests := []uint8{0, 7, 15, 127, 255}
	for _, idx := range tests {
		c := IndexedColor(idx)
		if c.Type != ColorIndexed {
			t.Errorf("IndexedColor(%d).Type = %v, want ColorIndexed", idx, c.Type)
		}
		if c.Index != idx {
			t.Errorf("IndexedColor(%d).Index = %d, want %d", idx, c.Index, idx)
		}
		if c.IsDefault() {
			t.Errorf("IndexedColor(%d).IsDefault() = true, want false", idx)
		}
	}
}

func TestRGBColor(t *testing.T) {
	c := RGBColor(100, 150, 200)
	if c.Type != ColorRGB {
		t.Errorf("RGBColor(100,150,200).Type = %v, want ColorRGB", c.Type)
	}
	if c.R != 100 || c.G != 150 || c.B != 200 {
		t.Errorf("RGBColor(100,150,200) = (%d,%d,%d), want (100,150,200)", c.R, c.G, c.B)
	}
	if c.IsDefault() {
		t.Error("RGBColor(...).IsDefault() = true, want false")
	}
}

func TestColorEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b Color
		want bool
	}{
		{"default == default", DefaultColor(), DefaultColor(), true},
		{"indexed same", IndexedColor(5), IndexedColor(5), true},
		{"indexed different", IndexedColor(5), IndexedColor(6), false},
		{"rgb same", RGBColor(10, 20, 30), RGBColor(10, 20, 30), true},
		{"rgb different r", RGBColor(10, 20, 30), RGBColor(11, 20, 30), false},
		{"rgb different g", RGBColor(10, 20, 30), RGBColor(10, 21, 30), false},
		{"rgb different b", RGBColor(10, 20, 30), RGBColor(10, 20, 31), false},
		{"default != indexed", DefaultColor(), IndexedColor(0), false},
		{"indexed != rgb", IndexedColor(0), RGBColor(0, 0, 0), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equal(tt.b); got != tt.want {
				t.Errorf("%s: Equal() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestToNRGBA_Default(t *testing.T) {
	defaultC := color.NRGBA{R: 200, G: 200, B: 200, A: 255}
	c := DefaultColor()
	got := c.ToNRGBA(defaultC)
	if got != defaultC {
		t.Errorf("DefaultColor().ToNRGBA() = %v, want %v", got, defaultC)
	}
}

func TestToNRGBA_RGB(t *testing.T) {
	c := RGBColor(128, 64, 32)
	defaultC := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	got := c.ToNRGBA(defaultC)
	want := color.NRGBA{R: 128, G: 64, B: 32, A: 255}
	if got != want {
		t.Errorf("RGBColor(128,64,32).ToNRGBA() = %v, want %v", got, want)
	}
}

func TestToNRGBA_Indexed16(t *testing.T) {
	// Test standard 16 colors
	tests := []struct {
		index uint8
		want  color.NRGBA
	}{
		{ColorBlack, color.NRGBA{R: 0, G: 0, B: 0, A: 255}},
		{ColorRed, color.NRGBA{R: 205, G: 49, B: 49, A: 255}},
		{ColorGreen, color.NRGBA{R: 13, G: 188, B: 121, A: 255}},
		{ColorYellow, color.NRGBA{R: 229, G: 229, B: 16, A: 255}},
		{ColorBlue, color.NRGBA{R: 36, G: 114, B: 200, A: 255}},
		{ColorMagenta, color.NRGBA{R: 188, G: 63, B: 188, A: 255}},
		{ColorCyan, color.NRGBA{R: 17, G: 168, B: 205, A: 255}},
		{ColorWhite, color.NRGBA{R: 229, G: 229, B: 229, A: 255}},
		{ColorBrightBlack, color.NRGBA{R: 102, G: 102, B: 102, A: 255}},
		{ColorBrightWhite, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
	}

	defaultC := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	for _, tt := range tests {
		c := IndexedColor(tt.index)
		got := c.ToNRGBA(defaultC)
		if got != tt.want {
			t.Errorf("IndexedColor(%d).ToNRGBA() = %v, want %v", tt.index, got, tt.want)
		}
	}
}

func TestToNRGBA_Indexed256_ColorCube(t *testing.T) {
	// Test 6x6x6 color cube (indices 16-231)
	// Index 16 = (0,0,0)
	// Index 21 = (0,0,5) -> (0, 0, 255)
	// Index 196 = (5,0,0) -> (255, 0, 0)
	tests := []struct {
		index uint8
		want  color.NRGBA
	}{
		{16, color.NRGBA{R: 0, G: 0, B: 0, A: 255}},           // (0,0,0)
		{21, color.NRGBA{R: 0, G: 0, B: 255, A: 255}},         // (0,0,5)
		{196, color.NRGBA{R: 255, G: 0, B: 0, A: 255}},        // (5,0,0)
		{46, color.NRGBA{R: 0, G: 255, B: 0, A: 255}},         // (0,5,0)
		{231, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},    // (5,5,5)
		{59, color.NRGBA{R: 95, G: 95, B: 95, A: 255}},        // (1,1,1)
	}

	defaultC := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	for _, tt := range tests {
		c := IndexedColor(tt.index)
		got := c.ToNRGBA(defaultC)
		if got != tt.want {
			t.Errorf("IndexedColor(%d).ToNRGBA() = %v, want %v", tt.index, got, tt.want)
		}
	}
}

func TestToNRGBA_Indexed256_Grayscale(t *testing.T) {
	// Test grayscale ramp (indices 232-255)
	// Index 232 = gray 8
	// Index 255 = gray 238
	tests := []struct {
		index uint8
		gray  uint8
	}{
		{232, 8},   // 8 + 0*10
		{233, 18},  // 8 + 1*10
		{244, 128}, // 8 + 12*10
		{255, 238}, // 8 + 23*10
	}

	defaultC := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	for _, tt := range tests {
		c := IndexedColor(tt.index)
		got := c.ToNRGBA(defaultC)
		want := color.NRGBA{R: tt.gray, G: tt.gray, B: tt.gray, A: 255}
		if got != want {
			t.Errorf("IndexedColor(%d).ToNRGBA() = %v, want %v (gray %d)", tt.index, got, want, tt.gray)
		}
	}
}

func TestGetPalette16Color(t *testing.T) {
	// Test valid indices
	got := GetPalette16Color(ColorRed)
	want := color.NRGBA{R: 205, G: 49, B: 49, A: 255}
	if got != want {
		t.Errorf("GetPalette16Color(ColorRed) = %v, want %v", got, want)
	}

	// Test index > 15 is clamped
	got = GetPalette16Color(100)
	want = GetPalette16Color(15)
	if got != want {
		t.Errorf("GetPalette16Color(100) = %v, want %v (clamped to 15)", got, want)
	}
}

func TestGetPalette256Color(t *testing.T) {
	// Test that it matches IndexedColor().ToNRGBA()
	for i := 0; i < 256; i++ {
		idx := uint8(i)
		got := GetPalette256Color(idx)
		c := IndexedColor(idx)
		want := c.ToNRGBA(color.NRGBA{})
		if got != want {
			t.Errorf("GetPalette256Color(%d) = %v, want %v", idx, got, want)
		}
	}
}

func TestPalette256_ColorCubeFormula(t *testing.T) {
	// Verify the 6x6x6 color cube formula
	// index = 16 + 36*r + 6*g + b, where r,g,b ∈ [0,5]
	levels := [6]uint8{0, 95, 135, 175, 215, 255}

	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				index := uint8(16 + 36*r + 6*g + b)
				got := GetPalette256Color(index)
				want := color.NRGBA{R: levels[r], G: levels[g], B: levels[b], A: 255}
				if got != want {
					t.Errorf("Color cube [%d,%d,%d] (index %d) = %v, want %v", r, g, b, index, got, want)
				}
			}
		}
	}
}

