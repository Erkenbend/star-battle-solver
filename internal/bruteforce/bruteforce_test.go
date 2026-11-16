package bruteforce_test

import (
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"testing"

	"starbattle/internal/bruteforce"
	"starbattle/internal/collection"
	"starbattle/internal/puzzle"
)

// refSolve is the original plain row-by-row backtracking: the oracle. Solve
// must return exactly its result, in the same order.
func refSolve(p *puzzle.Puzzle, limit int) [][]int {
	out, _ := refSolveNodes(p, limit)
	return out
}

func refSolveNodes(p *puzzle.Puzzle, limit int) ([][]int, uint64) {
	var nodes uint64
	n := p.N
	cols := make([]int, n)
	usedCol := make([]bool, n)
	usedReg := make([]bool, n)
	var out [][]int
	var rec func(r int) bool
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
			if r > 0 && cols[r-1] >= c-1 && cols[r-1] <= c+1 {
				continue
			}
			nodes++
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
	return out, nodes
}

// randomLayout grows n connected regions from n random seed cells.
func randomLayout(n int, rng *rand.Rand) *puzzle.Puzzle {
	regs := make([][]int, n)
	for r := range regs {
		regs[r] = make([]int, n)
		for c := range regs[r] {
			regs[r][c] = -1
		}
	}
	for g, i := range rng.Perm(n * n)[:n] {
		regs[i/n][i%n] = g
	}
	dirs := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	for left := n*n - n; left > 0; {
		r, c := rng.IntN(n), rng.IntN(n)
		if regs[r][c] >= 0 {
			continue
		}
		d := dirs[rng.IntN(4)]
		rr, cc := r+d[0], c+d[1]
		if rr < 0 || rr >= n || cc < 0 || cc >= n || regs[rr][cc] < 0 {
			continue
		}
		regs[r][c] = regs[rr][cc]
		left--
	}
	p, err := puzzle.New(regs)
	if err != nil {
		panic(err)
	}
	return p
}

func hardPuzzles(t testing.TB) []collection.Entry {
	es, err := collection.Load(os.DirFS("../.."), "testdata/hard")
	if err != nil {
		t.Fatal(err)
	}
	return es
}

func check(t *testing.T, name string, p *puzzle.Puzzle, limits ...int) {
	t.Helper()
	for _, l := range limits {
		want, got := refSolve(p, l), bruteforce.Solve(p, l)
		if !slices.EqualFunc(want, got, slices.Equal) {
			t.Fatalf("%s limit %d: got %v, want %v\n%s", name, l, got, want, p)
		}
	}
}

func TestSameAsReferenceRandom(t *testing.T) {
	count, maxN := 400, 12
	if testing.Short() {
		count, maxN = 60, 10
	}
	for n := 1; n <= maxN; n++ {
		rng := rand.New(rand.NewPCG(uint64(n), 99))
		for i := range count {
			p := randomLayout(n, rng)
			limits := []int{0, 1, 2, 9}
			if n <= 9 {
				limits = append(limits, 1<<30)
			}
			check(t, fmt.Sprintf("n=%d #%d", n, i), p, limits...)
		}
	}
}

func TestSameAsReferenceHard(t *testing.T) {
	for _, e := range hardPuzzles(t) {
		if testing.Short() && e.Puzzle.N > 11 {
			continue
		}
		check(t, e.File, e.Puzzle, 1, 2, 9)
	}
}

func TestSmall(t *testing.T) {
	for _, tc := range []struct {
		layout string
		want   int
	}{
		{"A\n", 1},
		{"AB\nAB\n", 0},
		{"AAB\nACB\nCCB\n", 0},
		{"AABB\nAABB\nCCDD\nCCDD\n", 2},
	} {
		p := puzzle.MustParse(tc.layout)
		if got := bruteforce.CountSolutions(p, 10); got != tc.want {
			t.Errorf("%q: %d solutions, want %d", tc.layout, got, tc.want)
		}
		check(t, tc.layout, p, 0, 1, 10)
	}
}

// TestNodes logs the search size on the hard puzzles (independent of CPU load).
func TestNodes(t *testing.T) {
	var total, ref uint64
	for _, e := range hardPuzzles(t) {
		_, nodes := bruteforce.SolveNodes(e.Puzzle, 2)
		total += nodes
		if !testing.Short() {
			_, nodes = refSolveNodes(e.Puzzle, 2)
			ref += nodes
		}
	}
	t.Logf("placements tried on all hard puzzles (limit 2): %d (plain backtracking: %d)", total, ref)
}

func BenchmarkUnique(b *testing.B) {
	es := hardPuzzles(b)
	for b.Loop() {
		for _, e := range es {
			bruteforce.Solve(e.Puzzle, 2)
		}
	}
}

func BenchmarkLimit9(b *testing.B) {
	for _, n := range []int{10, 14, 18} {
		rng := rand.New(rand.NewPCG(uint64(n), 7))
		ps := make([]*puzzle.Puzzle, 20)
		for i := range ps {
			ps[i] = randomLayout(n, rng)
		}
		b.Run(fmt.Sprint("n=", n), func(b *testing.B) {
			for b.Loop() {
				for _, p := range ps {
					bruteforce.Solve(p, 9)
				}
			}
		})
	}
}
