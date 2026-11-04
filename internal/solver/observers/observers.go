// Package observers contains the deduction rules. Each rule is an Observer for
// one unit (or one set of units); All builds the full set for a puzzle.
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
	for _, u := range p.Units() {
		obs = append(obs, UnitFinish{Unit: u})
		// TODO: RowColor / ColColor     – line contains only one color → rest of that color X
		// TODO: ColorInLine             – color lies in one line → rest of that line X
		// TODO: Elimination             – a star here would empty some unit → X
	}
	// TODO: MultiLine / MultiColor   – k lines ↔ k colors (pigeonhole)
	return obs
}
