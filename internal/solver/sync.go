package solver

import (
	"math"

	"starbattle/internal/board"
)

// RunSync is the simple reference runner. It sorts the observers by priority
// level (stable, so the given order breaks ties), calls them in that order, and
// as soon as one changes the board it starts again from the first. Simple rules
// therefore always get the first chance, and expensive or harder-to-explain
// rules only fire when nothing simpler applies. It stops when a full pass
// changes nothing, the puzzle is solved, or a contradiction shows up.
// start is not modified.
func RunSync(start *board.State, observers []Observer) Result {
	return runSync(start, observers, nil)
}

// noLimit is a step limit runSync never reaches.
func noLimit() int { return math.MaxInt }

// runSync is RunSync that gives up as soon as its trace holds more than
// maxSteps() steps (nil: no limit), for a lookahead trial that can no longer
// be chosen. A run given up ends Stuck, with the state and trace it had;
// until then it does exactly what RunSync does, so a run that ends within
// the limit is the same with or without it. maxSteps is called after every
// new step and may lower the limit while the run goes on.
func runSync(start *board.State, observers []Observer, maxSteps func() int) Result {
	if maxSteps == nil {
		maxSteps = noLimit
	}
	s := start.Clone()
	active := byPriority(observers)
	var trace []Step

	for progress := true; progress; {
		progress = false
		next := active[:0]
		for i, o := range active {
			if o.Done(s) {
				continue // unsubscribe
			}
			next = append(next, o)

			changed := false
			for _, m := range o.Deduce(s) {
				if m.Source == "" {
					m.Source = o.Name()
				}
				m.Level = PriorityOf(o)
				delta, err := Apply(s, m)
				n := len(trace)
				trace = record(trace, m, delta)
				changed = changed || len(delta) > 0
				if err != nil {
					return finish(s, trace, err)
				}
				if len(trace) > n && len(trace) > maxSteps() {
					return Result{State: s, Status: Stuck, Trace: trace}
				}
			}
			if changed {
				if err := s.Contradiction(); err != nil {
					return finish(s, trace, err)
				}
				if s.Solved() {
					return finish(s, trace, nil)
				}
				// Keep the observers not yet visited in this pass, then restart.
				next = append(next, active[i+1:]...)
				progress = true
				break
			}
		}
		active = next
	}
	return finish(s, trace, nil)
}
