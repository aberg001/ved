package main

import (
	"strings"
	"testing"
)

func runScript(t *testing.T, initial string, cmds []string) (*Buffer, *Engine) {
	t.Helper()
	b := NewBuffer()
	b.Lines = strings.Split(initial, "\n")
	for len(b.Lines) > 0 && b.Lines[len(b.Lines)-1] == "" {
		b.Lines = b.Lines[:len(b.Lines)-1]
	}
	e := NewEngine("t", b)
	for _, c := range cmds {
		if err := ApplyCommand(b, e, c, true); err != nil {
			t.Fatalf("cmd %q: %v", c, err)
		}
	}
	return b, e
}

func eq(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("lines:\n got %q\nwant %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestSubstituteAndAddress(t *testing.T) {
	b, _ := runScript(t, "alpha\nbeta\ngamma\n", []string{"1,$s/a/A/"})
	eq(t, b.Lines, []string{"Alpha", "betA", "gAmma"})
}

func TestAppendBody(t *testing.T) {
	b, _ := runScript(t, "one\n", []string{"2a\ntwo\n.", "1i\nzero\n."})
	eq(t, b.Lines, []string{"zero", "one", "two"})
}

func TestChangeDeleteMove(t *testing.T) {
	b, _ := runScript(t, "a\nb\nc\nd\n", []string{"2,3c\nx\n.", "1m3", "1d"})
	eq(t, b.Lines, []string{"d", "a"})
}

func TestPartialSubNoPanic(t *testing.T) {
	b := NewBuffer()
	b.Lines = []string{"beta"}
	e := NewEngine("t", b)
	// unterminated replacements must error, not panic
	for _, c := range []string{"s/beta", "s/beta/B", "s/beta/BETA/", "s/beta/BETA/g"} {
		if err := ApplyCommand(b, e, c, true); err != nil {
			// fine: incomplete commands error
			_ = err
		}
	}
}

func TestSearchAddress(t *testing.T) {
	b, _ := runScript(t, "one\ntwo\nthree\n", []string{"/two/s/o/0/"})
	eq(t, b.Lines, []string{"one", "tw0", "three"})
}

func TestDiffLine(t *testing.T) {
	cs := DiffLine(strings.Split("a\nb\nc\n", "\n"), strings.Split("a\nB\nc\nd\n", "\n"))
	if len(cs) != 2 {
		t.Fatalf("want 2 changes, got %d: %+v", len(cs), cs)
	}
}

func TestExHelp(t *testing.T) {
	e := NewEngine("t", &Buffer{})
	handleExCommand(e, "help i", false)
	if !strings.Contains(e.LastMsg, "insert text before") {
		t.Fatalf("help i: %q", e.LastMsg)
	}
	handleExCommand(e, "help", false)
	if !strings.Contains(e.LastMsg, "commands:") {
		t.Fatalf("bare help: %q", e.LastMsg)
	}
	if !strings.Contains(e.LastMsg, " m ") {
		t.Fatalf("bare help lists commands: %q", e.LastMsg)
	}
	handleExCommand(e, "help nope", false)
	if !strings.Contains(LastError, "no help for: nope") {
		t.Fatalf("unknown topic: LastError=%q", LastError)
	}
}
