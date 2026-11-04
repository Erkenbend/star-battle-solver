package generator

import (
	"math/rand/v2"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/bruteforce"
)

func TestGenerateUnique(t *testing.T) {
	for n := 4; n <= 10; n++ {
		for seed := range uint64(15) {
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
