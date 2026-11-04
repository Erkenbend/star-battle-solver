package solver

import (
	"fmt"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
)

// Runner is RunSync or RunConcurrent.
type Runner func(start *board.State, observers []Observer) Result

// Options configure Solve.
type Options struct {
	// Run drives the observers to a fixpoint (default RunSync).
	Run Runner
	// Lookahead: when stuck, try a star on free cells (in parallel) and run the
	// rules alone. The first cell where that leads to a contradiction becomes a
	// cross, then the rules run again (like a human would continue).
	Lookahead bool
	// Backtrack: when lookahead finds nothing, guess a star and solve recursively.
	// If the guess fails, its cell is a cross. With Backtrack, Solve never ends Stuck.
	Backtrack bool
}

// Stats measure how much search a puzzle needed beyond the rules.
type Stats struct {
	Lookahead int // cells crossed because a star there contradicts the rules directly
	Guesses   int // stars guessed (including inside failed branches)
	Refuted   int // guesses that failed and so proved a cross
	MaxDepth  int // deepest nesting of guesses
}

func (s *Stats) add(o Stats) {
	s.Lookahead += o.Lookahead
	s.Guesses += o.Guesses
	s.Refuted += o.Refuted
	s.MaxDepth = max(s.MaxDepth, o.MaxDepth)
}

// HypothesisSource is the Source of marks produced by lookahead and guessing.
const HypothesisSource = "Hypothesis"

// Solve runs the rules and, when they get stuck, lookahead and backtracking as
// configured:
//
//	loop:
//	  run the rules to a fixpoint           → solved / contradiction: done
//	  lookahead: star on c contradicts?     → first such c = X, loop
//	  guess: star on c, Solve recursively   → solved: done
//	                                         → contradiction: c = X, loop
//
// Every cross derived from a failed hypothesis is a valid deduction, so the
// trace stays explainable. Only the final successful guess (if any) is an
// unproven step; for puzzles with a unique solution it is the solution.
// start is not modified.
func Solve(start *board.State, observers []Observer, o Options) Result {
	return solve(start, observers, o, 0)
}

func solve(start *board.State, observers []Observer, o Options, depth int) Result {
	run := o.Run
	if run == nil {
		run = RunSync
	}
	s := start.Clone()
	var trace []Step
	stats := Stats{MaxDepth: depth}
	done := func(res Result) Result {
		res.Trace = append(trace, res.Trace...)
		res.Stats = stats
		return res
	}

	for {
		res := run(s, observers)
		if res.Status != Stuck {
			return done(res)
		}
		trace = append(trace, res.Trace...)
		s = res.State

		if o.Lookahead {
			if r, ok := lookahead(s, observers, run); ok {
				why := fmt.Sprintf("Annahme: Stern auf (%d,%d). Dann %v → X", r.pos.Row, r.pos.Col, r.err)
				m := Cross(r.pos, HypothesisSource).Why(why)
				delta, err := Apply(s, m)
				trace = record(trace, m, delta)
				if err != nil {
					return done(finish(s, nil, err))
				}
				stats.Lookahead++
				continue
			}
		}
		if !o.Backtrack {
			return done(finish(s, nil, nil))
		}

		// Guess a star in the open unit with the fewest free cells.
		unit, free := branchUnit(s)
		if unit == nil {
			return done(finish(s, nil, nil)) // nothing open: Contradiction/Solved already handled
		}
		pos := free[0]
		stats.Guesses++
		guessed := s.Clone()
		why := fmt.Sprintf("Raten: Stern auf (%d,%d) (eines von %d freien Feldern in %s)",
			pos.Row, pos.Col, len(free), unitName(*unit))
		guess := Star(pos, HypothesisSource).Why(why)
		delta, err := Apply(guessed, guess)
		var sub Result
		if err != nil {
			sub = finish(guessed, nil, err)
		} else {
			sub = solve(guessed, observers, o, depth+1)
		}
		stats.add(sub.Stats)
		if sub.Status == Solved {
			trace = record(trace, guess, delta)
			return done(sub)
		}
		// The guess failed: pos cannot hold a star.
		stats.Refuted++
		why = fmt.Sprintf("Annahme: Stern auf (%d,%d). Weiteres Raten führt immer zu einem Widerspruch → X", pos.Row, pos.Col)
		if sub.Stats.Guesses == 0 {
			why = fmt.Sprintf("Annahme: Stern auf (%d,%d). Dann %v → X", pos.Row, pos.Col, sub.Err)
		}
		m := Cross(pos, HypothesisSource).Why(why)
		delta, err = Apply(s, m)
		trace = record(trace, m, delta)
		if err != nil {
			return done(finish(s, nil, err))
		}
	}
}

type refutation struct {
	pos puzzle.Pos
	err error
}

// lookahead tries a star on free cells and returns the first one (in candidate
// order, see candidates) where the rules alone then find a contradiction.
// Trials run in parallel; once a refutation is known, later candidates are skipped.
func lookahead(s *board.State, observers []Observer, run Runner) (refutation, bool) {
	cells := candidates(s)
	errs := make([]error, len(cells))
	var first atomic.Int64 // lowest refuted index so far
	first.Store(int64(len(cells)))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if int64(i) > first.Load() {
					continue // an earlier candidate is already refuted
				}
				trial := s.Clone()
				_, err := Apply(trial, Star(cells[i], HypothesisSource))
				if err == nil {
					if res := run(trial, observers); res.Status == Contradiction {
						err = res.Err
					}
				}
				if err != nil {
					errs[i] = err
					for cur := first.Load(); int64(i) < cur && !first.CompareAndSwap(cur, int64(i)); cur = first.Load() {
					}
				}
			}
		}()
	}
	for i := range cells {
		if int64(i) > first.Load() {
			break
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	// Every index below first was fully tried, so the result is deterministic.
	if i := first.Load(); i < int64(len(cells)) {
		return refutation{cells[i], errs[i]}, true
	}
	return refutation{}, false
}

// candidates orders the free cells the way a human would try them: cells in the
// tightest unit first (fewest free cells among its row, column and region),
// ties in board order.
func candidates(s *board.State) []puzzle.Pos {
	p := s.P
	freeIn := map[puzzle.UnitKind][]int{}
	for _, u := range p.Units() {
		if freeIn[u.Kind] == nil {
			freeIn[u.Kind] = make([]int, p.N)
		}
		freeIn[u.Kind][u.Index] = len(s.Free(u.Cells))
	}
	tight := func(q puzzle.Pos) int {
		t := p.N * p.N
		for _, k := range []puzzle.UnitKind{puzzle.Row, puzzle.Col, puzzle.Region} {
			t = min(t, freeIn[k][p.UnitOf(k, q)])
		}
		return t
	}
	var cells []puzzle.Pos
	for r := range p.N {
		for c := range p.N {
			if s.At(r, c) == board.Free {
				cells = append(cells, puzzle.Pos{Row: r, Col: c})
			}
		}
	}
	slices.SortStableFunc(cells, func(a, b puzzle.Pos) int { return tight(a) - tight(b) })
	return cells
}

// branchUnit returns the open unit with the fewest free cells, and those cells.
func branchUnit(s *board.State) (*puzzle.Unit, []puzzle.Pos) {
	var best *puzzle.Unit
	var bestFree []puzzle.Pos
	for _, u := range s.P.Units() {
		if s.HasStar(u.Cells) {
			continue
		}
		free := s.Free(u.Cells)
		if best == nil || len(free) < len(bestFree) {
			best, bestFree = &u, free
		}
	}
	return best, bestFree
}

func unitName(u puzzle.Unit) string {
	switch u.Kind {
	case puzzle.Row:
		return fmt.Sprintf("Zeile %d", u.Index)
	case puzzle.Col:
		return fmt.Sprintf("Spalte %d", u.Index)
	default:
		return fmt.Sprintf("Farbe %c", puzzle.RegionLetter(u.Index))
	}
}
