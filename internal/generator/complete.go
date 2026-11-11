package generator

import (
	"errors"
	"math/rand/v2"
	"slices"

	"starbattle/internal/puzzle"
)

// Free marks a cell of a pattern that Complete may give to any region.
const Free = -1

// solutionBudget bounds the search for a solution that fits a pattern, in
// placed stars, so a pattern that leaves no room fails an attempt quickly.
const solutionBudget = 20_000

// layoutTries is how many solutions an attempt of Complete tries to fit into
// the pattern before it gives up.
const layoutTries = 30

// Complete returns a uniquely solvable puzzle that contains pattern (n×n,
// region index or Free per cell): every cell that is not Free keeps its
// region. The cells of one region need not be connected in pattern; Complete
// connects them through free cells. Regions missing from pattern are placed
// anywhere. Options work as for Generate.
//
// Complete first runs CheckPattern and returns its *PatternError, if any.
// An attempt picks a solution that fits the pattern (see CheckPattern),
// joins the parts of each region and the free stars along shortest paths
// through free cells (see layoutFor), grows the regions over the remaining
// free cells and finally repairs the layout until the solution is
// unique, as Generate does, without ever moving a cell of pattern and, as
// long as that works, without moving cells into a drawn region. Only if no
// attempt succeeds that way, a second round allows such moves as a last
// resort. With o.Strict, drawn regions never grow: no second round.
func Complete(pattern [][]int, rng *rand.Rand, o Options) (*puzzle.Puzzle, []int, error) {
	if err := CheckPattern(pattern, max(o.MinRegionSize, 1), o.Strict); err != nil {
		return nil, nil, err
	}
	n := len(pattern)
	lk := &locks{fixed: make([]bool, n*n), drawn: make([]bool, n)}
	for r, row := range pattern {
		for c, reg := range row {
			if reg != Free {
				lk.fixed[r*n+c], lk.drawn[reg] = true, true
			}
		}
	}
	info := analyze(pattern, o.Strict)
	// A placement on drawn cells alone survives every repair (drawn cells
	// never move), so if there is one, it has to be the solution.
	var forced []int
	if sols := info.drawnSolutions(1); len(sols) == 1 {
		forced = sols[0]
	}
	attempt := func(lk *locks) attemptFunc {
		return func(arng *rand.Rand, k, minSize int, stop func() bool) (*puzzle.Puzzle, []int, bool) {
			// Solution and layout are cheap next to makeUnique, so an attempt
			// tries a few before it gives up.
			var sol []int
			var regs [][]int
			for range layoutTries {
				if forced != nil {
					sol = slices.Clone(forced)
				} else {
					sol = solutionFor(info, arng)
				}
				if sol != nil {
					if regs = layoutFor(pattern, sol, o.Strict, arng); regs != nil {
						break
					}
				}
			}
			if regs == nil {
				return nil, nil, false
			}
			p := &puzzle.Puzzle{N: n, Regions: regs}
			return p, sol, makeUnique(p, sol, lk, arng, k, minSize, stop)
		}
	}
	p, sol, err := run(rng, o, attempt(lk))
	if err == nil || errors.Is(err, ErrCancelled) || o.Strict {
		return p, sol, err
	}
	// Rarely, the drawn regions have to grow to make the solution unique.
	return run(rng, o, attempt(&locks{fixed: lk.fixed, drawn: lk.drawn, growDrawn: true}))
}

// layoutFor completes pattern to a layout in which sol has one star per
// region and every region is connected, or returns nil if that fails.
//
// Gaps are closed one at a time, shortest first: a gap is a path of free cells
// from one part of a region to another part, or to a free star if the region
// has no star yet. Short gaps are the ones a drawing most likely means, and
// filling them first leaves the most room for the longer ones. The free stars
// left over start the regions missing from pattern, which then grow over the
// remaining free cells; the drawn regions only take the cells left after that.
//
// With strict, the drawn regions take no free cells: layoutFor returns nil if
// the missing regions cannot fill them all.
func layoutFor(pattern [][]int, sol []int, strict bool, rng *rand.Rand) [][]int {
	n := len(pattern)
	regs := make([][]int, n)
	for r := range regs {
		regs[r] = slices.Clone(pattern[r])
	}
	present := make([]bool, n) // region occurs in pattern
	for _, row := range pattern {
		for _, reg := range row {
			if reg != Free {
				present[reg] = true
			}
		}
	}
	hasStar := make([]bool, n)
	isStar := make([]bool, n*n)
	for r, c := range sol {
		isStar[r*n+c] = true
		if reg := regs[r][c]; reg != Free {
			hasStar[reg] = true
		}
	}

	for {
		label, parts := components(regs)
		var best []puzzle.Pos
		bestReg, ties := -1, 0
		for i, part := range parts {
			reg := regs[part.Row][part.Col]
			if !present[reg] {
				continue
			}
			if hasStar[reg] && countParts(parts, regs, reg) == 1 {
				continue
			}
			path := gap(regs, isStar, label, i, part, !hasStar[reg])
			if path == nil {
				return nil // this part can reach nothing it needs
			}
			switch {
			case best == nil || len(path) < len(best):
				best, bestReg, ties = path, reg, 1
			case len(path) == len(best):
				if ties++; rng.IntN(ties) == 0 {
					best, bestReg = path, reg
				}
			}
		}
		if best == nil {
			break
		}
		for _, q := range best {
			regs[q.Row][q.Col] = bestReg
			if isStar[q.Row*n+q.Col] {
				hasStar[bestReg] = true
			}
		}
	}

	// The remaining free stars start the regions missing from pattern.
	var missing []int
	for reg := range n {
		if !hasStar[reg] {
			missing = append(missing, reg)
		}
	}
	rng.Shuffle(len(missing), func(i, j int) { missing[i], missing[j] = missing[j], missing[i] })
	free := 0
	for r := range n {
		for c := range n {
			if regs[r][c] != Free {
				continue
			}
			if isStar[r*n+c] {
				regs[r][c] = missing[0]
				missing = missing[1:]
			} else {
				free++
			}
		}
	}
	// The regions missing from pattern grow first, so the drawn shapes stay
	// recognizable; the drawn regions take only what is left.
	undrawn := make([]bool, n)
	for reg := range n {
		undrawn[reg] = !present[reg]
	}
	free = grow(regs, free, rng, undrawn)
	if free > 0 && strict {
		return nil
	}
	grow(regs, free, rng, nil)
	return regs
}

// components labels the orthogonally connected parts of the regions in regs
// (free cells get -1) and returns the label of each cell and the first cell
// of each part.
func components(regs [][]int) ([]int, []puzzle.Pos) {
	n := len(regs)
	label := make([]int, n*n)
	for i := range label {
		label[i] = -1
	}
	var parts []puzzle.Pos
	for r := range n {
		for c := range n {
			if regs[r][c] == Free || label[r*n+c] != -1 {
				continue
			}
			id, reg := len(parts), regs[r][c]
			parts = append(parts, puzzle.Pos{Row: r, Col: c})
			label[r*n+c] = id
			stack := []puzzle.Pos{{Row: r, Col: c}}
			for len(stack) > 0 {
				cur := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				for _, d := range orth {
					rr, cc := cur.Row+d.Row, cur.Col+d.Col
					if rr >= 0 && rr < n && cc >= 0 && cc < n && label[rr*n+cc] == -1 && regs[rr][cc] == reg {
						label[rr*n+cc] = id
						stack = append(stack, puzzle.Pos{Row: rr, Col: cc})
					}
				}
			}
		}
	}
	return label, parts
}

// countParts counts the parts (see components) of region reg.
func countParts(parts []puzzle.Pos, regs [][]int, reg int) int {
	k := 0
	for _, q := range parts {
		if regs[q.Row][q.Col] == reg {
			k++
		}
	}
	return k
}

// gap returns the shortest path of free cells from part id (starting at
// cell first) to another part of its region or, with wantStar, to a free cell
// with a star (included in the path). Stars are never passed through. It
// returns nil if there is no such path.
func gap(regs [][]int, isStar []bool, label []int, id int, first puzzle.Pos, wantStar bool) []puzzle.Pos {
	n := len(regs)
	reg := regs[first.Row][first.Col]
	parent := make([]int, n*n)
	for i := range parent {
		parent[i] = -1
	}
	var queue []int
	for i, l := range label {
		if l == id {
			parent[i] = i
			queue = append(queue, i)
		}
	}
	path := func(i int) []puzzle.Pos {
		var out []puzzle.Pos
		for ; label[i] != id; i = parent[i] {
			out = append(out, puzzle.Pos{Row: i / n, Col: i % n})
		}
		return out
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range orth {
			rr, cc := cur/n+d.Row, cur%n+d.Col
			if rr < 0 || rr >= n || cc < 0 || cc >= n {
				continue
			}
			i := rr*n + cc
			if parent[i] != -1 {
				continue
			}
			switch {
			case regs[rr][cc] == reg: // another part
				return path(cur)
			case regs[rr][cc] != Free:
			case isStar[i]:
				if wantStar {
					parent[i] = cur
					return path(i)
				}
			default:
				parent[i] = cur
				queue = append(queue, i)
			}
		}
	}
	return nil
}
