package observers

import (
	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/words"
)

// Elimination replaces the "Form" idea from the original plan: a star on cell c
// would cross out c's row, column, region and 8 neighbors. If that covers every
// free cell of some other unit U, U could no longer get a star, so c is a cross.
//
// This covers all the shape special cases at once, e.g. a 2x1 region crosses
// the cells next to it in the neighboring rows, and a small L-shaped region
// crosses the cell in its inner corner.
//
// One instance watches one unit U.
type Elimination struct {
	Unit puzzle.Unit
}

func (o Elimination) Name() string { return "Elimination[" + o.Unit.String() + "]" }

func (Elimination) Priority() int { return LevelElimination }

func (o Elimination) Done(s *board.State) bool { return s.HasStar(o.Unit.Cells) }

func (o Elimination) Deduce(s *board.State) []solver.Mark {
	if s.HasStar(o.Unit.Cells) {
		return nil
	}
	free := s.Free(o.Unit.Cells)
	if len(free) == 0 {
		return nil // contradiction, the runner reports it
	}
	inUnit := make(map[puzzle.Pos]bool, len(o.Unit.Cells))
	for _, q := range o.Unit.Cells {
		inUnit[q] = true
	}
	// Group the crosses by explanation so each one becomes a single step.
	byReason := map[string][]puzzle.Pos{}
	refs := map[string]*words.Refs{}
	var order []string
	n := s.P.N
	for r := range n {
		for c := range n {
			cell := puzzle.Pos{Row: r, Col: c}
			if s.At(r, c) != board.Free || inUnit[cell] {
				continue
			}
			if s.CrossesAll(cell, free) {
				why, r := o.explain(s.P, cell, free)
				if _, seen := byReason[why]; !seen {
					order = append(order, why)
					refs[why] = r
				}
				byReason[why] = append(byReason[why], cell)
			}
		}
	}
	var marks []solver.Mark
	for _, why := range order {
		for _, cell := range byReason[why] {
			marks = append(marks, solver.Cross(cell, o.Name()).Why(why, refs[why]))
		}
	}
	return marks
}

// explain says why a star on cell would cross all free cells of the unit:
//   - they all lie in one region / row / column that also contains cell
//     (same wording as Confinement with k=1),
//   - they all touch cell,
//   - or a mix of both.
//
// It also returns the units the text names.
func (o Elimination) explain(p *puzzle.Puzzle, cell puzzle.Pos, free []puzzle.Pos) (string, *words.Refs) {
	var w words.Namer
	self := []int{o.Unit.Index}
	for _, kind := range []puzzle.UnitKind{puzzle.Region, puzzle.Row, puzzle.Col} {
		if kind == o.Unit.Kind {
			continue
		}
		v := p.UnitOf(kind, cell)
		all := true
		for _, q := range free {
			all = all && p.UnitOf(kind, q) == v
		}
		if all {
			why := confined(&w, o.Unit.Kind, self, kind, []int{v})
			return why, w.Refs()
		}
	}
	touching := true
	for _, q := range free {
		touching = touching && abs(q.Row-cell.Row) <= 1 && abs(q.Col-cell.Col) <= 1
	}
	name := w.Unit(o.Unit.Kind, o.Unit.Index)
	if touching {
		return "Die markierten Felder berühren jeweils alle freien Felder von " + name + " → X", w.Refs()
	}
	return "Ein Stern auf einem der markierten Felder würde alle freien Felder von " + name +
		" streichen (über Zeile, Spalte, Farbe und Nachbarschaft) → X", w.Refs()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
