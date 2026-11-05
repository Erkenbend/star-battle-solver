package observers

import (
	"fmt"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// Confinement is the pigeonhole rule, generalizing the "Line/Column", "Color" and
// "Multiple ..." ideas from the original plan:
//
//	If the free cells of k unsolved units of kind From lie inside exactly k units of
//	kind To, then those k To-units must place their stars inside the k From-units
//	(the k From-units need k stars, and each To-unit holds only one). So every other
//	free cell of those To-units is a cross.
//
// Examples (From → To):
//
//	k=1 row → region:     a row only contains one color → rest of that color X
//	k=1 region → row:     a color lies in one row → rest of that row X
//	k=2 row → region:     two rows only contain two colors → those colors elsewhere X
//	k=2 row → col:        two rows only use two columns → those columns elsewhere X
//
// One instance handles one (From, To, K) triple over all K-subsets of From-units.
// Only K ≤ m/2 is needed (m = unsolved From-units): a larger confinement implies a
// smaller one in the opposite direction (To → From), which another instance covers.
//
// Contiguous restricts the subsets to blocks of K consecutive open rows or
// columns (solved ones in between are skipped), the way humans look at the
// board: "Zeilen 3 bis 5" instead of "Zeilen 1, 4 und 7". The complement of a
// block need not be a block, so the K ≤ m/2 argument no longer holds and K runs
// up to m−1. Contiguous has no effect when From is Region.
type Confinement struct {
	From, To   puzzle.UnitKind
	K          int
	Contiguous bool
	units      []puzzle.Unit // all units of kind From, indexed by unit index
}

// NewConfinement creates the observer for one (from, to, k) triple.
func NewConfinement(p *puzzle.Puzzle, from, to puzzle.UnitKind, k int) Confinement {
	var units []puzzle.Unit
	for _, u := range p.Units() {
		if u.Kind == from {
			units = append(units, u)
		}
	}
	return Confinement{From: from, To: to, K: k, units: units}
}

func (o Confinement) Name() string {
	return fmt.Sprintf("Confinement[k=%d %v→%v]", o.K, o.From, o.To)
}

// blocks reports whether o only looks at consecutive From-units.
func (o Confinement) blocks() bool { return o.Contiguous && o.From != puzzle.Region }

// minOpen is the number of open From-units below which o can deduce nothing.
func (o Confinement) minOpen() int {
	switch {
	case o.K == 1:
		return 1
	case o.blocks():
		return o.K + 1 // with K = m the block covers everything: nothing to cross
	default:
		return 2 * o.K
	}
}

// Priority: k=1 is as easy to see as a single unit, larger k is a real pigeonhole step.
func (o Confinement) Priority() int {
	if o.K == 1 {
		return LevelConfinementSingle
	}
	return LevelConfinementMulti
}

// Done once too few units of kind From are open (see minOpen).
func (o Confinement) Done(s *board.State) bool {
	open := 0
	for _, u := range o.units {
		if !s.HasStar(u.Cells) {
			open++
		}
	}
	return open < o.minOpen()
}

func (o Confinement) Deduce(s *board.State) []solver.Mark {
	p := s.P
	// For each open From-unit: the set of To-units its free cells touch.
	type open struct {
		index   int
		targets map[int]bool
	}
	var opens []open
	for _, u := range o.units {
		if s.HasStar(u.Cells) {
			continue
		}
		t := map[int]bool{}
		for _, q := range s.Free(u.Cells) {
			t[p.UnitOf(o.To, q)] = true
		}
		opens = append(opens, open{u.Index, t})
	}
	if len(opens) < o.minOpen() {
		return nil
	}

	var marks []solver.Mark
	if o.blocks() {
		for i := 0; i+o.K <= len(opens); i++ {
			from, union := map[int]bool{}, map[int]bool{}
			for _, op := range opens[i : i+o.K] {
				from[op.index] = true
				for t := range op.targets {
					union[t] = true
				}
			}
			if len(union) == o.K {
				marks = append(marks, o.crosses(s, from, union, o.explain(from, union))...)
			}
		}
		return marks
	}

	chosen := make([]int, 0, o.K) // indices into opens
	var rec func(start int, union map[int]bool)
	rec = func(start int, union map[int]bool) {
		if len(union) > o.K {
			return // prune: already too many targets
		}
		if len(chosen) == o.K {
			if len(union) == o.K {
				from := make(map[int]bool, o.K)
				for _, i := range chosen {
					from[opens[i].index] = true
				}
				marks = append(marks, o.crosses(s, from, union, o.explain(from, union))...)
			}
			return
		}
		for i := start; i < len(opens); i++ {
			next := make(map[int]bool, len(union)+len(opens[i].targets))
			for t := range union {
				next[t] = true
			}
			for t := range opens[i].targets {
				next[t] = true
			}
			chosen = append(chosen, i)
			rec(i+1, next)
			chosen = chosen[:len(chosen)-1]
		}
	}
	rec(0, map[int]bool{})
	return marks
}

// crosses returns X for every free cell inside the target To-units but outside the
// chosen From-units.
func (o Confinement) crosses(s *board.State, from, to map[int]bool, why string) []solver.Mark {
	var marks []solver.Mark
	n := s.P.N
	for r := range n {
		for c := range n {
			q := puzzle.Pos{Row: r, Col: c}
			if s.At(r, c) == board.Free && to[s.P.UnitOf(o.To, q)] && !from[s.P.UnitOf(o.From, q)] {
				marks = append(marks, solver.Cross(q, o.Name()).Why(why))
			}
		}
	}
	return marks
}

// explain builds e.g. "Farben A und B liegen nur noch in Spalten 2 und 3 → alle
// anderen Felder der Spalten 2 und 3 X" (see confined).
func (o Confinement) explain(from, to map[int]bool) string {
	return confined(o.From, keys(from), o.To, keys(to))
}
