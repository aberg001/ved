package main

import (
	"strings"
	"testing"
)

// run applies commands to a fresh engine loaded with one..five.
// marks/errs do NOT persist between commands (fresh engine each call);
// use runSess for stateful tests.
func run(cmds ...string) (*Buffer, *Engine, []error) {
	b := NewBuffer()
	b.Lines = strings.Split("one\ntwo\nthree\nfour\nfive", "\n")
	e := NewEngine("t.txt", b)
	var errs []error
	for _, c := range cmds {
		if err := ApplyCommand(b, e, c, false); err != nil {
			errs = append(errs, err)
		}
	}
	return b, e, errs
}

func TestGlobalNoAddrDefaultsAll(t *testing.T) {
	b, e, errs := run("g/one/d")
	if len(errs) > 0 || strings.Join(b.Lines, "|") != "two|three|four|five" || e.CurLine != 1 {
		t.Errorf("g: %v %q cur=%d", errs, b.Lines, e.CurLine)
	}
	b, e, errs = run("v/e/d")
	// deletes lines that do NOT contain 'e': one(four?) ...
	if len(errs) > 0 {
		t.Errorf("v: %v", errs)
	}
	if strings.Join(b.Lines, "|") != "one|three|five" {
		t.Errorf("v: %q", b.Lines)
	}
}

func TestGlobalWithBody(t *testing.T) {
	b, e, errs := run("g/two/a\nMORE\n.")
	if len(errs) > 0 {
		t.Errorf("g+body: %v", errs)
		return
	}
	got := strings.Join(b.Lines, "|")
	want := "one|two|MORE|three|four|five"
	if got != want || e.CurLine != 2 {
		t.Errorf("g+body: got %q want %q cur=%d", got, want, e.CurLine)
	}
}

func TestGlobalBottomUp(t *testing.T) {
	b, _, errs := run("g/.*/d")
	// ascending-order matches would leave stale lines; bottom-up deletes all
	if len(errs) > 0 || len(b.Lines) != 0 {
		t.Errorf("g delete-all: %v %q", errs, b.Lines)
	}
}

func TestJoinMarkListZ(t *testing.T) {
	b, e, errs := run("2j")
	// explicit single address: no-op (ed joins just that line)
	if len(errs) > 0 || strings.Join(b.Lines, "|") != "one|two|three|four|five" || e.CurLine != 2 {
		t.Errorf("j: %v %q cur=%d", errs, b.Lines, e.CurLine)
	}
	b, e, errs = run("1", "j")
	// bare j: default (.,.+1) joins dot and the next line; dot = joined line
	if len(errs) > 0 || strings.Join(b.Lines, "|") != "one two|three|four|five" || e.CurLine != 1 {
		t.Errorf("j bare: %v %q cur=%d", errs, b.Lines, e.CurLine)
	}
	b, e, errs = run("2,3j")
	if len(errs) > 0 || strings.Join(b.Lines, "|") != "one|two three|four|five" {
		t.Errorf("j range: %v %q", errs, b.Lines)
	}
	// marks need one engine across commands (like a real session)
	b = NewBuffer()
	b.Lines = strings.Split("one\ntwo\nthree\nfour\nfive", "\n")
	e = NewEngine("t.txt", b)
	ApplyCommand(b, e, "1", false)
	if err := ApplyCommand(b, e, "kx", false); err != nil {
		t.Fatalf("k: %v", err)
	}
	if err := ApplyCommand(b, e, "'x", false); err != nil || e.CurLine != 1 {
		t.Errorf("'x: %v cur=%d", err, e.CurLine)
	}
	if err := ApplyCommand(b, e, "kq", false); err != nil {
		t.Fatalf("kq: %v", err)
	}
	// bare j after moving dot away; then jump via mark
	if err := ApplyCommand(b, e, "3", false); err != nil {
		t.Fatal(err)
	}
	if err := ApplyCommand(b, e, "'q", false); err != nil || e.CurLine != 1 {
		t.Errorf("'q: %v cur=%d", err, e.CurLine)
	}
	// unset mark
	_, _, errs = run("1", "'q")
	if len(errs) == 0 || errs[0].Error() != "no such mark" {
		t.Errorf("unset mark: %v", errs)
	}
	// z: scroll and set dot to last line shown
	b, e, errs = run("1", "z2")
	if len(errs) > 0 || e.CurLine != 3 {
		t.Errorf("z: %v cur=%d", errs, e.CurLine)
	}
}

func TestSedLikeCmds(t *testing.T) {
	// m: move; t: copy
	b, e, errs := run("1,2m$")
	if len(errs) > 0 || strings.Join(b.Lines, "|") != "three|four|five|one|two" || e.CurLine != 5 {
		t.Errorf("m: %v %q cur=%d", errs, b.Lines, e.CurLine)
	}
	b, e, errs = run("1t$")
	if len(errs) > 0 || strings.Join(b.Lines, "|") != "one|two|three|four|five|one" || e.CurLine != 6 {
		t.Errorf("t: %v %q cur=%d", errs, b.Lines, e.CurLine)
	}
	// \l escape lists control chars
	b, e, errs = run("$", "i\n\x07\n.\n")
	if len(errs) > 0 {
		t.Errorf("append ctrl: %v", errs)
	}
	if len(b.Lines) != 6 {
		t.Fatalf("append: %q", b.Lines)
	}
	if got := b.Lines[4]; got == "\x07" {
		// live render will escape; the buffer keeps the raw char
		_ = e
	} else {
		t.Errorf("append: %q", got)
	}
}

func TestHelpH(t *testing.T) {
	// h on a fresh engine: no error yet, prints nothing (clear prior error)
	LastError = ""
	_, e, _ := run("h")
	if e.LastMsg != "" {
		t.Errorf("h fresh: %q", e.LastMsg)
	}
	// h repeats the last error; the error persists across commands
	b := NewBuffer()
	b.Lines = strings.Split("one\ntwo\nthree", "\n")
	e2 := NewEngine("t.txt", b)
	ApplyCommand(b, e2, "999p", false)
	ApplyCommand(b, e2, "p", false) // success does not clear the last error
	ApplyCommand(b, e2, "h", false)
	if e2.LastMsg != "address out of range" {
		t.Errorf("h: %q", e2.LastMsg)
	}
	// help topics exist
	if len(helpTopics) == 0 {
		t.Errorf("helpTopics empty")
	}
}

func TestUndoPop(t *testing.T) {
	b := NewBuffer()
	b.Lines = strings.Split("one\ntwo\nthree", "\n")
	e := NewEngine("t.txt", b)
	// the UI commit records history before applying; test that flow
	e.Hist.Cmds = append(e.Hist.Cmds, Command{Text: "s/three/THREE/"})
	e.Hist.Cursor = 1
	// state with the command applied
	ap := e.StateAt(1)
	if strings.Join(ap.Lines, "|") != "one|two|THREE" {
		t.Errorf("applied: %q", ap.Lines)
	}
	// undo: drop the last command, replay gives the original buffer
	e.Hist.Cmds = e.Hist.Cmds[:len(e.Hist.Cmds)-1]
	e.Hist.Cursor = 0
	out := e.StateAt(0)
	if strings.Join(out.Lines, "|") != "one|two|three" {
		t.Errorf("undo replay: %q", out.Lines)
	}
}
