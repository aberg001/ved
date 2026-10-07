package main

import (
	"fmt"
	"os"
	"regexp"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

type addrParser struct {
	s        string
	i        int
	e        *Engine
	buf      *Buffer
	hadComma bool // a range separator was seen (for j's default addressing)
}

func (a *addrParser) parseOne() (int, error) {
	// parse one term: base [+/- n]
	skipWS := func() {
		for a.i < len(a.s) && (a.s[a.i] == ' ' || a.s[a.i] == '\t') {
			a.i++
		}
	}
	skipWS()
	base := 0
	have := false
	if a.i < len(a.s) {
		switch a.s[a.i] {
		case '.':
			a.i++
			if a.e.getDot() == 0 && len(a.buf.Lines) > 0 {
				return 0, fmt.Errorf("no current line")
			}
			base = a.e.getDot()
			have = true
		case '$':
			a.i++
			base = len(a.buf.Lines)
			have = true
		case '\'':
			a.i++
			if a.i >= len(a.s) {
				return 0, fmt.Errorf("no mark")
			}
			ch := a.s[a.i]
			a.i++
			if a.e.Marks == nil {
				return 0, fmt.Errorf("no such mark")
			}
			l, ok := a.e.Marks[ch]
			if !ok || l < 1 || (len(a.buf.Lines) > 0 && l > len(a.buf.Lines)) {
				return 0, fmt.Errorf("no such mark")
			}
			base = l
			have = true
		case '/':
			end := strings.IndexByte(a.s[a.i+1:], '/')
			if end < 0 {
				return 0, fmt.Errorf("unterminated regex")
			}
			pat := a.s[a.i+1 : a.i+1+end]
			a.i += end + 2
			if pat == "" {
				pat = a.e.LastRegex
			} else {
				a.e.LastRegex = pat
			}
			start := a.e.getDot()
			if start == 0 {
				start = 0
			}
			re, err := regexp.Compile(pat)
			if err != nil {
				return 0, err
			}
			for k := 1; k <= len(a.buf.Lines); k++ {
				l := (start+k-1)%len(a.buf.Lines) + 1
				if re.MatchString(a.buf.Lines[l-1]) {
					return l, nil
				}
			}
			return 0, fmt.Errorf("no match")
		case '?':
			end := strings.IndexByte(a.s[a.i+1:], '?')
			if end < 0 {
				return 0, fmt.Errorf("unterminated regex")
			}
			pat := a.s[a.i+1 : a.i+1+end]
			a.i += end + 2
			if pat == "" {
				pat = a.e.LastRegex
			} else {
				a.e.LastRegex = pat
			}
			re, err := regexp.Compile(pat)
			if err != nil {
				return 0, err
			}
			start := a.e.getDot()
			if start == 0 {
				start = len(a.buf.Lines) + 1
			}
			for k := 0; k < len(a.buf.Lines); k++ {
				l := start - 1 - k
				if l < 1 {
					l += len(a.buf.Lines)
				}
				if re.MatchString(a.buf.Lines[l-1]) {
					return l, nil
				}
			}
			return 0, fmt.Errorf("no match")
		}
	}
	if !have {
		// number?
		j := a.i
		for j < len(a.s) && a.s[j] >= '0' && a.s[j] <= '9' {
			j++
		}
		if j > a.i {
			n, err := strconv.Atoi(a.s[a.i:j])
			if err != nil {
				return 0, err
			}
			a.i = j
			base = n
			have = true
		}
	}
	if !have {
		// An empty buffer defaults to address 0 (ed allows `a`/`i` into a
		// brand-new file; commands like `s` will fail later with a sensible
		// address error).
		if a.e.getDot() == 0 && len(a.buf.Lines) > 0 {
			return 0, fmt.Errorf("no current line")
		}
		base = a.e.getDot()
		have = true
	}
	// suffixes +/-n
	for a.i < len(a.s) && (a.s[a.i] == '+' || a.s[a.i] == '-' || a.s[a.i] == ' ' || a.s[a.i] == '\t') {
		sign := 1
		if a.s[a.i] == '-' {
			sign = -1
		}
		a.i++
		j := a.i
		for j < len(a.s) && a.s[j] >= '0' && a.s[j] <= '9' {
			j++
		}
		n := 1
		if j > a.i {
			v, err := strconv.Atoi(a.s[a.i:j])
			if err != nil {
				return 0, err
			}
			n = v
		}
		a.i = j
		base += sign * n
	}
	if base < 0 {
		return 0, fmt.Errorf("address out of range")
	}
	return base, nil
}

func (a *addrParser) parseRange() (int, int, error) {
	skip := func() {
		for a.i < len(a.s) && (a.s[a.i] == ' ' || a.s[a.i] == '\t') {
			a.i++
		}
	}
	skip()
	if a.i < len(a.s) && (a.s[a.i] == ',' || a.s[a.i] == '%' || a.s[a.i] == ';') {
		a.hadComma = true
		a.i++
		skip()
		l1 := 1
		l2 := len(a.buf.Lines)
		afterComma := func() bool {
			if a.i >= len(a.s) {
				return false
			}
			c := a.s[a.i]
			return c == '.' || c == '$' || c == '/' || c == '?' ||
				(c >= '0' && c <= '9') || c == '+' || c == '-'
		}
		if afterComma() {
			var err error
			l1, err = a.parseOne()
			if err != nil {
				return 0, 0, err
			}
			skip()
			if a.i < len(a.s) && a.s[a.i] == ',' {
				a.i++
				skip()
				if a.i >= len(a.s) {
					l2 = len(a.buf.Lines)
				} else {
					l2, err = a.parseOne()
					if err != nil {
						return 0, 0, err
					}
				}
			} else {
				l2 = l1
			}
		}
		if l1 > l2 {
			return 0, 0, fmt.Errorf("bad address range")
		}
		return l1, l2, nil
	}
	l1, err := a.parseOne()
	if err != nil {
		return 0, 0, err
	}
	l2 := l1
	if a.i < len(a.s) && (a.s[a.i] == ',' || a.s[a.i] == ';') {
		sep := a.s[a.i]
		a.hadComma = true
		a.i++
		if a.i >= len(a.s) || a.s[a.i] == ',' || a.s[a.i] == ';' {
			l2 = len(a.buf.Lines)
		} else {
			if sep == ';' {
				a.e.setDot(l1)
			}
			l2, err = a.parseOne()
			if err != nil {
				return 0, 0, err
			}
		}
	}
	if l1 > l2 {
		return 0, 0, fmt.Errorf("bad address range")
	}
	return l1, l2, nil
}

// ApplyCommand applies one command (possibly multi-line) to b.
// When live is false it also updates engine state (current line, modified).
func ApplyCommand(b *Buffer, e *Engine, text string, live bool) error {
	err := applyCommand(b, e, text, live)
	// LastError persists (ed: it is the reason for the most recent '?')
	// until the next error replaces it; 'h' reads it. Nothing clears it.
	return err
}

func applyCommand(b *Buffer, e *Engine, text string, live bool) error {
	lines := strings.Split(text, "\n")
	// trim a single trailing "" if text ended with \n
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil
	}
	cmdline := lines[0]
	body := lines[1:]
	// drop the "." terminator if present
	if len(body) > 0 && body[len(body)-1] == "." {
		body = body[:len(body)-1]
	}
	ap := &addrParser{s: cmdline, e: e, buf: b}
	l1, l2, err := ap.parseRange()
	if err != nil {
		LastError = err.Error()
		return err
	}
	rest := cmdline[ap.i:]
	rest = strings.TrimLeft(rest, " \t")
	if rest == "" {
		// bare address: print
		if l1 < 1 || l1 > len(b.Lines) {
			LastError = "address out of range"
			return fmt.Errorf("%s", LastError)
		}
		e.setDot(l1)
		e.LastMsg = b.Lines[l1-1]
		return nil
	}
	if len(rest) == 0 {
		// empty command text (mid-typing preview): no-op
		return nil
	}
	c := rest[0]
	if c == ':' {
		// extension command: strip the ':' prefix before dispatching
		rest = strings.TrimLeft(rest[1:], " \t")
		c = ':'
	}
	arg := ""
	if len(rest) > 1 {
		arg = strings.TrimLeft(rest[1:], " \t")
	}
	switch c {
	case 'a':
		b.InsertBefore(l2+1, body)
		if !live && len(body) > 0 {
			e.setDot(l2 + len(body))
			e.Modified = true
		}
	case 'i':
		b.InsertBefore(l1, body)
		if !live && len(body) > 0 {
			e.setDot(l1 + len(body) - 1)
			e.Modified = true
		}
	case 'c':
		if l1 < 1 || l2 > len(b.Lines) {
			LastError = "address out of range"
			return fmt.Errorf("%s", LastError)
		}
		b.Delete(l1, l2)
		b.InsertBefore(l1, body)
		if !live && len(body) > 0 {
			e.setDot(l1 + len(body) - 1)
			e.Modified = true
		}
	case 'd':
		if l1 < 1 || l2 > len(b.Lines) {
			LastError = "address out of range"
			return fmt.Errorf("%s", LastError)
		}
		b.Delete(l1, l2)
		if !live {
			e.Modified = true
			if l1 <= len(b.Lines) {
				e.setDot(l1)
			} else {
				e.setDot(len(b.Lines))
			}
		}
	case 'm':
		if l1 < 1 || l2 > len(b.Lines) {
			LastError = "address out of range"
			return fmt.Errorf("%s", LastError)
		}
		dest, err := parseSingle(arg, e, b)
		if err != nil {
			LastError = err.Error()
			return err
		}
		if dest >= l1 && dest <= l2 {
			LastError = "invalid destination"
			return fmt.Errorf("%s", LastError)
		}
		moved := make([]string, l2-l1+1)
		copy(moved, b.Lines[l1-1:l2])
		b.Delete(l1, l2)
		if dest > l2 {
			dest -= (l2 - l1 + 1)
		}
		b.InsertBefore(dest+1, moved)
		if !live {
			// dot = the last line of the moved block
			e.setDot(dest + (l2 - l1 + 1))
			e.Modified = true
		}
	case 't':
		dest, err := parseSingle(arg, e, b)
		if err != nil {
			LastError = err.Error()
			return err
		}
		cp := make([]string, l2-l1+1)
		copy(cp, b.Lines[l1-1:l2])
		b.InsertBefore(dest+1, cp)
		if !live {
			e.setDot(dest + len(cp))
			e.Modified = true
		}
	case 's':
		if l1 < 1 || l2 > len(b.Lines) {
			LastError = "address out of range"
			return fmt.Errorf("%s", LastError)
		}
		err := doSubstitute(b, arg, l1, l2)
		if err != nil {
			LastError = err.Error()
			return err
		}
		if !live {
			e.Modified = true
			e.setDot(l2)
		}
	case 'p', 'n':
		if l1 < 1 || l2 > len(b.Lines) {
			LastError = "address out of range"
			return fmt.Errorf("%s", LastError)
		}
		var sb strings.Builder
		for i := l1; i <= l2; i++ {
			if c == 'n' {
				fmt.Fprintf(&sb, "%d\t%s\n", i, b.Lines[i-1])
			} else {
				sb.WriteString(b.Lines[i-1])
				sb.WriteByte('\n')
			}
		}
		msg := strings.TrimRight(sb.String(), "\n")
		if strings.Contains(msg, "\n") {
			n := l2 - l1 + 1
			if c == 'n' {
				e.LastMsg = fmt.Sprintf("%d lines:", n)
			} else {
				e.LastMsg = fmt.Sprintf("printed %d lines", n)
			}
		} else {
			e.LastMsg = msg
		}
		if !live {
			e.setDot(l2)
		}
	case '=':
		_ = arg
		e.LastMsg = fmt.Sprintf("%d", l2)
		if !live {
			e.setDot(l2)
		}
	case 'w':
		if live {
			return nil
		}
		fname := arg
		if fname == "" {
			fname = e.Filename
		}
		data := strings.Join(b.Lines, "\n") + "\n"
		if err := os.WriteFile(fname, []byte(data), 0644); err != nil {
			LastError = err.Error()
			return err
		}
		e.Modified = false
	case 'W':
		if live {
			return nil
		}
		fname := arg
		if fname == "" {
			fname = e.Filename
		}
		f, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			LastError = err.Error()
			return err
		}
		f.WriteString(strings.Join(b.Lines, "\n") + "\n")
		f.Close()
	case 'r':
		if live {
			return nil
		}
		fname := arg
		if fname == "" {
			fname = e.Filename
		}
		data, err := os.ReadFile(fname)
		if err != nil {
			LastError = err.Error()
			return err
		}
		nl := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		b.InsertBefore(l2+1, nl)
		e.setDot(l2 + len(nl))
		e.Modified = true
	case 'e':
		if live {
			return nil
		}
		fname := arg
		if fname == "" {
			fname = e.Filename
		}
		data, err := os.ReadFile(fname)
		if err != nil {
			LastError = err.Error()
			return err
		}
		nl := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		if len(nl) == 1 && nl[0] == "" {
			nl = nil
		}
		b.Lines = nl
		e.Filename = fname
		e.Base = b.Clone()
		e.Hist = History{Cursor: 0}
		e.Snapshots = []Snapshot{{AfterIndex: -1, Buf: b.Clone()}}
		e.setDot(0)
		e.Modified = false
		e.Marks = map[byte]int{}
	case 'f':
		if !live && arg != "" {
			e.Filename = arg
		}
	case 'q', 'Q':
		// handled by UI layer
		if live {
			return nil
		}
		if c == 'q' && e.Modified && arg != "!" {
			if e.WarnedQuit {
				// second q: accept, like ed
				e.QuitRequested = true
				return nil
			}
			e.WarnedQuit = true
			LastError = "warning: buffer modified (use Q to force); q again to quit"
			return fmt.Errorf("%s", LastError)
		}
		e.QuitRequested = true
	case 'g', 'v', 'G', 'V':
		if ap.i == 0 {
			l1, l2 = 1, len(b.Lines) // ed: g/v default to the whole buffer
		}
		if l1 < 1 {
			l1 = 1
		}
		if l2 > len(b.Lines) {
			l2 = len(b.Lines)
		}
		if err := doGlobal(b, e, arg, body, c == 'v' || c == 'V', c == 'G' || c == 'V', l1, l2, live); err != nil {
			LastError = err.Error()
			return err
		}
	case 'j':
		// ed: j's default addresses are (.,.+1) only when no address is
		// given; an explicit single address joins that line alone (no-op)
		if ap.i == 0 {
			l1 = e.getDot()
			l2 = e.getDot() + 1
		}
		if l1 < 1 || l2 > len(b.Lines) {
			LastError = "address out of range"
			return fmt.Errorf("%s", LastError)
		}
		if l1 == l2 {
			e.setDot(l1)
			break
		}
		for i := l2 - 1; i >= l1; i-- {
			a2 := b.Lines[i]
			a1 := b.Lines[i-1]
			sep := ""
			if a1 != "" && a2 != "" &&
				!isSpaceByte(a1[len(a1)-1]) && !isSpaceByte(a2[0]) {
				sep = " "
			}
			b.Lines[i-1] = a1 + sep + a2
			b.Delete(i+1, i+1)
		}
		if !live {
			e.setDot(l1)
			e.Modified = true
		}
	case 'k':
		if arg == "" || len(arg) > 1 {
			LastError = "k: missing mark"
			return fmt.Errorf("%s", LastError)
		}
		if l1 < 1 || l2 > len(b.Lines) {
			LastError = "address out of range"
			return fmt.Errorf("%s", LastError)
		}
		if e.Marks == nil {
			e.Marks = map[byte]int{}
		}
		e.Marks[arg[0]] = l1
		if !live {
			e.setDot(l1)
		}
	case 'l':
		if l1 < 1 || l2 > len(b.Lines) {
			LastError = "address out of range"
			return fmt.Errorf("%s", LastError)
		}
		var sb strings.Builder
		for i := l1; i <= l2; i++ {
			sb.WriteString(escapeList(b.Lines[i-1]))
			sb.WriteByte('\n')
		}
		msg := strings.TrimRight(sb.String(), "\n")
		if strings.Contains(msg, "\n") {
			e.LastMsg = fmt.Sprintf("printed %d lines", l2-l1+1)
		} else {
			e.LastMsg = msg
		}
		if !live {
			e.setDot(l2)
		}
	case 'z':
		start := l2 + 1
		if ap.i == 0 {
			start = e.getDot() + 1
		}
		n := 22
		if arg != "" {
			if v, err := strconv.Atoi(arg); err == nil && v > 0 {
				n = v
			}
		}
		if start < 1 {
			start = 1
		}
		if start > len(b.Lines) {
			if !live {
				e.setDot(len(b.Lines))
			}
			e.LastMsg = "end of buffer"
			return nil
		}
		end := start + n - 1
		if end > len(b.Lines) {
			end = len(b.Lines)
		}
		if len(b.Lines) > 0 {
			e.LastMsg = b.Lines[end-1]
		} else {
			e.LastMsg = ""
		}
		if !live {
			e.setDot(end)
		}
	case '!':
		if live {
			out, err := exec.Command("/bin/sh", "-c", arg).CombinedOutput()
			if err != nil && len(out) == 0 {
				LastError = err.Error()
				return err
			}
			msg := strings.TrimRight(string(out), "\n")
			if msg == "" {
				msg = "!"
			}
			if strings.Contains(msg, "\n") {
				msg = msg[strings.LastIndex(msg, "\n")+1:]
				e.LastMsg = "!" + msg
			} else {
				e.LastMsg = "!" + msg
			}
		} else {
			// replay: shell effects are not re-executed
			return nil
		}
	case 'h':
		// ed: prints the reason for the most recent '?'; nothing if no error
		if LastError != "" {
			e.LastMsg = LastError
		}
	case 'H':
		e.HMode = !e.HMode
		if e.HMode && LastError != "" {
			e.LastMsg = LastError
		} else if e.HMode {
			e.LastMsg = "error messages on"
		} else {
			e.LastMsg = "error messages off"
		}
	case 'E':
		if live {
			return nil
		}
		fname := arg
		if fname == "" {
			fname = e.Filename
		}
		data, err := os.ReadFile(fname)
		if err != nil {
			LastError = err.Error()
			return err
		}
		nl := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		if len(nl) == 1 && nl[0] == "" {
			nl = nil
		}
		b.Lines = nl
		e.Filename = fname
		e.Base = b.Clone()
		e.Hist = History{Cursor: 0}
		e.Snapshots = []Snapshot{{AfterIndex: -1, Buf: b.Clone()}}
		e.setDot(0)
		e.Modified = false
		e.Marks = map[byte]int{}
	case 'u':
		// undo is handled live by the UI (it pops the last command from the
		// timeline); a u in a replayed timeline is a no-op
		return nil
	case ':':
		// extension commands: :set nu, :set nonu, ...
		handleExCommand(e, rest, live)

	default:
		// a bare number is a print command
		if n, err := strconv.Atoi(cmdline); err == nil {
			if n < 1 || n > len(b.Lines) {
				LastError = "address out of range"
				return fmt.Errorf("%s", LastError)
			}
			e.LastMsg = b.Lines[n-1]
			if !live {
				e.setDot(n)
			}
			return nil
		}
		LastError = fmt.Sprintf("unknown command: %q", c)
		return fmt.Errorf("%s", LastError)
	}
	return nil
}

func parseSingle(arg string, e *Engine, b *Buffer) (int, error) {
	ap := &addrParser{s: arg, e: e, buf: b}
	return ap.parseOne()
}

func doSubstitute(b *Buffer, arg string, l1, l2 int) error {
	if arg == "" {
		return fmt.Errorf("s: missing pattern")
	}
	delim := arg[0]
	rest := arg[1:]
	idx := strings.IndexByte(rest, delim)
	if idx < 0 {
		return fmt.Errorf("s: unterminated pattern")
	}
	pat := rest[:idx]
	tail := rest[idx+1:]
	end := strings.IndexByte(tail, delim)
	var repl, flags string
	if end < 0 {
		// still being typed: pattern complete, replacement is whatever
		// is on screen so far, no flags yet
		repl, flags = tail, ""
	} else {
		repl, flags = tail[:end], tail[end+1:]
	}
	global := strings.Contains(flags, "g")
	_ = repl
	re, err := regexp.Compile(pat)
	if err != nil {
		return err
	}
	if pat == "" && re != nil {
		// empty pattern: reuse last regex? keep simple: error
		return fmt.Errorf("s: empty pattern")
	}
	count := 0
	for i := l1; i <= l2; i++ {
		if global {
			nl := re.ReplaceAllString(b.Lines[i-1], repl)
			if nl != b.Lines[i-1] {
				b.Lines[i-1] = nl
				count++
			}
		} else {
			if loc := re.FindStringIndex(b.Lines[i-1]); loc != nil {
				b.Lines[i-1] = b.Lines[i-1][:loc[0]] + repl + b.Lines[i-1][loc[1]:]
				count++
			}
		}
	}
	if count == 0 {
		return fmt.Errorf("no match")
	}
	return nil
}

// isSpaceByte reports whether b is a space or tab.
func isSpaceByte(b byte) bool { return b == ' ' || b == '\t' }

// escapeList renders a line like ed's l command: control characters escaped,
// a trailing $ marking the end of line.
func escapeList(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString("\\\\")
		case '\a':
			sb.WriteString("\\a")
		case '\b':
			sb.WriteString("\\b")
		case '\f':
			sb.WriteString("\\f")
		case '\n':
			sb.WriteString("\\n")
		case '\r':
			sb.WriteString("\\r")
		case '\t':
			sb.WriteString("\\t")
		case '\v':
			sb.WriteString("\\v")
		default:
			if r < 32 || r == 127 {
				fmt.Fprintf(&sb, "\\%03o", r)
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('$')
	return sb.String()
}

// doGlobal implements g/v/G/V: apply tail (or interactive responses from
// body) to lines matching (or, for v/V, not matching) the pattern.
// Matches are collected up front, like ed.
func doGlobal(b *Buffer, e *Engine, arg string, body []string, inverse, interactive bool, l1, l2 int, live bool) error {
	if arg == "" {
		return fmt.Errorf("g: missing pattern")
	}
	delim := arg[0]
	rest := arg[1:]
	idx := strings.IndexByte(rest, delim)
	if idx < 0 {
		return fmt.Errorf("g: unterminated pattern")
	}
	pat := rest[:idx]
	tail := strings.TrimLeft(rest[idx+1:], " \t")
	if pat == "" {
		pat = e.LastRegex
		if pat == "" {
			return fmt.Errorf("g: no previous pattern")
		}
	} else {
		e.LastRegex = pat
	}
	if tail != "" && (tail[0] == 'g' || tail[0] == 'G' || tail[0] == 'v' || tail[0] == 'V') &&
		(len(tail) == 1 || tail[1] == ' ' || tail[1] == '\t' || tail[1] == '/') {
		return fmt.Errorf("g: cannot nest global commands")
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return err
	}
	var matches []int
	for i := l1; i <= l2 && i <= len(b.Lines); i++ {
		m := re.MatchString(b.Lines[i-1])
		if m != inverse {
			matches = append(matches, i)
		}
	}
	if interactive {
		// G/V: body lines are the per-match responses. Empty line: skip.
		// A lone ".": stop processing further matches.
		// Matches are ascending, but each response can change the buffer
		// size (d deletes, a/i/c inserts), so track the net shift and
		// compensate the index of later matches (ed uses line pointers).
		shift := 0
		for k, m := range matches {
			if k >= len(body) {
				break
			}
			resp := strings.TrimRight(body[k], " \t")
			if resp == "." {
				break
			}
			if resp == "" {
				continue
			}
			m += shift
			if m > len(b.Lines) {
				continue
			}
			e.setDot(m)
			before := len(b.Lines)
			if err := ApplyCommand(b, e, resp, live); err != nil {
				return err
			}
			shift += len(b.Lines) - before
		}
		if len(matches) > 0 {
			last := matches[len(matches)-1]
			if last <= len(b.Lines) {
				e.setDot(last)
			}
		}
		return nil
	}
	// non-interactive g/v: default command is p
	if tail == "" {
		tail = "p"
	}
	if len(body) > 0 {
		// fold the body into the tail command text
		tail = tail + "\n" + strings.Join(body, "\n")
	}
	// ed processes matches bottom-up so deletions don't shift pending
	// match line numbers
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		if m > len(b.Lines) {
			continue
		}
		e.setDot(m)
		if err := ApplyCommand(b, e, tail, live); err != nil {
			return err
		}
	}
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		if last <= len(b.Lines) {
			e.setDot(last)
		}
	}
	return nil
}

// handleExCommand parses ':' extension commands. These are display/settings
// commands with no buffer effect.
// helpTopics maps each command to a one-line usage shown by :help <cmd>.
// The status line is a single row, so topics must fit on one line.
var helpTopics = map[string]string{
	"a":    "[addr] a — append text after the addressed line; end with a lone '.'",
	"i":    "[addr] i — insert text before the addressed line; end with a lone '.'",
	"c":    "[addr[,addr]] c — change lines to new text; end with a lone '.'",
	"d":    "[addr[,addr]] d — delete the addressed lines",
	"m":    "[addr[,addr]] m dest — move lines after the destination address",
	"t":    "[addr[,addr]] t dest — copy (transfer) lines after the destination",
	"s":    "[addr[,addr]] s/pat/repl/[g] — substitute; 'g' replaces every match",
	"p":    "[addr[,addr]] p — print the addressed lines (sets dot)",
	"n":    "[addr[,addr]] n — print lines with line numbers",
	"=":    "= — print the line number of the addressed line (default: last)",
	"w":    "w [file] — write the buffer to file (default: the file being edited)",
	"W":    "W [file] — append the buffer to file",
	"r":    "r [file] — read file and insert its lines after dot",
	"e":    "e [file] — replace the buffer with the contents of file",
	"f":    "f [file] — show or set the current filename",
	"q":    "q — quit; warns if the buffer is modified (repeat to accept)",
	"Q":    "Q — quit unconditionally, even if the buffer is modified",
	".":    ". — address: the current line (dot); typed alone, prints it",
	"$":    "$ — address: the last line of the buffer",
	"set":  ":set nu|nonu — toggle line numbers in the buffer pane",
	"help": ":help [cmd] — show usage for cmd; no arg lists all commands",
	"g":    "[addr] g/pat/cmd — run cmd on lines matching pat (default: whole buffer)",
	"v":    "[addr] v/pat/cmd — run cmd on lines NOT matching pat",
	"G":    "[addr] G/pat — interactive: show each match, type a command per line",
	"V":    "[addr] V/pat — interactive: like G, on lines NOT matching pat",
	"j":    "[addr] j — join lines with spaces (default: dot and dot+1)",
	"k":    "[addr] kx — set mark x at the addressed line; address it as 'x",
	"l":    "[addr] l — print lines with escapes and a trailing $",
	"z":    "z[n] — print n lines starting after dot (default 22), set dot",
	"!":    "!cmd — run cmd through the shell; output shows on the status line",
	"h":    "h — print the last error message",
	"H":    "H — toggle printing of error messages as they happen",
	"E":    "E [file] — like e, but without the modified-buffer warning",
	"u":    "u — undo: removes the last command from the timeline",
}

func handleExCommand(e *Engine, rest string, live bool) {
	arg := strings.TrimSpace(rest)
	if arg == "" {
		LastError = "usage: :set nu|nonu|number|nonumber"
		return
	}
	f := strings.Fields(arg)
	switch f[0] {
	case "help":
		topic := strings.Join(f[1:], " ")
		if topic == "" {
			names := make([]string, 0, len(helpTopics))
			for k := range helpTopics {
				names = append(names, k)
			}
			sort.Strings(names)
			e.LastMsg = "commands: " + strings.Join(names, " ") + "  (:help cmd for details)"
			return
		}
		if txt, ok := helpTopics[topic]; ok {
			e.LastMsg = txt
			return
		}
		LastError = "no help for: " + topic
		return
	case "set":
		if len(f) < 2 {
			LastError = "usage: :set nu|nonu|number|nonumber"
			return
		}
		switch f[1] {
		case "nu", "number":
			e.ShowNumbers = true
		case "nonu", "nonumber":
			e.ShowNumbers = false
		default:
			LastError = "unknown option: " + f[1]
		}
	default:
		LastError = "unknown command: :" + f[0]
	}
}
