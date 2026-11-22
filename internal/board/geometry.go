package board

import "starbattle/internal/puzzle"

// geometry holds what follows from the layout alone: the units and what a
// star on each cell implies. New builds it once per state it creates and
// Clone shares it, so it is read-only after New returns; goroutines that each
// own a clone share it without locks. It relies on a puzzle not changing once
// a board exists for it.
type geometry struct {
	units   []puzzle.Unit  // rows, then columns, then regions, as puzzle.Units
	implied [][]puzzle.Pos // implied[cell] = crosses of a star on cell, in the order of implied
	covers  []bool         // covers[cell*N*N+other] = a star on cell crosses other
}

func newGeometry(p *puzzle.Puzzle) *geometry {
	nn := p.N * p.N
	g := &geometry{
		units:   p.Units(),
		implied: make([][]puzzle.Pos, nn),
		covers:  make([]bool, nn*nn),
	}
	regions := p.RegionCells()
	for r := range p.N {
		for c := range p.N {
			cell := r*p.N + c
			g.implied[cell] = implied(p, regions, r, c)
			for _, q := range g.implied[cell] {
				g.covers[cell*nn+q.Row*p.N+q.Col] = true
			}
		}
	}
	return g
}

// implied lists the cells that must be crosses if a star sits at (r, c): the
// rest of its row, column and region, plus its 8 neighbors. The order (row and
// column interleaved, then region, then neighbors, each cell once) is the order
// of the crosses in a step, so the trace depends on it.
func implied(p *puzzle.Puzzle, regions [][]puzzle.Pos, r, c int) []puzzle.Pos {
	seen := make([]bool, p.N*p.N)
	seen[r*p.N+c] = true
	out := make([]puzzle.Pos, 0, 4*p.N)
	add := func(q puzzle.Pos) {
		if !seen[q.Row*p.N+q.Col] {
			seen[q.Row*p.N+q.Col] = true
			out = append(out, q)
		}
	}
	for i := range p.N {
		add(puzzle.Pos{Row: r, Col: i})
		add(puzzle.Pos{Row: i, Col: c})
	}
	for _, q := range regions[p.Region(r, c)] {
		add(q)
	}
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			if p.In(r+dr, c+dc) {
				add(puzzle.Pos{Row: r + dr, Col: c + dc})
			}
		}
	}
	return out
}

// Units returns all 3N units of the puzzle: rows, then columns, then regions.
// The slice is shared by all clones and must not be modified.
func (s *State) Units() []puzzle.Unit { return s.g.units }

// Implied returns the cells that must be crosses if a star sits at q: the rest
// of its row, column and region, plus its 8 neighbors. The slice is shared by
// all clones and must not be modified.
func (s *State) Implied(q puzzle.Pos) []puzzle.Pos { return s.g.implied[q.Row*s.P.N+q.Col] }

// CrossesAll reports whether a star on star would cross every cell of cells.
func (s *State) CrossesAll(star puzzle.Pos, cells []puzzle.Pos) bool {
	n := s.P.N
	row := s.g.covers[(star.Row*n+star.Col)*n*n:][:n*n]
	for _, q := range cells {
		if !row[q.Row*n+q.Col] {
			return false
		}
	}
	return true
}
