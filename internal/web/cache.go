package web

import (
	"container/list"
	"sync"

	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// served is a puzzle as its page shows it: the puzzle, its unique solution
// (nil if there is none) and the solver's result.
type served struct {
	p   *puzzle.Puzzle
	sol []int
	res solver.Result
}

// results keeps the last max served puzzles, so a reload or a shared link
// of a page that was solved recently costs neither the generator nor the
// solver. Keys are "n/seed" for generated puzzles and the puzzle's text for
// pasted ones. Solving is deterministic, so an entry never goes stale; the
// least recently used one goes when there is no room. A nil *results keeps
// nothing.
type results struct {
	max   int
	mu    sync.Mutex
	order *list.List               // of *resultEntry, most recently used first
	byKey map[string]*list.Element // into order
}

type resultEntry struct {
	key string
	v   *served
}

// cachedResults is how many solved pages a Handler keeps: 3.5 MB for 8×8
// puzzles, about 26 MB at most for hard 15×15 ones (decision 33).
const cachedResults = 64

func newResults(max int) *results {
	return &results{max: max, order: list.New(), byKey: map[string]*list.Element{}}
}

// get returns the entry of key, or nil, and marks it used.
func (c *results) get(key string) *served {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el := c.byKey[key]
	if el == nil {
		return nil
	}
	c.order.MoveToFront(el)
	return el.Value.(*resultEntry).v
}

// put stores v under key, dropping the least recently used entry if full.
func (c *results) put(key string, v *served) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el := c.byKey[key]; el != nil {
		el.Value.(*resultEntry).v = v
		c.order.MoveToFront(el)
		return
	}
	c.byKey[key] = c.order.PushFront(&resultEntry{key, v})
	if c.order.Len() > c.max {
		last := c.order.Back()
		c.order.Remove(last)
		delete(c.byKey, last.Value.(*resultEntry).key)
	}
}
