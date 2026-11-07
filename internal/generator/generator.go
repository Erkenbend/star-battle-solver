// Package generator creates random 1-star puzzles with exactly one solution.
package generator

import (
	"errors"
	"math/rand/v2"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"starbattle/internal/bruteforce"
	"starbattle/internal/puzzle"
)

// Options tune the generator.
type Options struct {
	// Accept is an optional extra filter (e.g. "solvable by logic alone"). Puzzles
	// for which it returns false are discarded. It is called from several
	// goroutines at once unless Workers is 1.
	Accept func(p *puzzle.Puzzle, solution []int) bool
	// MaxAttempts bounds the number of fresh layouts tried (default 1000).
	MaxAttempts int
	// Alternatives is how many alternative solutions each repair step considers
	// when choosing a move (default 8).
	Alternatives int
	// MinRegionSize rejects puzzles with a smaller region, and repair moves never
	// shrink a region below it (default 1, i.e. no limit).
	MinRegionSize int
	// Workers is how many attempts of this call run at the same time (default
	// 4, at most one less than the available cores). It does not change the
	// result, only how soon it is found. All calls together are also limited
	// to one less than the available cores, see cpuSlots.
	// Only a failed attempt is worth overlapping, and with N = 12 to 14 at
	// most every third first attempt fails; more workers mostly slow the
	// first attempt down by competing for the CPU.
	Workers int
}

// DefaultMinRegionSize is the MinRegionSize the CLI and the web server use for
// an n×n puzzle. From n = 10 on, regions of one cell (a free star) are ruled
// out: at n = 12 to 15 this gives 20 to 30 % longer solutions that need the
// harder rules more often, at 1.5 to 2.5 times the generation time.
func DefaultMinRegionSize(n int) int {
	if n >= 10 {
		return 2
	}
	return 1
}

// cpuSlots limits the attempts running at once across all Generate calls of
// the process (e.g. serve's background pool and a request) to one less than
// the available cores, so generating always leaves a core free when there is
// more than one. An attempt runs on a single goroutine.
var cpuSlots = make(chan struct{}, max(1, runtime.GOMAXPROCS(0)-1))

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
	k := o.Alternatives
	if k <= 0 {
		k = 8
	}
	minSize := max(o.MinRegionSize, 1)
	workers := o.Workers
	if workers <= 0 {
		workers = min(4, cap(cpuSlots))
	}

	// Attempt i draws its own generator from rng, in order, and the successful
	// attempt with the lowest index wins. So the result depends only on rng,
	// not on the number of workers or on scheduling. Once attempt i has
	// succeeded, attempts after i are not started and running ones give up.
	type result struct {
		p   *puzzle.Puzzle
		sol []int
	}
	results := make([]result, attempts)
	var best atomic.Int64 // lowest successful index so far
	best.Store(int64(attempts))
	slots := make(chan struct{}, workers) // at most workers attempts at a time
	var wg sync.WaitGroup
	for i := range attempts {
		slots <- struct{}{}
		cpuSlots <- struct{}{}
		if int64(i) > best.Load() {
			<-cpuSlots
			<-slots
			break
		}
		arng := rand.New(rand.NewPCG(rng.Uint64(), rng.Uint64()))
		wg.Go(func() {
			defer func() { <-cpuSlots; <-slots }()
			stop := func() bool { return int64(i) > best.Load() }
			sol := randomSolution(n, arng)
			p := growRegions(n, sol, arng)
			if !makeUnique(p, sol, arng, k, minSize, stop) || smallestRegion(p) < minSize {
				return
			}
			if o.Accept != nil && !o.Accept(p, sol) {
				return
			}
			results[i] = result{p, sol}
			for {
				b := best.Load()
				if int64(i) >= b || best.CompareAndSwap(b, int64(i)) {
					break
				}
			}
		})
	}
	wg.Wait()
	if b := best.Load(); b < int64(attempts) {
		return results[b].p, results[b].sol, nil
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

// makeUnique repairs the layout until sol is its only solution. Each step looks
// at up to k alternative solutions and moves one piece of a region into a
// neighboring region (see bestMove). Returns false if it gives up, or as soon
// as stop (if not nil) returns true.
func makeUnique(p *puzzle.Puzzle, sol []int, rng *rand.Rand, k, minSize int, stop func() bool) bool {
	var removed [][]int // alternatives destroyed so far; moves that revive them are penalized
	for range 6 * p.N { // successful repairs need well under 6n steps; failing ones would loop
		if stop != nil && stop() {
			return false
		}
		var alts [][]int
		for _, s := range bruteforce.Solve(p, k+1) {
			if !slices.Equal(s, sol) {
				alts = append(alts, s)
			}
		}
		if len(alts) == 0 {
			return true // sol is always a solution, so it is the only one
		}
		m, ok := bestMove(p, sol, alts, removed, rng, minSize)
		if !ok {
			return false
		}
		removed = append(removed, alts...)
		for _, c := range m.cells {
			p.Regions[c.Row][c.Col] = m.to
		}
	}
	return false
}

// move reassigns cells (all from one region) to region to.
type move struct {
	cells []puzzle.Pos
	to    int
}

// bestMove picks the move that destroys the most alternatives. A candidate takes
// a star x of some alternative out of its region A and gives it to a neighboring
// region B. If x does not touch another region, the shortest path from x to a
// cell of A that does goes along. So does every cell of A that would lose its
// connection to A's star in sol. A stays connected, the piece is connected and
// touches B, and sol stays valid (the piece never holds a star of sol). So a
// move is blocked only by minSize, or when A touches other regions only at its
// star in sol.
//
// An alternative is destroyed exactly when it has a star in the piece (A loses
// its only star of that alternative). Score: 10 per destroyed alternative,
// -20 per previously destroyed alternative that becomes valid again, -1 per
// moved cell. Ties are broken with rng.
func bestMove(p *puzzle.Puzzle, sol []int, alts, removed [][]int, rng *rand.Rand, minSize int) (move, bool) {
	n := p.N
	size := make([]int, n)
	for r := range n {
		for c := range n {
			size[p.Regions[r][c]]++
		}
	}
	anchor := make([]puzzle.Pos, n) // star of sol in each region
	for r, c := range sol {
		anchor[p.Regions[r][c]] = puzzle.Pos{Row: r, Col: c}
	}
	tried := make([]bool, n*n)
	var best []move
	bestScore := 0
	for _, alt := range alts {
		for r, c := range alt {
			if c == sol[r] || tried[r*n+c] {
				continue
			}
			tried[r*n+c] = true
			x := puzzle.Pos{Row: r, Col: c}
			from := p.Regions[r][c]
			path := pathToBorder(p, anchor[from], x)
			if path == nil {
				continue
			}
			piece := detached(p, anchor[from], path)
			if size[from]-len(piece) < minSize {
				continue
			}
			inPiece := make([]bool, n*n)
			for _, q := range piece {
				inPiece[q.Row*n+q.Col] = true
			}
			kills := 0
			for _, s := range alts {
				for rr, cc := range s {
					if inPiece[rr*n+cc] {
						kills++
						break
					}
				}
			}
			for _, to := range neighborRegions(p, path[len(path)-1]) {
				for _, q := range piece {
					p.Regions[q.Row][q.Col] = to
				}
				revived := 0
				for _, s := range removed {
					if validFor(p, s) {
						revived++
					}
				}
				for _, q := range piece {
					p.Regions[q.Row][q.Col] = from
				}
				score := 10*kills - 20*revived - len(piece)
				switch {
				case best == nil || score > bestScore:
					best, bestScore = []move{{piece, to}}, score
				case score == bestScore:
					best = append(best, move{piece, to})
				}
			}
		}
	}
	if best == nil {
		return move{}, false
	}
	return best[rng.IntN(len(best))], true
}

// pathToBorder returns the shortest path inside x's region from x to a cell
// that touches another region (just x if it already does), avoiding anchor.
// Returns nil if there is none.
func pathToBorder(p *puzzle.Puzzle, anchor, x puzzle.Pos) []puzzle.Pos {
	n, reg := p.N, p.Regions[x.Row][x.Col]
	parent := make([]int, n*n)
	for i := range parent {
		parent[i] = -1
	}
	parent[x.Row*n+x.Col] = x.Row*n + x.Col
	parent[anchor.Row*n+anchor.Col] = anchor.Row*n + anchor.Col // never entered
	queue := []puzzle.Pos{x}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if len(neighborRegions(p, cur)) > 0 {
			path := []puzzle.Pos{cur}
			for i := cur.Row*n + cur.Col; i != x.Row*n+x.Col; {
				i = parent[i]
				path = append(path, puzzle.Pos{Row: i / n, Col: i % n})
			}
			slices.Reverse(path)
			return path
		}
		for _, d := range orth {
			rr, cc := cur.Row+d.Row, cur.Col+d.Col
			if p.In(rr, cc) && parent[rr*n+cc] == -1 && p.Regions[rr][cc] == reg {
				parent[rr*n+cc] = cur.Row*n + cur.Col
				queue = append(queue, puzzle.Pos{Row: rr, Col: cc})
			}
		}
	}
	return nil
}

// detached returns the cells of cut plus every cell of their region that is cut
// off from anchor once they are removed. anchor must lie in the same region and
// not in cut.
func detached(p *puzzle.Puzzle, anchor puzzle.Pos, cut []puzzle.Pos) []puzzle.Pos {
	n, reg := p.N, p.Regions[anchor.Row][anchor.Col]
	seen := make([]bool, n*n)
	for _, c := range cut {
		seen[c.Row*n+c.Col] = true
	}
	seen[anchor.Row*n+anchor.Col] = true
	stack := []puzzle.Pos{anchor}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, d := range orth {
			rr, cc := cur.Row+d.Row, cur.Col+d.Col
			if p.In(rr, cc) && !seen[rr*n+cc] && p.Regions[rr][cc] == reg {
				seen[rr*n+cc] = true
				stack = append(stack, puzzle.Pos{Row: rr, Col: cc})
			}
		}
	}
	piece := slices.Clone(cut)
	for r := range n {
		for c := range n {
			if p.Regions[r][c] == reg && !seen[r*n+c] {
				piece = append(piece, puzzle.Pos{Row: r, Col: c})
			}
		}
	}
	return piece
}

// neighborRegions lists the distinct regions orthogonally adjacent to x, other than x's own.
func neighborRegions(p *puzzle.Puzzle, x puzzle.Pos) []int {
	own := p.Regions[x.Row][x.Col]
	var out []int
	for _, d := range orth {
		rr, cc := x.Row+d.Row, x.Col+d.Col
		if !p.In(rr, cc) || p.Regions[rr][cc] == own || slices.Contains(out, p.Regions[rr][cc]) {
			continue
		}
		out = append(out, p.Regions[rr][cc])
	}
	return out
}

// validFor reports whether s (star column per row) has exactly one star per region.
// Rows, columns and touching are properties of s alone and were checked when s was found.
func validFor(p *puzzle.Puzzle, s []int) bool {
	var cnt [puzzle.MaxN]int
	for r, c := range s {
		cnt[p.Regions[r][c]]++
	}
	for _, x := range cnt[:p.N] {
		if x != 1 {
			return false
		}
	}
	return true
}

func smallestRegion(p *puzzle.Puzzle) int {
	m := p.N * p.N
	for _, cells := range p.RegionCells() {
		m = min(m, len(cells))
	}
	return m
}
