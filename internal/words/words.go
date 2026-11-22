// Package words names units and cells in German, the one source of these
// names for every explanation (board, solver and the rules), and records
// what an explanation names: a Namer gives the same names as the functions
// here and collects the units and cells in Refs, so a page can highlight
// them without reading the German text.
package words

import (
	"fmt"
	"slices"
	"strings"

	"starbattle/internal/puzzle"
)

var unitWords = [...][2]string{ // by puzzle.UnitKind: singular, plural
	puzzle.Row:    {"Zeile", "Zeilen"},
	puzzle.Col:    {"Spalte", "Spalten"},
	puzzle.Region: {"Farbe", "Farben"},
}

// Unit names one unit: "Zeile 3", "Spalte 0", "Farbe B".
func Unit(kind puzzle.UnitKind, i int) string {
	return unitWords[kind][0] + " " + index(kind, i)
}

// Units names a set of units, e.g. "Zeile 3", "Spalten 2 und 3", "Farben A, B und D".
// Three or more consecutive rows or columns become a range: "Zeilen 3 bis 5".
func Units(kind puzzle.UnitKind, idx []int) string {
	idx = slices.Sorted(slices.Values(idx))
	if n := len(idx); kind != puzzle.Region && n >= 3 && idx[n-1]-idx[0] == n-1 {
		return fmt.Sprintf("%s %d bis %d", unitWords[kind][1], idx[0], idx[n-1])
	}
	if len(idx) == 1 {
		return Unit(kind, idx[0])
	}
	names := make([]string, len(idx))
	for i, x := range idx {
		names[i] = index(kind, x)
	}
	return unitWords[kind][1] + " " + and(names)
}

// index is how a unit is told from the others of its kind: the number of a
// row or column, the letter of a region.
func index(kind puzzle.UnitKind, i int) string {
	if kind == puzzle.Region {
		return string(puzzle.RegionLetter(i))
	}
	return fmt.Sprint(i)
}

// Cell names a cell: "(2,3)".
func Cell(q puzzle.Pos) string { return fmt.Sprintf("(%d,%d)", q.Row, q.Col) }

// Cells lists cells as "(1,2)", "(1,2) und (3,4)", "(1,2), (3,4) und (5,6)".
func Cells(qs []puzzle.Pos) string {
	out := make([]string, len(qs))
	for i, q := range qs {
		out[i] = Cell(q)
	}
	return and(out)
}

// and joins "a", "a und b", "a, b und c".
func and(s []string) string {
	if len(s) <= 1 {
		return strings.Join(s, "")
	}
	return strings.Join(s[:len(s)-1], ", ") + " und " + s[len(s)-1]
}

// Refs are the cells and units an explanation names, each once, in the
// order they are first named in the text (a set of units in the order
// Units lists it), so the same text always has the same Refs.
type Refs struct {
	Cells   []puzzle.Pos
	Rows    []int
	Cols    []int
	Regions []int
}

// Empty reports whether r names nothing.
func (r *Refs) Empty() bool {
	return r == nil || len(r.Cells)+len(r.Rows)+len(r.Cols)+len(r.Regions) == 0
}

func (r *Refs) addUnit(kind puzzle.UnitKind, i int) {
	list := [...]*[]int{puzzle.Row: &r.Rows, puzzle.Col: &r.Cols, puzzle.Region: &r.Regions}[kind]
	if !slices.Contains(*list, i) {
		*list = append(*list, i)
	}
}

func (r *Refs) addCell(q puzzle.Pos) {
	if !slices.Contains(r.Cells, q) {
		r.Cells = append(r.Cells, q)
	}
}

// Namer names units and cells like the functions of this package and
// records everything it names. Its zero value is ready to use; use one per
// explanation.
type Namer struct {
	refs Refs
}

// Unit is Unit, recorded.
func (n *Namer) Unit(kind puzzle.UnitKind, i int) string {
	n.refs.addUnit(kind, i)
	return Unit(kind, i)
}

// Units is Units, recorded in the order of the text, not of idx: the
// rules pass map keys.
func (n *Namer) Units(kind puzzle.UnitKind, idx []int) string {
	for _, i := range slices.Sorted(slices.Values(idx)) {
		n.refs.addUnit(kind, i)
	}
	return Units(kind, idx)
}

// Cell is Cell, recorded.
func (n *Namer) Cell(q puzzle.Pos) string {
	n.refs.addCell(q)
	return Cell(q)
}

// Cells is Cells, recorded.
func (n *Namer) Cells(qs []puzzle.Pos) string {
	for _, q := range qs {
		n.refs.addCell(q)
	}
	return Cells(qs)
}

// Refs returns what n has named so far, or nil if nothing. Later names do
// not change the result.
func (n *Namer) Refs() *Refs {
	if n.refs.Empty() {
		return nil
	}
	return &Refs{
		Cells:   slices.Clip(n.refs.Cells),
		Rows:    slices.Clip(n.refs.Rows),
		Cols:    slices.Clip(n.refs.Cols),
		Regions: slices.Clip(n.refs.Regions),
	}
}
