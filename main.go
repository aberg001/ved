package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
)

const (
	syntaxConf = "/rool-drive/ved/syntax.conf"
)

type UI struct {
	screen tcell.Screen
	eng    *Engine
	syn    *SyntaxDB
	// edit state: the pending command being edited, possibly multi-line
	editLines []string
	editRow   int
	editCol   int
	killRing  []string // most recent kill first
	quit      bool
	// append mode: after entering a/i/c, subsequent lines become the body
	// of that command until a lone "." is entered.
	appendMode bool
	appendIdx  int // index of the command collecting the body
}

func (u *UI) kill(text string) {
	if text == "" {
		return
	}
	u.killRing = append([]string{text}, u.killRing...)
	if len(u.killRing) > 10 {
		u.killRing = u.killRing[:10]
	}
}

// yank inserts the most recent kill at the cursor.
func (u *UI) yank() {
	if len(u.killRing) == 0 {
		return
	}
	u.insertText(u.killRing[0])
}

// insertText splices text at the current cursor position, handling
// embedded newlines by splitting rows.
func (u *UI) insertText(text string) {
	line := u.editLines[u.editRow]
	rr := []rune(line)
	if u.editCol > len(rr) {
		u.editCol = len(rr)
	}
	left := string(rr[:u.editCol]) + text
	right := string(rr[u.editCol:])
	parts := strings.Split(left, "\n")
	// the last part keeps the right side of the original line
	u.editLines[u.editRow] = parts[len(parts)-1] + right
	// insert the earlier parts as new rows above
	for i := len(parts) - 1; i > 0; i-- {
		u.editLines = append(u.editLines[:u.editRow], append([]string{parts[i-1]}, u.editLines[u.editRow:]...)...)
	}
	if len(parts) > 1 {
		u.editRow += len(parts) - 1
		u.editCol = len([]rune(parts[len(parts)-1]))
	} else {
		u.editCol = len([]rune(left))
	}
}

// wordForward returns (index of end of word at/after col, newCol).
func wordNext(rr []rune, col int) int {
	i := col
	for i < len(rr) && !isWordChar(rr[i]) {
		i++
	}
	for i < len(rr) && isWordChar(rr[i]) {
		i++
	}
	return i
}

func wordPrev(rr []rune, col int) int {
	i := col
	for i > 0 && !isWordChar(rr[i-1]) {
		i--
	}
	for i > 0 && isWordChar(rr[i-1]) {
		i--
	}
	return i
}

func isWordChar(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

var sigCont = make(chan os.Signal, 1)

func main() {
	signal.Notify(sigCont, syscall.SIGCONT)
	filename := ""
	if len(os.Args) > 1 {
		filename = os.Args[1]
	}
	buf := NewBuffer()
	if filename != "" {
		if data, err := os.ReadFile(filename); err == nil {
			t := strings.TrimRight(string(data), "\n")
			if t != "" {
				buf.Lines = strings.Split(t, "\n")
			}
		}
	}
	eng := NewEngine(filename, buf)
	syn := LoadSyntaxDB(syntaxConf)

	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer screen.Fini()

	ui := &UI{screen: screen, eng: eng, syn: syn, editLines: []string{""}}
	screen.SetStyle(tcell.StyleDefault.Background(tcell.ColorDefault).Foreground(tcell.ColorDefault))

	for !ui.quit {
		ui.draw()
		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			screen.Sync()
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyCtrlZ {
				screen.Fini() // restore terminal so the shell prompt is clean
				syscall.Kill(syscall.Getpid(), syscall.SIGTSTP)
				// Wait for SIGCONT (delivered on resume on a real OS).
				// If the environment does not implement job-control stop
				// (sandboxes), fall through after a short grace period.
				select {
				case <-sigCont:
				case <-time.After(500 * time.Millisecond):
					fmt.Println("ved: suspend not supported in this environment")
				}
				screen.Init()
				screen.Clear()
				screen.Sync() // full repaint after resume
				continue
			}
			ui.handleKey(ev)
		}
	}
}

func vedlog(msg string) {
	if p := os.Getenv("VEDLOG"); p != "" {
		f, _ := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			fmt.Fprintln(f, msg)
			f.Close()
		}
	}
}

func NewBuffer() *Buffer { return &Buffer{} }

// currentEdit returns the full pending command text.
func (u *UI) currentEdit() string { return strings.Join(u.editLines, "\n") }

func (u *UI) handleKey(ev *tcell.EventKey) {
	k := ev.Key()
	vedlog(fmt.Sprintf("key: k=%d rune=%q mods=%v", k, ev.Rune(), ev.Modifiers()))
	line := u.editLines[u.editRow]
	switch k {
	case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyCtrlH:
		rr := []rune(line)
		if u.editCol > 0 {
			line = string(rr[:u.editCol-1]) + string(rr[u.editCol:])
			u.editCol--
			u.editLines[u.editRow] = line
		} else if u.editRow > 0 {
			prev := u.editLines[u.editRow-1]
			u.editLines[u.editRow-1] = prev + line
			u.editLines = append(u.editLines[:u.editRow], u.editLines[u.editRow+1:]...)
			u.editRow--
			u.editCol = len([]rune(prev))
		}
	case tcell.KeyDelete:
		rr := []rune(line)
		if u.editCol < len(rr) {
			line = string(rr[:u.editCol]) + string(rr[u.editCol+1:])
			u.editLines[u.editRow] = line
		} else if u.editRow < len(u.editLines)-1 {
			u.editLines[u.editRow] = line + u.editLines[u.editRow+1]
			u.editLines = append(u.editLines[:u.editRow+1], u.editLines[u.editRow+2:]...)
		}
	case tcell.KeyLeft, tcell.KeyCtrlB:
		if u.editCol > 0 {
			u.editCol--
		} else if u.editRow > 0 {
			u.editRow--
			u.editCol = len([]rune(u.editLines[u.editRow]))
		}
	case tcell.KeyRight, tcell.KeyCtrlF:
		rr := []rune(line)
		if u.editCol < len(rr) {
			u.editCol++
		} else if u.editRow < len(u.editLines)-1 {
			u.editRow++
			u.editCol = 0
		}
	case tcell.KeyHome, tcell.KeyCtrlA:
		u.editCol = 0
	case tcell.KeyEnd, tcell.KeyCtrlE:
		u.editCol = len([]rune(line))
	case tcell.KeyCtrlK:
		rr := []rune(line)
		if u.editCol < len(rr) {
			u.kill(string(rr[u.editCol:]))
		} else if u.editRow < len(u.editLines)-1 {
			// at EOL: kill the line break
			u.kill("\n")
		}
		u.editLines[u.editRow] = string(rr[:u.editCol])
		if u.editRow < len(u.editLines)-1 {
			u.editLines = append(u.editLines[:u.editRow+1], u.editLines[u.editRow+2:]...)
		}
	case tcell.KeyCtrlP, tcell.KeyUp:
		if u.editRow > 0 {
			u.editRow--
			u.editCol = len([]rune(u.editLines[u.editRow]))
		} else {
			u.historyUp()
		}
	case tcell.KeyCtrlN, tcell.KeyDown:
		if u.editRow < len(u.editLines)-1 {
			u.editRow++
			u.editCol = len([]rune(u.editLines[u.editRow]))
		} else {
			u.historyDown()
		}
	case tcell.KeyCtrlY:
		u.yank()
	case tcell.KeyCtrlW:
		// delete word before cursor (readline: unix-word-rubout)
		rr := []rune(line)
		if u.editCol > 0 {
			n := wordPrev(rr, u.editCol)
			u.kill(string(rr[n:u.editCol]))
			line = string(rr[:n]) + string(rr[u.editCol:])
			u.editCol = n
			u.editLines[u.editRow] = line
		}
	case tcell.KeyCtrlU:
		// kill from cursor to start of line
		rr := []rune(line)
		if u.editCol > 0 {
			u.kill(string(rr[:u.editCol]))
			line = string(rr[u.editCol:])
			u.editCol = 0
			u.editLines[u.editRow] = line
		}
	case tcell.KeyCtrlD:
		// delete char under cursor (forward delete)
		rr := []rune(line)
		if u.editCol < len(rr) {
			line = string(rr[:u.editCol]) + string(rr[u.editCol+1:])
			u.editLines[u.editRow] = line
		} else if u.editRow < len(u.editLines)-1 {
			u.editLines[u.editRow] = line + u.editLines[u.editRow+1]
			u.editLines = append(u.editLines[:u.editRow+1], u.editLines[u.editRow+2:]...)
		}
	case tcell.KeyCtrlT:
		// transpose chars
		rr := []rune(line)
		if u.editCol > 0 && u.editCol < len(rr) {
			rr[u.editCol-1], rr[u.editCol] = rr[u.editCol], rr[u.editCol-1]
			u.editCol++
			u.editLines[u.editRow] = string(rr)
		}
	case tcell.KeyRune:
		r := ev.Rune()
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			// control-modified runes are not printable text; ignore
			break
		}
		if ev.Modifiers()&tcell.ModAlt != 0 {
			rr := []rune(line)
			switch r {
			case 'b': // backward word
				if u.editCol > 0 {
					u.editCol = wordPrev(rr, u.editCol)
				}
			case 'f': // forward word
				u.editCol = wordNext(rr, u.editCol)
			case 'd': // kill word forward
				if u.editCol < len(rr) {
					n := wordNext(rr, u.editCol)
					u.kill(string(rr[u.editCol:n]))
					line = string(rr[:u.editCol]) + string(rr[n:])
					u.editLines[u.editRow] = line
				}
			case 127, 8: // alt-backspace: like ctrl-w
				if u.editCol > 0 {
					n := wordPrev(rr, u.editCol)
					u.kill(string(rr[n:u.editCol]))
					line = string(rr[:n]) + string(rr[u.editCol:])
					u.editCol = n
					u.editLines[u.editRow] = line
				}
			}
			return
		}
		rr := []rune(line)
		if u.editCol > len(rr) {
			u.editCol = len(rr)
		}
		line = string(rr[:u.editCol]) + string(r) + string(rr[u.editCol:])
		u.editCol++
		u.editLines[u.editRow] = line
	case tcell.KeyEnter:
		u.commit()
	}
}

// historyUp moves the edit cursor to the previous command in history.
func (u *UI) historyUp() {
	eng := u.eng
	h := &eng.Hist
	if h.Cursor == 0 {
		return
	}
	h.Cursor--
	u.loadCurrent()
}

// historyDown moves the edit cursor to the next command (redo direction).
func (u *UI) historyDown() {
	eng := u.eng
	h := &eng.Hist
	if h.Cursor >= len(h.Cmds) {
		return
	}
	// if the current command text was modified, save it first
	h.Cmds[h.Cursor].Text = u.currentEdit()
	h.Cursor++
	if h.Cursor == len(h.Cmds) {
		u.editLines = []string{""}
		u.editRow = 0
		u.editCol = 0
	} else {
		u.loadCurrent()
	}
}

// loadCurrent puts the command at the history cursor into the editor.
func (u *UI) loadCurrent() {
	eng := u.eng
	h := &eng.Hist
	if h.Cursor < len(h.Cmds) {
		u.editLines = strings.Split(h.Cmds[h.Cursor].Text, "\n")
		if len(u.editLines) == 0 {
			u.editLines = []string{""}
		}
		u.editRow = len(u.editLines) - 1
		u.editCol = len([]rune(u.editLines[u.editRow]))
	}
}

// commit handles Enter on the pending command.
// cmdTakesBody returns true if the command text is a/i/c (with any address)
// and thus consumes following lines as its body.
func cmdTakesBody(text string) bool {
	t := strings.TrimSpace(text)
	// strip an address prefix crudely: skip leading chars valid in addresses
	i := 0
	for i < len(t) {
		r := rune(t[i])
		if r >= '0' && r <= '9' || r == '.' || r == '$' || r == '+' || r == '-' ||
			r == ',' || r == ';' || r == '/' || r == '?' || r == ' ' || r == '\t' ||
			r == '%' {
			// regex addresses may contain letters; bail if we hit one unquoted
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				break
			}
			i++
			continue
		}
		break
	}
	rest := strings.TrimSpace(t[i:])
	if len(rest) >= 1 && (rest[0] == 'a' || rest[0] == 'i' || rest[0] == 'c') &&
		(len(rest) == 1 || rest[1] == ' ' || rest[1] == '\t') {
		return true
	}
	return false
}

func (u *UI) commit() {
	eng := u.eng
	h := &eng.Hist
	text := u.currentEdit()

	// append mode: fold the line into the collecting command body
	if u.appendMode {
		if strings.TrimSpace(text) == "." {
			u.appendMode = false
			u.recomputeModified()
			eng.MaybeSnapshot()
			u.editLines = []string{""}
			u.editRow, u.editCol = 0, 0
			h.Cursor = u.appendIdx + 1
			return
		}
		c := &h.Cmds[u.appendIdx]
		c.Text += "\n" + text
		// move cursor to just after the collecting command; stay in append
		h.Cursor = u.appendIdx + 1
		u.editLines = []string{""}
		u.editRow, u.editCol = 0, 0
		return
	}

	if h.Cursor < len(h.Cmds) {
		// editing an existing command: replace it, then insert a fresh
		// command line below it (pushing the rest down), like Enter in a
		// normal text editor.
		h.Cmds[h.Cursor].Text = text
		histCmds := append(h.Cmds[:h.Cursor+1:h.Cursor+1], Command{})
		histCmds = append(histCmds, h.Cmds[h.Cursor+1:]...)
		h.Cmds = histCmds
		h.Cursor++
	} else {
		// appending at the end
		if strings.TrimSpace(text) == "" {
			return
		}
		h.Cmds = append(h.Cmds, Command{Text: text})
		h.Cursor = len(h.Cmds)
	}
	vedlog(fmt.Sprintf("commit: cursor=%d len=%d text=%q appendMode=%v", h.Cursor, len(h.Cmds), text, u.appendMode))
	// if this command consumes a body, enter append mode so subsequent
	// lines are folded in until "."
	if u.appendIdx = h.Cursor - 1; cmdTakesBody(text) {
		u.appendMode = true
		u.editLines = []string{""}
		u.editRow, u.editCol = 0, 0
		return
	}

	// recompute modified flag: run the timeline with file effects allowed
	u.recomputeModified()
	eng.MaybeSnapshot()
	// check for quit
	if eng.QuitRequested {
		u.quit = true
		return
	}
	// fresh editing line below the committed command
	u.editLines = []string{""}
	u.editRow = 0
	u.editCol = 0
}

// recomputeModified replays the timeline from Base, honoring w/e/r effects,
// to keep the Modified flag honest.
func (u *UI) recomputeModified() {
	eng := u.eng
	eng.WarnedQuit = false
	mod := false
	b := eng.Base.Clone()
	eng.CurLine = 0
	eng.QuitRequested = false
	for _, c := range eng.Hist.Cmds {
		before := b.Clone()
		ApplyCommand(b, eng, c.Text, false)
		if !buffersEqual(b, before) {
			mod = true
		}
		if strings.HasPrefix(strings.TrimSpace(c.Text), "w") ||
			strings.HasPrefix(strings.TrimSpace(c.Text), "e") {
			mod = false
		}
		if eng.QuitRequested {
			break
		}
	}
	eng.Modified = mod
}

// previewBuffer returns the buffer to display in the top pane.
func (u *UI) previewBuffer() (*Buffer, *Buffer) {
	// returns (preview, diffBase).
	// preview = replay of history up to the cursor with the current edit applied.
	// base    = the current buffer (state after every committed command), so the
	// diff shows what re-applying the edited command would change.
	eng := u.eng
	h := &eng.Hist
	preview := eng.StateAtPreview(h.Cursor, u.currentEdit())
	base := eng.StateAt(len(h.Cmds))
	return preview, base
}

func (u *UI) draw() {
	s := u.screen
	w, h := s.Size()
	s.Clear()
	statusRow := h - 1 - cmdPaneHeight(h)
	topRows := statusRow

	eng := u.eng
	preview, base := u.previewBuffer()
	vedlog(fmt.Sprintf("draw: previewLines=%d curLine=%d cursor=%d cmds=%d lastMsg=%q",
		preview.NumLines(), eng.CurLine, eng.Hist.Cursor, len(eng.Hist.Cmds), eng.LastMsg))

	// Build the render list: merged diff view.
	type rline struct {
		text    string
		kind    int // 0 normal, 1 old(red), 2 new(green)
		styles  []tcell.Style
		lineNum int // 1-based line number in preview buffer for normal lines
	}
	var rows []rline
	changes := DiffLine(base.Lines, preview.Lines)
	ci := 0
	oi, ni := 0, 0
	for oi < len(base.Lines) || ni < len(preview.Lines) {
		if ci < len(changes) && oi >= changes[ci].OldStart && ni >= changes[ci].NewStart {
			ch := changes[ci]
			for i := ch.OldStart; i < ch.OldEnd; i++ {
				rows = append(rows, rline{text: base.Lines[i], kind: 1})
			}
			for i := ch.NewStart; i < ch.NewEnd; i++ {
				rows = append(rows, rline{text: preview.Lines[i], kind: 2, lineNum: i + 1})
			}
			oi, ni = ch.OldEnd, ch.NewEnd
			ci++
			continue
		}
		if ni < len(preview.Lines) {
			rows = append(rows, rline{text: preview.Lines[ni], kind: 0, lineNum: ni + 1})
			oi++
			ni++
		}
	}
	// vertical scroll: keep current line visible
	syn := u.syn.For(eng.Filename)
	viewTop := 0
	if eng.CurLine > 0 {
		viewTop = eng.CurLine - topRows/2
		if viewTop < 0 {
			viewTop = 0
		}
	}
	if viewTop > len(rows)-topRows {
		viewTop = len(rows) - topRows
	}
	if viewTop < 0 {
		viewTop = 0
	}
	for r := 0; r < topRows; r++ {
		idx := viewTop + r
		if idx >= len(rows) {
			break
		}
		rl := rows[idx]
		st := tcell.StyleDefault
		prefix := ""
		switch rl.kind {
		case 1:
			st = st.Foreground(tcell.ColorRed)
			prefix = "- "
		case 2:
			st = st.Foreground(tcell.ColorGreen)
			prefix = "+ "
		default:
			st = tcell.StyleDefault.Foreground(tcell.ColorDefault)
		}
		text := prefix + rl.text
		var styles []tcell.Style
		if rl.kind == 0 {
			styles = LineStyles(rl.text, syn, st)
		}
		drawStyledLine(s, 0, r, w, text, styles, st, rl.kind == 0)
	}
	if len(rows) == 0 && topRows > 0 {
		// empty buffer
	}

	// status line
	statusSt := tcell.StyleDefault.Background(tcell.ColorDarkBlue).Foreground(tcell.ColorWhite)
	status := fmt.Sprintf(" %s%s | %d lines, %d words, %d chars | cur %d | cmd %d/%d",
		eng.Filename, modMark(eng.Modified), preview.NumLines(), preview.WordCount(),
		preview.CharCount(), eng.CurLine, u.eng.Hist.Cursor, len(u.eng.Hist.Cmds))
	if eng.LastMsg != "" {
		status = " " + eng.LastMsg
	}
	for x := 0; x < w; x++ {
		r, _, _ := statusSt.Decompose()
		_ = r
		s.SetContent(x, statusRow, ' ', nil, statusSt)
	}
	drawText(s, 0, statusRow, w, status, statusSt)

	// command pane
	u.drawCmdPane(statusRow+1, h-statusRow-1, w)

	s.Show()
}

func modMark(m bool) string {
	if m {
		return " [modified]"
	}
	return ""
}

const minCmdRows = 3

func cmdPaneHeight(total int) int {
	ph := total / 5
	if ph < minCmdRows {
		ph = minCmdRows
	}
	if ph > 12 {
		ph = 12
	}
	return ph
}

// drawCmdPane renders the command history with the editing cursor line.
func (u *UI) drawCmdPane(top, height, width int) {
	s := u.screen
	eng := u.eng
	h := &eng.Hist
	// Build display rows: each history command is one or more lines; the
	// current edit occupies its position.
	type prow struct {
		text    string
		current bool
		body    bool
	}
	var rows []prow
	for i, c := range h.Cmds {
		if i == h.Cursor {
			continue
		}
		prompt := fmt.Sprintf("%3d ", i+1)
		for j, l := range strings.Split(c.Text, "\n") {
			p := prompt
			if j > 0 {
				p = "    "
			}
			rows = append(rows, prow{text: p + l, current: false, body: j > 0})
		}
	}
	// current editing command
	cur := h.Cursor
	if u.appendMode && u.appendIdx < len(h.Cmds) {
		// show body lines collected so far above the editing line
		for _, l := range strings.Split(h.Cmds[u.appendIdx].Text, "\n")[1:] {
			rows = append(rows, prow{text: "    " + l, current: true, body: true})
		}
	}
	for j, l := range u.editLines {
		p := fmt.Sprintf("%3d ", cur+1)
		if j > 0 {
			p = "    "
		}
		if j == u.editRow {
			p = fmt.Sprintf("%3d ", cur+1)
		}
		rows = append(rows, prow{text: p + l, current: true})
	}
	// scroll so the cursor row is visible
	curRow := 0
	for i := 0; i < len(rows); i++ {
		if rows[i].current {
			curRow = i
		}
	}
	viewTop := curRow - height + 1
	if viewTop > curRow {
		viewTop = curRow
	}
	if viewTop < 0 {
		viewTop = 0
	}
	for r := 0; r < height; r++ {
		idx := viewTop + r
		if idx >= len(rows) {
			break
		}
		pr := rows[idx]
		st := tcell.StyleDefault
		if pr.current {
			if pr.body {
				st = st.Foreground(tcell.ColorYellow)
			} else {
				st = st.Foreground(tcell.ColorYellow).Bold(true)
			}
		} else if pr.body {
			st = st.Foreground(tcell.ColorTeal)
		} else {
			st = st.Foreground(tcell.ColorGray)
		}
		drawText(s, 0, top+r, width, pr.text, st)
	}
}

// drawStyledLine draws text; if styles is nil, use fallback style for all.
func drawStyledLine(s tcell.Screen, x, y, maxw int, text string, styles []tcell.Style, fallback tcell.Style, isNormal bool) {
	runes := []rune(text)
	for i, r := range runes {
		if x+i >= maxw {
			break
		}
		st := fallback
		if isNormal && styles != nil {
			// styles are indexed by rune of the original text; adjust for prefix
			off := len(runes) - len(styles)
			if i-off >= 0 && i-off < len(styles) {
				st = styles[i-off]
			}
		}
		s.SetContent(x+i, y, r, nil, st)
	}
}

func drawText(s tcell.Screen, x, y, maxw int, text string, st tcell.Style) {
	for i, r := range []rune(text) {
		if x+i >= maxw {
			break
		}
		s.SetContent(x+i, y, r, nil, st)
	}
}

// buffersEqual compares two buffers line by line.
func buffersEqual(a, b *Buffer) bool {
	if a.NumLines() != b.NumLines() {
		return false
	}
	for i := range a.Lines {
		if a.Lines[i] != b.Lines[i] {
			return false
		}
	}
	return true
}
