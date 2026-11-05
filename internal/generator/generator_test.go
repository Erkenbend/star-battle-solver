package generator

import (
	"math/rand/v2"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/bruteforce"
	"starbattle/internal/puzzle"
)

func TestGenerateUnique(t *testing.T) {
	for n := 4; n <= 12; n++ {
		seeds := uint64(15)
		if n > 10 {
			seeds = 3
		}
		for seed := range seeds {
			rng := rand.New(rand.NewPCG(seed, uint64(n)))
			p, sol, err := Generate(n, rng, Options{})
			if err != nil {
				t.Fatalf("n=%d seed=%d: %v", n, seed, err)
			}
			if err := p.Validate(); err != nil {
				t.Fatalf("n=%d seed=%d: invalid layout: %v\n%s", n, seed, err, p)
			}
			if !board.FromSolution(p, sol).Solved() {
				t.Fatalf("n=%d seed=%d: returned solution is invalid\n%s", n, seed, p)
			}
			sols := bruteforce.Solve(p, 2)
			if len(sols) != 1 {
				t.Fatalf("n=%d seed=%d: %d solutions\n%s", n, seed, len(sols), p)
			}
			for r := range sol {
				if sols[0][r] != sol[r] {
					t.Fatalf("n=%d seed=%d: brute force found a different solution", n, seed)
				}
			}
		}
	}
}

func TestGenerateDeterministic(t *testing.T) {
	a, _, _ := Generate(8, rand.New(rand.NewPCG(7, 7)), Options{})
	b, _, _ := Generate(8, rand.New(rand.NewPCG(7, 7)), Options{})
	if a.String() != b.String() {
		t.Fatal("same seed produced different puzzles")
	}
}

// TestRepairMoves: on fresh layouts a repair move always exists and keeps the
// layout valid (connected regions) and the intended solution valid.
func TestRepairMoves(t *testing.T) {
	for n := 5; n <= 10; n++ {
		for seed := range uint64(10) {
			rng := rand.New(rand.NewPCG(seed, uint64(n)))
			sol := randomSolution(n, rng)
			p := growRegions(n, sol, rng)
			var removed [][]int
			for range 3 * n {
				var alts [][]int
				for _, s := range bruteforce.Solve(p, 9) {
					if !equal(s, sol) {
						alts = append(alts, s)
					}
				}
				if len(alts) == 0 {
					break
				}
				m, ok := bestMove(p, sol, alts, removed, rng, 1)
				if !ok {
					t.Fatalf("n=%d seed=%d: no move found\n%s", n, seed, p)
				}
				removed = append(removed, alts...)
				for _, c := range m.cells {
					p.Regions[c.Row][c.Col] = m.to
				}
				if err := p.Validate(); err != nil {
					t.Fatalf("n=%d seed=%d: move broke the layout: %v\n%s", n, seed, err, p)
				}
				if !validFor(p, sol) {
					t.Fatalf("n=%d seed=%d: move broke the intended solution\n%s", n, seed, p)
				}
				if validFor(p, alts[0]) && slicesContainCell(m.cells, alts[0]) {
					t.Fatalf("n=%d seed=%d: moved a star of alts[0] but it is still valid", n, seed)
				}
			}
		}
	}
}

func slicesContainCell(cells []puzzle.Pos, s []int) bool {
	for _, c := range cells {
		if s[c.Row] == c.Col {
			return true
		}
	}
	return false
}

func TestMinRegionSize(t *testing.T) {
	for n := 6; n <= 9; n++ {
		for seed := range uint64(5) {
			p, _, err := Generate(n, rand.New(rand.NewPCG(seed, uint64(n))), Options{MinRegionSize: 3})
			if err != nil {
				t.Fatalf("n=%d seed=%d: %v", n, seed, err)
			}
			if m := smallestRegion(p); m < 3 {
				t.Fatalf("n=%d seed=%d: region of size %d\n%s", n, seed, m, p)
			}
			if bruteforce.CountSolutions(p, 2) != 1 {
				t.Fatalf("n=%d seed=%d: not unique\n%s", n, seed, p)
			}
		}
	}
}
