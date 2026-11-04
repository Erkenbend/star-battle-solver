package observers

import (
	"fmt"
	"slices"
	"strings"

	"starbattle/internal/puzzle"
)

// German phrases for the explanations shown by `solve -verbose`.

var unitWords = map[puzzle.UnitKind][2]string{ // singular, plural
	puzzle.Row:    {"Zeile", "Zeilen"},
	puzzle.Col:    {"Spalte", "Spalten"},
	puzzle.Region: {"Farbe", "Farben"},
}

// units names a set of units, e.g. "Zeile 3", "Spalten 2 und 3", "Farben A, B und D".
func units(kind puzzle.UnitKind, idx []int) string {
	idx = slices.Sorted(slices.Values(idx))
	names := make([]string, len(idx))
	for i, x := range idx {
		if kind == puzzle.Region {
			names[i] = string(puzzle.RegionLetter(x))
		} else {
			names[i] = fmt.Sprint(x)
		}
	}
	w := unitWords[kind]
	if len(names) == 1 {
		return w[0] + " " + names[0]
	}
	return w[1] + " " + strings.Join(names[:len(names)-1], ", ") + " und " + names[len(names)-1]
}

// confined explains a confinement: the free cells of the From-units lie only in
// the To-units, so the rest of the To-units is crossed. Examples:
//
//	Spalte 11 hat nur noch freie Felder der Farbe I → alle anderen Felder der Farbe I X
//	Zeilen 3 und 4 haben nur noch freie Felder in Spalten 0 und 1 → alle anderen Felder der Spalten 0 und 1 X
//	Farben B und C liegen nur noch in Zeilen 0 und 1 → alle anderen Felder der Zeilen 0 und 1 X
func confined(from puzzle.UnitKind, fromIdx []int, to puzzle.UnitKind, toIdx []int) string {
	f, t := units(from, fromIdx), units(to, toIdx)
	one := len(fromIdx) == 1
	var premise string
	switch {
	case from == puzzle.Region:
		premise = f + verb(one, " liegt", " liegen") + " nur noch in " + t
	case to == puzzle.Region:
		premise = f + verb(one, " hat", " haben") + " nur noch freie Felder der " + t
	default:
		premise = f + verb(one, " hat", " haben") + " nur noch freie Felder in " + t
	}
	return premise + " → alle anderen Felder der " + t + " X"
}

func verb(singular bool, s, p string) string {
	if singular {
		return s
	}
	return p
}

func cell(q puzzle.Pos) string { return fmt.Sprintf("(%d,%d)", q.Row, q.Col) }

func cells(qs []puzzle.Pos) string {
	out := make([]string, len(qs))
	for i, q := range qs {
		out[i] = cell(q)
	}
	return strings.Join(out, " ")
}

// cellsAnd lists cells as "(1,2)", "(1,2) und (3,4)", "(1,2), (3,4) und (5,6)".
func cellsAnd(qs []puzzle.Pos) string {
	out := make([]string, len(qs))
	for i, q := range qs {
		out[i] = cell(q)
	}
	if len(out) <= 1 {
		return strings.Join(out, "")
	}
	return strings.Join(out[:len(out)-1], ", ") + " und " + out[len(out)-1]
}

func keys(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
