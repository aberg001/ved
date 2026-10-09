package main

import "testing"

func TestCommandsOnEmptyBuffer(t *testing.T) {
	for _, c := range []string{"t1", "t", "m1", "c", "d", "s/a/b/"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on %q: %v", c, r)
				}
			}()
			b := &Buffer{}
			e := NewEngine("", b)
			ApplyCommand(b, e, c, false)
		}()
	}
}
