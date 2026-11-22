package observers

import (
	"fmt"

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

func (UnitFinish) Priority() int { return LevelUnitFinish }

func (o UnitFinish) Done(s *board.State) bool { return s.HasStar(o.Unit.Cells) }

func (o UnitFinish) Deduce(s *board.State) []solver.Mark {
	if s.HasStar(o.Unit.Cells) {
		return nil
	}
	if s.FreeCount(o.Unit.Cells) == 1 {
		q, _ := s.FirstFree(o.Unit.Cells)
		why := fmt.Sprintf("In %s ist nur noch das Feld %s frei → Stern",
			units(o.Unit.Kind, []int{o.Unit.Index}), cell(q))
		return []solver.Mark{solver.Star(q, o.Name()).Why(why)}
	}
	return nil
}
