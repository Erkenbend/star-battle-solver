package solver

import (
	"cmp"
	"slices"
)

// Prioritized is implemented by observers that declare how simple their rule
// is. Lower levels are simpler and take precedence: both runners only apply a
// deduction of level L when no observer of a lower level can deduce anything
// on the current board. Observers without a Priority method have level 0.
type Prioritized interface {
	Priority() int
}

// Levels of the search steps in Solve, above every rule.
const (
	LevelLookahead = 100 // a cross proven by refuting a star with the rules alone
	LevelGuess     = 101 // a guessed star, or a cross proven by a failed guess
)

// PriorityOf returns o's level (0 if o does not declare one).
func PriorityOf(o Observer) int {
	if p, ok := o.(Prioritized); ok {
		return p.Priority()
	}
	return 0
}

// byPriority returns a copy of observers sorted by level. The sort is stable, so
// within a level the given order is kept as a tie-break.
func byPriority(observers []Observer) []Observer {
	sorted := slices.Clone(observers)
	slices.SortStableFunc(sorted, func(a, b Observer) int {
		return cmp.Compare(PriorityOf(a), PriorityOf(b))
	})
	return sorted
}
