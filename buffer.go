package main

// Buffer is the text being edited. Lines do not include trailing newlines.
type Buffer struct {
	Lines []string
}

func (b *Buffer) Clone() *Buffer {
	n := make([]string, len(b.Lines))
	copy(n, b.Lines)
	return &Buffer{Lines: n}
}

func (b *Buffer) NumLines() int { return len(b.Lines) }

func (b *Buffer) WordCount() int {
	n := 0
	for _, l := range b.Lines {
		inWord := false
		for _, r := range l {
			if r == ' ' || r == '\t' || r == '\n' {
				inWord = false
			} else if !inWord {
				inWord = true
				n++
			}
		}
	}
	return n
}

func (b *Buffer) CharCount() int {
	n := 0
	for _, l := range b.Lines {
		n += len([]rune(l)) + 1
	}
	if n > 0 {
		n-- // last line has no trailing newline
	}
	return n
}

// Delete removes lines [start,end] (1-based, inclusive).
func (b *Buffer) Delete(start, end int) []string {
	removed := make([]string, end-start+1)
	copy(removed, b.Lines[start-1:end])
	rest := make([]string, 0, len(b.Lines)-(end-start+1))
	rest = append(rest, b.Lines[:start-1]...)
	rest = append(rest, b.Lines[end:]...)
	b.Lines = rest
	return removed
}

// InsertBefore inserts lines before the given 1-based line (len+1 => append).
func (b *Buffer) InsertBefore(line int, lines []string) {
	idx := line - 1
	if idx < 0 {
		idx = 0
	}
	if idx > len(b.Lines) {
		idx = len(b.Lines)
	}
	out := make([]string, 0, len(b.Lines)+len(lines))
	out = append(out, b.Lines[:idx]...)
	out = append(out, lines...)
	out = append(out, b.Lines[idx:]...)
	b.Lines = out
}
