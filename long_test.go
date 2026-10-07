package main

import (
	"strings"
	"testing"
)

func TestLongLine(t *testing.T) {
	big := strings.Repeat("y", 5000)
	b := NewBuffer()
	b.Lines = []string{big, "tail"}
	e := NewEngine("t.txt", b)
	for _, c := range []string{
		"1s/y/Y/", "1s/y$/Z/", "g/y/s/y/Y/g", "1c\n" + big + "\n.", "1i\n" + big + "\n.",
		"1,2j", "$", "1t2", "2m0", "1a\n" + big + "\n.",
	} {
		if err := ApplyCommand(b, e, c, false); err != nil {
			t.Fatalf("%q: %v", c, err)
		}
	}
	// no panic, no truncation surprises
	if len(b.Lines) != 4 {
		t.Fatalf("lines=%d", len(b.Lines))
	}
}
