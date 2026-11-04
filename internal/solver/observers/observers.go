// Package observers contains the deduction rules. Each rule is an Observer for
// one unit (or one family of unit subsets); All builds the full set for a puzzle.
//
// To add a rule: write a type implementing solver.Observer, add a table test
// next to it (see unitfinish_test.go), and register it in All.
package observers

import (
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// All returns every observer instance for p.
func All(p *puzzle.Puzzle) []solver.Observer {
	var obs []solver.Observer
	implied := newImpliedTable(p)
	for _, u := range p.Units() {
		obs = append(obs, UnitFinish{Unit: u})
		obs = append(obs, Elimination{Unit: u, implied: implied})
	}
	kinds := []puzzle.UnitKind{puzzle.Row, puzzle.Col, puzzle.Region}
	for _, from := range kinds {
		for _, to := range kinds {
			if from == to {
				continue
			}
			for k := 1; k <= max(1, p.N/2); k++ {
				obs = append(obs, NewConfinement(p, from, to, k))
			}
		}
	}
	return obs
}
