package ts_dockerfile

// #cgo CFLAGS: -std=c11 -fPIC
// #include "src/parser.c"
// #include "src/scanner.c"
import "C"

import (
	_ "embed"
	"unsafe"
)

// Get the tree-sitter Language for this grammar.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_dockerfile())
}

//go:embed queries/highlights.scm
var QueryHighlights string
