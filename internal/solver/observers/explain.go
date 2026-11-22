package observers

import (
	"maps"
	"slices"

	"starbattle/internal/puzzle"
	"starbattle/internal/words"
)

// German phrases for the explanations shown by `solve -verbose` and the
// pages. The names of units and cells come from package words; a rule writes
// its explanation in a function given to solver.Mark.Why or solver.Because,
// which names them through the words.Namer it gets.

// confined explains a confinement: the free cells of the From-units lie only in
// the To-units, so the rest of the To-units is crossed. Examples:
//
//	Spalte 11 hat nur noch freie Felder der Farbe I → alle anderen Felder der Farbe I X
//	Zeilen 3 und 4 haben nur noch freie Felder in Spalten 0 und 1 → alle anderen Felder der Spalten 0 und 1 X
//	Farben B und C liegen nur noch in Zeilen 0 und 1 → alle anderen Felder der Zeilen 0 und 1 X
func confined(w *words.Namer, from puzzle.UnitKind, fromIdx []int, to puzzle.UnitKind, toIdx []int) string {
	f, t := w.Units(from, fromIdx), w.Units(to, toIdx)
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

func keys(m map[int]bool) []int { return slices.Collect(maps.Keys(m)) }
