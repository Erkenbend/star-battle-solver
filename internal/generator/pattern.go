package generator

import (
	"fmt"
	"math/rand/v2"

	"starbattle/internal/bruteforce"
	"starbattle/internal/puzzle"
)

// PatternProblem is why a pattern cannot be completed (see CheckPattern).
type PatternProblem int

const (
	// Unreachable: the parts of a region cannot be joined through free cells
	// (strict: the region is not connected as drawn).
	Unreachable PatternProblem = iota
	// TooSmall: a region is enclosed (no free cell next to it; strict: any
	// region) and smaller than the minimum region size.
	TooSmall
	// NoStars: no placement of stars fits the pattern, e.g. a region that
	// covers two whole rows, or two enclosed regions inside the same row.
	NoStars
	// Ambiguous: two placements of stars lie entirely on drawn cells. Both
	// stay solutions however the free cells are filled, since drawn cells
	// keep their region.
	Ambiguous
	// Leftover (strict only): there are more separate areas of free cells
	// than regions missing from the pattern, and the drawn regions cannot
	// take free cells.
	Leftover
	// Fixed (strict only): every area of free cells is exactly one missing
	// region, so the pattern already is the whole puzzle, and that has more
	// than one solution.
	Fixed
)

// PatternError describes a problem of a pattern found by CheckPattern.
type PatternError struct {
	Problem PatternProblem
	Region  int          // the region concerned, -1 if none (TooSmall: -1 for an area of free cells, strict)
	Cells   []puzzle.Pos // the cells to point at (Unreachable: the part cut off; Ambiguous, Fixed: the stars of two solutions; Leftover: the free cells)
	Areas   int          // Leftover: separate areas of free cells
	Missing int          // Leftover: regions missing from the pattern
}

func (e *PatternError) Error() string {
	switch e.Problem {
	case Unreachable:
		return fmt.Sprintf("region %c: parts cannot be connected", puzzle.RegionLetter(e.Region))
	case TooSmall:
		return fmt.Sprintf("region %c: enclosed and too small", puzzle.RegionLetter(e.Region))
	case Ambiguous:
		return "two star placements lie on drawn cells alone"
	case Fixed:
		return "the pattern fixes the puzzle, which has several solutions"
	case Leftover:
		return fmt.Sprintf("%d areas of free cells, but only %d regions left", e.Areas, e.Missing)
	default:
		return "no star placement fits the pattern"
	}
}

// checkBudget bounds CheckPattern's search for a star placement, in placed
// stars. If it runs out, the pattern counts as possible: the generator then
// simply tries.
const checkBudget = 2_000_000

// CheckPattern reports whether pattern (see Complete) can be part of a valid
// puzzle, as far as can be told without completing it: the parts of every
// region can reach each other through free cells, every enclosed region has
// at least minSize cells, and a star placement exists with one star per row,
// column and region, none touching, in which every region gets a star it can
// reach. It returns nil or a *PatternError.
//
// With strict (see Options.Strict), every drawn region is final: it must be
// connected as drawn and have at least minSize cells, its star lies on its
// own cells, and every separate area of free cells needs a region of its own
// that is missing from the pattern.
//
// It cannot see paths of different regions that would have to cross, so
// Complete can still fail on a pattern that passes. A pattern of the wrong
// shape (not n×n with 4 ≤ n ≤ puzzle.MaxN, values outside Free..n-1) gives a
// plain error.
func CheckPattern(pattern [][]int, minSize int, strict bool) error {
	n := len(pattern)
	if n < 4 || n > puzzle.MaxN {
		return fmt.Errorf("size %d out of range [4,%d]", n, puzzle.MaxN)
	}
	for r, row := range pattern {
		if len(row) != n {
			return fmt.Errorf("row %d has %d cells, want %d", r, len(row), n)
		}
		for c, reg := range row {
			if reg < Free || reg >= n {
				return fmt.Errorf("cell (%d,%d): region %d out of range", r, c, reg)
			}
		}
	}
	info := analyze(pattern, strict)
	for reg := range n {
		if !info.present[reg] {
			continue
		}
		if cut := info.unreachablePart(reg); cut != nil {
			return &PatternError{Problem: Unreachable, Region: reg, Cells: cut}
		}
		if (strict || len(info.touches[reg]) == 0) && len(info.cells[reg]) < minSize {
			return &PatternError{Problem: TooSmall, Region: reg, Cells: info.cells[reg]}
		}
	}
	if strict {
		missing := 0
		for _, ok := range info.present {
			if !ok {
				missing++
			}
		}
		if info.areas > missing {
			var cells []puzzle.Pos
			for i, fc := range info.freeComp {
				if fc >= 0 {
					cells = append(cells, puzzle.Pos{Row: i / n, Col: i % n})
				}
			}
			return &PatternError{Problem: Leftover, Region: -1, Cells: cells, Areas: info.areas, Missing: missing}
		}
		// Each area holds at least one whole missing region.
		areaCells := map[int][]puzzle.Pos{}
		for i, fc := range info.freeComp {
			if fc >= 0 {
				areaCells[fc] = append(areaCells[fc], puzzle.Pos{Row: i / n, Col: i % n})
			}
		}
		for _, cells := range areaCells {
			if len(cells) < minSize {
				return &PatternError{Problem: TooSmall, Region: -1, Cells: cells}
			}
		}
	}
	budget := checkBudget
	identity := make([]int, n)
	for i := range identity {
		identity[i] = i
	}
	if info.search(func() []int { return identity }, &budget) == nil && budget >= 0 {
		return &PatternError{Problem: NoStars, Region: -1}
	}
	if sols := info.drawnSolutions(2); len(sols) == 2 {
		return &PatternError{Problem: Ambiguous, Region: -1, Cells: differing(sols[0], sols[1])}
	}
	if p := info.fixedLayout(); p != nil {
		if sols := bruteforce.Solve(p, 2); len(sols) == 2 {
			return &PatternError{Problem: Fixed, Region: -1, Cells: differing(sols[0], sols[1])}
		}
	}
	return nil
}

// differing lists the stars of a and b (star column per row) in the rows
// where they differ.
func differing(a, b []int) []puzzle.Pos {
	var cells []puzzle.Pos
	for r := range a {
		if a[r] != b[r] {
			cells = append(cells, puzzle.Pos{Row: r, Col: a[r]}, puzzle.Pos{Row: r, Col: b[r]})
		}
	}
	return cells
}

// fixedLayout returns the only layout a strict pattern allows when every
// area of free cells is exactly one missing region, nil otherwise.
func (info *patternInfo) fixedLayout() *puzzle.Puzzle {
	if !info.strict {
		return nil
	}
	var missing []int
	for reg, ok := range info.present {
		if !ok {
			missing = append(missing, reg)
		}
	}
	if len(missing) != info.areas {
		return nil
	}
	n := info.n
	regionOf := map[int]int{} // area -> region
	regs := make([][]int, n)
	for r := range n {
		regs[r] = make([]int, n)
		for c := range n {
			regs[r][c] = info.pattern[r][c]
			if fc := info.freeComp[r*n+c]; fc >= 0 {
				if _, ok := regionOf[fc]; !ok {
					regionOf[fc] = missing[len(regionOf)]
				}
				regs[r][c] = regionOf[fc]
			}
		}
	}
	return &puzzle.Puzzle{N: n, Regions: regs}
}

// drawnSolutions returns up to limit star placements that lie on drawn cells
// only, one star per row, column and region, none touching. There are none
// unless every region is drawn. They are solutions of every completion.
func (info *patternInfo) drawnSolutions(limit int) [][]int {
	n := info.n
	for _, ok := range info.present {
		if !ok {
			return nil
		}
	}
	var out [][]int
	cols := make([]int, n)
	usedCol := make([]bool, n)
	usedReg := make([]bool, n)
	var rec func(r int)
	rec = func(r int) {
		if len(out) == limit {
			return
		}
		if r == n {
			out = append(out, append([]int(nil), cols...))
			return
		}
		for c := range n {
			reg := info.pattern[r][c]
			if reg == Free || usedCol[c] || usedReg[reg] || (r > 0 && cols[r-1] >= c-1 && cols[r-1] <= c+1) {
				continue
			}
			cols[r], usedCol[c], usedReg[reg] = c, true, true
			rec(r + 1)
			usedCol[c], usedReg[reg] = false, false
		}
	}
	rec(0)
	return out
}

// patternInfo is what the star search needs to know about a pattern: the
// parts of the free cells and which regions each part can reach.
type patternInfo struct {
	pattern  [][]int
	n        int
	strict   bool // see Options.Strict
	areas    int  // separate parts of the free cells
	present  []bool
	cells    [][]puzzle.Pos // drawn cells per region
	freeComp []int          // per cell: index of its part of the free cells, -1 if drawn
	touches  [][]int        // per region: free parts next to it
	reaches  [][]bool       // [free part][region]: the region can take a star there
	maxRow   []int          // per region: last row where its star can be
}

// analyze collects the patternInfo. With strict, drawn regions reach no
// free part.
func analyze(pattern [][]int, strict bool) *patternInfo {
	n := len(pattern)
	info := &patternInfo{pattern: pattern, n: n, strict: strict, present: make([]bool, n), cells: make([][]puzzle.Pos, n), touches: make([][]int, n), maxRow: make([]int, n)}
	missing := false
	for r, row := range pattern {
		for c, reg := range row {
			if reg != Free {
				info.present[reg] = true
				info.cells[reg] = append(info.cells[reg], puzzle.Pos{Row: r, Col: c})
			}
		}
	}
	for reg := range n {
		missing = missing || !info.present[reg]
	}
	free := make([][]int, n)
	for r := range n {
		free[r] = make([]int, n)
		for c := range n {
			free[r][c] = 0
			if pattern[r][c] == Free {
				free[r][c] = 1
			}
		}
	}
	// Parts of the free cells: components of the "region" 1 in free.
	label, parts := components(free)
	info.freeComp = make([]int, n*n)
	seenArea := map[int]bool{}
	for i, l := range label {
		info.freeComp[i] = -1
		if l >= 0 && free[i/n][i%n] == 1 {
			info.freeComp[i] = l
			seenArea[l] = true
		}
	}
	info.areas = len(seenArea)
	info.reaches = make([][]bool, len(parts))
	for i := range info.reaches {
		info.reaches[i] = make([]bool, n)
		if missing {
			// A region missing from the pattern can start in any free part.
			for reg := range n {
				info.reaches[i][reg] = !info.present[reg]
			}
		}
	}
	for r := range n {
		for c := range n {
			reg := pattern[r][c]
			if reg == Free || strict {
				continue
			}
			for _, d := range orth {
				rr, cc := r+d.Row, c+d.Col
				if rr < 0 || rr >= n || cc < 0 || cc >= n {
					continue
				}
				if fc := info.freeComp[rr*n+cc]; fc >= 0 && !info.reaches[fc][reg] {
					info.reaches[fc][reg] = true
					info.touches[reg] = append(info.touches[reg], fc)
				}
			}
		}
	}
	for reg := range n {
		info.maxRow[reg] = -1
	}
	for r := range n {
		for c := range n {
			if reg := pattern[r][c]; reg != Free {
				info.maxRow[reg] = r
			} else {
				for reg, ok := range info.reaches[info.freeComp[r*n+c]] {
					if ok {
						info.maxRow[reg] = r
					}
				}
			}
		}
	}
	return info
}

// unreachablePart returns a part of region reg that cannot be joined to its
// first part through free cells (strict: at all), or nil if all parts can.
func (info *patternInfo) unreachablePart(reg int) []puzzle.Pos {
	n := info.n
	seen := make([]bool, n*n)
	start := info.cells[reg][0]
	seen[start.Row*n+start.Col] = true
	stack := []puzzle.Pos{start}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, d := range orth {
			rr, cc := cur.Row+d.Row, cur.Col+d.Col
			if rr < 0 || rr >= n || cc < 0 || cc >= n || seen[rr*n+cc] {
				continue
			}
			if v := info.pattern[rr][cc]; v == reg || (v == Free && !info.strict) {
				seen[rr*n+cc] = true
				stack = append(stack, puzzle.Pos{Row: rr, Col: cc})
			}
		}
	}
	for _, q := range info.cells[reg] {
		if !seen[q.Row*n+q.Col] {
			// The whole part of q, so the message can point at it.
			var part []puzzle.Pos
			mark := make([]bool, n*n)
			mark[q.Row*n+q.Col] = true
			stack = []puzzle.Pos{q}
			for len(stack) > 0 {
				cur := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				part = append(part, cur)
				for _, d := range orth {
					rr, cc := cur.Row+d.Row, cur.Col+d.Col
					if rr >= 0 && rr < n && cc >= 0 && cc < n && !mark[rr*n+cc] && info.pattern[rr][cc] == reg {
						mark[rr*n+cc] = true
						stack = append(stack, puzzle.Pos{Row: rr, Col: cc})
					}
				}
			}
			return part
		}
	}
	return nil
}

// search looks for a star placement (column per row) that fits the pattern:
// stars not touching, one per column, at most one on the drawn cells of each
// region, and the stars on free cells can be shared out among the regions
// without a star so that each takes one it can reach. order gives the columns
// to try in each row. Returns nil if there is none or budget (placed stars,
// decremented) runs out first.
func (info *patternInfo) search(order func() []int, budget *int) []int {
	n := info.n
	cols := make([]int, n)
	usedCol := make([]bool, n)
	usedReg := make([]bool, n)
	var rec func(r int) bool
	rec = func(r int) bool {
		if r == n {
			return info.shareFreeStars(cols, usedReg)
		}
		// A region without a star that can no longer get one: dead end. Its
		// star may still be one placed on a free cell above that it reaches.
		for reg := range n {
			if usedReg[reg] || !info.present[reg] || info.maxRow[reg] >= r {
				continue
			}
			reachable := false
			for rr := range r {
				if fc := info.freeComp[rr*n+cols[rr]]; fc >= 0 && info.reaches[fc][reg] {
					reachable = true
					break
				}
			}
			if !reachable {
				return false
			}
		}
		for _, c := range order() {
			reg := info.pattern[r][c]
			if usedCol[c] || (r > 0 && cols[r-1] >= c-1 && cols[r-1] <= c+1) || (reg != Free && usedReg[reg]) {
				continue
			}
			if *budget--; *budget < 0 {
				return false
			}
			cols[r], usedCol[c] = c, true
			if reg != Free {
				usedReg[reg] = true
			}
			if rec(r + 1) {
				return true
			}
			usedCol[c] = false
			if reg != Free {
				usedReg[reg] = false
			}
		}
		return false
	}
	if !rec(0) {
		return nil
	}
	return cols
}

// shareFreeStars reports whether the stars of cols on free cells can be
// matched to the regions without a star in usedReg, each region to a star in
// a free part it reaches (bipartite matching with augmenting paths).
func (info *patternInfo) shareFreeStars(cols []int, usedReg []bool) bool {
	n := info.n
	var stars []int // free part of each free star
	for r, c := range cols {
		if fc := info.freeComp[r*n+c]; fc >= 0 {
			stars = append(stars, fc)
		}
	}
	owner := make([]int, len(stars)) // region matched to each star, -1 if none
	for i := range owner {
		owner[i] = -1
	}
	var try func(reg int, seen []bool) bool
	try = func(reg int, seen []bool) bool {
		for i, fc := range stars {
			if seen[i] || !info.reaches[fc][reg] {
				continue
			}
			seen[i] = true
			if owner[i] < 0 || try(owner[i], seen) {
				owner[i] = reg
				return true
			}
		}
		return false
	}
	for reg := range n {
		if !usedReg[reg] && !try(reg, make([]bool, len(stars))) {
			return false
		}
	}
	if info.strict {
		// Every free part is filled by regions missing from the pattern
		// only, so each needs a star.
		has := map[int]bool{}
		for _, fc := range stars {
			has[fc] = true
		}
		if len(has) < info.areas {
			return false
		}
	}
	return true
}

// solutionFor returns a random star placement that fits the pattern (see
// search), or nil if none is found within solutionBudget.
func solutionFor(info *patternInfo, rng *rand.Rand) []int {
	budget := solutionBudget
	return info.search(func() []int { return rng.Perm(info.n) }, &budget)
}
