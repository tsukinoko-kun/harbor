package terminal

import "testing"

func TestNewBuffer(t *testing.T) {
	b := NewBuffer(80)
	if b.Width() != 80 {
		t.Errorf("Width() = %d, want 80", b.Width())
	}
	if b.Height() != 0 {
		t.Errorf("Height() = %d, want 0", b.Height())
	}
}

func TestEmptyCell(t *testing.T) {
	c := EmptyCell()
	if c.Char != ' ' {
		t.Errorf("EmptyCell().Char = %q, want ' '", c.Char)
	}
	if !c.Style.Equal(DefaultStyle()) {
		t.Error("EmptyCell().Style should be default")
	}
}

func TestBuffer_EnsureRows(t *testing.T) {
	b := NewBuffer(0)
	b.EnsureRows(5)
	if b.Height() != 5 {
		t.Errorf("Height() = %d, want 5", b.Height())
	}
	// Ensure it doesn't shrink
	b.EnsureRows(3)
	if b.Height() != 5 {
		t.Errorf("Height() = %d after EnsureRows(3), want 5", b.Height())
	}
}

func TestBuffer_SetCell(t *testing.T) {
	b := NewBuffer(0)
	style := DefaultStyle().WithFg(IndexedColor(ColorRed))

	b.SetCell(2, 5, 'X', style)

	// Check the buffer expanded
	if b.Height() != 3 {
		t.Errorf("Height() = %d, want 3", b.Height())
	}
	if b.RowWidth(2) != 6 {
		t.Errorf("RowWidth(2) = %d, want 6", b.RowWidth(2))
	}

	// Check the cell
	c := b.GetCell(2, 5)
	if c.Char != 'X' {
		t.Errorf("GetCell(2,5).Char = %q, want 'X'", c.Char)
	}
	if !c.Style.Equal(style) {
		t.Error("GetCell(2,5).Style should match")
	}

	// Check gaps are empty cells
	c = b.GetCell(2, 3)
	if c.Char != ' ' {
		t.Errorf("Gap cell Char = %q, want ' '", c.Char)
	}
}

func TestBuffer_GetCell_OutOfBounds(t *testing.T) {
	b := NewBuffer(0)
	b.SetCell(0, 0, 'A', DefaultStyle())

	// Out of bounds should return empty cell
	c := b.GetCell(-1, 0)
	if c.Char != ' ' {
		t.Error("Out of bounds row should return empty cell")
	}

	c = b.GetCell(0, -1)
	if c.Char != ' ' {
		t.Error("Out of bounds col should return empty cell")
	}

	c = b.GetCell(100, 100)
	if c.Char != ' ' {
		t.Error("Far out of bounds should return empty cell")
	}
}

func TestBuffer_GetRow(t *testing.T) {
	b := NewBuffer(0)
	b.SetCell(0, 0, 'A', DefaultStyle())
	b.SetCell(0, 1, 'B', DefaultStyle())
	b.SetCell(0, 2, 'C', DefaultStyle())

	row := b.GetRow(0)
	if len(row) != 3 {
		t.Errorf("len(GetRow(0)) = %d, want 3", len(row))
	}
	if row[0].Char != 'A' || row[1].Char != 'B' || row[2].Char != 'C' {
		t.Error("Row content mismatch")
	}

	// Verify it's a copy
	row[0].Char = 'X'
	if b.GetCell(0, 0).Char != 'A' {
		t.Error("GetRow should return a copy")
	}

	// Non-existent row
	if b.GetRow(100) != nil {
		t.Error("Non-existent row should return nil")
	}
}

func TestBuffer_ClearRow(t *testing.T) {
	b := NewBuffer(0)
	style := DefaultStyle().WithFg(IndexedColor(ColorRed))
	b.SetCell(0, 0, 'A', style)
	b.SetCell(0, 1, 'B', style)

	b.ClearRow(0)

	c := b.GetCell(0, 0)
	if c.Char != ' ' || !c.Style.Equal(DefaultStyle()) {
		t.Error("ClearRow should set cells to empty")
	}

	// Row width should remain
	if b.RowWidth(0) != 2 {
		t.Errorf("RowWidth after clear = %d, want 2", b.RowWidth(0))
	}
}

func TestBuffer_ClearRowRange(t *testing.T) {
	b := NewBuffer(0)
	for i := 0; i < 5; i++ {
		b.SetCell(0, i, rune('A'+i), DefaultStyle())
	}

	b.ClearRowRange(0, 1, 4)

	if b.GetCell(0, 0).Char != 'A' {
		t.Error("Cell before range should be unchanged")
	}
	if b.GetCell(0, 1).Char != ' ' {
		t.Error("Cell in range should be cleared")
	}
	if b.GetCell(0, 2).Char != ' ' {
		t.Error("Cell in range should be cleared")
	}
	if b.GetCell(0, 3).Char != ' ' {
		t.Error("Cell in range should be cleared")
	}
	if b.GetCell(0, 4).Char != 'E' {
		t.Error("Cell after range should be unchanged")
	}
}

func TestBuffer_ClearToEndOfLine(t *testing.T) {
	b := NewBuffer(0)
	for i := 0; i < 5; i++ {
		b.SetCell(0, i, rune('A'+i), DefaultStyle())
	}

	b.ClearToEndOfLine(0, 2)

	if b.GetCell(0, 0).Char != 'A' {
		t.Error("Cell before cursor should be unchanged")
	}
	if b.GetCell(0, 1).Char != 'B' {
		t.Error("Cell before cursor should be unchanged")
	}
	if b.GetCell(0, 2).Char != ' ' {
		t.Error("Cell at cursor should be cleared")
	}
	if b.GetCell(0, 4).Char != ' ' {
		t.Error("Cell after cursor should be cleared")
	}
}

func TestBuffer_ClearToStartOfLine(t *testing.T) {
	b := NewBuffer(0)
	for i := 0; i < 5; i++ {
		b.SetCell(0, i, rune('A'+i), DefaultStyle())
	}

	b.ClearToStartOfLine(0, 2)

	if b.GetCell(0, 0).Char != ' ' {
		t.Error("Cell before cursor should be cleared")
	}
	if b.GetCell(0, 2).Char != ' ' {
		t.Error("Cell at cursor should be cleared")
	}
	if b.GetCell(0, 3).Char != 'D' {
		t.Error("Cell after cursor should be unchanged")
	}
}

func TestBuffer_Clear(t *testing.T) {
	b := NewBuffer(0)
	b.SetCell(0, 0, 'A', DefaultStyle())
	b.SetCell(1, 0, 'B', DefaultStyle())

	b.Clear()

	if b.Height() != 0 {
		t.Errorf("Height() after Clear = %d, want 0", b.Height())
	}
}

func TestBuffer_ClearFromRow(t *testing.T) {
	b := NewBuffer(0)
	for i := 0; i < 5; i++ {
		b.SetCell(i, 0, rune('A'+i), DefaultStyle())
	}

	b.ClearFromRow(2)

	if b.GetCell(0, 0).Char != 'A' {
		t.Error("Row before should be unchanged")
	}
	if b.GetCell(1, 0).Char != 'B' {
		t.Error("Row before should be unchanged")
	}
	if b.GetCell(2, 0).Char != ' ' {
		t.Error("Row at start should be cleared")
	}
	if b.GetCell(4, 0).Char != ' ' {
		t.Error("Row after should be cleared")
	}
}

func TestBuffer_ClearToRow(t *testing.T) {
	b := NewBuffer(0)
	for i := 0; i < 5; i++ {
		b.SetCell(i, 0, rune('A'+i), DefaultStyle())
	}

	b.ClearToRow(2)

	if b.GetCell(0, 0).Char != ' ' {
		t.Error("Row before should be cleared")
	}
	if b.GetCell(2, 0).Char != ' ' {
		t.Error("Row at end should be cleared")
	}
	if b.GetCell(3, 0).Char != 'D' {
		t.Error("Row after should be unchanged")
	}
}

func TestBuffer_RowToString(t *testing.T) {
	b := NewBuffer(0)
	b.SetCell(0, 0, 'H', DefaultStyle())
	b.SetCell(0, 1, 'i', DefaultStyle())
	b.SetCell(0, 2, '!', DefaultStyle())

	s := b.RowToString(0)
	if s != "Hi!" {
		t.Errorf("RowToString(0) = %q, want %q", s, "Hi!")
	}

	// Non-existent row
	if b.RowToString(100) != "" {
		t.Error("Non-existent row should return empty string")
	}
}

func TestBuffer_String(t *testing.T) {
	b := NewBuffer(0)
	b.SetCell(0, 0, 'A', DefaultStyle())
	b.SetCell(0, 1, 'B', DefaultStyle())
	b.SetCell(1, 0, 'C', DefaultStyle())
	b.SetCell(1, 1, 'D', DefaultStyle())

	s := b.String()
	want := "AB\nCD"
	if s != want {
		t.Errorf("String() = %q, want %q", s, want)
	}
}

func TestBuffer_GetRowSpans(t *testing.T) {
	b := NewBuffer(0)
	style1 := DefaultStyle()
	style2 := DefaultStyle().WithFg(IndexedColor(ColorRed))

	b.SetCell(0, 0, 'A', style1)
	b.SetCell(0, 1, 'B', style1)
	b.SetCell(0, 2, 'C', style2)
	b.SetCell(0, 3, 'D', style2)
	b.SetCell(0, 4, 'E', style1)

	spans := b.GetRowSpans(0)
	if len(spans) != 3 {
		t.Fatalf("len(spans) = %d, want 3", len(spans))
	}

	if spans[0].Text != "AB" || !spans[0].Style.Equal(style1) {
		t.Errorf("spans[0] = %+v, want AB with style1", spans[0])
	}
	if spans[1].Text != "CD" || !spans[1].Style.Equal(style2) {
		t.Errorf("spans[1] = %+v, want CD with style2", spans[1])
	}
	if spans[2].Text != "E" || !spans[2].Style.Equal(style1) {
		t.Errorf("spans[2] = %+v, want E with style1", spans[2])
	}

	// Non-existent row
	if b.GetRowSpans(100) != nil {
		t.Error("Non-existent row should return nil")
	}
}

func TestBuffer_GetAllSpans(t *testing.T) {
	b := NewBuffer(0)
	b.SetCell(0, 0, 'A', DefaultStyle())
	b.SetCell(1, 0, 'B', DefaultStyle())

	allSpans := b.GetAllSpans()
	if len(allSpans) != 2 {
		t.Errorf("len(GetAllSpans()) = %d, want 2", len(allSpans))
	}
}

func TestBuffer_InsertRow(t *testing.T) {
	b := NewBuffer(0)
	b.SetCell(0, 0, 'A', DefaultStyle())
	b.SetCell(1, 0, 'B', DefaultStyle())
	b.SetCell(2, 0, 'C', DefaultStyle())

	b.InsertRow(1)

	if b.Height() != 4 {
		t.Errorf("Height() = %d, want 4", b.Height())
	}
	if b.GetCell(0, 0).Char != 'A' {
		t.Error("Row 0 should be unchanged")
	}
	if b.RowWidth(1) != 0 {
		t.Error("Inserted row should be empty")
	}
	if b.GetCell(2, 0).Char != 'B' {
		t.Error("Row 2 should have old row 1 content")
	}
	if b.GetCell(3, 0).Char != 'C' {
		t.Error("Row 3 should have old row 2 content")
	}
}

func TestBuffer_DeleteRow(t *testing.T) {
	b := NewBuffer(0)
	b.SetCell(0, 0, 'A', DefaultStyle())
	b.SetCell(1, 0, 'B', DefaultStyle())
	b.SetCell(2, 0, 'C', DefaultStyle())

	b.DeleteRow(1)

	if b.Height() != 2 {
		t.Errorf("Height() = %d, want 2", b.Height())
	}
	if b.GetCell(0, 0).Char != 'A' {
		t.Error("Row 0 should be unchanged")
	}
	if b.GetCell(1, 0).Char != 'C' {
		t.Error("Row 1 should have old row 2 content")
	}
}

func TestBuffer_TrimTrailingEmptyRows(t *testing.T) {
	b := NewBuffer(0)
	b.SetCell(0, 0, 'A', DefaultStyle())
	b.SetCell(1, 0, ' ', DefaultStyle())
	b.SetCell(2, 0, ' ', DefaultStyle())

	b.TrimTrailingEmptyRows()

	if b.Height() != 1 {
		t.Errorf("Height() = %d, want 1", b.Height())
	}
	if b.GetCell(0, 0).Char != 'A' {
		t.Error("Non-empty row should remain")
	}
}

func TestBuffer_TrimTrailingEmptyRows_KeepsStyledCells(t *testing.T) {
	b := NewBuffer(0)
	b.SetCell(0, 0, 'A', DefaultStyle())
	// Row 1 has a space with non-default style - should NOT be trimmed
	b.SetCell(1, 0, ' ', DefaultStyle().WithFg(IndexedColor(ColorRed)))
	b.SetCell(2, 0, ' ', DefaultStyle())

	b.TrimTrailingEmptyRows()

	// Row 2 should be trimmed, but row 1 kept because of styled space
	if b.Height() != 2 {
		t.Errorf("Height() = %d, want 2", b.Height())
	}
}
