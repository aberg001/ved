package main

import "github.com/gdamore/tcell/v2"

// Palette holds every color the UI paints with, so :set lightmode/darkmode
// can switch them in one place. Default is dark (terminal default bg).
type Palette struct {
	NumFg    tcell.Color // gutter line numbers
	DelFg    tcell.Color // diff: removed
	AddFg    tcell.Color // diff: added
	ModFg    tcell.Color // diff: changed
	MoveFg   tcell.Color // diff: moved/copied overlay
	DotBg    tcell.Color // background of the dot line
	StatusBg tcell.Color
	StatusFg tcell.Color
	ErrBg    tcell.Color
	ErrFg    tcell.Color
	HistCur  tcell.Color // history pane: current command
	HistBody tcell.Color // history pane: command body
	HistHint tcell.Color // history pane: hints / dim rows
}

var darkPalette = Palette{
	NumFg:    tcell.ColorSteelBlue,
	DelFg:    tcell.ColorRed,
	AddFg:    tcell.ColorGreen,
	ModFg:    tcell.ColorYellow,
	MoveFg:   tcell.ColorTeal,
	DotBg:    tcell.ColorDarkSlateGray,
	StatusBg: tcell.ColorDarkBlue,
	StatusFg: tcell.ColorWhite,
	ErrBg:    tcell.ColorDarkRed,
	ErrFg:    tcell.ColorWhite,
	HistCur:  tcell.ColorYellow,
	HistBody: tcell.ColorTeal,
	HistHint: tcell.ColorGray,
}

var lightPalette = Palette{
	NumFg:    tcell.ColorRoyalBlue,
	DelFg:    tcell.ColorOrangeRed,
	AddFg:    tcell.ColorDarkGreen,
	ModFg:    tcell.ColorOrange,
	MoveFg:   tcell.ColorDarkCyan,
	DotBg:    tcell.ColorLightGray,
	StatusBg: tcell.ColorDarkSlateGray,
	StatusFg: tcell.ColorWhite,
	ErrBg:    tcell.ColorOrangeRed,
	ErrFg:    tcell.ColorWhite,
	HistCur:  tcell.ColorOrange,
	HistBody: tcell.ColorDarkCyan,
	HistHint: tcell.ColorDarkGray,
}

// theme is the active palette; setLightMode switches it.
var theme = darkPalette

func setLightMode(on bool) {
	if on {
		theme = lightPalette
	} else {
		theme = darkPalette
	}
}
