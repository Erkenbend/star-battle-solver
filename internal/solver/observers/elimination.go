package observers

import (
	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
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
	Unit    puzzle.Unit
	implied *impliedTable
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
	var order []string
	n := s.P.N
	for r := range n {
		for c := range n {
			cell := puzzle.Pos{Row: r, Col: c}
			if s.At(r, c) != board.Free || inUnit[cell] {
				continue
			}
			if o.implied.coversAll(cell, free) {
				why := o.explain(s.P, cell, free)
				if _, seen := byReason[why]; !seen {
					order = append(order, why)
				}
				byReason[why] = append(byReason[why], cell)
			}
		}
	}
	var marks []solver.Mark
	for _, why := range order {
		for _, cell := range byReason[why] {
			marks = append(marks, solver.Cross(cell, o.Name()).Why(why))
		}
	}
	return marks
}

// explain says why a star on cell would cross all free cells of the unit:
//   - they all lie in one region / row / column that also contains cell
//     (same wording as Confinement with k=1),
//   - they all touch cell,
//   - or a mix of both.
func (o Elimination) explain(p *puzzle.Puzzle, cell puzzle.Pos, free []puzzle.Pos) string {
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
			return confined(o.Unit.Kind, self, kind, []int{v})
		}
	}
	touching := true
	for _, q := range free {
		touching = touching && abs(q.Row-cell.Row) <= 1 && abs(q.Col-cell.Col) <= 1
	}
	name := units(o.Unit.Kind, self)
	if touching {
		return "Die markierten Felder berühren jeweils alle freien Felder von " + name + " → X"
	}
	return "Ein Stern auf einem der markierten Felder würde alle freien Felder von " + name +
		" streichen (über Zeile, Spalte, Farbe und Nachbarschaft) → X"
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// impliedTable precomputes board.Implied for every cell (shared, read-only).
type impliedTable struct {
	n     int
	cover [][]bool // cover[cell][other] = a star on cell crosses other
}

func newImpliedTable(p *puzzle.Puzzle) *impliedTable {
	t := &impliedTable{n: p.N, cover: make([][]bool, p.N*p.N)}
	for r := range p.N {
		for c := range p.N {
			row := make([]bool, p.N*p.N)
			for _, q := range board.Implied(p, r, c) {
				row[q.Row*p.N+q.Col] = true
			}
			t.cover[r*p.N+c] = row
		}
	}
	return t
}

func (t *impliedTable) coversAll(cell puzzle.Pos, cells []puzzle.Pos) bool {
	row := t.cover[cell.Row*t.n+cell.Col]
	for _, q := range cells {
		if !row[q.Row*t.n+q.Col] {
			return false
		}
	}
	return true
}
