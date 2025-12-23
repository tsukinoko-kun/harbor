package terminal

// ParserState represents the current state of the ANSI parser.
type ParserState int

const (
	// StateNormal is the default state, processing regular characters.
	StateNormal ParserState = iota
	// StateEscape is entered after receiving ESC (0x1B).
	StateEscape
	// StateCSI is entered after receiving ESC[ (CSI - Control Sequence Introducer).
	StateCSI
	// StateCSIParam is collecting parameters in a CSI sequence.
	StateCSIParam
	// StateOSC is entered after receiving ESC] (OSC - Operating System Command).
	StateOSC
)

// Action represents an action to be taken by the terminal.
type Action interface {
	isAction()
}

// ActionPrint represents printing a character at the current cursor position.
type ActionPrint struct {
	Char rune
}

func (ActionPrint) isAction() {}

// ActionExecute represents executing a C0/C1 control character.
type ActionExecute struct {
	Char byte
}

func (ActionExecute) isAction() {}

// ActionCSI represents a CSI (Control Sequence Introducer) sequence.
type ActionCSI struct {
	Params       []int
	Intermediate []byte
	Final        byte
}

func (ActionCSI) isAction() {}

// ActionOSC represents an OSC (Operating System Command) sequence.
type ActionOSC struct {
	Data []byte
}

func (ActionOSC) isAction() {}

// Parser is an ANSI escape sequence parser.
// It processes input bytes and emits Actions.
type Parser struct {
	state        ParserState
	params       []int  // current parameter values
	curParam     int    // current parameter being built (-1 if not started)
	intermediate []byte // intermediate bytes
	oscData      []byte // OSC payload
}

// NewParser creates a new ANSI parser.
func NewParser() *Parser {
	return &Parser{
		state:    StateNormal,
		curParam: -1,
	}
}

// Reset resets the parser to its initial state.
func (p *Parser) Reset() {
	p.state = StateNormal
	p.params = p.params[:0]
	p.curParam = -1
	p.intermediate = p.intermediate[:0]
	p.oscData = p.oscData[:0]
}

// Parse processes input bytes and returns a slice of Actions.
func (p *Parser) Parse(input []byte) []Action {
	var actions []Action

	for _, b := range input {
		a := p.processByte(b)
		if a != nil {
			actions = append(actions, a...)
		}
	}

	return actions
}

// processByte processes a single byte and returns any resulting actions.
func (p *Parser) processByte(b byte) []Action {
	switch p.state {
	case StateNormal:
		return p.processNormal(b)
	case StateEscape:
		return p.processEscape(b)
	case StateCSI, StateCSIParam:
		return p.processCSI(b)
	case StateOSC:
		return p.processOSC(b)
	default:
		p.state = StateNormal
		return nil
	}
}

// processNormal handles bytes in the normal state.
func (p *Parser) processNormal(b byte) []Action {
	switch {
	case b == 0x1B: // ESC
		p.state = StateEscape
		return nil

	case b < 0x20: // C0 control characters
		return []Action{ActionExecute{Char: b}}

	case b == 0x7F: // DEL
		return nil // ignore

	default:
		// Regular printable character or start of UTF-8
		// For simplicity, we'll pass bytes as runes directly
		// A more complete implementation would handle UTF-8 decoding
		return []Action{ActionPrint{Char: rune(b)}}
	}
}

// processEscape handles bytes after receiving ESC.
func (p *Parser) processEscape(b byte) []Action {
	switch b {
	case '[': // CSI
		p.state = StateCSI
		p.params = p.params[:0]
		p.curParam = -1
		p.intermediate = p.intermediate[:0]
		return nil

	case ']': // OSC
		p.state = StateOSC
		p.oscData = p.oscData[:0]
		return nil

	case 0x1B: // Another ESC - stay in escape state
		return nil

	case 'c': // RIS - Reset to Initial State
		p.state = StateNormal
		return []Action{ActionCSI{Final: 'c', Params: nil}}

	default:
		// Unknown escape sequence, return to normal
		p.state = StateNormal
		// Could emit the ESC + byte as prints, but most terminals ignore unknown
		return nil
	}
}

// processCSI handles bytes in CSI sequences (ESC[...).
func (p *Parser) processCSI(b byte) []Action {
	switch {
	case b >= '0' && b <= '9':
		// Parameter digit
		p.state = StateCSIParam
		if p.curParam < 0 {
			p.curParam = 0
		}
		p.curParam = p.curParam*10 + int(b-'0')
		return nil

	case b == ';':
		// Parameter separator
		p.state = StateCSIParam
		if p.curParam >= 0 {
			p.params = append(p.params, p.curParam)
		} else {
			// Empty parameter, use default (0)
			p.params = append(p.params, 0)
		}
		p.curParam = -1
		return nil

	case b == ':':
		// Subparameter separator (used in some sequences like SGR)
		// For now, treat like ';'
		p.state = StateCSIParam
		if p.curParam >= 0 {
			p.params = append(p.params, p.curParam)
		} else {
			p.params = append(p.params, 0)
		}
		p.curParam = -1
		return nil

	case b >= 0x20 && b <= 0x2F:
		// Intermediate bytes (space through /)
		p.intermediate = append(p.intermediate, b)
		return nil

	case b >= 0x40 && b <= 0x7E:
		// Final byte - sequence complete
		if p.curParam >= 0 {
			p.params = append(p.params, p.curParam)
		}

		action := ActionCSI{
			Params:       make([]int, len(p.params)),
			Intermediate: make([]byte, len(p.intermediate)),
			Final:        b,
		}
		copy(action.Params, p.params)
		copy(action.Intermediate, p.intermediate)

		p.state = StateNormal
		return []Action{action}

	case b == 0x1B:
		// ESC in middle of sequence - abort and start new escape
		p.state = StateEscape
		return nil

	default:
		// Ignore other bytes in CSI
		return nil
	}
}

// processOSC handles bytes in OSC sequences (ESC]...).
func (p *Parser) processOSC(b byte) []Action {
	switch b {
	case 0x07: // BEL - OSC terminator
		p.state = StateNormal
		return []Action{ActionOSC{Data: append([]byte(nil), p.oscData...)}}

	case 0x1B: // ESC - might be ST (ESC \)
		// For simplicity, treat as terminator
		p.state = StateNormal
		return []Action{ActionOSC{Data: append([]byte(nil), p.oscData...)}}

	default:
		p.oscData = append(p.oscData, b)
		return nil
	}
}

// IsCSI returns true if the action is a CSI sequence.
func IsCSI(a Action) bool {
	_, ok := a.(ActionCSI)
	return ok
}

// IsPrint returns true if the action is a print action.
func IsPrint(a Action) bool {
	_, ok := a.(ActionPrint)
	return ok
}

// IsExecute returns true if the action is an execute action.
func IsExecute(a Action) bool {
	_, ok := a.(ActionExecute)
	return ok
}

// CSICommand identifies common CSI commands.
type CSICommand int

const (
	CSIUnknown        CSICommand = iota
	CSICursorUp                  // A
	CSICursorDown                // B
	CSICursorForward             // C
	CSICursorBack                // D
	CSICursorNextLine            // E
	CSICursorPrevLine            // F
	CSICursorColumn              // G
	CSICursorPosition            // H (and f)
	CSIEraseDisplay              // J
	CSIEraseLine                 // K
	CSIScrollUp                  // S
	CSIScrollDown                // T
	CSISGR                       // m (Select Graphic Rendition)
	CSISaveCursor                // s
	CSIRestoreCursor             // u
)

// GetCSICommand returns the command type for a CSI action.
func GetCSICommand(csi ActionCSI) CSICommand {
	switch csi.Final {
	case 'A':
		return CSICursorUp
	case 'B':
		return CSICursorDown
	case 'C':
		return CSICursorForward
	case 'D':
		return CSICursorBack
	case 'E':
		return CSICursorNextLine
	case 'F':
		return CSICursorPrevLine
	case 'G':
		return CSICursorColumn
	case 'H', 'f':
		return CSICursorPosition
	case 'J':
		return CSIEraseDisplay
	case 'K':
		return CSIEraseLine
	case 'S':
		return CSIScrollUp
	case 'T':
		return CSIScrollDown
	case 'm':
		return CSISGR
	case 's':
		return CSISaveCursor
	case 'u':
		return CSIRestoreCursor
	default:
		return CSIUnknown
	}
}

// GetParam returns the parameter at the given index, or the default value if not present.
func (csi ActionCSI) GetParam(index, defaultValue int) int {
	if index < 0 || index >= len(csi.Params) {
		return defaultValue
	}
	if csi.Params[index] == 0 {
		return defaultValue
	}
	return csi.Params[index]
}
