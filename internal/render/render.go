// Package render draws puzzles and solving states in the terminal using ANSI
// 256-color backgrounds, one color per region.
package render

import (
	"fmt"
	"strings"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
)

// palette of reasonably distinct, light ANSI-256 background colors (black text stays readable).
var palette = []int{
	203, 215, 228, 120, 87, 111, 177, 218, 180, 250, 157, 229,
	75, 141, 210, 114, 222, 159, 183, 224, 151, 117, 189, 216,
}

// Options control the output.
type Options struct {
	// NoColor prints region letters instead of colored backgrounds.
	NoColor bool
	// Highlight marks cells (e.g. the most recent deductions) with red text.
	Highlight map[puzzle.Pos]bool
}

// Puzzle renders an empty puzzle layout.
func Puzzle(p *puzzle.Puzzle, o Options) string {
	return State(board.New(p), o)
}

// State renders the puzzle with the stars and crosses of s.
func State(s *board.State, o Options) string {
	p := s.P
	var b strings.Builder
	b.WriteString("   ")
	for c := range p.N {
		fmt.Fprintf(&b, "%2d ", c)
	}
	b.WriteByte('\n')
	for r := range p.N {
		fmt.Fprintf(&b, "%2d ", r)
		for c := range p.N {
			reg := p.Region(r, c)
			sym := " "
			switch s.At(r, c) {
			case board.Star:
				sym = "★"
			case board.Cross:
				sym = "·"
			}
			hl := o.Highlight[puzzle.Pos{Row: r, Col: c}]
			free := sym == " "
			if free {
				// Show the region letter so explanations like "Farbe B" can be matched.
				sym = string(puzzle.RegionLetter(reg))
			}
			if o.NoColor {
				if hl {
					fmt.Fprintf(&b, "[%s]", sym)
				} else {
					fmt.Fprintf(&b, " %s ", sym)
				}
				continue
			}
			fg := "30" // black
			if free {
				fg = "38;5;240" // gray letter, so stars and crosses stand out
			}
			if hl {
				fg = "1;31" // bold red
			}
			fmt.Fprintf(&b, "\x1b[48;5;%dm\x1b[%sm %s \x1b[0m", palette[reg%len(palette)], fg, sym)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
