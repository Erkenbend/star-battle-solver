package observers

import (
	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// UnitFinish: a unit (row, column or region) without a star and with exactly one
// free cell gets its star there.
type UnitFinish struct {
	Unit puzzle.Unit
}

func (o UnitFinish) Name() string { return "UnitFinish[" + o.Unit.String() + "]" }

func (o UnitFinish) Done(s *board.State) bool { return s.HasStar(o.Unit.Cells) }

func (o UnitFinish) Deduce(s *board.State) []solver.Mark {
	if s.HasStar(o.Unit.Cells) {
		return nil
	}
	if free := s.Free(o.Unit.Cells); len(free) == 1 {
		return []solver.Mark{solver.Star(free[0], o.Name())}
	}
	return nil
}
