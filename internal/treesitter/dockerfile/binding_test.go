package ts_dockerfile_test

import (
	"testing"

	ts "github.com/tree-sitter/go-tree-sitter"
	ts_dockerfile "github.com/tsukinoko-kun/harbor/internal/treesitter/dockerfile"
)

func TestCanLoadGrammar(t *testing.T) {
	language := ts.NewLanguage(ts_dockerfile.Language())
	if language == nil {
		t.Errorf("Error loading Dockerfile grammar")
	}
}
