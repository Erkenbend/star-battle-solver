package words

import (
	"fmt"
	"reflect"
	"testing"

	"starbattle/internal/puzzle"
)

func TestUnits(t *testing.T) {
	for _, tt := range []struct {
		kind puzzle.UnitKind
		idx  []int
		want string
	}{
		{puzzle.Row, []int{3}, "Zeile 3"},
		{puzzle.Col, []int{3, 2}, "Spalten 2 und 3"},
		{puzzle.Region, []int{3, 0, 1}, "Farben A, B und D"},
		{puzzle.Row, []int{5, 3, 4}, "Zeilen 3 bis 5"},
		{puzzle.Col, []int{1, 2, 4}, "Spalten 1, 2 und 4"},
		{puzzle.Region, []int{0, 1, 2}, "Farben A, B und C"},
		{puzzle.Region, []int{4}, "Farbe E"},
	} {
		if got := Units(tt.kind, tt.idx); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}

func TestCells(t *testing.T) {
	q := func(r, c int) puzzle.Pos { return puzzle.Pos{Row: r, Col: c} }
	for _, tt := range []struct {
		cells []puzzle.Pos
		want  string
	}{
		{nil, ""},
		{[]puzzle.Pos{q(1, 2)}, "(1,2)"},
		{[]puzzle.Pos{q(1, 2), q(3, 4)}, "(1,2) und (3,4)"},
		{[]puzzle.Pos{q(1, 2), q(3, 4), q(5, 6)}, "(1,2), (3,4) und (5,6)"},
	} {
		if got := Cells(tt.cells); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}

// A Namer gives the same names and records each unit and cell once.
func TestNamer(t *testing.T) {
	var n Namer
	if n.Refs() != nil {
		t.Fatal("refs before anything was named")
	}
	text := fmt.Sprintf("%s und %s, dann %s: %s, %s",
		n.Units(puzzle.Row, []int{4, 3}), n.Units(puzzle.Col, []int{1}), n.Unit(puzzle.Region, 2),
		n.Cells([]puzzle.Pos{{Row: 2, Col: 3}, {Row: 0, Col: 0}}), n.Cell(puzzle.Pos{Row: 2, Col: 3}))
	if want := "Zeilen 3 und 4 und Spalte 1, dann Farbe C: (2,3) und (0,0), (2,3)"; text != want {
		t.Errorf("got %q, want %q", text, want)
	}
	got := n.Refs()
	want := &Refs{Cells: []puzzle.Pos{{Row: 2, Col: 3}, {Row: 0, Col: 0}}, Rows: []int{4, 3}, Cols: []int{1}, Regions: []int{2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	n.Unit(puzzle.Row, 7)
	n.Cell(puzzle.Pos{Row: 1, Col: 1})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("later names changed earlier refs: %+v", got)
	}
}
