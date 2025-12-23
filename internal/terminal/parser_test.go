package terminal

import (
	"reflect"
	"testing"
)

func TestParser_PlainText(t *testing.T) {
	p := NewParser()
	actions := p.Parse([]byte("Hello"))

	if len(actions) != 5 {
		t.Fatalf("len(actions) = %d, want 5", len(actions))
	}

	want := []rune{'H', 'e', 'l', 'l', 'o'}
	for i, a := range actions {
		print, ok := a.(ActionPrint)
		if !ok {
			t.Errorf("actions[%d] is not ActionPrint", i)
			continue
		}
		if print.Char != want[i] {
			t.Errorf("actions[%d].Char = %q, want %q", i, print.Char, want[i])
		}
	}
}

func TestParser_ControlCharacters(t *testing.T) {
	p := NewParser()

	tests := []struct {
		input byte
		want  byte
	}{
		{'\n', '\n'},
		{'\r', '\r'},
		{'\t', '\t'},
		{'\b', '\b'},
		{0x07, 0x07}, // BEL
	}

	for _, tt := range tests {
		actions := p.Parse([]byte{tt.input})
		if len(actions) != 1 {
			t.Errorf("Parse(%q): len(actions) = %d, want 1", tt.input, len(actions))
			continue
		}
		exec, ok := actions[0].(ActionExecute)
		if !ok {
			t.Errorf("Parse(%q): not ActionExecute", tt.input)
			continue
		}
		if exec.Char != tt.want {
			t.Errorf("Parse(%q).Char = %d, want %d", tt.input, exec.Char, tt.want)
		}
	}
}

func TestParser_CSI_NoParams(t *testing.T) {
	p := NewParser()
	// ESC [ H - cursor home
	actions := p.Parse([]byte("\x1b[H"))

	if len(actions) != 1 {
		t.Fatalf("len(actions) = %d, want 1", len(actions))
	}

	csi, ok := actions[0].(ActionCSI)
	if !ok {
		t.Fatal("action is not ActionCSI")
	}

	if csi.Final != 'H' {
		t.Errorf("Final = %q, want 'H'", csi.Final)
	}
	if len(csi.Params) != 0 {
		t.Errorf("len(Params) = %d, want 0", len(csi.Params))
	}
}

func TestParser_CSI_SingleParam(t *testing.T) {
	p := NewParser()
	// ESC [ 5 A - cursor up 5
	actions := p.Parse([]byte("\x1b[5A"))

	if len(actions) != 1 {
		t.Fatalf("len(actions) = %d, want 1", len(actions))
	}

	csi, ok := actions[0].(ActionCSI)
	if !ok {
		t.Fatal("action is not ActionCSI")
	}

	if csi.Final != 'A' {
		t.Errorf("Final = %q, want 'A'", csi.Final)
	}
	if !reflect.DeepEqual(csi.Params, []int{5}) {
		t.Errorf("Params = %v, want [5]", csi.Params)
	}
}

func TestParser_CSI_MultipleParams(t *testing.T) {
	p := NewParser()
	// ESC [ 10 ; 20 H - cursor to row 10, col 20
	actions := p.Parse([]byte("\x1b[10;20H"))

	if len(actions) != 1 {
		t.Fatalf("len(actions) = %d, want 1", len(actions))
	}

	csi := actions[0].(ActionCSI)
	if !reflect.DeepEqual(csi.Params, []int{10, 20}) {
		t.Errorf("Params = %v, want [10, 20]", csi.Params)
	}
}

func TestParser_CSI_EmptyParams(t *testing.T) {
	p := NewParser()
	// ESC [ ; ; m - SGR with empty params (defaults to 0)
	// Two semicolons create two parameter boundaries -> [0, 0]
	actions := p.Parse([]byte("\x1b[;;m"))

	if len(actions) != 1 {
		t.Fatalf("len(actions) = %d, want 1", len(actions))
	}

	csi := actions[0].(ActionCSI)
	if !reflect.DeepEqual(csi.Params, []int{0, 0}) {
		t.Errorf("Params = %v, want [0, 0]", csi.Params)
	}
}

func TestParser_CSI_SGR_Colors(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name   string
		input  string
		params []int
	}{
		{"reset", "\x1b[0m", []int{0}},
		{"bold", "\x1b[1m", []int{1}},
		{"red fg", "\x1b[31m", []int{31}},
		{"blue bg", "\x1b[44m", []int{44}},
		{"bold red", "\x1b[1;31m", []int{1, 31}},
		{"256 fg", "\x1b[38;5;196m", []int{38, 5, 196}},
		{"256 bg", "\x1b[48;5;21m", []int{48, 5, 21}},
		{"24bit fg", "\x1b[38;2;128;64;32m", []int{38, 2, 128, 64, 32}},
		{"24bit bg", "\x1b[48;2;255;128;0m", []int{48, 2, 255, 128, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p.Reset()
			actions := p.Parse([]byte(tt.input))
			if len(actions) != 1 {
				t.Fatalf("len(actions) = %d, want 1", len(actions))
			}
			csi := actions[0].(ActionCSI)
			if csi.Final != 'm' {
				t.Errorf("Final = %q, want 'm'", csi.Final)
			}
			if !reflect.DeepEqual(csi.Params, tt.params) {
				t.Errorf("Params = %v, want %v", csi.Params, tt.params)
			}
		})
	}
}

func TestParser_CSI_CursorMovement(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		final  byte
		params []int
	}{
		{"up", "\x1b[3A", 'A', []int{3}},
		{"down", "\x1b[2B", 'B', []int{2}},
		{"forward", "\x1b[4C", 'C', []int{4}},
		{"back", "\x1b[1D", 'D', []int{1}},
		{"position", "\x1b[5;10H", 'H', []int{5, 10}},
		{"column", "\x1b[15G", 'G', []int{15}},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p.Reset()
			actions := p.Parse([]byte(tt.input))
			if len(actions) != 1 {
				t.Fatalf("len(actions) = %d, want 1", len(actions))
			}
			csi := actions[0].(ActionCSI)
			if csi.Final != tt.final {
				t.Errorf("Final = %q, want %q", csi.Final, tt.final)
			}
			if !reflect.DeepEqual(csi.Params, tt.params) {
				t.Errorf("Params = %v, want %v", csi.Params, tt.params)
			}
		})
	}
}

func TestParser_CSI_Erase(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		final  byte
		params []int
	}{
		{"erase to end of line", "\x1b[K", 'K', nil},
		{"erase to end of line explicit", "\x1b[0K", 'K', []int{0}},
		{"erase to start of line", "\x1b[1K", 'K', []int{1}},
		{"erase entire line", "\x1b[2K", 'K', []int{2}},
		{"erase to end of screen", "\x1b[J", 'J', nil},
		{"erase to start of screen", "\x1b[1J", 'J', []int{1}},
		{"erase entire screen", "\x1b[2J", 'J', []int{2}},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p.Reset()
			actions := p.Parse([]byte(tt.input))
			if len(actions) != 1 {
				t.Fatalf("len(actions) = %d, want 1", len(actions))
			}
			csi := actions[0].(ActionCSI)
			if csi.Final != tt.final {
				t.Errorf("Final = %q, want %q", csi.Final, tt.final)
			}
			if len(tt.params) == 0 && len(csi.Params) != 0 {
				t.Errorf("Params = %v, want empty", csi.Params)
			} else if len(tt.params) > 0 && !reflect.DeepEqual(csi.Params, tt.params) {
				t.Errorf("Params = %v, want %v", csi.Params, tt.params)
			}
		})
	}
}

func TestParser_MixedContent(t *testing.T) {
	p := NewParser()
	// "AB" + red + "CD" + reset + "EF"
	actions := p.Parse([]byte("AB\x1b[31mCD\x1b[0mEF"))

	expected := []struct {
		typ  string
		data interface{}
	}{
		{"print", 'A'},
		{"print", 'B'},
		{"csi", []int{31}},
		{"print", 'C'},
		{"print", 'D'},
		{"csi", []int{0}},
		{"print", 'E'},
		{"print", 'F'},
	}

	if len(actions) != len(expected) {
		t.Fatalf("len(actions) = %d, want %d", len(actions), len(expected))
	}

	for i, e := range expected {
		switch e.typ {
		case "print":
			p, ok := actions[i].(ActionPrint)
			if !ok {
				t.Errorf("actions[%d] is not ActionPrint", i)
				continue
			}
			if p.Char != e.data.(rune) {
				t.Errorf("actions[%d].Char = %q, want %q", i, p.Char, e.data)
			}
		case "csi":
			c, ok := actions[i].(ActionCSI)
			if !ok {
				t.Errorf("actions[%d] is not ActionCSI", i)
				continue
			}
			if !reflect.DeepEqual(c.Params, e.data.([]int)) {
				t.Errorf("actions[%d].Params = %v, want %v", i, c.Params, e.data)
			}
		}
	}
}

func TestParser_IncompleteSequence(t *testing.T) {
	p := NewParser()

	// Send ESC only
	actions := p.Parse([]byte("\x1b"))
	if len(actions) != 0 {
		t.Errorf("ESC alone should produce no actions, got %d", len(actions))
	}

	// Send [ to continue
	actions = p.Parse([]byte("["))
	if len(actions) != 0 {
		t.Errorf("ESC[ should produce no actions, got %d", len(actions))
	}

	// Send final byte
	actions = p.Parse([]byte("H"))
	if len(actions) != 1 {
		t.Fatalf("Complete sequence should produce 1 action, got %d", len(actions))
	}

	csi, ok := actions[0].(ActionCSI)
	if !ok {
		t.Fatal("Expected ActionCSI")
	}
	if csi.Final != 'H' {
		t.Errorf("Final = %q, want 'H'", csi.Final)
	}
}

func TestParser_EscapeInMiddleOfCSI(t *testing.T) {
	p := NewParser()

	// Start a CSI, then send another ESC to interrupt
	actions := p.Parse([]byte("\x1b[5\x1b[H"))

	// Should abort first CSI and start new one
	if len(actions) != 1 {
		t.Fatalf("len(actions) = %d, want 1", len(actions))
	}

	csi := actions[0].(ActionCSI)
	if csi.Final != 'H' {
		t.Errorf("Final = %q, want 'H'", csi.Final)
	}
}

func TestParser_Reset(t *testing.T) {
	p := NewParser()

	// Start incomplete sequence
	p.Parse([]byte("\x1b[5"))

	// Reset parser
	p.Reset()

	// Normal text should work
	actions := p.Parse([]byte("A"))
	if len(actions) != 1 {
		t.Fatalf("len(actions) = %d, want 1", len(actions))
	}
	if _, ok := actions[0].(ActionPrint); !ok {
		t.Error("Expected ActionPrint after reset")
	}
}

func TestParser_OSC(t *testing.T) {
	p := NewParser()

	// OSC 0 ; title BEL - set window title
	actions := p.Parse([]byte("\x1b]0;My Title\x07"))

	if len(actions) != 1 {
		t.Fatalf("len(actions) = %d, want 1", len(actions))
	}

	osc, ok := actions[0].(ActionOSC)
	if !ok {
		t.Fatal("Expected ActionOSC")
	}

	if string(osc.Data) != "0;My Title" {
		t.Errorf("OSC.Data = %q, want %q", string(osc.Data), "0;My Title")
	}
}

func TestGetCSICommand(t *testing.T) {
	tests := []struct {
		final byte
		want  CSICommand
	}{
		{'A', CSICursorUp},
		{'B', CSICursorDown},
		{'C', CSICursorForward},
		{'D', CSICursorBack},
		{'E', CSICursorNextLine},
		{'F', CSICursorPrevLine},
		{'G', CSICursorColumn},
		{'H', CSICursorPosition},
		{'f', CSICursorPosition},
		{'J', CSIEraseDisplay},
		{'K', CSIEraseLine},
		{'S', CSIScrollUp},
		{'T', CSIScrollDown},
		{'m', CSISGR},
		{'s', CSISaveCursor},
		{'u', CSIRestoreCursor},
		{'X', CSIUnknown},
	}

	for _, tt := range tests {
		csi := ActionCSI{Final: tt.final}
		got := GetCSICommand(csi)
		if got != tt.want {
			t.Errorf("GetCSICommand(%q) = %d, want %d", tt.final, got, tt.want)
		}
	}
}

func TestActionCSI_GetParam(t *testing.T) {
	csi := ActionCSI{Params: []int{10, 0, 30}}

	if got := csi.GetParam(0, 1); got != 10 {
		t.Errorf("GetParam(0, 1) = %d, want 10", got)
	}

	// Index 1 has value 0, should return default
	if got := csi.GetParam(1, 5); got != 5 {
		t.Errorf("GetParam(1, 5) = %d, want 5 (param is 0)", got)
	}

	if got := csi.GetParam(2, 1); got != 30 {
		t.Errorf("GetParam(2, 1) = %d, want 30", got)
	}

	// Out of bounds
	if got := csi.GetParam(5, 99); got != 99 {
		t.Errorf("GetParam(5, 99) = %d, want 99 (out of bounds)", got)
	}

	if got := csi.GetParam(-1, 99); got != 99 {
		t.Errorf("GetParam(-1, 99) = %d, want 99 (negative index)", got)
	}
}

func TestParser_LargeParam(t *testing.T) {
	p := NewParser()
	actions := p.Parse([]byte("\x1b[999999H"))

	if len(actions) != 1 {
		t.Fatalf("len(actions) = %d, want 1", len(actions))
	}

	csi := actions[0].(ActionCSI)
	if csi.Params[0] != 999999 {
		t.Errorf("Params[0] = %d, want 999999", csi.Params[0])
	}
}

func TestIsPrint_IsCSI_IsExecute(t *testing.T) {
	print := ActionPrint{Char: 'A'}
	csi := ActionCSI{Final: 'm'}
	exec := ActionExecute{Char: '\n'}

	if !IsPrint(print) {
		t.Error("IsPrint(ActionPrint) should be true")
	}
	if IsPrint(csi) {
		t.Error("IsPrint(ActionCSI) should be false")
	}

	if !IsCSI(csi) {
		t.Error("IsCSI(ActionCSI) should be true")
	}
	if IsCSI(print) {
		t.Error("IsCSI(ActionPrint) should be false")
	}

	if !IsExecute(exec) {
		t.Error("IsExecute(ActionExecute) should be true")
	}
	if IsExecute(print) {
		t.Error("IsExecute(ActionPrint) should be false")
	}
}
