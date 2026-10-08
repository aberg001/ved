package main

import "testing"

func TestSubstituteEscaping(t *testing.T) {
	cases := []struct {
		cmd, before, after string
	}{
		// escaped delimiter in replacement
		{"s/a/b\\/c/", "xax", "xb/cx"},
		// escaped delimiter in pattern: literal slash
		{"s/a\\/b/-/", "a/b", "-"},
		// & -> whole match
		{"s/o/[&]/", "foo", "f[o]o"},
		// \& -> literal &
		{"s/o/\\&/", "foo", "f&o"},
		// backreference \1 -> $1
		{"s/(f)(o)/\\2\\1/", "fo", "of"},
		// \n in replacement -> newline escape (Go \\n)
		{"s/o/\\n/", "foo", "f\\no"},
		// unescaped delim ends replacement; trailing text is flags
		{"s/a/X/g", "aa", "XX"},
		// only first occurrence without g
		{"s/a/X/", "aa", "Xa"},
	}
	for _, c := range cases {
		b := &Buffer{Lines: []string{c.before}}
		e := &Engine{}
		e.setDot(1)
		if err := ApplyCommand(b, e, c.cmd, false); err != nil {
			t.Errorf("%q: %v", c.cmd, err)
			continue
		}
		if b.Lines[0] != c.after {
			t.Errorf("%q: got %q want %q", c.cmd, b.Lines[0], c.after)
		}
	}
	// empty pattern rejected
	b := &Buffer{Lines: []string{"x"}}
	e := &Engine{}
		e.setDot(1)
	if err := ApplyCommand(b, e, "s//y/", false); err == nil {
		t.Error("empty pattern should error")
	}
}

func TestSubstituteLiveTyping(t *testing.T) {
	// while typing, no trailing delimiter yet: apply what's visible
	b := &Buffer{Lines: []string{"hello"}}
	e := &Engine{}
	e.setDot(1)
	if err := ApplyCommand(b, e, "s/l/L", true); err != nil {
		t.Fatalf("live: %v", err)
	}
	if b.Lines[0] != "heLlo" {
		t.Fatalf("live got %q", b.Lines[0])
	}
	// trailing backslash mid-typing must not panic
	b2 := &Buffer{Lines: []string{"x"}}
	if err := ApplyCommand(b2, e, "s/x/y\\", true); err != nil {
		t.Fatalf("trailing backslash: %v", err)
	}
}
