package solver

import (
	"fmt"
	"runtime"
	"slices"
	"sync"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
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
			if r, ok := lookahead(s, observers); ok {
				var err error
				if trace, err = refute(s, trace, r.pos, r.hyp); err != nil {
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
		var hyp *Hypothesis
		if sub.Stats.Guesses == 0 { // short chain of rules only: worth showing
			hyp = &Hypothesis{Steps: append([]Step{assumedStep(s.P, pos, delta)}, sub.Trace...), Contradiction: sub.Err}
		}
		if trace, err = refute(s, trace, pos, hyp); err != nil {
			return done(finish(s, nil, err))
		}
	}
}

type refutation struct {
	pos puzzle.Pos
	hyp *Hypothesis
}

// refute crosses pos because a star there was shown impossible, and records the
// step with its reasoning chain (hyp may be nil for deep backtracking).
func refute(s *board.State, trace []Step, pos puzzle.Pos, hyp *Hypothesis) ([]Step, error) {
	why := fmt.Sprintf("Annahme: Stern auf (%d,%d) führt zu einem Widerspruch → X", pos.Row, pos.Col)
	if hyp == nil {
		why = fmt.Sprintf("Annahme: Stern auf (%d,%d) führt nach weiterem Raten immer zu einem Widerspruch → X", pos.Row, pos.Col)
	}
	m := Cross(pos, HypothesisSource).Why(why)
	delta, err := Apply(s, m)
	if len(delta) > 0 {
		trace = append(trace, Step{Source: m.Source, Reason: why, Marks: delta, Hypothesis: hyp})
	}
	return trace, err
}

// assumedStep is the first step of a hypothesis chain: the star and its crosses.
func assumedStep(p *puzzle.Puzzle, pos puzzle.Pos, delta []Mark) Step {
	why := fmt.Sprintf("Angenommen, auf (%d,%d) ist ein Stern → X in Zeile %d, Spalte %d, Farbe %c und Nachbarschaft",
		pos.Row, pos.Col, pos.Row, pos.Col, puzzle.RegionLetter(p.Region(pos.Row, pos.Col)))
	for i := range delta {
		delta[i].Reason = why
	}
	return Step{Source: HypothesisSource, Reason: why, Marks: delta}
}

// lookahead tries a star on every free cell in parallel and returns the
// refutation that is easiest to follow: the shortest chain of steps to a
// contradiction (see simpler). We optimize for understandability, not speed.
//
// Each trial runs with RunSync: the trials already run in parallel, and the
// concurrent runner's step order (hence chain length) varies between runs,
// which would make the chosen explanation non-deterministic.
func lookahead(s *board.State, observers []Observer) (refutation, bool) {
	cells := candidates(s)
	hyps := make([]*Hypothesis, len(cells))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				trial := s.Clone()
				delta, err := Apply(trial, Star(cells[i], HypothesisSource))
				steps := []Step{assumedStep(s.P, cells[i], delta)}
				if err == nil {
					if res := RunSync(trial, observers); res.Status == Contradiction {
						err = res.Err
						steps = append(steps, res.Trace...)
					}
				}
				if err != nil {
					hyps[i] = &Hypothesis{Steps: steps, Contradiction: err}
				}
			}
		}()
	}
	for i := range cells {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	best := -1
	for i, h := range hyps {
		if h != nil && (best < 0 || simpler(h, hyps[best])) {
			best = i // strict comparison: ties keep the earlier candidate
		}
	}
	if best < 0 {
		return refutation{}, false
	}
	return refutation{cells[best], hyps[best]}, true
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
