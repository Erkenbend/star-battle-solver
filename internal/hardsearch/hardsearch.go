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
	// How hard it is for players, from the trace of all rules (or of the
	// full search, if they get stuck); only set if Base is stuck.
	Rating Rating
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
	if v.All != solver.Solved {
		s += fmt.Sprintf(" lookahead=%d guesses=%d", v.Lookahead, v.Guesses)
	}
	return s + fmt.Sprintf(" rating=%d", v.Rating.Stars)
}

// Classify runs the checks of Verdict on p, as TestHardPuzzles does. An error
// means the full search did not solve p, which is a bug in the solver.
func Classify(p *puzzle.Puzzle) (Verdict, error) {
	var v Verdict
	v.Base = solver.RunSync(board.New(p), observers.AllWith(p, observers.BaseConfig)).Status
	if v.Base == solver.Solved {
		return v, nil
	}
	all := solver.RunSync(board.New(p), observers.All(p))
	v.All = all.Status
	if v.All == solver.Solved {
		v.Rating = Rate(all)
		return v, nil
	}
	res := solver.Solve(board.New(p), observers.All(p), solver.Options{Lookahead: true, Backtrack: true})
	if res.Status != solver.Solved {
		return v, fmt.Errorf("full search ends %v: %v", res.Status, res.Err)
	}
	v.Lookahead, v.Guesses = res.Stats.Lookahead, res.Stats.Guesses
	v.Rating = Rate(res)
	return v, nil
}

// Rating is how hard a puzzle of the library is for players, 1 to 3 stars,
// judged by what the solver needed (the trace the library shows):
//
//	3 stars: guessing, ManyLookahead lookahead steps or more, or a thought
//	         experiment (lookahead chain) of LongChain steps or more
//	2 stars: any lookahead step, or the most demanding rule (Pigeonhole)
//	1 star:  the rules alone solve it
//
// In the library at the time of writing, about a quarter has one star and
// the rest splits evenly between two and three.
type Rating struct {
	Stars     int
	Lookahead int // lookahead steps
	Longest   int // steps of the longest thought experiment
	Guesses   int
	MaxLevel  int // highest priority level of a rule used, inside thought experiments too
}

// Thresholds of Rating.
const (
	ManyLookahead = 10
	LongChain     = 7
)

// Rate rates a puzzle by its solved trace (see Rating).
func Rate(res solver.Result) Rating {
	r := Rating{Lookahead: res.Stats.Lookahead, Guesses: res.Stats.Guesses}
	rules := func(st solver.Step) {
		if st.Level < solver.LevelLookahead { // not the search's own steps
			r.MaxLevel = max(r.MaxLevel, st.Level)
		}
	}
	for _, st := range res.Trace {
		rules(st)
		if h := st.Hypothesis; h != nil {
			r.Longest = max(r.Longest, len(h.Steps))
			for _, hs := range h.Steps {
				rules(hs)
			}
		}
	}
	switch {
	case r.Guesses > 0 || r.Lookahead >= ManyLookahead || r.Longest >= LongChain:
		r.Stars = 3
	case r.Lookahead > 0 || r.MaxLevel >= observers.LevelPigeonhole:
		r.Stars = 2
	default:
		r.Stars = 1
	}
	return r
}

// String is the rating as the puzzle files give it (after
// starbattle.RatingKey): "2 (4 lookahead steps, longest 5)".
func (r Rating) String() string {
	var why string
	switch {
	case r.Guesses > 0:
		why = fmt.Sprintf("%d lookahead steps, longest %d, %d guesses", r.Lookahead, r.Longest, r.Guesses)
	case r.Lookahead > 0:
		why = fmt.Sprintf("%d lookahead steps, longest %d", r.Lookahead, r.Longest)
	case r.MaxLevel >= observers.LevelPigeonhole:
		why = "rules alone, with Pigeonhole"
	default:
		why = "rules alone"
	}
	return fmt.Sprintf("%d (%s)", r.Stars, why)
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
