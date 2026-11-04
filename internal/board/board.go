// Package board holds the mutable solving state of a puzzle. Cells only ever move
// from Free to Star or Cross, never back, which makes merging updates
// order-independent and idempotent.
package board

import (
	"fmt"
	"strings"

	"starbattle/internal/puzzle"
)

// Cell is the state of a single cell.
type Cell uint8

const (
	Free  Cell = iota // undecided
	Star              // "M": star / queen placed
	Cross             // "X": no star possible
)

func (c Cell) String() string {
	switch c {
	case Star:
		return "M"
	case Cross:
		return "X"
	default:
		return "."
	}
}

// State is the solving state for one puzzle. It is not safe for concurrent use;
// each goroutine should own its own copy (see Clone).
type State struct {
	P     *puzzle.Puzzle
	cells []Cell
}

// New returns an all-free state for p.
func New(p *puzzle.Puzzle) *State {
	return &State{P: p, cells: make([]Cell, p.N*p.N)}
}

// FromSolution returns a state with stars at (r, cols[r]) and crosses elsewhere.
func FromSolution(p *puzzle.Puzzle, cols []int) *State {
	s := New(p)
	for r := range p.N {
		for c := range p.N {
			if cols[r] == c {
				s.cells[r*p.N+c] = Star
			} else {
				s.cells[r*p.N+c] = Cross
			}
		}
	}
	return s
}

// At returns the state of cell (r, c).
func (s *State) At(r, c int) Cell { return s.cells[r*s.P.N+c] }

// Set sets cell (r, c) to v. It reports whether the cell changed. Setting a cell to
// its current value is a no-op; overwriting a decided cell with a different value
// is a conflict and returns an error (the state is left unchanged).
func (s *State) Set(r, c int, v Cell) (bool, error) {
	cur := s.cells[r*s.P.N+c]
	switch {
	case cur == v:
		return false, nil
	case cur != Free:
		return false, fmt.Errorf("Konflikt bei (%d,%d): ist %v, soll %v werden", r, c, cur, v)
	case v == Free:
		return false, fmt.Errorf("cannot reset (%d,%d) to free", r, c)
	}
	s.cells[r*s.P.N+c] = v
	return true, nil
}

// Clone returns an independent copy (the puzzle layout is shared, it is immutable).
func (s *State) Clone() *State {
	return &State{P: s.P, cells: append([]Cell(nil), s.cells...)}
}

// Equal reports whether two states have identical cells.
func (s *State) Equal(o *State) bool {
	if len(s.cells) != len(o.cells) {
		return false
	}
	for i := range s.cells {
		if s.cells[i] != o.cells[i] {
			return false
		}
	}
	return true
}

// Implied returns the cells that must be crosses if a star sits at (r, c): the rest of
// its row, column and region, plus its 8 neighbors.
func Implied(p *puzzle.Puzzle, r, c int) []puzzle.Pos {
	seen := map[puzzle.Pos]bool{}
	var out []puzzle.Pos
	add := func(q puzzle.Pos) {
		if q != (puzzle.Pos{Row: r, Col: c}) && !seen[q] {
			seen[q] = true
			out = append(out, q)
		}
	}
	for i := range p.N {
		add(puzzle.Pos{Row: r, Col: i})
		add(puzzle.Pos{Row: i, Col: c})
	}
	for _, q := range p.RegionCells()[p.Region(r, c)] {
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

// unitCounts tallies stars and free cells of one unit.
type unitCounts struct{ stars, free int }

func (s *State) units() (rows, cols, regs []unitCounts) {
	n := s.P.N
	rows, cols, regs = make([]unitCounts, n), make([]unitCounts, n), make([]unitCounts, n)
	for r := range n {
		for c := range n {
			reg := s.P.Region(r, c)
			switch s.At(r, c) {
			case Star:
				rows[r].stars++
				cols[c].stars++
				regs[reg].stars++
			case Free:
				rows[r].free++
				cols[c].free++
				regs[reg].free++
			}
		}
	}
	return
}

// Contradiction returns a non-nil error if the state can no longer lead to a valid
// solution: a unit with two stars, a unit with no star and no free cell, or two
// touching stars.
func (s *State) Contradiction() error {
	rows, cols, regs := s.units()
	check := func(kind string, us []unitCounts, name func(int) string) error {
		for i, u := range us {
			if u.stars > 1 {
				return fmt.Errorf("%s %s hätte %d Sterne", kind, name(i), u.stars)
			}
			if u.stars == 0 && u.free == 0 {
				return fmt.Errorf("%s %s hätte keinen Stern und kein freies Feld mehr", kind, name(i))
			}
		}
		return nil
	}
	num := func(i int) string { return fmt.Sprint(i) }
	if err := check("Zeile", rows, num); err != nil {
		return err
	}
	if err := check("Spalte", cols, num); err != nil {
		return err
	}
	if err := check("Farbe", regs, func(i int) string { return string(puzzle.RegionLetter(i)) }); err != nil {
		return err
	}
	n := s.P.N
	for r := range n {
		for c := range n {
			if s.At(r, c) != Star {
				continue
			}
			// Only look "forward" so each pair is checked once.
			for _, d := range [4]puzzle.Pos{{Row: 0, Col: 1}, {Row: 1, Col: -1}, {Row: 1, Col: 0}, {Row: 1, Col: 1}} {
				rr, cc := r+d.Row, c+d.Col
				if s.P.In(rr, cc) && s.At(rr, cc) == Star {
					return fmt.Errorf("die Sterne auf (%d,%d) und (%d,%d) würden sich berühren", r, c, rr, cc)
				}
			}
		}
	}
	return nil
}

// Solved reports whether every row has exactly one star and the state is consistent.
func (s *State) Solved() bool {
	rows, _, _ := s.units()
	for _, u := range rows {
		if u.stars != 1 {
			return false
		}
	}
	return s.Contradiction() == nil
}

// StarCols returns, for each row, the column of its star or -1.
func (s *State) StarCols() []int {
	out := make([]int, s.P.N)
	for r := range s.P.N {
		out[r] = -1
		for c := range s.P.N {
			if s.At(r, c) == Star {
				out[r] = c
				break
			}
		}
	}
	return out
}

// String is a plain-text dump (M / X / .), useful in test failures.
func (s *State) String() string {
	var b strings.Builder
	for r := range s.P.N {
		for c := range s.P.N {
			b.WriteString(s.At(r, c).String())
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// Parse builds a state from N lines of 'M' (star), 'X' (cross) and '.' (free),
// the same format String produces. Intended for tests. Blank lines and spaces are ignored.
func Parse(p *puzzle.Puzzle, text string) (*State, error) {
	s := New(p)
	r := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.ReplaceAll(strings.TrimSpace(line), " ", "")
		if line == "" {
			continue
		}
		if r >= p.N || len(line) != p.N {
			return nil, fmt.Errorf("line %d: want %d rows of %d cells", r, p.N, p.N)
		}
		for c, ch := range line {
			switch ch {
			case 'M', 'm', '*':
				s.cells[r*p.N+c] = Star
			case 'X', 'x':
				s.cells[r*p.N+c] = Cross
			case '.':
			default:
				return nil, fmt.Errorf("line %d: unexpected %q", r, ch)
			}
		}
		r++
	}
	if r != p.N {
		return nil, fmt.Errorf("got %d rows, want %d", r, p.N)
	}
	return s, nil
}

// MustParse is Parse that panics on error.
func MustParse(p *puzzle.Puzzle, text string) *State {
	s, err := Parse(p, text)
	if err != nil {
		panic(err)
	}
	return s
}

// Free returns the free cells among cells.
func (s *State) Free(cells []puzzle.Pos) []puzzle.Pos {
	var out []puzzle.Pos
	for _, q := range cells {
		if s.At(q.Row, q.Col) == Free {
			out = append(out, q)
		}
	}
	return out
}

// HasStar reports whether any of cells holds a star.
func (s *State) HasStar(cells []puzzle.Pos) bool {
	for _, q := range cells {
		if s.At(q.Row, q.Col) == Star {
			return true
		}
	}
	return false
}
