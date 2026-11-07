// Package puzzle defines a 1-star Star Battle (Meowdoku/Queens) puzzle layout:
// an NxN grid partitioned into N colored regions.
package puzzle

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"slices"
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
		//lint:ignore ST1005 N is the grid size
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
		if !p.connected(reg, cells) {
			return fmt.Errorf("region %c is not connected", alphabet[reg])
		}
	}
	return nil
}

// connected reports whether the cells of region reg (non-empty) are
// orthogonally connected.
func (p *Puzzle) connected(reg int, cells []Pos) bool {
	seen := map[Pos]bool{cells[0]: true}
	stack := []Pos{cells[0]}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, d := range [4]Pos{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			n := Pos{cur.Row + d.Row, cur.Col + d.Col}
			if seen[n] || !p.In(n.Row, n.Col) || p.Regions[n.Row][n.Col] != reg {
				continue
			}
			seen[n] = true
			stack = append(stack, n)
		}
	}
	return len(seen) == len(cells)
}

// Clone returns a deep copy.
func (p *Puzzle) Clone() *Puzzle {
	regs := make([][]int, p.N)
	for r := range regs {
		regs[r] = slices.Clone(p.Regions[r])
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
// letter is one region. If the letters are exactly the first N of the alphabet
// (A, B, C, ...), they keep their meaning (A = region 0), so explanations and
// rendering use the same letters as the file. Otherwise letters are mapped to
// indices in order of first appearance.
// Blank lines and lines starting with '#' are ignored; whitespace inside a line is ignored.
func Parse(r io.Reader) (*Puzzle, error) {
	sc := bufio.NewScanner(r)
	ids := map[rune]int{} // letter -> index in order of first appearance
	var rows [][]rune
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var row []rune
		for _, ch := range line {
			if ch == ' ' || ch == '\t' {
				continue
			}
			if _, ok := ids[ch]; !ok {
				ids[ch] = len(ids)
			}
			row = append(row, ch)
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(ids) != len(rows) {
		return nil, fmt.Errorf("found %d distinct regions in %d rows, want equal", len(ids), len(rows))
	}
	alphabetic := true
	for ch := range ids {
		if i := strings.IndexRune(alphabet, ch); i < 0 || i >= len(rows) {
			alphabetic = false
		}
	}
	if alphabetic {
		for ch := range ids {
			ids[ch] = strings.IndexRune(alphabet, ch)
		}
	}
	regions := make([][]int, len(rows))
	for r, row := range rows {
		for _, ch := range row {
			regions[r] = append(regions[r], ids[ch])
		}
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

// UnitKind distinguishes the three kinds of units that each hold exactly one star.
type UnitKind uint8

const (
	Row UnitKind = iota
	Col
	Region
)

func (k UnitKind) String() string {
	return [...]string{"row", "col", "region"}[k]
}

// Unit is a row, column or region together with its cells.
type Unit struct {
	Kind  UnitKind
	Index int
	Cells []Pos
}

func (u Unit) String() string {
	if u.Kind == Region {
		return fmt.Sprintf("region %c", alphabet[u.Index])
	}
	return fmt.Sprintf("%v %d", u.Kind, u.Index)
}

// Units returns all 3N units: rows, then columns, then regions.
func (p *Puzzle) Units() []Unit {
	units := make([]Unit, 0, 3*p.N)
	for r := range p.N {
		u := Unit{Kind: Row, Index: r}
		for c := range p.N {
			u.Cells = append(u.Cells, Pos{r, c})
		}
		units = append(units, u)
	}
	for c := range p.N {
		u := Unit{Kind: Col, Index: c}
		for r := range p.N {
			u.Cells = append(u.Cells, Pos{r, c})
		}
		units = append(units, u)
	}
	for reg, cells := range p.RegionCells() {
		units = append(units, Unit{Kind: Region, Index: reg, Cells: cells})
	}
	return units
}

// UnitOf returns the index of the unit of the given kind that contains q.
func (p *Puzzle) UnitOf(kind UnitKind, q Pos) int {
	switch kind {
	case Row:
		return q.Row
	case Col:
		return q.Col
	default:
		return p.Regions[q.Row][q.Col]
	}
}
