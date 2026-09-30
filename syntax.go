package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Rule is one highlighting rule: a regex plus a tcell style.
type Rule struct {
	Re    *regexp.Regexp
	Style tcell.Style
}

// Syntax is a set of rules for one file type.
type Syntax struct {
	Exts  []string
	Rules []Rule
}

// SyntaxDB maps extension -> rules.
type SyntaxDB struct {
	ByExt  map[string]*Syntax
	Fallback *Syntax
}

// Color names accepted in the config.
var colorNames = map[string]tcell.Color{
	"red": tcell.ColorRed, "green": tcell.ColorGreen, "yellow": tcell.ColorYellow,
	"blue": tcell.ColorBlue, "magenta": tcell.ColorPurple, "cyan": tcell.ColorTeal,
	"white": tcell.ColorWhite, "gray": tcell.ColorGray, "grey": tcell.ColorGray,
	"orange": tcell.ColorOrangeRed, "darkred": tcell.ColorDarkRed, "darkgreen": tcell.ColorDarkGreen,
	"darkblue": tcell.ColorDarkBlue, "darkcyan": tcell.ColorDarkCyan, "darkmagenta": tcell.ColorDarkMagenta,
	"darkyellow": tcell.ColorOlive, "brown": tcell.ColorOlive,
	"bold": tcell.ColorDefault, // handled as attribute below
}

func parseStyle(spec string) tcell.Style {
	st := tcell.StyleDefault
	for _, tok := range strings.Fields(strings.ToLower(spec)) {
		switch tok {
		case "bold":
			st = st.Bold(true)
		case "italic":
			st = st.Italic(true)
		case "underline":
			st = st.Underline(true)
		case "dim":
			st = st.Dim(true)
		default:
			if c, ok := colorNames[tok]; ok && c != tcell.ColorDefault {
				st = st.Foreground(c)
			}
		}
	}
	return st
}

// LoadSyntaxDB reads a config file. Format:
//
//	# comment
//	syntax go .go .golang
//	rule  \b(func|if|else|for|range|var|const|return|package|import)\b  bold
//	rule  ".*"                                                          yellow
//	rule  //.*$                                                          gray
func LoadSyntaxDB(path string) *SyntaxDB {
	db := &SyntaxDB{ByExt: map[string]*Syntax{}}
	f, err := os.Open(path)
	if err != nil {
		return db
	}
	defer f.Close()
	var cur *Syntax
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "syntax":
			s := &Syntax{}
			for _, ext := range fields[2:] {
				s.Exts = append(s.Exts, strings.TrimPrefix(ext, "."))
			}
			for _, ext := range s.Exts {
				db.ByExt[ext] = s
			}
			cur = s
		case "rule":
			if cur == nil || len(fields) < 3 {
				continue
			}
			pat := strings.Join(fields[1:len(fields)-1], " ")
			spec := fields[len(fields)-1]
			re, err := regexp.Compile(pat)
			if err != nil {
				continue
			}
			cur.Rules = append(cur.Rules, Rule{Re: re, Style: parseStyle(spec)})
		}
	}
	return db
}

// For returns the syntax for a filename, or a generic fallback.
func (db *SyntaxDB) For(filename string) *Syntax {
	if filename == "" {
		return nil
	}
	ext := strings.TrimPrefix(filepath.Ext(filename), ".")
	if s, ok := db.ByExt[ext]; ok {
		return s
	}
	return nil
}

// LineStyles returns per-cell styles for a line, as a slice parallel to runes.
func LineStyles(line string, syn *Syntax, defStyle tcell.Style) []tcell.Style {
	runes := []rune(line)
	styles := make([]tcell.Style, len(runes))
	for i := range styles {
		styles[i] = defStyle
	}
	if syn == nil {
		return styles
	}
	for _, r := range syn.Rules {
		for _, loc := range r.Re.FindAllStringIndex(line, -1) {
			for i := loc[0]; i < loc[1] && i < len(runes); i++ {
				styles[i] = r.Style
			}
		}
	}
	return styles
}
