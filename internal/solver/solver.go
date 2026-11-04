// Package solver contains the observer framework: the Mark and Observer types,
// the shared Apply logic and the runners that drive observers to a fixpoint.
//
// Observers hold the deduction logic as pure functions (state in, marks out).
// Runners decide how they are scheduled: RunSync calls them in a loop, a later
// concurrent runner will give each its own goroutine and channels. Both share
// Apply, so they must reach the same fixpoint.
package solver

import (
	"fmt"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
)

// Mark is one deduction: put a star (M) or a cross (X) on a cell.
type Mark struct {
	Pos    puzzle.Pos
	Kind   board.Cell // board.Star or board.Cross
	Source string     // name of the observer that produced it
	Reason string     // human-readable explanation (German), shown by solve -verbose
}

// Why returns m with its explanation set.
func (m Mark) Why(reason string) Mark {
	m.Reason = reason
	return m
}

func (m Mark) String() string {
	return fmt.Sprintf("%v@(%d,%d) by %s", m.Kind, m.Pos.Row, m.Pos.Col, m.Source)
}

// Star and Cross are convenience constructors for observers.
func Star(p puzzle.Pos, source string) Mark  { return Mark{Pos: p, Kind: board.Star, Source: source} }
func Cross(p puzzle.Pos, source string) Mark { return Mark{Pos: p, Kind: board.Cross, Source: source} }

// Observer watches one aspect of the board (usually one rule applied to one unit).
//
// Deduce must be a pure function of s: it must not modify s and should only return
// marks that follow logically from s. Returning marks that are already on the board
// is allowed (they are ignored), so observers need not filter carefully.
//
// Done reports that the observer can never deduce anything again (e.g. its unit
// already holds a star); runners then stop calling it.
type Observer interface {
	Name() string
	Deduce(s *board.State) []Mark
	Done(s *board.State) bool
}

// Apply puts m on s. For a star it also adds the implied crosses (rest of row,
// column, region and the 8 neighbors), with the star's Source and Reason. It returns
// the marks that actually changed s (possibly none, for a duplicate).
// An error means m conflicts with s; s may then be partially updated, which is
// fine because the state is contradictory anyway.
func Apply(s *board.State, m Mark) ([]Mark, error) {
	changed, err := s.Set(m.Pos.Row, m.Pos.Col, m.Kind)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", m, err)
	}
	if !changed {
		return nil, nil
	}
	delta := []Mark{m}
	if m.Kind == board.Star {
		for _, q := range board.Implied(s.P, m.Pos.Row, m.Pos.Col) {
			ch, err := s.Set(q.Row, q.Col, board.Cross)
			if err != nil {
				return delta, fmt.Errorf("%v: %w", m, err)
			}
			if ch {
				delta = append(delta, Cross(q, m.Source).Why(m.Reason))
			}
		}
	}
	return delta, nil
}

// Status is the outcome of a run.
type Status uint8

const (
	Stuck         Status = iota // fixpoint reached, puzzle not solved: needs stronger rules or guessing
	Solved                      // every unit has its star
	Contradiction               // the observers derived an impossible state (bug in a rule, or a wrong guess)
)

func (s Status) String() string {
	return [...]string{"stuck", "solved", "contradiction"}[s]
}

// Step is one deduction: the marks that share a Source and Reason, including
// the crosses implied by its stars.
type Step struct {
	Source string
	Reason string
	Marks  []Mark
}

// record adds the cells changed by applying m to the trace, extending the last
// step if it is the same deduction.
func record(trace []Step, m Mark, delta []Mark) []Step {
	if len(delta) == 0 {
		return trace
	}
	if n := len(trace); n > 0 && trace[n-1].Source == m.Source && trace[n-1].Reason == m.Reason {
		trace[n-1].Marks = append(trace[n-1].Marks, delta...)
		return trace
	}
	return append(trace, Step{Source: m.Source, Reason: m.Reason, Marks: delta})
}

// Result of a run.
type Result struct {
	State  *board.State
	Status Status
	Err    error  // reason for Contradiction
	Trace  []Step // in order of application; useful for step-by-step rendering
}

// finish classifies the final state.
func finish(s *board.State, trace []Step, err error) Result {
	if err == nil {
		err = s.Contradiction()
	}
	switch {
	case err != nil:
		return Result{State: s, Status: Contradiction, Err: err, Trace: trace}
	case s.Solved():
		return Result{State: s, Status: Solved, Trace: trace}
	default:
		return Result{State: s, Status: Stuck, Trace: trace}
	}
}
