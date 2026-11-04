package observers

import (
	"math/rand/v2"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/bruteforce"
	"starbattle/internal/generator"
	"starbattle/internal/puzzle"
	"starbattle/internal/render"
	"starbattle/internal/solver"
)

// expectDeduce runs o.Deduce on the "before" state and checks that adding exactly
// the returned marks (without implied crosses) yields the "after" state.
// States use the board.Parse format: M = star, X = cross, . = free.
func expectDeduce(t *testing.T, p *puzzle.Puzzle, o solver.Observer, before, after string) {
	t.Helper()
	s := board.MustParse(p, before)
	want := board.MustParse(p, after)
	orig := s.Clone()
	got := s.Clone()
	for _, m := range o.Deduce(s) {
		if _, err := got.Set(m.Pos.Row, m.Pos.Col, m.Kind); err != nil {
			t.Fatalf("%s: %v", o.Name(), err)
		}
	}
	if !s.Equal(orig) {
		t.Fatalf("%s modified its input state", o.Name())
	}
	if !got.Equal(want) {
		t.Fatalf("%s\nlayout:\n%s\nbefore:\n%s\ngot:\n%s\nwant:\n%s", o.Name(), p, orig, got, want)
	}
}

// TestSoundness runs all observers on generated puzzles and checks that every
// deduction agrees with the (unique) brute-force solution. This catches wrong rules
// even while the solver is still too weak to finish puzzles.
func TestSoundness(t *testing.T) {
	solved, total := 0, 0
	for n := 4; n <= 10; n++ {
		for seed := range uint64(20) {
			rng := rand.New(rand.NewPCG(seed, uint64(n)))
			p, sol, err := generator.Generate(n, rng, generator.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if bruteforce.CountSolutions(p, 2) != 1 {
				t.Fatal("generator returned a non-unique puzzle")
			}
			res := solver.RunSync(board.New(p), All(p))
			total++
			if res.Status == solver.Contradiction {
				t.Fatalf("n=%d seed=%d: contradiction: %v\n%s", n, seed, res.Err, render.State(res.State, render.Options{NoColor: true}))
			}
			for r := range n {
				for c := range n {
					v := res.State.At(r, c)
					if (v == board.Star && sol[r] != c) || (v == board.Cross && sol[r] == c) {
						t.Fatalf("n=%d seed=%d: wrong %v at (%d,%d), trace:\n%v\n%s",
							n, seed, v, r, c, res.Trace, p)
					}
				}
			}
			if res.Status == solver.Solved {
				solved++
			} else {
				t.Logf("n=%d seed=%d: stuck with %d free cells", n, seed, countFree(res.State))
			}
		}
	}
	t.Logf("solved by logic alone: %d/%d", solved, total)
}

// TestRunnersAgree checks the concurrent runner reaches the same fixpoint as the
// synchronous one. Run with -race to also check for data races.
func TestRunnersAgree(t *testing.T) {
	for n := 4; n <= 10; n++ {
		for seed := range uint64(10) {
			rng := rand.New(rand.NewPCG(seed, uint64(n)))
			p, _, err := generator.Generate(n, rng, generator.Options{})
			if err != nil {
				t.Fatal(err)
			}
			want := solver.RunSync(board.New(p), All(p))
			for range 5 {
				got := solver.RunConcurrent(board.New(p), All(p))
				if got.Status != want.Status || !got.State.Equal(want.State) {
					t.Fatalf("n=%d seed=%d: concurrent %v\n%s\nsync %v\n%s\n%s",
						n, seed, got.Status, got.State, want.Status, want.State, p)
				}
			}
		}
	}
}

func countFree(s *board.State) int {
	free := 0
	for r := range s.P.N {
		for c := range s.P.N {
			if s.At(r, c) == board.Free {
				free++
			}
		}
	}
	return free
}
