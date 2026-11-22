package observers

import (
	"fmt"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/words"
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
		u := o.Unit
		return []solver.Mark{solver.Star(q, o.Name()).Why(func(w *words.Namer) string {
			return fmt.Sprintf("In %s ist nur noch das Feld %s frei → Stern", w.Unit(u.Kind, u.Index), w.Cell(q))
		})}
	}
	return nil
}
