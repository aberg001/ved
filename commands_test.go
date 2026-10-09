package main
import "testing"

// TestSetTheme exercises :set lightmode/darkmode and rejects unknown options.
func TestSetTheme(t *testing.T) {
	b := NewBuffer()
e := NewEngine("", b)
	ApplyCommand(b, e, ":set lightmode", false)
	if err := ApplyCommand(b, e, ":set lightmode", false); err != nil || LastError != "" {
		t.Fatalf("lightmode errored: %v %q", err, LastError)
	}
	ApplyCommand(b, e, ":set darkmode", false)
	if LastError != "" {
		t.Fatalf("darkmode errored: %q", LastError)
	}
	ApplyCommand(b, e, ":set bogus", false)
	if LastError != "unknown option: bogus" {
		t.Fatalf("want unknown-option error, got %q", LastError)
	}
}

