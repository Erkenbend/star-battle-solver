package solver

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/words"
)

// Runner is RunSync or RunConcurrent.
type Runner func(start *board.State, observers []Observer) Result

// Options configure Solve.
type Options struct {
	// Run drives the observers to a fixpoint (default RunSync).
	Run Runner
	// Lookahead: when stuck, try a star on every free cell (in parallel) and run
	// the rules alone. Of the cells where that leads to a contradiction, the one
	// with the shortest chain becomes a cross, then the rules run again (like a
	// human would continue).
	Lookahead bool
	// Backtrack: when lookahead finds nothing, guess a star and solve recursively.
	// If the guess fails, its cell is a cross. With Backtrack, Solve never ends Stuck.
	Backtrack bool
	// Done, if not nil, cancels the solve once closed: it is checked before
	// each lookahead trial and each guess, and Solve then ends Cancelled.
	// The rules' fixpoint runs between those points are not interrupted.
	Done <-chan struct{}
}

// ErrCancelled is the Err of a Result that ended Cancelled.
var ErrCancelled = errors.New("solver: cancelled")

// cancelled reports whether done is closed (never for nil).
func cancelled(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
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
//	  lookahead: star on c contradicts?     → c with shortest chain = X, loop
//	  guess: star on c, Solve recursively   → solved: done
//	                                         → contradiction: c = X, loop
//
// Every cross derived from a failed hypothesis is a valid deduction, so the
// trace stays explainable. Only the final successful guess (if any) is an
// unproven step; for puzzles with a unique solution it is the solution.
// start is not modified. A solve cancelled through o.Done ends Cancelled;
// its State and Trace hold what was proven until then, no guess.
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
			r, ok := lookahead(s, observers, o.Done)
			if cancelled(o.Done) {
				return done(cancel(s))
			}
			if ok {
				var err error
				if trace, err = refute(s, trace, r.pos, r.hyp, LevelLookahead); err != nil {
					return done(finish(s, nil, err))
				}
				stats.Lookahead++
				continue
			}
		}
		if !o.Backtrack {
			return done(finish(s, nil, nil))
		}

		if cancelled(o.Done) {
			return done(cancel(s))
		}
		// Guess a star in the open unit with the fewest free cells.
		unit, free := branchUnit(s)
		if unit == nil {
			return done(finish(s, nil, nil)) // nothing open: Contradiction/Solved already handled
		}
		pos := free[0]
		stats.Guesses++
		guessed := s.Clone()
		u, k := *unit, len(free)
		guess := Star(pos, HypothesisSource).Why(func(w *words.Namer) string {
			return fmt.Sprintf("Raten: Stern auf %s (eines von %d freien Feldern in %s)", w.Cell(pos), k, w.Unit(u.Kind, u.Index))
		})
		guess.Level = LevelGuess
		delta, err := Apply(guessed, guess)
		var sub Result
		if err != nil {
			sub = finish(guessed, nil, err)
		} else {
			sub = solve(guessed, observers, o, depth+1)
		}
		stats.add(sub.Stats)
		if sub.Status == Cancelled {
			// Not a refutation: the guess was never decided. s holds only
			// what is proven, sub.State the unproven guess too.
			return done(cancel(s))
		}
		if sub.Status == Solved {
			trace = record(trace, guess, delta)
			return done(sub)
		}
		// The guess failed: pos cannot hold a star.
		stats.Refuted++
		var hyp *Hypothesis
		if sub.Stats.Guesses == 0 { // short chain of rules only: worth showing
			hyp = &Hypothesis{Steps: append([]Step{assumedStep(s.P, pos, delta)}, sub.Trace...), Contradiction: sub.Err}
		}
		if trace, err = refute(s, trace, pos, hyp, LevelGuess); err != nil {
			return done(finish(s, nil, err))
		}
	}
}

// cancel is the result of a cancelled solve at state s.
func cancel(s *board.State) Result {
	return Result{State: s, Status: Cancelled, Err: ErrCancelled}
}

type refutation struct {
	pos puzzle.Pos
	hyp *Hypothesis
}

// refute crosses pos because a star there was shown impossible, and records the
// step with its reasoning chain (hyp may be nil for deep backtracking). level is
// LevelLookahead or LevelGuess, depending on how the star was refuted.
func refute(s *board.State, trace []Step, pos puzzle.Pos, hyp *Hypothesis, level int) ([]Step, error) {
	m := Cross(pos, HypothesisSource).Why(func(w *words.Namer) string {
		if hyp == nil {
			return fmt.Sprintf("Annahme: Stern auf %s führt nach weiterem Raten immer zu einem Widerspruch → X", w.Cell(pos))
		}
		return fmt.Sprintf("Annahme: Stern auf %s führt zu einem Widerspruch → X", w.Cell(pos))
	})
	m.Level = level
	delta, err := Apply(s, m)
	if len(delta) > 0 {
		trace = append(trace, Step{Source: m.Source, Reason: m.Reason, Level: m.Level, Marks: delta, Hypothesis: hyp})
	}
	return trace, err
}

// assumedStep is the first step of a hypothesis chain: the star and its crosses.
func assumedStep(p *puzzle.Puzzle, pos puzzle.Pos, delta []Mark) Step {
	region := p.Region(pos.Row, pos.Col)
	why := Because(func(w *words.Namer) string {
		return fmt.Sprintf("Angenommen, auf %s ist ein Stern → X in %s, %s, %s und Nachbarschaft",
			w.Cell(pos), w.Unit(puzzle.Row, pos.Row), w.Unit(puzzle.Col, pos.Col), w.Unit(puzzle.Region, region))
	})
	for i := range delta {
		delta[i].Reason = why
		delta[i].Level = LevelLookahead
	}
	return Step{Source: HypothesisSource, Reason: why, Level: LevelLookahead, Marks: delta}
}

// lookahead tries a star on every free cell in parallel and returns the
// refutation that is easiest to follow: the shortest chain of steps to a
// contradiction (see simpler). We optimize for understandability, not speed.
//
// Each trial runs with RunSync: the trials already run in parallel, and the
// concurrent runner's step order (hence chain length) varies between runs,
// which would make the chosen explanation non-deterministic.
//
// A trial stops once its chain is longer than the shortest refutation found
// so far (best, shared by the workers): it can no longer be chosen. That
// does not change which one is: best never drops below the shortest chain
// of all, so no chain of that length is stopped, and the choice among them
// is made as before. Only how much is computed depends on scheduling.
//
// Once done is closed, the workers skip the remaining trials; the caller
// then must not use the result.
func lookahead(s *board.State, observers []Observer, done <-chan struct{}) (refutation, bool) {
	cells := candidates(s)
	hyps := make([]*Hypothesis, len(cells))
	jobs := make(chan int)
	var best atomic.Int32 // steps of the shortest refutation so far
	best.Store(math.MaxInt32)
	// The assumed star is the chain's first step, the trial's trace the rest.
	maxSteps := func() int { return int(best.Load() - 1) }
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for i := range jobs {
				if cancelled(done) {
					continue
				}
				trial := s.Clone()
				delta, err := Apply(trial, Star(cells[i], HypothesisSource))
				steps := []Step{assumedStep(s.P, cells[i], delta)}
				if err == nil {
					if res := runSync(trial, observers, maxSteps); res.Status == Contradiction {
						err = res.Err
						steps = append(steps, res.Trace...)
					}
				}
				if err != nil {
					hyps[i] = &Hypothesis{Steps: steps, Contradiction: err}
					for n := int32(len(steps)); ; {
						cur := best.Load()
						if n >= cur || best.CompareAndSwap(cur, n) {
							break
						}
					}
				}
			}
		})
	}
	for i := range cells {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	pick := -1
	for i, h := range hyps {
		if h != nil && (pick < 0 || simpler(h, hyps[pick])) {
			pick = i // strict comparison: ties keep the earlier candidate
		}
	}
	if pick < 0 {
		return refutation{}, false
	}
	return refutation{cells[pick], hyps[pick]}, true
}

// simpler reports whether chain a is easier to follow than b: fewer steps,
// then fewer changed cells.
func simpler(a, b *Hypothesis) bool {
	if len(a.Steps) != len(b.Steps) {
		return len(a.Steps) < len(b.Steps)
	}
	return a.cells() < b.cells()
}

func (h *Hypothesis) cells() int {
	n := 0
	for _, st := range h.Steps {
		n += len(st.Marks)
	}
	return n
}

// candidates orders the free cells the way a human would try them: cells in the
// tightest unit first (fewest free cells among its row, column and region),
// ties in board order.
func candidates(s *board.State) []puzzle.Pos {
	p := s.P
	var freeIn [3][]int // by puzzle.UnitKind, then unit index
	for k := range freeIn {
		freeIn[k] = make([]int, p.N)
	}
	for _, u := range s.Units() {
		freeIn[u.Kind][u.Index] = s.FreeCount(u.Cells)
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
	slices.SortStableFunc(cells, func(a, b puzzle.Pos) int { return cmp.Compare(tight(a), tight(b)) })
	return cells
}

// branchUnit returns the open unit with the fewest free cells, and those cells.
func branchUnit(s *board.State) (*puzzle.Unit, []puzzle.Pos) {
	var best *puzzle.Unit
	bestFree := 0
	us := s.Units()
	for i := range us {
		if s.HasStar(us[i].Cells) {
			continue
		}
		free := s.FreeCount(us[i].Cells)
		if best == nil || free < bestFree {
			best, bestFree = &us[i], free
		}
	}
	if best == nil {
		return nil, nil
	}
	u := *best // a copy: the units are shared by all clones
	return &u, s.Free(u.Cells)
}
