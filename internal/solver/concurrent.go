package solver

import (
	"sync"

	"starbattle/internal/board"
)

// RunConcurrent runs every observer in its own goroutine.
//
// Data flow:
//
//		observer goroutines --(msg: marks / idle / done)--> runner --(delta batches)--> mailboxes
//
//	  - The runner owns the authoritative state. It applies incoming marks via Apply
//	    (which adds implied crosses), bumps the version and broadcasts the resulting
//	    delta to every active observer.
//	  - Each observer owns a private copy of the state that it updates from its
//	    mailbox. Because cells only move from Free to Star/Cross, applying the deltas
//	    in version order makes the copy equal to the runner's state at that version.
//	  - Mailboxes are unbounded (mutex + slice + 1-slot notify channel), so the runner
//	    never blocks while broadcasting. Observers block on sending to the runner only
//	    until the runner reads, and the runner always reads. This rules out the
//	    classic "both sides blocked on send" deadlock.
//
// Termination (quiescence): after draining its mailbox and finding nothing new, an
// observer reports Idle together with the version it has seen. The run is stuck
// once every active observer is idle at the runner's current version. Messages
// from one goroutine arrive in order, so any marks it found earlier have
// already been processed by then.
//
// start is not modified. The result is the same fixpoint RunSync reaches as long
// as the rules are monotone (more information never prevents a deduction).
func RunConcurrent(start *board.State, observers []Observer) Result {
	s := start.Clone()
	if err := s.Contradiction(); err != nil || s.Solved() {
		return finish(s, nil, err)
	}

	quit := make(chan struct{})
	out := make(chan msg, len(observers))
	boxes := make([]*mailbox, len(observers))
	var wg sync.WaitGroup
	for i, o := range observers {
		boxes[i] = newMailbox()
		local := s.Clone() // must happen here: once the runner loop starts, s changes
		wg.Add(1)
		go func() {
			defer wg.Done()
			runObserver(i, o, local, boxes[i], out, quit)
		}()
	}
	// Stop all observers before returning so no goroutine outlives the call.
	defer func() {
		close(quit)
		wg.Wait()
	}()

	const busy = -1
	version := 0
	idleAt := make(map[int]int, len(observers)) // active observer → version it is idle at
	for i := range observers {
		idleAt[i] = busy
	}
	var trace []Step

	for len(idleAt) > 0 {
		m := <-out
		if _, active := idleAt[m.id]; !active {
			continue
		}
		switch m.kind {
		case msgDone:
			delete(idleAt, m.id)
		case msgIdle:
			idleAt[m.id] = m.version
		case msgMarks:
			idleAt[m.id] = busy
			var all []Mark // every cell changed by this message
			var err error
			for _, mk := range m.marks {
				var delta []Mark
				delta, err = Apply(s, mk)
				trace = record(trace, mk, delta)
				all = append(all, delta...)
				if err != nil {
					break
				}
			}
			if err != nil {
				return finish(s, trace, err)
			}
			if len(all) == 0 {
				// All duplicates: the sender's copy is behind and a broadcast is
				// already waiting in its mailbox, so it will wake up again.
				continue
			}
			if err := s.Contradiction(); err != nil {
				return finish(s, trace, err)
			}
			if s.Solved() {
				return finish(s, trace, nil)
			}
			version++
			b := batch{version: version, marks: all}
			for id := range idleAt {
				boxes[id].put(b)
			}
		}
		if quiescent(idleAt, version) {
			break
		}
	}
	return finish(s, trace, nil)
}

func quiescent(idleAt map[int]int, version int) bool {
	for _, v := range idleAt {
		if v != version {
			return false
		}
	}
	return true
}

// runObserver is the goroutine wrapper around one observer's pure Deduce.
func runObserver(id int, o Observer, local *board.State, box *mailbox, out chan<- msg, quit <-chan struct{}) {
	send := func(m msg) bool {
		m.id = id
		select {
		case out <- m:
			return true
		case <-quit:
			return false
		}
	}
	seen := 0
	for {
		// 1. Catch up with everything published so far.
		for _, b := range box.take() {
			for _, mk := range b.marks {
				// Cannot fail: deltas come from the runner's consistent state, in order.
				local.Set(mk.Pos.Row, mk.Pos.Col, mk.Kind)
			}
			seen = b.version
		}
		// 2. Unit full? Unsubscribe and exit.
		if o.Done(local) {
			send(msg{kind: msgDone})
			return
		}
		// 3. Deduce; report new marks, or that we are idle at this version.
		var fresh []Mark
		for _, mk := range o.Deduce(local) {
			if local.At(mk.Pos.Row, mk.Pos.Col) != mk.Kind {
				if mk.Source == "" {
					mk.Source = o.Name()
				}
				fresh = append(fresh, mk)
			}
		}
		m := msg{kind: msgIdle, version: seen}
		if len(fresh) > 0 {
			m = msg{kind: msgMarks, version: seen, marks: fresh}
		}
		if !send(m) {
			return
		}
		// 4. Wait for the next broadcast (our own marks come back this way too).
		select {
		case <-box.notify:
		case <-quit:
			return
		}
	}
}

type msgKind uint8

const (
	msgMarks msgKind = iota
	msgIdle
	msgDone
)

type msg struct {
	id      int
	kind    msgKind
	version int // version of the observer's local state when it sent this
	marks   []Mark
}

type batch struct {
	version int
	marks   []Mark
}

// mailbox is an unbounded FIFO: put never blocks, notify holds at most one
// wake-up signal, take returns everything queued so far.
type mailbox struct {
	mu     sync.Mutex
	queue  []batch
	notify chan struct{}
}

func newMailbox() *mailbox { return &mailbox{notify: make(chan struct{}, 1)} }

func (b *mailbox) put(x batch) {
	b.mu.Lock()
	b.queue = append(b.queue, x)
	b.mu.Unlock()
	select {
	case b.notify <- struct{}{}:
	default: // a wake-up is already pending
	}
}

func (b *mailbox) take() []batch {
	b.mu.Lock()
	defer b.mu.Unlock()
	q := b.queue
	b.queue = nil
	return q
}
