package observers

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"starbattle"
	"starbattle/internal/board"
	"starbattle/internal/bruteforce"
	"starbattle/internal/collection"
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

// hardTestFiles returns the paths of the hard puzzles the tests run, see
// starbattle.HardTestSet.
func hardTestFiles(t *testing.T) []string {
	t.Helper()
	names, err := starbattle.HardTestSet()
	if err != nil || len(names) == 0 {
		t.Fatalf("no hard puzzles: %v", err)
	}
	files := make([]string, len(names))
	for i, name := range names {
		files[i] = filepath.Join("../../..", starbattle.HardDir, name)
	}
	return files
}

// TestHardPuzzles: puzzles where the base rules alone get stuck (testdata/hard,
// the sample unless STARBATTLE_ALL_HARD=1). With -short without the 3-star
// ones, which cost most of the time.
func TestHardPuzzles(t *testing.T) {
	files := hardTestFiles(t)
	needGuess, byNewRules, tested := 0, 0, 0
	for _, f := range files {
		text, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if testing.Short() && collection.Rating(text) >= 3 {
			continue
		}
		tested++
		p, err := puzzle.Parse(bytes.NewReader(text))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		name := filepath.Base(f)

		if res := solver.RunSync(board.New(p), AllWith(p, BaseConfig)); res.Status != solver.Stuck {
			t.Errorf("%s: base rules alone now give %v, testdata no longer hard", name, res.Status)
		}
		if res := solver.RunSync(board.New(p), All(p)); res.Status == solver.Solved {
			byNewRules++
		} else {
			t.Logf("%s: still needs lookahead with all rules", name)
		}
		full := solver.Options{Lookahead: true, Backtrack: true}
		res := solver.Solve(board.New(p), All(p), full)
		checkSolution(t, name, p, res)
		needGuess += min(res.Stats.Guesses, 1)

		full.Run = solver.RunConcurrent
		checkSolution(t, name+" (concurrent)", p, solver.Solve(board.New(p), All(p), full))
	}
	t.Logf("%d/%d solved by all rules without lookahead", byNewRules, tested)
	t.Logf("%d/%d hard puzzles needed guessing beyond lookahead", needGuess, tested)
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

// TestLookaheadExplains: every lookahead step carries its reasoning chain, starting
// with the assumed star and ending in a contradiction that names its cells.
func TestLookaheadExplains(t *testing.T) {
	p := puzzle.MustParse("CCAAA\nCCADA\nEDDDA\nEDEBB\nEEEEB\n") // testdata/hard/n05_s029.txt
	res := solver.Solve(board.New(p), AllWith(p, BaseConfig), solver.Options{Lookahead: true})
	found := 0
	for _, st := range res.Trace {
		if st.Source != solver.HypothesisSource {
			continue
		}
		found++
		h := st.Hypothesis
		if h == nil || len(h.Steps) == 0 || h.Steps[0].Marks[0].Kind != board.Star {
			t.Fatalf("step %q has no proper chain", st.Reason)
		}
		var ce *board.ContradictionError
		if !errors.As(h.Contradiction, &ce) || len(ce.Cells) == 0 {
			t.Fatalf("step %q: contradiction %v without cells", st.Reason, h.Contradiction)
		}
	}
	if found == 0 {
		t.Fatal("no lookahead step")
	}
}

// closer wraps an observer and closes done on the at-th call of Deduce
// among all observers sharing calls, so a test can cancel a solve at a
// point in the search instead of at a time.
type closer struct {
	solver.Observer
	calls *atomic.Int64
	at    int64
	done  chan struct{}
	once  *sync.Once
}

func (c closer) Deduce(s *board.State) []solver.Mark {
	if c.calls.Add(1) == c.at {
		c.once.Do(func() { close(c.done) })
	}
	return c.Observer.Deduce(s)
}

// cancelAt returns obs wrapped so that done closes on the at-th call of
// Deduce (counting all observers and goroutines), and the counter.
func cancelAt(obs []solver.Observer, at int64) ([]solver.Observer, chan struct{}, *atomic.Int64) {
	done := make(chan struct{})
	calls, once := new(atomic.Int64), new(sync.Once)
	out := make([]solver.Observer, len(obs))
	for i, o := range obs {
		out[i] = closer{Observer: o, calls: calls, at: at, done: done, once: once}
	}
	return out, done, calls
}

// TestSolveCancelled: a solve whose Done is closed ends Cancelled with
// ErrCancelled, whether that happens before it starts, during lookahead or
// inside a guess (a cancelled guess must not be taken for a refuted one,
// which would cross the cell of the solution's star).
// A Done that is never closed changes nothing.
func TestSolveCancelled(t *testing.T) {
	hard, err := os.ReadFile(filepath.Join("../../..", starbattle.HardDir, "n10_s1271.txt"))
	if err != nil {
		t.Fatal(err)
	}
	p := puzzle.MustParse(string(hard))
	full := solver.Options{Lookahead: true, Backtrack: true}
	want := solver.Solve(board.New(p), All(p), full)
	checkSolution(t, "uncancelled", p, want)

	open := full
	open.Done = make(chan struct{})
	if got := solver.Solve(board.New(p), All(p), open); !reflect.DeepEqual(got.Trace, want.Trace) {
		t.Error("a Done that is never closed changes the trace")
	}

	closed := make(chan struct{})
	close(closed)
	for _, o := range []solver.Options{
		{Lookahead: true, Done: closed},
		{Backtrack: true, Done: closed},
		{Lookahead: true, Backtrack: true, Done: closed, Run: solver.RunConcurrent},
	} {
		if res := solver.Solve(board.New(p), All(p), o); res.Status != solver.Cancelled || !errors.Is(res.Err, solver.ErrCancelled) {
			t.Errorf("closed before the start: %v (%v), want cancelled", res.Status, res.Err)
		}
	}

	// During the search: most calls of the first observer are in lookahead
	// trials. After the last lookahead the rules run to the end unchecked,
	// so the call to close at must come before that.
	counted, _, counter := cancelAt(All(p), -1)
	solver.Solve(board.New(p), counted, full)
	calls := counter.Load()
	for _, at := range []int64{calls / 4, calls / 2, calls * 3 / 4} {
		obs, done, _ := cancelAt(All(p), at)
		o := full
		o.Done = done
		if res := solver.Solve(board.New(p), obs, o); res.Status != solver.Cancelled {
			t.Errorf("closed at call %d of %d: %v, want cancelled", at, calls, res.Status)
		}
	}

	// Inside a guess: with UnitFinish alone the solver guesses, nested.
	var g *puzzle.Puzzle
	var weak []solver.Observer
	for seed := uint64(0); g == nil; seed++ {
		q, _, err := generator.Generate(9, rand.New(rand.NewPCG(seed, 9)), generator.Options{})
		if err != nil {
			t.Fatal(err)
		}
		weak = nil
		for _, u := range q.Units() {
			weak = append(weak, UnitFinish{Unit: u})
		}
		counted, _, counter = cancelAt(weak, -1)
		if res := solver.Solve(board.New(q), counted, solver.Options{Backtrack: true}); res.Stats.MaxDepth >= 3 {
			g = q
		}
	}
	sol := bruteforce.Solve(g, 1)[0]
	calls = counter.Load()
	for k := range int64(20) {
		at := 1 + k*calls/20
		obs, done, _ := cancelAt(weak, at)
		res := solver.Solve(board.New(g), obs, solver.Options{Backtrack: true, Done: done})
		if res.Status != solver.Cancelled {
			t.Errorf("guessing, closed at call %d of %d: %v, want cancelled", at, calls, res.Status)
			continue
		}
		// What a cancelled solve decided so far is still sound.
		for r := range g.N {
			for c := range g.N {
				if v := res.State.At(r, c); v != board.Free && (v == board.Star) != (sol[r] == c) {
					t.Errorf("guessing, closed at call %d of %d: wrong %v at (%d,%d)", at, calls, v, r, c)
				}
			}
		}
	}
}
