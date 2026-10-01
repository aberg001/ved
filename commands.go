package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type addrParser struct {
	s   string
	i   int
	e   *Engine
	buf *Buffer
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
			if a.e.CurLine == 0 && len(a.buf.Lines) > 0 {
				return 0, fmt.Errorf("no current line")
			}
			base = a.e.CurLine
			have = true
		case '$':
			a.i++
			base = len(a.buf.Lines)
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
			start := a.e.CurLine
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
			start := a.e.CurLine
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
		if a.e.CurLine == 0 && len(a.buf.Lines) > 0 {
			return 0, fmt.Errorf("no current line")
		}
		base = a.e.CurLine
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
	if a.i < len(a.s) && (a.s[a.i] == ',' || a.s[a.i] == '%') {
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
		a.i++
		if a.i >= len(a.s) || a.s[a.i] == ',' || a.s[a.i] == ';' {
			l2 = len(a.buf.Lines)
		} else {
			if sep == ';' {
				a.e.CurLine = l1
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
	LastError = ""
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
		e.CurLine = l1
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
			e.CurLine = l2 + len(body)
			e.Modified = true
		}
	case 'i':
		b.InsertBefore(l1, body)
		if !live && len(body) > 0 {
			e.CurLine = l1 + len(body) - 1
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
			e.CurLine = l1 + len(body) - 1
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
				e.CurLine = l1
			} else {
				e.CurLine = len(b.Lines)
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
			e.CurLine = dest + 1
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
			e.CurLine = dest + len(cp)
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
			e.CurLine = l2
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
			e.CurLine = l2
		}
	case '=':
		_ = arg
		if !live {
			e.CurLine = l2
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
		e.CurLine = l2 + len(nl)
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
		e.CurLine = 0
		e.Modified = false
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
				e.CurLine = n
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

// handleExCommand parses ':' extension commands. These are display/settings
// commands with no buffer effect.
func handleExCommand(e *Engine, rest string, live bool) {
	arg := strings.TrimSpace(rest)
	if arg == "" {
		LastError = "usage: :set nu|nonu|number|nonumber"
		return
	}
	f := strings.Fields(arg)
	switch f[0] {
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
