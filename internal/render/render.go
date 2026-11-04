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
	// Highlight marks cells (e.g. the most recent deductions): drawn in [brackets].
	Highlight map[puzzle.Pos]bool
	// Conflict marks the cells that show a contradiction: drawn in (parentheses).
	Conflict map[puzzle.Pos]bool
}

// Puzzle renders an empty puzzle layout.
func Puzzle(p *puzzle.Puzzle, o Options) string {
	return State(board.New(p), o)
}

// State renders the puzzle with the stars and crosses of s. Highlighted cells
// (new marks) are drawn in brackets, new crosses as "×" instead of "·", and
// arrows on the edges point at the rows and columns that contain them.
func State(s *board.State, o Options) string {
	p := s.P
	hlRow, hlCol := make([]bool, p.N), make([]bool, p.N)
	for q := range o.Highlight {
		hlRow[q.Row], hlCol[q.Col] = true, true
	}
	for q := range o.Conflict {
		hlRow[q.Row], hlCol[q.Col] = true, true
	}
	mark := func(on bool, arrow string) string {
		if !on {
			return " "
		}
		if o.NoColor {
			return arrow
		}
		return "\x1b[1;31m" + arrow + "\x1b[0m"
	}

	var b strings.Builder
	if len(o.Highlight)+len(o.Conflict) > 0 {
		b.WriteString("    ")
		for c := range p.N {
			fmt.Fprintf(&b, " %s ", mark(hlCol[c], "▼"))
		}
		b.WriteByte('\n')
	}
	b.WriteString("    ")
	for c := range p.N {
		fmt.Fprintf(&b, "%2d ", c)
	}
	b.WriteByte('\n')
	for r := range p.N {
		fmt.Fprintf(&b, "%s%2d ", mark(hlRow[r], "▶"), r)
		for c := range p.N {
			reg := p.Region(r, c)
			hl := o.Highlight[puzzle.Pos{Row: r, Col: c}]
			free := false
			var sym string
			switch s.At(r, c) {
			case board.Star:
				sym = "★"
			case board.Cross:
				sym = "·"
				if hl {
					sym = "×"
				}
			default:
				// Show the region letter so explanations like "Farbe B" can be matched.
				sym, free = string(puzzle.RegionLetter(reg)), true
			}
			conflict := o.Conflict[puzzle.Pos{Row: r, Col: c}]
			left, right := " ", " "
			switch {
			case hl:
				left, right = "[", "]"
			case conflict:
				left, right = "(", ")"
			}
			if o.NoColor {
				b.WriteString(left + sym + right)
				continue
			}
			fg := "30" // black
			if free {
				fg = "38;5;240" // gray letter, so stars and crosses stand out
			}
			if hl || conflict {
				fg = "1;31" // bold red
			}
			fmt.Fprintf(&b, "\x1b[48;5;%dm\x1b[%sm%s%s%s\x1b[0m", palette[reg%len(palette)], fg, left, sym, right)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
