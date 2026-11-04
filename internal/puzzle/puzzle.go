// Package puzzle defines a 1-star Star Battle (Meowdoku/Queens) puzzle layout:
// an NxN grid partitioned into N colored regions.
package puzzle

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Pos is a cell coordinate.
type Pos struct{ Row, Col int }

// Puzzle is an NxN grid; Regions[r][c] is the region (color) index in [0, N).
type Puzzle struct {
	N       int
	Regions [][]int
}

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// MaxN is the largest supported grid size (limited by the text format).
const MaxN = len(alphabet)

// New creates a puzzle from a region matrix and validates it.
func New(regions [][]int) (*Puzzle, error) {
	p := &Puzzle{N: len(regions), Regions: regions}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// Region returns the region index of a cell.
func (p *Puzzle) Region(r, c int) int { return p.Regions[r][c] }

// In reports whether (r, c) lies on the grid.
func (p *Puzzle) In(r, c int) bool { return r >= 0 && r < p.N && c >= 0 && c < p.N }

// RegionCells returns the cells of each region, indexed by region.
func (p *Puzzle) RegionCells() [][]Pos {
	out := make([][]Pos, p.N)
	for r := range p.N {
		for c := range p.N {
			reg := p.Regions[r][c]
			out[reg] = append(out[reg], Pos{r, c})
		}
	}
	return out
}

// Validate checks the grid is square, uses exactly N regions and every region is
// orthogonally connected.
func (p *Puzzle) Validate() error {
	if p.N == 0 {
		return errors.New("empty puzzle")
	}
	if p.N > MaxN {
		return fmt.Errorf("N=%d exceeds maximum %d", p.N, MaxN)
	}
	for r, row := range p.Regions {
		if len(row) != p.N {
			return fmt.Errorf("row %d has %d cells, want %d", r, len(row), p.N)
		}
		for c, reg := range row {
			if reg < 0 || reg >= p.N {
				return fmt.Errorf("cell (%d,%d): region %d out of range [0,%d)", r, c, reg, p.N)
			}
		}
	}
	for reg, cells := range p.RegionCells() {
		if len(cells) == 0 {
			return fmt.Errorf("region %c is empty", alphabet[reg])
		}
		if !p.connected(reg, cells[0], Pos{-1, -1}) {
			return fmt.Errorf("region %c is not connected", alphabet[reg])
		}
	}
	return nil
}

// RegionConnectedWithout reports whether region reg stays connected (and non-empty)
// when cell skip is removed from it.
func (p *Puzzle) RegionConnectedWithout(reg int, skip Pos) bool {
	for _, c := range p.RegionCells()[reg] {
		if c != skip {
			return p.connected(reg, c, skip)
		}
	}
	return false
}

func (p *Puzzle) connected(reg int, start, skip Pos) bool {
	seen := map[Pos]bool{start: true}
	stack := []Pos{start}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, d := range [4]Pos{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			n := Pos{cur.Row + d.Row, cur.Col + d.Col}
			if n == skip || seen[n] || !p.In(n.Row, n.Col) || p.Regions[n.Row][n.Col] != reg {
				continue
			}
			seen[n] = true
			stack = append(stack, n)
		}
	}
	want := 0
	for _, c := range p.RegionCells()[reg] {
		if c != skip {
			want++
		}
	}
	return len(seen) == want
}

// Clone returns a deep copy.
func (p *Puzzle) Clone() *Puzzle {
	regs := make([][]int, p.N)
	for r := range regs {
		regs[r] = append([]int(nil), p.Regions[r]...)
	}
	return &Puzzle{N: p.N, Regions: regs}
}

// String formats the puzzle as N lines of region letters.
func (p *Puzzle) String() string {
	var b strings.Builder
	for _, row := range p.Regions {
		for _, reg := range row {
			b.WriteByte(alphabet[reg])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// RegionLetter returns the letter used for a region in the text format.
func RegionLetter(reg int) byte { return alphabet[reg] }

// Parse reads the text format: N non-empty lines of N letters each. Each distinct
// letter is one region; letters are mapped to indices in order of first appearance.
// Blank lines and lines starting with '#' are ignored; whitespace inside a line is ignored.
func Parse(r io.Reader) (*Puzzle, error) {
	sc := bufio.NewScanner(r)
	ids := map[rune]int{}
	var regions [][]int
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var row []int
		for _, ch := range line {
			if ch == ' ' || ch == '\t' {
				continue
			}
			id, ok := ids[ch]
			if !ok {
				id = len(ids)
				ids[ch] = id
			}
			row = append(row, id)
		}
		regions = append(regions, row)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(ids) != len(regions) {
		return nil, fmt.Errorf("found %d distinct regions in %d rows, want equal", len(ids), len(regions))
	}
	return New(regions)
}

// MustParse parses a puzzle from a string and panics on error (for tests).
func MustParse(s string) *Puzzle {
	p, err := Parse(strings.NewReader(s))
	if err != nil {
		panic(err)
	}
	return p
}
