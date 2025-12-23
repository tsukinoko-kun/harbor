package terminal

import (
	"testing"
)

func TestTerminal_PlainText(t *testing.T) {
	term := New()
	term.Write([]byte("Hello"))

	if term.Lines() != 1 {
		t.Errorf("Lines() = %d, want 1", term.Lines())
	}
	if term.GetLine(0) != "Hello" {
		t.Errorf("GetLine(0) = %q, want %q", term.GetLine(0), "Hello")
	}
}

func TestTerminal_Newline(t *testing.T) {
	term := New()
	term.Write([]byte("Line1\nLine2\nLine3"))

	if term.Lines() != 3 {
		t.Errorf("Lines() = %d, want 3", term.Lines())
	}
	if term.GetLine(0) != "Line1" {
		t.Errorf("GetLine(0) = %q, want %q", term.GetLine(0), "Line1")
	}
	if term.GetLine(1) != "Line2" {
		t.Errorf("GetLine(1) = %q, want %q", term.GetLine(1), "Line2")
	}
	if term.GetLine(2) != "Line3" {
		t.Errorf("GetLine(2) = %q, want %q", term.GetLine(2), "Line3")
	}
}

func TestTerminal_CarriageReturn(t *testing.T) {
	term := New()
	term.Write([]byte("AAAAA\rBBB"))

	if term.Lines() != 1 {
		t.Errorf("Lines() = %d, want 1", term.Lines())
	}
	// "AAAAA" then CR moves to col 0, "BBB" overwrites first 3 chars
	if term.GetLine(0) != "BBBAA" {
		t.Errorf("GetLine(0) = %q, want %q", term.GetLine(0), "BBBAA")
	}
}

func TestTerminal_ProgressBar(t *testing.T) {
	term := New()

	// Simulate a progress bar that updates in place
	term.Write([]byte("Progress: 0%"))
	term.Write([]byte("\rProgress: 50%"))
	term.Write([]byte("\rProgress: 100%"))

	if term.Lines() != 1 {
		t.Errorf("Lines() = %d, want 1", term.Lines())
	}
	if term.GetLine(0) != "Progress: 100%" {
		t.Errorf("GetLine(0) = %q, want %q", term.GetLine(0), "Progress: 100%")
	}
}

func TestTerminal_Tab(t *testing.T) {
	term := New()
	term.Write([]byte("A\tB"))

	line := term.GetLine(0)
	// Tab should move to column 8, so "A" at 0, spaces at 1-7, "B" at 8
	if len(line) != 9 {
		t.Errorf("len(line) = %d, want 9", len(line))
	}
	if line[0] != 'A' || line[8] != 'B' {
		t.Errorf("GetLine(0) = %q, want A followed by spaces then B", line)
	}
}

func TestTerminal_Backspace(t *testing.T) {
	term := New()
	term.Write([]byte("ABC\b\bX"))

	// "ABC" then two backspaces -> col 1, "X" -> "AXC"
	if term.GetLine(0) != "AXC" {
		t.Errorf("GetLine(0) = %q, want %q", term.GetLine(0), "AXC")
	}
}

func TestTerminal_CursorUp(t *testing.T) {
	term := New()
	term.Write([]byte("Line1\nLine2\nLine3"))

	// Cursor is at row 2 (0-indexed), col 5
	row, col := term.CursorPosition()
	if row != 2 || col != 5 {
		t.Errorf("CursorPosition() = (%d, %d), want (2, 5)", row, col)
	}

	// Move up 2 lines
	term.Write([]byte("\x1b[2A"))
	row, col = term.CursorPosition()
	if row != 0 {
		t.Errorf("After cursor up: row = %d, want 0", row)
	}
}

func TestTerminal_CursorDown(t *testing.T) {
	term := New()
	term.Write([]byte("Line1"))

	// Move down 3 lines
	term.Write([]byte("\x1b[3B"))
	row, _ := term.CursorPosition()
	if row != 3 {
		t.Errorf("After cursor down: row = %d, want 3", row)
	}
}

func TestTerminal_CursorForwardBack(t *testing.T) {
	term := New()
	term.Write([]byte("ABCDE"))

	// Move back 3
	term.Write([]byte("\x1b[3D"))
	_, col := term.CursorPosition()
	if col != 2 {
		t.Errorf("After cursor back: col = %d, want 2", col)
	}

	// Move forward 1
	term.Write([]byte("\x1b[1C"))
	_, col = term.CursorPosition()
	if col != 3 {
		t.Errorf("After cursor forward: col = %d, want 3", col)
	}
}

func TestTerminal_CursorPosition(t *testing.T) {
	term := New()
	term.Write([]byte("Some text"))

	// ESC[5;10H - move to row 5, col 10 (1-indexed)
	term.Write([]byte("\x1b[5;10H"))
	row, col := term.CursorPosition()
	if row != 4 || col != 9 {
		t.Errorf("CursorPosition() = (%d, %d), want (4, 9)", row, col)
	}

	// ESC[H - home (1,1)
	term.Write([]byte("\x1b[H"))
	row, col = term.CursorPosition()
	if row != 0 || col != 0 {
		t.Errorf("CursorPosition() = (%d, %d), want (0, 0)", row, col)
	}
}

func TestTerminal_CursorColumn(t *testing.T) {
	term := New()
	term.Write([]byte("ABCDEFGH"))

	// ESC[5G - move to column 5 (1-indexed)
	term.Write([]byte("\x1b[5G"))
	_, col := term.CursorPosition()
	if col != 4 {
		t.Errorf("col = %d, want 4", col)
	}
}

func TestTerminal_EraseToEndOfLine(t *testing.T) {
	term := New()
	term.Write([]byte("ABCDEFGH"))

	// Move back to column 3, erase to end
	term.Write([]byte("\x1b[4G\x1b[K"))

	if term.GetLine(0) != "ABC     " {
		t.Errorf("GetLine(0) = %q, want %q", term.GetLine(0), "ABC     ")
	}
}

func TestTerminal_EraseToStartOfLine(t *testing.T) {
	term := New()
	term.Write([]byte("ABCDEFGH"))

	// Move to column 5 (1-indexed = col 4 0-indexed), erase to start (inclusive)
	// Erases cols 0-4 (A,B,C,D,E), leaves F,G,H at cols 5,6,7
	term.Write([]byte("\x1b[5G\x1b[1K"))

	line := term.GetLine(0)
	if line != "     FGH" {
		t.Errorf("GetLine(0) = %q, want %q", line, "     FGH")
	}
}

func TestTerminal_EraseEntireLine(t *testing.T) {
	term := New()
	term.Write([]byte("ABCDEFGH"))

	// Erase entire line
	term.Write([]byte("\x1b[2K"))

	line := term.GetLine(0)
	// Line should be all spaces
	for _, c := range line {
		if c != ' ' {
			t.Errorf("Line should be all spaces, got %q", line)
			break
		}
	}
}

func TestTerminal_EraseDisplay(t *testing.T) {
	term := New()
	term.Write([]byte("Line1\nLine2\nLine3\nLine4"))

	// Move to line 2, col 3, erase to end of screen
	term.Write([]byte("\x1b[2;3H\x1b[J"))

	if term.GetLine(0) != "Line1" {
		t.Errorf("GetLine(0) = %q, want %q", term.GetLine(0), "Line1")
	}
	// Line 1 (0-indexed row 1) should have "Li" then spaces
	line1 := term.GetLine(1)
	if line1[:2] != "Li" {
		t.Errorf("GetLine(1) first 2 chars = %q, want %q", line1[:2], "Li")
	}
}

func TestTerminal_ClearScreen(t *testing.T) {
	term := New()
	term.Write([]byte("Line1\nLine2\nLine3"))

	// Clear entire screen
	term.Write([]byte("\x1b[2J"))

	if term.Lines() != 0 {
		t.Errorf("Lines() = %d, want 0", term.Lines())
	}
	row, col := term.CursorPosition()
	if row != 0 || col != 0 {
		t.Errorf("CursorPosition() = (%d, %d), want (0, 0)", row, col)
	}
}

func TestTerminal_SGR_Colors(t *testing.T) {
	term := New()

	// Red text
	term.Write([]byte("\x1b[31mRed\x1b[0m Normal"))

	spans := term.GetLineSpans(0)
	if len(spans) < 2 {
		t.Fatalf("len(spans) = %d, want >= 2", len(spans))
	}

	// First span should be red
	if spans[0].Text != "Red" {
		t.Errorf("spans[0].Text = %q, want %q", spans[0].Text, "Red")
	}
	if spans[0].Style.Fg.Type != ColorIndexed || spans[0].Style.Fg.Index != ColorRed {
		t.Errorf("spans[0] should be red, got %+v", spans[0].Style.Fg)
	}

	// Find the "Normal" span (might be preceded by space)
	found := false
	for _, span := range spans {
		if span.Text == " Normal" || span.Text == "Normal" {
			if !span.Style.Fg.IsDefault() {
				t.Errorf("'Normal' span should have default fg, got %+v", span.Style.Fg)
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Could not find 'Normal' span in %+v", spans)
	}
}

func TestTerminal_SGR_256Color(t *testing.T) {
	term := New()
	term.Write([]byte("\x1b[38;5;196mTest"))

	spans := term.GetLineSpans(0)
	if len(spans) != 1 {
		t.Fatalf("len(spans) = %d, want 1", len(spans))
	}

	if spans[0].Style.Fg.Type != ColorIndexed || spans[0].Style.Fg.Index != 196 {
		t.Errorf("Fg should be indexed 196, got %+v", spans[0].Style.Fg)
	}
}

func TestTerminal_SGR_24BitColor(t *testing.T) {
	term := New()
	term.Write([]byte("\x1b[38;2;128;64;32mTest"))

	spans := term.GetLineSpans(0)
	if len(spans) != 1 {
		t.Fatalf("len(spans) = %d, want 1", len(spans))
	}

	fg := spans[0].Style.Fg
	if fg.Type != ColorRGB || fg.R != 128 || fg.G != 64 || fg.B != 32 {
		t.Errorf("Fg should be RGB(128,64,32), got %+v", fg)
	}
}

func TestTerminal_SGR_Bold(t *testing.T) {
	term := New()
	term.Write([]byte("\x1b[1mBold\x1b[0mNormal"))

	spans := term.GetLineSpans(0)
	if len(spans) < 2 {
		t.Fatalf("len(spans) = %d, want >= 2", len(spans))
	}

	if !spans[0].Style.Bold {
		t.Error("First span should be bold")
	}
	// Find normal span
	for _, span := range spans[1:] {
		if span.Text == "Normal" {
			if span.Style.Bold {
				t.Error("'Normal' span should not be bold")
			}
			break
		}
	}
}

func TestTerminal_SaveRestoreCursor(t *testing.T) {
	term := New()
	term.Write([]byte("ABCDE"))

	// Save cursor at position (0, 5)
	term.Write([]byte("\x1b[s"))

	// Move and write
	term.Write([]byte("\x1b[1;1H"))
	term.Write([]byte("X"))

	// Restore cursor
	term.Write([]byte("\x1b[u"))

	row, col := term.CursorPosition()
	if row != 0 || col != 5 {
		t.Errorf("CursorPosition() = (%d, %d), want (0, 5)", row, col)
	}
}

func TestTerminal_MultiWriteCalls(t *testing.T) {
	term := New()

	// Split an escape sequence across multiple writes
	term.Write([]byte("\x1b"))
	term.Write([]byte("[31"))
	term.Write([]byte("m"))
	term.Write([]byte("Red"))

	spans := term.GetLineSpans(0)
	if len(spans) != 1 {
		t.Fatalf("len(spans) = %d, want 1", len(spans))
	}
	if spans[0].Style.Fg.Index != ColorRed {
		t.Errorf("Text should be red")
	}
}

func TestTerminal_CursorNextPrevLine(t *testing.T) {
	term := New()
	term.Write([]byte("Line1\nLine2\nLine3"))

	// Cursor is at (2, 5)
	// E = cursor to beginning of next line
	term.Write([]byte("\x1b[1E"))
	row, col := term.CursorPosition()
	if row != 3 || col != 0 {
		t.Errorf("After E: (%d, %d), want (3, 0)", row, col)
	}

	// F = cursor to beginning of previous line
	term.Write([]byte("\x1b[2F"))
	row, col = term.CursorPosition()
	if row != 1 || col != 0 {
		t.Errorf("After F: (%d, %d), want (1, 0)", row, col)
	}
}

func TestTerminal_CursorBoundsCheck(t *testing.T) {
	term := New()
	term.Write([]byte("Test"))

	// Try to move cursor up beyond top
	term.Write([]byte("\x1b[100A"))
	row, _ := term.CursorPosition()
	if row != 0 {
		t.Errorf("Cursor should be clamped to row 0, got %d", row)
	}

	// Try to move cursor left beyond start
	term.Write([]byte("\x1b[100D"))
	_, col := term.CursorPosition()
	if col != 0 {
		t.Errorf("Cursor should be clamped to col 0, got %d", col)
	}
}

func TestTerminal_Reset(t *testing.T) {
	term := New()
	term.Write([]byte("\x1b[31mSome colored text"))

	term.Reset()

	if term.Lines() != 0 {
		t.Errorf("Lines() = %d, want 0", term.Lines())
	}
	row, col := term.CursorPosition()
	if row != 0 || col != 0 {
		t.Errorf("CursorPosition() = (%d, %d), want (0, 0)", row, col)
	}
	if !term.CurrentStyle().Equal(DefaultStyle()) {
		t.Error("Style should be reset to default")
	}
}

func TestTerminal_String(t *testing.T) {
	term := New()
	term.Write([]byte("Line1\nLine2"))

	s := term.String()
	if s != "Line1\nLine2" {
		t.Errorf("String() = %q, want %q", s, "Line1\nLine2")
	}
}

func TestTerminal_Concurrency(t *testing.T) {
	term := New()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				term.Write([]byte("test"))
				_ = term.Lines()
				_ = term.String()
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
	// Just ensure no panic/race
}

func TestTerminal_ComplexSequence(t *testing.T) {
	term := New()

	// Simulate colored ls output
	term.Write([]byte("\x1b[0m\x1b[01;34mdir1\x1b[0m  \x1b[01;32mfile.txt\x1b[0m\n"))

	spans := term.GetLineSpans(0)
	if len(spans) < 3 {
		t.Fatalf("Expected at least 3 spans, got %d: %+v", len(spans), spans)
	}

	// Check that we have a blue span (dir) and green span (file)
	hasBlue := false
	hasGreen := false
	for _, span := range spans {
		if span.Style.Fg.Type == ColorIndexed {
			if span.Style.Fg.Index == ColorBlue {
				hasBlue = true
			} else if span.Style.Fg.Index == ColorGreen {
				hasGreen = true
			}
		}
	}
	if !hasBlue {
		t.Error("Expected blue span for directory")
	}
	if !hasGreen {
		t.Error("Expected green span for file")
	}
}
