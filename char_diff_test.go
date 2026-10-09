package main

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestDiffRunesBasic(t *testing.T) {
	ch := DiffRunes([]rune("hello world"), []rune("hello brave world"))
	if len(ch) != 1 {
		t.Fatalf("want 1 change, got %d: %+v", len(ch), ch)
	}
	c := ch[0]
	if string([]rune("hello world")[c.OldStart:c.OldEnd]) != "" ||
		string([]rune("hello brave world")[c.NewStart:c.NewEnd]) != "brave " {
		t.Fatalf("unexpected change: %+v", c)
	}
}

func TestDiffRunesIdentical(t *testing.T) {
	if ch := DiffRunes([]rune("same"), []rune("same")); len(ch) != 0 {
		t.Fatalf("want no changes, got %+v", ch)
	}
}

func TestDiffRunesUnicode(t *testing.T) {
	ch := DiffRunes([]rune("héllo"), []rune("héllo wörld"))
	if len(ch) == 0 {
		t.Fatal("want changes")
	}
	// indexes must be in rune space: "héllo"[4] == 'o'
	last := ch[len(ch)-1]
	if last.NewEnd > len([]rune("héllo wörld")) || last.OldEnd > len([]rune("héllo")) {
		t.Fatalf("out of range: %+v", ch)
	}
}

func TestCharDiffPairStyles(t *testing.T) {
	del := tcell.StyleDefault.Foreground(tcell.ColorRed)
	add := tcell.StyleDefault.Foreground(tcell.ColorGreen)
	plain := tcell.StyleDefault.Foreground(tcell.ColorDefault)
	oSt, nSt := charDiffPair("hello world", "hello big world", nil, del, add, plain)
	if len(oSt) != len([]rune("hello world")) || len(nSt) != len([]rune("hello big world")) {
		t.Fatalf("style lengths %d/%d", len(oSt), len(nSt))
	}
	// char 0 ('h') unchanged on both: plain
	if oSt[0] != plain || nSt[0] != plain {
		t.Fatal("common chars should stay plain")
	}
	// new: 'big ' inserted -> green
	if nSt[6] != add || nSt[7] != add || nSt[8] != add || nSt[9] != add {
		t.Fatal("inserted text should be green")
	}
	// substitution case: old chars get red
	oSt2, nSt2 := charDiffPair("the quick brown fox", "the quick red fox", nil, del, add, plain)
	if oSt2[13] != del { // 'b' of brown
		t.Fatal("replaced chars should be red on old line")
	}
	if nSt2[11] != add { // 'r' of red
		t.Fatal("replaced chars should be green on new line")
	}
}

func TestEditIsSub(t *testing.T) {
	u := &UI{editLines: []string{"s/foo/bar/"}}
	if !u.editIsSub() {
		t.Fatal("s/foo/bar/ should be sub")
	}
	for _, e := range []string{"1,3s/a/b/", "d", "s", "5", "m3", "sub/a/"} {
		u.editLines = []string{e}
		if u.editIsSub() {
			t.Fatalf("%q should not be sub", e)
		}
	}
	u.editLines = []string{"2s/x/y/"}
	if u.editIsSub() {
		t.Fatal("addressed s at start of line without leading digits handled by engine, gate is best-effort")
	}
}
