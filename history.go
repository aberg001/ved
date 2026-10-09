package main

import "strings"

// Command is one entry in the history: one ed command, possibly multi-line
// (a/i/c bodies live in the same entry until the terminating ".").
type Command struct {
	Text string
	// Err records an execution error so re-running during replay surfaces it.
	Err string
	// Hidden marks commands executed silently (e.g. from ~/.vedrc): applied
	// by replay but not shown in the command history pane.
	Hidden bool
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
	Dot        int // current address (1-based) after those commands
}

// Engine holds buffer, history, snapshots, and current addresses.
type MoveInfo struct {
	L1, L2 int  // 1-based source range in the pre-command buffer
	Dest   int  // destination (block inserted after this line, pre-command coords)
	Copy   bool // true for t, false for m
}

type Engine struct {
	Filename  string
	Base      *Buffer // pristine buffer as loaded
	Hist      History
	Snapshots []Snapshot // snapshots[0] = base (AfterIndex -1)
	CurLine   int        // current address (1-based); 0 = none
	Replay    *int       // during replay: dot lives here, not CurLine
	LastRegex string
	Modified bool
	WarnedQuit bool
	QuitRequested bool
	LastMsg string
	PreviewErr string // live-edit preview error, shown while typing
	LastMove *MoveInfo // set by m/t; nil for any other command
	ShowNumbers bool // display preference: gutter line numbers
	Marks map[byte]int // named marks set by k, addressed with 'x
	HMode bool // auto-print error messages (H toggle)
	snapEvery int
}

func NewEngine(filename string, b *Buffer) *Engine {
	e := &Engine{
		Filename:  filename,
		Base:      b.Clone(),
		CurLine:   len(b.Lines),
		Marks:     map[byte]int{},
		snapEvery: 16,
	}
	e.Snapshots = []Snapshot{{AfterIndex: -1, Buf: b.Clone(), Dot: len(b.Lines)}}
	return e
}

// getDot returns the current address: the replay cursor when replaying,
// otherwise the live dot.
func (e *Engine) getDot() int {
	if e.Replay != nil {
		return *e.Replay
	}
	return e.CurLine
}

// setDot sets the current address for both live and replay contexts.
func (e *Engine) setDot(n int) {
	if e.Replay != nil {
		*e.Replay = n
	}
	e.CurLine = n
}

// StateAt returns the buffer after commands [0..n] have been applied.
func (e *Engine) StateAt(n int) *Buffer {
	// find nearest snapshot at or before n
	// replay must not clobber the live LastError or LastMsg (e.g. the q warning)
	saved := LastError
	savedMsg := e.LastMsg
	defer func() { LastError = saved; e.LastMsg = savedMsg }()
	si := 0
	for i, s := range e.Snapshots {
		if s.AfterIndex <= n {
			si = i
		} else {
			break
		}
	}
	b := e.Snapshots[si].Buf.Clone()
	dot := e.Snapshots[si].Dot
	e.Replay = &dot
	defer func() { e.Replay = nil }()
	for i := e.Snapshots[si].AfterIndex + 1; i <= n && i < len(e.Hist.Cmds); i++ {
		ApplyCommand(b, e, e.Hist.Cmds[i].Text, true)
	}
	return b
}

// StateAtPreview is StateAt but with the given text substituted for the
// command at index i (used for live edit preview). i must be < len(cmds).
func (e *Engine) StateAtPreview(i int, text string) *Buffer {
	saved := LastError
	savedMsg := e.LastMsg
	e.PreviewErr = ""
	defer func() { LastError = saved; e.LastMsg = savedMsg }()
	if i >= len(e.Hist.Cmds) {
		// editing the fresh line: apply new text after all commands
		b := e.StateAt(len(e.Hist.Cmds) - 1)
		if strings.TrimSpace(text) != "" {
			if err := ApplyCommand(b, e, text, true); err != nil {
				e.PreviewErr = err.Error()
			}
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
		if err := ApplyCommand(b, e, text, true); err != nil {
			e.PreviewErr = err.Error()
		}
	}
	return b
}

// MaybeSnapshot records a snapshot every snapEvery commands. Called on the
// live path: the live buffer and dot ARE the state after len(Cmds)-1.
func (e *Engine) MaybeSnapshot() {
	n := len(e.Hist.Cmds) - 1
	last := e.Snapshots[len(e.Snapshots)-1].AfterIndex
	if n-last >= e.snapEvery {
		e.Snapshots = append(e.Snapshots, Snapshot{AfterIndex: n, Buf: e.StateAt(n), Dot: e.CurLine})
	}
}

// Error message from the last command, shown in the status area.
var LastError string
