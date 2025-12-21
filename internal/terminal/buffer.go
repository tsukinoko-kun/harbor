package terminal

// Cell represents a single character cell in the terminal buffer.
type Cell struct {
	Char  rune
	Style Style
}

// EmptyCell returns an empty cell with default style.
func EmptyCell() Cell {
	return Cell{
		Char:  ' ',
		Style: DefaultStyle(),
	}
}

// Buffer represents a 2D grid of terminal cells.
// Row 0 is the top, column 0 is the left.
type Buffer struct {
	cells [][]Cell
	width int // current max width (0 means unbounded)
}

// NewBuffer creates a new empty buffer.
// If width is 0, the buffer width is unbounded (grows as needed).
func NewBuffer(width int) *Buffer {
	return &Buffer{
		cells: make([][]Cell, 0),
		width: width,
	}
}

// Height returns the number of rows in the buffer.
func (b *Buffer) Height() int {
	return len(b.cells)
}

// Width returns the configured width (0 if unbounded).
func (b *Buffer) Width() int {
	return b.width
}

// RowWidth returns the actual width of a specific row.
func (b *Buffer) RowWidth(row int) int {
	if row < 0 || row >= len(b.cells) {
		return 0
	}
	return len(b.cells[row])
}

// EnsureRows ensures the buffer has at least n rows.
func (b *Buffer) EnsureRows(n int) {
	for len(b.cells) < n {
		b.cells = append(b.cells, make([]Cell, 0))
	}
}

// EnsureCell ensures a cell exists at (row, col), expanding as needed.
// Fills gaps with empty cells.
func (b *Buffer) EnsureCell(row, col int) {
	b.EnsureRows(row + 1)

	// Expand the row to include col
	for len(b.cells[row]) <= col {
		b.cells[row] = append(b.cells[row], EmptyCell())
	}
}

// SetCell sets the cell at (row, col) to the given character and style.
// Expands the buffer as needed.
func (b *Buffer) SetCell(row, col int, char rune, style Style) {
	b.EnsureCell(row, col)
	b.cells[row][col] = Cell{Char: char, Style: style}
}

// GetCell returns the cell at (row, col).
// Returns an empty cell if out of bounds.
func (b *Buffer) GetCell(row, col int) Cell {
	if row < 0 || row >= len(b.cells) {
		return EmptyCell()
	}
	if col < 0 || col >= len(b.cells[row]) {
		return EmptyCell()
	}
	return b.cells[row][col]
}

// GetRow returns a copy of the cells in the specified row.
// Returns nil if the row doesn't exist.
func (b *Buffer) GetRow(row int) []Cell {
	if row < 0 || row >= len(b.cells) {
		return nil
	}
	result := make([]Cell, len(b.cells[row]))
	copy(result, b.cells[row])
	return result
}

// ClearRow clears all cells in a row (sets to empty cells).
// Does nothing if the row doesn't exist.
func (b *Buffer) ClearRow(row int) {
	if row < 0 || row >= len(b.cells) {
		return
	}
	for i := range b.cells[row] {
		b.cells[row][i] = EmptyCell()
	}
}

// ClearRowRange clears cells in a row from startCol to endCol (exclusive).
// If endCol is -1, clears to end of row.
func (b *Buffer) ClearRowRange(row, startCol, endCol int) {
	if row < 0 || row >= len(b.cells) {
		return
	}
	if startCol < 0 {
		startCol = 0
	}
	if endCol < 0 || endCol > len(b.cells[row]) {
		endCol = len(b.cells[row])
	}
	for i := startCol; i < endCol; i++ {
		b.cells[row][i] = EmptyCell()
	}
}

// ClearToEndOfLine clears from the given column to the end of the row.
func (b *Buffer) ClearToEndOfLine(row, col int) {
	b.ClearRowRange(row, col, -1)
}

// ClearToStartOfLine clears from the start of the row to the given column (inclusive).
func (b *Buffer) ClearToStartOfLine(row, col int) {
	b.ClearRowRange(row, 0, col+1)
}

// Clear clears the entire buffer.
func (b *Buffer) Clear() {
	b.cells = make([][]Cell, 0)
}

// ClearFromRow clears all rows from the given row to the end of the buffer.
func (b *Buffer) ClearFromRow(row int) {
	if row < 0 {
		row = 0
	}
	if row >= len(b.cells) {
		return
	}
	for i := row; i < len(b.cells); i++ {
		b.ClearRow(i)
	}
}

// ClearToRow clears all rows from the start up to and including the given row.
func (b *Buffer) ClearToRow(row int) {
	if row < 0 {
		return
	}
	if row >= len(b.cells) {
		row = len(b.cells) - 1
	}
	for i := 0; i <= row; i++ {
		b.ClearRow(i)
	}
}

// TrimTrailingEmptyRows removes trailing empty rows from the buffer.
func (b *Buffer) TrimTrailingEmptyRows() {
	for len(b.cells) > 0 {
		lastRow := b.cells[len(b.cells)-1]
		isEmpty := true
		for _, cell := range lastRow {
			if cell.Char != ' ' || !cell.Style.Equal(DefaultStyle()) {
				isEmpty = false
				break
			}
		}
		if isEmpty {
			b.cells = b.cells[:len(b.cells)-1]
		} else {
			break
		}
	}
}

// StyledSpan represents a run of characters with the same style.
type StyledSpan struct {
	Text  string
	Style Style
}

// GetRowSpans returns the cells of a row as a slice of styled spans.
// Consecutive cells with the same style are merged into a single span.
// Returns nil if the row doesn't exist.
func (b *Buffer) GetRowSpans(row int) []StyledSpan {
	if row < 0 || row >= len(b.cells) {
		return nil
	}

	cells := b.cells[row]
	if len(cells) == 0 {
		return nil
	}

	var spans []StyledSpan
	var currentText []rune
	currentStyle := cells[0].Style

	for _, cell := range cells {
		if cell.Style.Equal(currentStyle) {
			currentText = append(currentText, cell.Char)
		} else {
			// Flush current span
			if len(currentText) > 0 {
				spans = append(spans, StyledSpan{
					Text:  string(currentText),
					Style: currentStyle,
				})
			}
			currentText = []rune{cell.Char}
			currentStyle = cell.Style
		}
	}

	// Flush final span
	if len(currentText) > 0 {
		spans = append(spans, StyledSpan{
			Text:  string(currentText),
			Style: currentStyle,
		})
	}

	return spans
}

// GetAllSpans returns all rows as slices of styled spans.
func (b *Buffer) GetAllSpans() [][]StyledSpan {
	result := make([][]StyledSpan, len(b.cells))
	for i := range b.cells {
		result[i] = b.GetRowSpans(i)
	}
	return result
}

// RowToString converts a row to a string (for debugging/testing).
func (b *Buffer) RowToString(row int) string {
	if row < 0 || row >= len(b.cells) {
		return ""
	}
	runes := make([]rune, len(b.cells[row]))
	for i, cell := range b.cells[row] {
		runes[i] = cell.Char
	}
	return string(runes)
}

// String returns the entire buffer as a string with newlines (for debugging).
func (b *Buffer) String() string {
	if len(b.cells) == 0 {
		return ""
	}
	var result []rune
	for i, row := range b.cells {
		for _, cell := range row {
			result = append(result, cell.Char)
		}
		if i < len(b.cells)-1 {
			result = append(result, '\n')
		}
	}
	return string(result)
}

// InsertRow inserts a new empty row at the given position, shifting existing rows down.
func (b *Buffer) InsertRow(row int) {
	if row < 0 {
		row = 0
	}
	if row > len(b.cells) {
		b.EnsureRows(row + 1)
		return
	}
	b.cells = append(b.cells, nil)
	copy(b.cells[row+1:], b.cells[row:])
	b.cells[row] = make([]Cell, 0)
}

// DeleteRow deletes the row at the given position, shifting subsequent rows up.
func (b *Buffer) DeleteRow(row int) {
	if row < 0 || row >= len(b.cells) {
		return
	}
	b.cells = append(b.cells[:row], b.cells[row+1:]...)
}

