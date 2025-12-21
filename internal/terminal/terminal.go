package terminal

import (
	"sync"
	"unicode/utf8"
)

// Terminal is a virtual terminal emulator that processes ANSI escape sequences.
// It maintains a 2D buffer of styled cells and a cursor position.
type Terminal struct {
	mu           sync.RWMutex
	buffer       *Buffer
	parser       *Parser
	cursorRow    int
	cursorCol    int
	currentStyle Style
	savedCursor  struct {
		row, col int
		style    Style
	}

	// UTF-8 decoding state
	utf8Buf   []byte
	utf8Need  int
}

// New creates a new Terminal with unbounded width.
func New() *Terminal {
	return &Terminal{
		buffer:       NewBuffer(0),
		parser:       NewParser(),
		currentStyle: DefaultStyle(),
	}
}

// NewWithWidth creates a new Terminal with a fixed width.
// Lines will wrap at this width.
func NewWithWidth(width int) *Terminal {
	return &Terminal{
		buffer:       NewBuffer(width),
		parser:       NewParser(),
		currentStyle: DefaultStyle(),
	}
}

// ResetStyle resets the current style to default.
// This is useful before a new command to ensure fresh styling.
func (t *Terminal) ResetStyle() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.currentStyle = DefaultStyle()
}

// Write implements io.Writer. It processes input bytes including ANSI escape sequences.
func (t *Terminal) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// First, handle UTF-8 decoding
	decoded := t.decodeUTF8(p)

	// Parse ANSI sequences
	actions := t.parser.Parse(decoded)

	// Execute actions
	for _, action := range actions {
		t.executeAction(action)
	}

	return len(p), nil
}

// decodeUTF8 handles multi-byte UTF-8 sequences that may be split across Write calls.
func (t *Terminal) decodeUTF8(p []byte) []byte {
	if len(t.utf8Buf) == 0 && t.utf8Need == 0 {
		// Fast path: no pending UTF-8 bytes
		return p
	}

	// Combine pending bytes with new input
	combined := append(t.utf8Buf, p...)
	t.utf8Buf = nil
	t.utf8Need = 0

	// Check for incomplete UTF-8 at end
	for i := len(combined) - 1; i >= 0 && i > len(combined)-4; i-- {
		if combined[i]&0xC0 == 0x80 {
			// Continuation byte, keep looking
			continue
		}
		// Start byte found, check if complete
		r, size := utf8.DecodeRune(combined[i:])
		if r == utf8.RuneError && size == 1 {
			// Incomplete sequence at end, save for next Write
			t.utf8Buf = append([]byte(nil), combined[i:]...)
			return combined[:i]
		}
		break
	}

	return combined
}

// executeAction executes a parsed action.
func (t *Terminal) executeAction(action Action) {
	switch a := action.(type) {
	case ActionPrint:
		t.printRune(a.Char)
	case ActionExecute:
		t.executeControl(a.Char)
	case ActionCSI:
		t.executeCSI(a)
	case ActionOSC:
		// OSC sequences are typically for window titles, etc.
		// We'll ignore them for now
	}
}

// printRune prints a character at the current cursor position.
func (t *Terminal) printRune(r rune) {
	t.buffer.SetCell(t.cursorRow, t.cursorCol, r, t.currentStyle)
	t.cursorCol++

	// Handle line wrapping if width is set
	width := t.buffer.Width()
	if width > 0 && t.cursorCol >= width {
		t.cursorCol = 0
		t.cursorRow++
	}
}

// executeControl executes a C0 control character.
func (t *Terminal) executeControl(c byte) {
	switch c {
	case '\n': // Line Feed
		t.cursorRow++
		t.cursorCol = 0 // Auto-CR on LF (practical behavior for most terminal output)

	case '\r': // Carriage Return
		t.cursorCol = 0

	case '\t': // Tab
		// Move to next tab stop (every 8 columns)
		t.cursorCol = ((t.cursorCol / 8) + 1) * 8

	case '\b': // Backspace
		if t.cursorCol > 0 {
			t.cursorCol--
		}

	case 0x07: // BEL
		// Bell - could trigger a notification, but we'll ignore

	case 0x0C: // Form Feed (often treated as clear screen)
		t.buffer.Clear()
		t.cursorRow = 0
		t.cursorCol = 0
	}
}

// executeCSI executes a CSI (Control Sequence Introducer) sequence.
func (t *Terminal) executeCSI(csi ActionCSI) {
	cmd := GetCSICommand(csi)

	switch cmd {
	case CSICursorUp:
		n := csi.GetParam(0, 1)
		t.cursorRow -= n
		if t.cursorRow < 0 {
			t.cursorRow = 0
		}

	case CSICursorDown:
		n := csi.GetParam(0, 1)
		t.cursorRow += n

	case CSICursorForward:
		n := csi.GetParam(0, 1)
		t.cursorCol += n

	case CSICursorBack:
		n := csi.GetParam(0, 1)
		t.cursorCol -= n
		if t.cursorCol < 0 {
			t.cursorCol = 0
		}

	case CSICursorNextLine:
		n := csi.GetParam(0, 1)
		t.cursorRow += n
		t.cursorCol = 0

	case CSICursorPrevLine:
		n := csi.GetParam(0, 1)
		t.cursorRow -= n
		if t.cursorRow < 0 {
			t.cursorRow = 0
		}
		t.cursorCol = 0

	case CSICursorColumn:
		// Column is 1-indexed in ANSI
		col := csi.GetParam(0, 1) - 1
		if col < 0 {
			col = 0
		}
		t.cursorCol = col

	case CSICursorPosition:
		// Row and column are 1-indexed in ANSI
		row := csi.GetParam(0, 1) - 1
		col := csi.GetParam(1, 1) - 1
		if row < 0 {
			row = 0
		}
		if col < 0 {
			col = 0
		}
		t.cursorRow = row
		t.cursorCol = col

	case CSIEraseDisplay:
		mode := csi.GetParam(0, 0)
		switch mode {
		case 0: // Erase from cursor to end of screen
			t.buffer.ClearToEndOfLine(t.cursorRow, t.cursorCol)
			t.buffer.ClearFromRow(t.cursorRow + 1)
		case 1: // Erase from start to cursor
			t.buffer.ClearToStartOfLine(t.cursorRow, t.cursorCol)
			t.buffer.ClearToRow(t.cursorRow - 1)
		case 2, 3: // Erase entire screen (3 also clears scrollback, same for us)
			t.buffer.Clear()
			t.cursorRow = 0
			t.cursorCol = 0
		}

	case CSIEraseLine:
		mode := csi.GetParam(0, 0)
		switch mode {
		case 0: // Erase from cursor to end of line
			t.buffer.ClearToEndOfLine(t.cursorRow, t.cursorCol)
		case 1: // Erase from start of line to cursor
			t.buffer.ClearToStartOfLine(t.cursorRow, t.cursorCol)
		case 2: // Erase entire line
			t.buffer.ClearRow(t.cursorRow)
		}

	case CSISGR:
		// Apply all SGR parameters
		t.currentStyle = t.currentStyle.ApplySGRParams(csi.Params)

	case CSISaveCursor:
		t.savedCursor.row = t.cursorRow
		t.savedCursor.col = t.cursorCol
		t.savedCursor.style = t.currentStyle

	case CSIRestoreCursor:
		t.cursorRow = t.savedCursor.row
		t.cursorCol = t.savedCursor.col
		t.currentStyle = t.savedCursor.style

	case CSIScrollUp:
		// Scroll up - delete lines from top
		n := csi.GetParam(0, 1)
		for i := 0; i < n; i++ {
			t.buffer.DeleteRow(0)
		}

	case CSIScrollDown:
		// Scroll down - insert lines at top
		n := csi.GetParam(0, 1)
		for i := 0; i < n; i++ {
			t.buffer.InsertRow(0)
		}
	}
}

// String returns the buffer content as a plain string (for debugging).
func (t *Terminal) String() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.buffer.String()
}

// Lines returns the number of lines in the terminal buffer.
func (t *Terminal) Lines() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.buffer.Height()
}

// GetLine returns the content of a line as a string.
func (t *Terminal) GetLine(row int) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.buffer.RowToString(row)
}

// GetLineSpans returns a line as styled spans for rendering.
func (t *Terminal) GetLineSpans(row int) []StyledSpan {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.buffer.GetRowSpans(row)
}

// GetAllSpans returns all lines as styled spans.
func (t *Terminal) GetAllSpans() [][]StyledSpan {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.buffer.GetAllSpans()
}

// CursorPosition returns the current cursor position (row, col).
func (t *Terminal) CursorPosition() (row, col int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.cursorRow, t.cursorCol
}

// Reset clears the terminal and resets all state.
func (t *Terminal) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buffer.Clear()
	t.parser.Reset()
	t.cursorRow = 0
	t.cursorCol = 0
	t.currentStyle = DefaultStyle()
	t.utf8Buf = nil
	t.utf8Need = 0
}

// CurrentStyle returns the current text style.
func (t *Terminal) CurrentStyle() Style {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.currentStyle
}

