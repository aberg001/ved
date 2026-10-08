package main

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// LineStyles must return one style per rune, not per byte, and callers must
// never index it with byte offsets (panicked on multibyte lines like
// 2026-09-17.md's em-dashes and accented characters).
func TestLineStylesRuneParallelMultibyte(t *testing.T) {
	lines := []string{
		"café — naïve — résumé",
		"日本語のテキスト",
		"mixed 🚀 emoji and — dash",
		strings.Repeat("é", 500),
	}
	db := LoadSyntaxDB("syntax.conf")
	for _, line := range lines {
		st := LineStyles(line, db.For("x.md"), tcell.StyleDefault)
		if got := len([]rune(line)); len(st) != got {
			t.Fatalf("LineStyles returned %d styles for %d runes of %q", len(st), got, line)
		}
	}
	// empty line
	if st := LineStyles("", db.For("x.md"), tcell.StyleDefault); len(st) != 0 {
		t.Fatalf("empty line: got %d styles", len(st))
	}
}

