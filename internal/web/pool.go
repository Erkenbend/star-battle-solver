package web

import (
	"log"
	"sync"
	"time"

	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// Pool keeps one generated and solved puzzle ready per size, so that "new
// puzzle" does not wait for the generator (seconds for 14×14, more on a
// Raspberry Pi). A single background goroutine fills the sizes one at a time,
// smallest first, and refills a size as soon as its puzzle is handed out.
//
// A pooled puzzle is exactly the one its seed generates, so its URL stays
// valid after it has left the pool. All methods may be called on a nil *Pool,
// which never has a puzzle ready.
type Pool struct {
	solve  SolveFunc
	queue  chan int // sizes to (re)fill; each size is queued at most once
	mu     sync.Mutex
	ready  map[int]*pooled // waiting to be handed out
	handed map[int]*pooled // last one handed out per size, until its page is requested
}

type pooled struct {
	seed uint64
	p    *puzzle.Puzzle
	sol  []int
	res  solver.Result
}

// NewPool starts filling the pool for the given sizes in the background.
func NewPool(solve SolveFunc, sizes []int) *Pool {
	pl := &Pool{
		solve:  solve,
		queue:  make(chan int, len(sizes)),
		ready:  map[int]*pooled{},
		handed: map[int]*pooled{},
	}
	for _, n := range sizes {
		pl.queue <- n
	}
	go pl.fill()
	return pl
}

func (pl *Pool) fill() {
	for n := range pl.queue {
		for try := 0; ; try++ {
			start := time.Now()
			e, err := pl.make(n)
			if err == nil {
				log.Printf("%d×%d bereit (Nr. %d, %.1f s)", n, n, e.seed, time.Since(start).Seconds())
				pl.mu.Lock()
				pl.ready[n] = e
				pl.mu.Unlock()
				break
			}
			log.Printf("%d×%d: %v", n, n, err)
			if try == 2 {
				break // size stays empty; the handler generates on demand
			}
		}
	}
}

func (pl *Pool) make(n int) (*pooled, error) {
	seed := newSeed()
	p, sol, err := generate(n, seed)
	if err != nil {
		return nil, err
	}
	res, err := pl.solve(p)
	if err != nil {
		return nil, err
	}
	return &pooled{seed, p, sol, res}, nil
}

// Take hands out the seed of the ready n×n puzzle, if there is one, and
// queues the size for refilling. The puzzle stays available via Lookup.
func (pl *Pool) Take(n int) (uint64, bool) {
	if pl == nil {
		return 0, false
	}
	pl.mu.Lock()
	defer pl.mu.Unlock()
	e := pl.ready[n]
	if e == nil {
		return 0, false
	}
	delete(pl.ready, n)
	pl.handed[n] = e
	pl.queue <- n // cannot block: n was not queued while it was ready
	return e.seed, true
}

// Lookup returns the handed-out n×n puzzle with this seed, or nil.
func (pl *Pool) Lookup(n int, seed uint64) *pooled {
	if pl == nil {
		return nil
	}
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if e := pl.handed[n]; e != nil && e.seed == seed {
		return e
	}
	return nil
}
