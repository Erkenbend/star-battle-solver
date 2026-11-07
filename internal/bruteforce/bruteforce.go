// Package bruteforce is a simple reference solver: row-by-row backtracking. It is
// used to check uniqueness in the generator and as a test oracle for the
// observer-based solver.
package bruteforce

import (
	"slices"

	"starbattle/internal/puzzle"
)

// Solve returns up to limit solutions. Each solution gives the star column per row.
func Solve(p *puzzle.Puzzle, limit int) [][]int {
	n := p.N
	cols := make([]int, n)
	usedCol := make([]bool, n)
	usedReg := make([]bool, n)
	var out [][]int

	var rec func(r int) bool // returns true when limit reached
	rec = func(r int) bool {
		if r == n {
			out = append(out, slices.Clone(cols))
			return len(out) >= limit
		}
		for c := range n {
			reg := p.Region(r, c)
			if usedCol[c] || usedReg[reg] {
				continue
			}
			// One star per row, so only the previous row can touch (incl. diagonally).
			if r > 0 && cols[r-1] >= c-1 && cols[r-1] <= c+1 {
				continue
			}
			cols[r], usedCol[c], usedReg[reg] = c, true, true
			done := rec(r + 1)
			usedCol[c], usedReg[reg] = false, false
			if done {
				return true
			}
		}
		return false
	}
	rec(0)
	return out
}

// CountSolutions returns the number of solutions, capped at limit.
func CountSolutions(p *puzzle.Puzzle, limit int) int {
	return len(Solve(p, limit))
}
