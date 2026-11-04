// Package generator creates random 1-star puzzles with exactly one solution.
package generator

import (
	"errors"
	"math/rand/v2"

	"starbattle/internal/bruteforce"
	"starbattle/internal/puzzle"
)

// Options tune the generator.
type Options struct {
	// Accept is an optional extra filter (e.g. "solvable by logic alone"). Puzzles
	// for which it returns false are discarded.
	Accept func(p *puzzle.Puzzle, solution []int) bool
	// MaxAttempts bounds the number of fresh layouts tried (default 1000).
	MaxAttempts int
}

var orth = [4]puzzle.Pos{{Row: -1, Col: 0}, {Row: 1, Col: 0}, {Row: 0, Col: -1}, {Row: 0, Col: 1}}

// Generate returns a uniquely solvable n×n puzzle and its solution (star column per row).
func Generate(n int, rng *rand.Rand, o Options) (*puzzle.Puzzle, []int, error) {
	if n < 4 {
		// n=1 is trivial; n=2 and n=3 have no valid star placement.
		if n == 1 {
			return &puzzle.Puzzle{N: 1, Regions: [][]int{{0}}}, []int{0}, nil
		}
		return nil, nil, errors.New("no 1-star puzzle exists for n=2 or n=3")
	}
	if n > puzzle.MaxN {
		return nil, nil, errors.New("n too large")
	}
	attempts := o.MaxAttempts
	if attempts <= 0 {
		attempts = 1000
	}
	for range attempts {
		sol := randomSolution(n, rng)
		p := growRegions(n, sol, rng)
		if !makeUnique(p, sol, rng) {
			continue
		}
		if o.Accept != nil && !o.Accept(p, sol) {
			continue
		}
		return p, sol, nil
	}
	return nil, nil, errors.New("no puzzle found within attempt limit")
}

// randomSolution returns a random permutation with |p[i]-p[i+1]| > 1.
func randomSolution(n int, rng *rand.Rand) []int {
	cols := make([]int, n)
	used := make([]bool, n)
	var rec func(r int) bool
	rec = func(r int) bool {
		if r == n {
			return true
		}
		for _, c := range rng.Perm(n) {
			if used[c] || (r > 0 && cols[r-1] >= c-1 && cols[r-1] <= c+1) {
				continue
			}
			cols[r], used[c] = c, true
			if rec(r + 1) {
				return true
			}
			used[c] = false
		}
		return false
	}
	rec(0)
	return cols
}

// growRegions seeds region i at the star of row i and grows all regions randomly
// until the grid is covered. Regions are connected by construction.
func growRegions(n int, sol []int, rng *rand.Rand) *puzzle.Puzzle {
	regs := make([][]int, n)
	for r := range regs {
		regs[r] = make([]int, n)
		for c := range regs[r] {
			regs[r][c] = -1
		}
	}
	// Shuffle which region letter belongs to which row so letters don't follow rows.
	ids := rng.Perm(n)
	for r, c := range sol {
		regs[r][c] = ids[r]
	}
	remaining := n*n - n
	in := func(r, c int) bool { return r >= 0 && r < n && c >= 0 && c < n }
	for remaining > 0 {
		// Collect unassigned cells on the frontier, pick one, join a random neighbor region.
		var frontier []puzzle.Pos
		for r := range n {
			for c := range n {
				if regs[r][c] != -1 {
					continue
				}
				for _, d := range orth {
					if in(r+d.Row, c+d.Col) && regs[r+d.Row][c+d.Col] != -1 {
						frontier = append(frontier, puzzle.Pos{Row: r, Col: c})
						break
					}
				}
			}
		}
		cell := frontier[rng.IntN(len(frontier))]
		var nbRegs []int
		for _, d := range orth {
			rr, cc := cell.Row+d.Row, cell.Col+d.Col
			if in(rr, cc) && regs[rr][cc] != -1 {
				nbRegs = append(nbRegs, regs[rr][cc])
			}
		}
		regs[cell.Row][cell.Col] = nbRegs[rng.IntN(len(nbRegs))]
		remaining--
	}
	return &puzzle.Puzzle{N: n, Regions: regs}
}

// makeUnique repeatedly breaks alternative solutions by moving one of their stars
// into a neighboring region (which then holds two stars of that alternative).
// The intended solution stays valid because only non-solution cells are moved.
// Returns false if it gives up.
func makeUnique(p *puzzle.Puzzle, sol []int, rng *rand.Rand) bool {
	maxSteps := 4 * p.N * p.N
	for range maxSteps {
		sols := bruteforce.Solve(p, 2)
		var alt []int
		for _, s := range sols {
			if !equal(s, sol) {
				alt = s
				break
			}
		}
		if alt == nil {
			return len(sols) == 1
		}
		if !breakAlternative(p, sol, alt, rng) {
			return false
		}
	}
	return false
}

func breakAlternative(p *puzzle.Puzzle, sol, alt []int, rng *rand.Rand) bool {
	type move struct {
		cell puzzle.Pos
		to   int
	}
	var moves []move
	for r := range p.N {
		if alt[r] == sol[r] {
			continue
		}
		cell := puzzle.Pos{Row: r, Col: alt[r]} // a star of alt, never a star of sol
		from := p.Region(cell.Row, cell.Col)
		if !p.RegionConnectedWithout(from, cell) {
			continue
		}
		for _, d := range orth {
			rr, cc := cell.Row+d.Row, cell.Col+d.Col
			if p.In(rr, cc) && p.Region(rr, cc) != from {
				moves = append(moves, move{cell, p.Region(rr, cc)})
			}
		}
	}
	if len(moves) == 0 {
		return false
	}
	m := moves[rng.IntN(len(moves))]
	p.Regions[m.cell.Row][m.cell.Col] = m.to
	return true
}

func equal(a, b []int) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
