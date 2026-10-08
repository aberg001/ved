package main

import "testing"

// Regression tests for move/copy destination validation (notes.txt:
// "moving two lines, maybe from the end of the file" — out-of-range
// destinations were silently clamped and left dot beyond EOF).
func runM(t *testing.T, cmds ...string) (lines []string, dot int) {
	t.Helper()
	b := &Buffer{Lines: []string{"a", "b", "c", "d", "e"}}
	e := NewEngine("t.txt", b)
	for _, c := range cmds {
		if err := ApplyCommand(b, e, c, false); err != nil {
			t.Fatalf("cmd %q: unexpected error %v", c, err)
		}
	}
	return b.Lines, e.CurLine
}

func runMErr(t *testing.T, cmds ...string) error {
	t.Helper()
	b := &Buffer{Lines: []string{"a", "b", "c", "d", "e"}}
	e := NewEngine("t.txt", b)
	var last error
	for _, c := range cmds {
		last = ApplyCommand(b, e, c, false)
	}
	return last
}

func TestMoveDestRange(t *testing.T) {
	ls, dot := runM(t, "5m0")
	if ls[0] != "e" || dot != 1 {
		t.Fatalf("5m0: %v dot=%d", ls, dot)
	}
	ls, dot = runM(t, "4,5m0")
	if ls[0] != "d" || ls[1] != "e" || dot != 2 {
		t.Fatalf("4,5m0: %v dot=%d", ls, dot)
	}
	ls, dot = runM(t, "1m5")
	if ls[4] != "a" || dot != 5 {
		t.Fatalf("1m5: %v dot=%d", ls, dot)
	}
	ls, dot = runM(t, "1,2m$")
	if ls[3] != "a" || ls[4] != "b" || dot != 5 {
		t.Fatalf("1,2m$: %v dot=%d", ls, dot)
	}
}

func TestMoveDestInvalid(t *testing.T) {
	for _, cmd := range []string{"3m3", "2,3m3", "2,3m2", "5m5", "3,3m3", "4,5m4"} {
		if err := runMErr(t, cmd); err == nil {
			t.Errorf("%s: expected invalid destination, got nil", cmd)
		}
	}
}

func TestDestOutOfRange(t *testing.T) {
	// These used to be silently clamped to EOF with dot beyond the buffer.
	for _, cmd := range []string{"1m6", "1t6", "1,2t99", "1,2m99"} {
		if err := runMErr(t, cmd); err == nil {
			t.Errorf("%s: expected address out of range, got nil", cmd)
		}
	}
}

func TestCopyDestRange(t *testing.T) {
	ls, dot := runM(t, "1t5")
	if ls[5] != "a" || dot != 6 {
		t.Fatalf("1t5: %v dot=%d", ls, dot)
	}
}
