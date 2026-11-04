package observers

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/bruteforce"
	"starbattle/internal/generator"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// checkSolution fails unless res is solved with the unique brute-force solution.
func checkSolution(t *testing.T, name string, p *puzzle.Puzzle, res solver.Result) {
	t.Helper()
	if res.Status != solver.Solved {
		t.Fatalf("%s: status %v (%v)\n%s", name, res.Status, res.Err, res.State)
	}
	sols := bruteforce.Solve(p, 2)
	if len(sols) != 1 || !res.State.Equal(board.FromSolution(p, sols[0])) {
		t.Fatalf("%s: does not match brute-force solution\n%s", name, res.State)
	}
}

// TestHardPuzzles: puzzles where the rules alone get stuck (testdata/hard).
func TestHardPuzzles(t *testing.T) {
	files, _ := filepath.Glob("../../../testdata/hard/*.txt")
	if len(files) == 0 {
		t.Fatal("no testdata found")
	}
	needGuess := 0
	for _, f := range files {
		in, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		p, err := puzzle.Parse(in)
		in.Close()
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		name := filepath.Base(f)

		if res := solver.RunSync(board.New(p), All(p)); res.Status != solver.Stuck {
			t.Errorf("%s: rules alone now give %v, testdata no longer hard", name, res.Status)
		}
		full := solver.Options{Lookahead: true, Backtrack: true}
		res := solver.Solve(board.New(p), All(p), full)
		checkSolution(t, name, p, res)
		needGuess += min(res.Stats.Guesses, 1)

		full.Run = solver.RunConcurrent
		checkSolution(t, name+" (concurrent)", p, solver.Solve(board.New(p), All(p), full))
	}
	t.Logf("%d/%d hard puzzles needed guessing beyond lookahead", needGuess, len(files))
}

// TestBacktrackingWithWeakRules forces real guessing by giving the solver only the
// UnitFinish rule.
func TestBacktrackingWithWeakRules(t *testing.T) {
	guessed := 0
	for n := 4; n <= 8; n++ {
		for seed := range uint64(10) {
			p, _, err := generator.Generate(n, rand.New(rand.NewPCG(seed, uint64(n))), generator.Options{})
			if err != nil {
				t.Fatal(err)
			}
			var weak []solver.Observer
			for _, u := range p.Units() {
				weak = append(weak, UnitFinish{Unit: u})
			}
			for _, o := range []solver.Options{
				{Backtrack: true},
				{Backtrack: true, Lookahead: true},
				{Backtrack: true, Run: solver.RunConcurrent},
			} {
				res := solver.Solve(board.New(p), weak, o)
				checkSolution(t, p.String(), p, res)
				if res.Stats.Guesses > 0 {
					guessed++
				}
			}
		}
	}
	if guessed == 0 {
		t.Fatal("backtracking was never exercised")
	}
}

// TestLookaheadOnlyIsSound: without backtracking, Solve may end stuck, but every
// cell it decided must agree with the solution.
func TestLookaheadOnlyIsSound(t *testing.T) {
	for n := 4; n <= 9; n++ {
		for seed := range uint64(10) {
			p, sol, err := generator.Generate(n, rand.New(rand.NewPCG(seed, uint64(n))), generator.Options{})
			if err != nil {
				t.Fatal(err)
			}
			var weak []solver.Observer
			for _, u := range p.Units() {
				weak = append(weak, UnitFinish{Unit: u})
			}
			res := solver.Solve(board.New(p), weak, solver.Options{Lookahead: true})
			if res.Status == solver.Contradiction {
				t.Fatalf("contradiction: %v\n%s", res.Err, p)
			}
			for r := range n {
				for c := range n {
					v := res.State.At(r, c)
					if (v == board.Star) != (sol[r] == c) && v != board.Free {
						t.Fatalf("wrong %v at (%d,%d)\n%s", v, r, c, p)
					}
				}
			}
		}
	}
}
