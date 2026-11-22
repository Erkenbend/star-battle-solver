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
	// The cause decides the text, so grouping by it is grouping by text,
	// without writing any.
	byCause := map[cause][]puzzle.Pos{}
	var order []cause
	n := s.P.N
	for r := range n {
		for c := range n {
			cell := puzzle.Pos{Row: r, Col: c}
			if s.At(r, c) != board.Free || inUnit[cell] {
				continue
			}
			if s.CrossesAll(cell, free) {
				why := o.cause(s.P, cell, free)
				if _, seen := byCause[why]; !seen {
					order = append(order, why)
				}
				byCause[why] = append(byCause[why], cell)
			}
		}
	}
	var marks []solver.Mark
	for _, why := range order {
		reason := solver.Because(o.explain(why))
		for _, cell := range byCause[why] {
			m := solver.Cross(cell, o.Name())
			m.Reason = reason
			marks = append(marks, m)
		}
	}
	return marks
}

// cause is why a star on a cell would cross all free cells of the unit:
// they all lie in unit index of kind, which also contains the cell (kind ≥ 0,
// a puzzle.UnitKind), they all touch the cell (kind touching), or a mix of
// both (kind mixed).
type cause struct{ kind, index int }

const (
	touching = -1
	mixed    = -2
)

func (o Elimination) cause(p *puzzle.Puzzle, cell puzzle.Pos, free []puzzle.Pos) cause {
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
			return cause{int(kind), v}
		}
	}
	for _, q := range free {
		if abs(q.Row-cell.Row) > 1 || abs(q.Col-cell.Col) > 1 {
			return cause{kind: mixed}
		}
	}
	return cause{kind: touching}
}

// explain writes the text of a cause: for a unit the same wording as
// Confinement with k=1.
func (o Elimination) explain(c cause) func(w *words.Namer) string {
	u := o.Unit
	return func(w *words.Namer) string {
		switch c.kind {
		case touching:
			return "Die markierten Felder berühren jeweils alle freien Felder von " + w.Unit(u.Kind, u.Index) + " → X"
		case mixed:
			return "Ein Stern auf einem der markierten Felder würde alle freien Felder von " + w.Unit(u.Kind, u.Index) +
				" streichen (über Zeile, Spalte, Farbe und Nachbarschaft) → X"
		}
		return confined(w, u.Kind, []int{u.Index}, puzzle.UnitKind(c.kind), []int{c.index})
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
