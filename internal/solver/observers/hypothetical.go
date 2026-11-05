package observers

import (
	"fmt"
	"strings"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// Both rules in this file look one assumption ahead: "if cell c held a star ...".
// They are the two most common patterns found in the lookahead chains of
// testdata/hard, turned into rules with a direct explanation.

// ForcedPartner: a star on c would leave some unit U with a single free cell q,
// so q would need a star too. If c and its forced partner(s) together cross out
// every cell of some unit V, c is a cross.
//
//	Ein Stern auf (0,2) ließe in Zeile 1 nur (1,0) übrig. Beide Sterne zusammen
//	würden Zeile 3 komplett streichen → X
//
// Depth is the maximal number of forced partners in a row (1 = just one).
// Without any partner the case is plain Elimination, so at least one is required.
type ForcedPartner struct {
	Cell  puzzle.Pos
	Depth int
	units []puzzle.Unit
}

func (ForcedPartner) Priority() int { return LevelForcedPartner }

func (o ForcedPartner) Name() string {
	return fmt.Sprintf("ForcedPartner[%s d=%d]", cell(o.Cell), o.Depth)
}

func (o ForcedPartner) Done(s *board.State) bool { return s.At(o.Cell.Row, o.Cell.Col) != board.Free }

func (o ForcedPartner) Deduce(s *board.State) []solver.Mark {
	t, ok := assume(s, o.Cell)
	if !ok || emptyUnit(t, o.units) != nil {
		return nil // no state, or plain Elimination: not this rule's job
	}
	chain, empty, ok := o.search(t, nil)
	if !ok {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Ein Stern auf %s ließe ", cell(o.Cell))
	for i, f := range chain {
		if i > 0 {
			b.WriteString(", dann ")
		}
		fmt.Fprintf(&b, "in %s nur %s übrig", units(f.unit.Kind, []int{f.unit.Index}), cell(f.cell))
	}
	together := "Beide Sterne zusammen"
	if len(chain) > 1 {
		together = "Alle diese Sterne zusammen"
	}
	fmt.Fprintf(&b, ". %s würden %s komplett streichen → X", together, units(empty.Kind, []int{empty.Index}))
	return []solver.Mark{solver.Cross(o.Cell, o.Name()).Why(b.String())}
}

type forced struct {
	unit puzzle.Unit
	cell puzzle.Pos
}

// search adds forced partners (depth-first, up to Depth) until some unit is empty.
// Forced stars all follow from the assumption, so any order is valid.
func (o ForcedPartner) search(t *board.State, chain []forced) ([]forced, *puzzle.Unit, bool) {
	if len(chain) >= o.Depth {
		return nil, nil, false
	}
	for _, u := range o.units {
		if t.HasStar(u.Cells) {
			continue
		}
		free := t.Free(u.Cells)
		if len(free) != 1 {
			continue
		}
		next := append(append([]forced(nil), chain...), forced{u, free[0]})
		t2, ok := assume(t, free[0])
		if !ok {
			continue
		}
		if empty := emptyUnit(t2, o.units); empty != nil {
			return next, empty, true
		}
		if c, e, ok := o.search(t2, next); ok {
			return c, e, true
		}
	}
	return nil, nil, false
}

// DeadCells: after a star on c, some unit V keeps only 2..MaxCells free cells,
// and a star on any of them would cross out all of another unit. V could not get
// its star, so c is a cross.
//
//	Nach einem Stern auf (2,5) bleiben in Spalte 3 nur (4,3) und (6,3). Ein Stern
//	auf (4,3) würde Farbe B streichen, einer auf (6,3) würde Zeile 7 streichen → X
//
// A single remaining cell is ForcedPartner's case, so at least 2 are required.
type DeadCells struct {
	Cell     puzzle.Pos
	MaxCells int
	units    []puzzle.Unit
}

func (DeadCells) Priority() int { return LevelDeadCells }

func (o DeadCells) Name() string {
	return fmt.Sprintf("DeadCells[%s max=%d]", cell(o.Cell), o.MaxCells)
}

func (o DeadCells) Done(s *board.State) bool { return s.At(o.Cell.Row, o.Cell.Col) != board.Free }

func (o DeadCells) Deduce(s *board.State) []solver.Mark {
	t, ok := assume(s, o.Cell)
	if !ok || emptyUnit(t, o.units) != nil {
		return nil // plain Elimination
	}
	for _, v := range o.units {
		if t.HasStar(v.Cells) {
			continue
		}
		free := t.Free(v.Cells)
		if len(free) < 2 || len(free) > o.MaxCells {
			continue
		}
		// Which unit each remaining cell would empty, grouped by that unit so that
		// "(3,0) oder (4,0) würde Spalte 2 streichen" reads as one clause.
		var order []*puzzle.Unit
		byUnit := map[*puzzle.Unit][]puzzle.Pos{}
		for _, d := range free {
			t2, ok := assume(t, d)
			if !ok {
				break
			}
			w := emptyUnit(t2, o.units)
			if w == nil {
				break
			}
			if _, seen := byUnit[w]; !seen {
				order = append(order, w)
			}
			byUnit[w] = append(byUnit[w], d)
		}
		n := 0
		for _, ds := range byUnit {
			n += len(ds)
		}
		if n != len(free) {
			continue // some cell of V is still possible
		}
		clauses := make([]string, len(order))
		for i, w := range order {
			at := make([]string, len(byUnit[w]))
			for j, d := range byUnit[w] {
				at[j] = cell(d)
			}
			clauses[i] = fmt.Sprintf("auf %s würde %s streichen", strings.Join(at, " oder "), units(w.Kind, []int{w.Index}))
		}
		why := fmt.Sprintf("Nach einem Stern auf %s bleiben in %s nur %s. Ein Stern %s → X",
			cell(o.Cell), units(v.Kind, []int{v.Index}), cellsAnd(free), strings.Join(clauses, ", einer "))
		return []solver.Mark{solver.Cross(o.Cell, o.Name()).Why(why)}
	}
	return nil
}

// assume returns a copy of s with a star on q and its implied crosses.
// ok is false if that already conflicts.
func assume(s *board.State, q puzzle.Pos) (*board.State, bool) {
	t := s.Clone()
	if _, err := solver.Apply(t, solver.Star(q, "")); err != nil {
		return nil, false
	}
	return t, true
}

// emptyUnit returns a unit without star and without free cell, or nil.
func emptyUnit(t *board.State, us []puzzle.Unit) *puzzle.Unit {
	for i := range us {
		if !t.HasStar(us[i].Cells) && len(t.Free(us[i].Cells)) == 0 {
			return &us[i]
		}
	}
	return nil
}
