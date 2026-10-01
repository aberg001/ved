package main

import "strings"

// Command is one entry in the history: one ed command, possibly multi-line
// (a/i/c bodies live in the same entry until the terminating ".").
type Command struct {
	Text string
	// Err records an execution error so re-running during replay surfaces it.
	Err string
}

// History is a linear list of commands plus a cursor. cursor == len(cmds)
// means the cursor sits on the fresh empty line at the end.
type History struct {
	Cmds  []Command
	Cursor int // index of the command being edited; len(Cmds) = new line
}

// Nearest-snapshot replay support -------------------------------------------

type Snapshot struct {
	AfterIndex int // buffer state after commands [0..AfterIndex] applied
	Buf        *Buffer
}

// Engine holds buffer, history, snapshots, and current addresses.
type Engine struct {
	Filename  string
	Base      *Buffer // pristine buffer as loaded
	Hist      History
	Snapshots []Snapshot // snapshots[0] = base (AfterIndex -1)
	CurLine   int        // current address (1-based); 0 = none
	LastRegex string
	Modified bool
	WarnedQuit bool
	QuitRequested bool
	LastMsg string
	ShowNumbers bool // display preference: gutter line numbers
	snapEvery int
}

func NewEngine(filename string, b *Buffer) *Engine {
	e := &Engine{
		Filename:  filename,
		Base:      b.Clone(),
		CurLine:   len(b.Lines),
		snapEvery: 16,
	}
	e.Snapshots = []Snapshot{{AfterIndex: -1, Buf: b.Clone()}}
	return e
}

// StateAt returns the buffer after commands [0..n] have been applied.
func (e *Engine) StateAt(n int) *Buffer {
	// find nearest snapshot at or before n
	si := 0
	for i, s := range e.Snapshots {
		if s.AfterIndex <= n {
			si = i
		} else {
			break
		}
	}
	b := e.Snapshots[si].Buf.Clone()
	for i := e.Snapshots[si].AfterIndex + 1; i <= n && i < len(e.Hist.Cmds); i++ {
		ApplyCommand(b, e, e.Hist.Cmds[i].Text, true)
	}
	return b
}

// StateAtPreview is StateAt but with the given text substituted for the
// command at index i (used for live edit preview). i must be < len(cmds).
func (e *Engine) StateAtPreview(i int, text string) *Buffer {
	if i >= len(e.Hist.Cmds) {
		// editing the fresh line: apply new text after all commands
		b := e.StateAt(len(e.Hist.Cmds) - 1)
		if strings.TrimSpace(text) != "" {
			ApplyCommand(b, e, text, true)
		}
		return b
	}
	si := 0
	for j, s := range e.Snapshots {
		if s.AfterIndex < i {
			si = j
		} else {
			break
		}
	}
	b := e.Snapshots[si].Buf.Clone()
	for j := e.Snapshots[si].AfterIndex + 1; j < i; j++ {
		ApplyCommand(b, e, e.Hist.Cmds[j].Text, true)
	}
	if strings.TrimSpace(text) != "" {
		ApplyCommand(b, e, text, true)
	}
	return b
}

// MaybeSnapshot records a snapshot every snapEvery commands.
func (e *Engine) MaybeSnapshot() {
	n := len(e.Hist.Cmds) - 1
	last := e.Snapshots[len(e.Snapshots)-1].AfterIndex
	if n-last >= e.snapEvery {
		e.Snapshots = append(e.Snapshots, Snapshot{AfterIndex: n, Buf: e.StateAt(n)})
	}
}

// Error message from the last command, shown in the status area.
var LastError string
