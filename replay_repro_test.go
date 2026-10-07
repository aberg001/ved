package main

import (
	"strings"
	"testing"
)

// Replay must use its own dot: a stale live dot during preview once
// panicked (index out of range) when commands used default addresses.
func TestReplayUsesOwnDot(t *testing.T) {
	b := NewBuffer()
	b.Lines = strings.Split("one\ntwo\nthree\nfour\nfive", "\n")
	e := NewEngine("", b)
	for _, c := range []string{"t2", "m6", "4t6", "2m6", "1,2d", "$", "p", "g/three/s/three/3"} {
		if err := ApplyCommand(b, e, c, false); err != nil {
			t.Fatalf("%q: %v (dot=%d, lines=%v)", c, err, e.getDot(), b.Lines)
		}
	}
	s := strings.Join(e.StateAt(len(e.Hist.Cmds) - 1).Lines, "\n")
	t.Logf("final: %q", s)
}
