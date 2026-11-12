// Package hardsearch looks for new hard puzzles for testdata/hard (and so the
// library): it generates puzzles seed by seed, exactly as `starbattle
// generate` does, and checks which rules get stuck on them.
package hardsearch

import (
	"context"
	"fmt"
	"math/rand/v2"
	"runtime"
	"sync"
	"time"

	"starbattle/internal/board"
	"starbattle/internal/generator"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/solver/observers"
)

// Generate returns the puzzle `starbattle generate -n n -seed seed` prints:
// same seeding (rand.NewPCG(seed, n)) and the default minimum region size.
// done, if not nil, cancels it once closed.
func Generate(n int, seed uint64, done <-chan struct{}) (*puzzle.Puzzle, error) {
	rng := rand.New(rand.NewPCG(seed, uint64(n)))
	p, _, err := generator.Generate(n, rng, generator.Options{MinRegionSize: generator.DefaultMinRegionSize(n), Done: done})
	return p, err
}

// Verdict says how hard a puzzle is for the solver.
type Verdict struct {
	Base solver.Status // the base rules alone (observers.BaseConfig)
	All  solver.Status // all rules, without search; only checked if Base is stuck
	// With lookahead and backtracking, only checked if All is stuck: cells
	// crossed by lookahead, and stars guessed.
	Lookahead, Guesses int
}

// Hard reports whether v makes a puzzle for testdata/hard: the base rules
// get stuck, and with minLookahead > 0 all rules get stuck too and the
// solver needs at least minLookahead lookahead steps.
func (v Verdict) Hard(minLookahead int) bool {
	if minLookahead <= 0 {
		return v.Base != solver.Solved
	}
	return v.All == solver.Stuck && v.Lookahead >= minLookahead
}

func (v Verdict) String() string {
	s := "base=" + v.Base.String()
	if v.Base == solver.Solved {
		return s
	}
	s += " all=" + v.All.String()
	if v.All == solver.Solved {
		return s
	}
	return s + fmt.Sprintf(" lookahead=%d guesses=%d", v.Lookahead, v.Guesses)
}

// Classify runs the checks of Verdict on p, as TestHardPuzzles does. An error
// means the full search did not solve p, which is a bug in the solver.
func Classify(p *puzzle.Puzzle) (Verdict, error) {
	var v Verdict
	v.Base = solver.RunSync(board.New(p), observers.AllWith(p, observers.BaseConfig)).Status
	if v.Base == solver.Solved {
		return v, nil
	}
	v.All = solver.RunSync(board.New(p), observers.All(p)).Status
	if v.All == solver.Solved {
		return v, nil
	}
	res := solver.Solve(board.New(p), observers.All(p), solver.Options{Lookahead: true, Backtrack: true})
	if res.Status != solver.Solved {
		return v, fmt.Errorf("full search ends %v: %v", res.Status, res.Err)
	}
	v.Lookahead, v.Guesses = res.Stats.Lookahead, res.Stats.Guesses
	return v, nil
}

// Options of a Search.
type Options struct {
	N           int
	From, Count uint64 // seeds From to From+Count-1
	// Workers is how many seeds are checked at the same time (default: the
	// number of cores). The generator limits its own CPU use besides.
	Workers int
}

// Found is the outcome for one seed.
type Found struct {
	Seed    uint64
	Puzzle  *puzzle.Puzzle
	Verdict Verdict
	Took    time.Duration // generating and classifying
	Err     error
}

// Search checks the seeds of o and calls emit for each in seed order (from
// a single goroutine). Once ctx is done, no more seeds start; it returns the
// first seed not emitted, where a later search can continue.
func Search(ctx context.Context, o Options, emit func(Found)) uint64 {
	workers := o.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	jobs := make(chan uint64)
	results := make(chan Found)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for seed := range jobs {
				results <- check(ctx, o.N, seed)
			}
		})
	}
	go func() {
		defer close(jobs)
		for i := range o.Count {
			select {
			case jobs <- o.From + i:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	next := o.From
	pending := map[uint64]Found{}
	for f := range results {
		// A seed cancelled half-way is not done: the gap ends the emitted run.
		if f.Err != nil && ctx.Err() != nil {
			continue
		}
		pending[f.Seed] = f
		for {
			g, ok := pending[next]
			if !ok {
				break
			}
			delete(pending, next)
			emit(g)
			next++
		}
	}
	return next
}

func check(ctx context.Context, n int, seed uint64) Found {
	start := time.Now()
	f := Found{Seed: seed}
	f.Puzzle, f.Err = Generate(n, seed, ctx.Done())
	if f.Err == nil {
		f.Verdict, f.Err = Classify(f.Puzzle)
	}
	f.Took = time.Since(start)
	return f
}
