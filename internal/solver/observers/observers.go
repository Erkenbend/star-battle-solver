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

// Config enables and tunes the rules that look one assumption ahead.
type Config struct {
	// PartnerDepth: ForcedPartner may chain up to this many forced stars (0 = off).
	PartnerDepth int
	// DeadCellsMax: DeadCells checks units with up to this many cells left (0 = off).
	DeadCellsMax int
	// ConfinementAnySubset: Confinement with k ≥ 2 looks at any subset of rows
	// or columns. By default (false) it only looks at blocks of consecutive rows
	// or columns, which read more naturally; regions always use any subset.
	ConfinementAnySubset bool
}

// DefaultConfig is used by All.
var DefaultConfig = Config{PartnerDepth: 1, DeadCellsMax: 2}

// BaseConfig disables the look-ahead rules (only UnitFinish, Elimination, Confinement).
var BaseConfig = Config{}

// All returns every observer instance for p with DefaultConfig.
func All(p *puzzle.Puzzle) []solver.Observer { return AllWith(p, DefaultConfig) }

// Priority levels of the rules (see solver.Prioritized), from simplest to most
// demanding. Both runners apply a rule only when no simpler one applies, so the
// explanations stay as human as possible. Lookahead and guessing in Solve come
// after all of them (solver.LevelLookahead, solver.LevelGuess).
const (
	LevelUnitFinish        = 1 + iota // only one free cell left → star
	LevelConfinementSingle            // Confinement k=1: a unit lies within one other unit
	LevelElimination                  // a star here would empty a unit
	LevelConfinementMulti             // Confinement k≥2: pigeonhole over several units
	LevelForcedPartner                // a star here forces another one, together they empty a unit
	LevelDeadCells                    // after a star here, every remaining cell of a unit is dead
)

// AllWith returns the observer instances for p, sorted by priority level.
// Within a level the order is the tie-break: Confinement with smaller k first,
// then rows, columns, regions.
func AllWith(p *puzzle.Puzzle, cfg Config) []solver.Observer {
	var obs []solver.Observer
	units := p.Units()
	implied := newImpliedTable(p)
	kinds := []puzzle.UnitKind{puzzle.Row, puzzle.Col, puzzle.Region}
	confinement := func(k int) {
		for _, from := range kinds {
			blocks := !cfg.ConfinementAnySubset && from != puzzle.Region
			if k > p.N/2 && !(blocks && k < p.N) {
				continue
			}
			for _, to := range kinds {
				if from != to {
					c := NewConfinement(p, from, to, k)
					c.Contiguous = !cfg.ConfinementAnySubset
					obs = append(obs, c)
				}
			}
		}
	}
	forEachCell := func(f func(q puzzle.Pos)) {
		for r := range p.N {
			for c := range p.N {
				f(puzzle.Pos{Row: r, Col: c})
			}
		}
	}

	for _, u := range units {
		obs = append(obs, UnitFinish{Unit: u})
	}
	confinement(1)
	for _, u := range units {
		obs = append(obs, Elimination{Unit: u, implied: implied})
	}
	for k := 2; k < p.N; k++ {
		confinement(k)
	}
	if cfg.PartnerDepth > 0 {
		forEachCell(func(q puzzle.Pos) {
			obs = append(obs, ForcedPartner{Cell: q, Depth: cfg.PartnerDepth, units: units})
		})
	}
	if cfg.DeadCellsMax >= 2 {
		forEachCell(func(q puzzle.Pos) {
			obs = append(obs, DeadCells{Cell: q, MaxCells: cfg.DeadCellsMax, units: units})
		})
	}
	return obs
}
