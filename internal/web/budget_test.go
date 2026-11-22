package web

import (
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// gate is a SolveFunc that blocks until released (or its ctx ends) and
// counts how many calls run at once.
type gate struct {
	release        chan struct{}
	started        chan struct{} // one value per call that got to run
	running, peak  atomic.Int32
	calls, aborted atomic.Int32
}

func newGate() *gate {
	return &gate{release: make(chan struct{}), started: make(chan struct{}, 100)}
}

func (g *gate) solve(ctx context.Context, p *puzzle.Puzzle) (solver.Result, error) {
	g.calls.Add(1)
	n := g.running.Add(1)
	defer g.running.Add(-1)
	for {
		if old := g.peak.Load(); n <= old || g.peak.CompareAndSwap(old, n) {
			break
		}
	}
	g.started <- struct{}{}
	select {
	case <-g.release:
		return testSolve(context.Background(), p)
	case <-ctx.Done():
		g.aborted.Add(1)
		return solver.Result{}, ctx.Err()
	}
}

func waitStarted(t *testing.T, g *gate, k int) {
	t.Helper()
	for range k {
		select {
		case <-g.started:
		case <-time.After(10 * time.Second):
			t.Fatal("solve did not start")
		}
	}
}

// TestBudget: at most n solves run at once; the others wait, and one whose
// caller is gone gives up waiting without solving.
func TestBudget(t *testing.T) {
	g := newGate()
	solve := NewBudget(2).Limit(g.solve)
	p := puzzle.MustParse(tutorialPuzzle)

	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := range 4 {
		wg.Go(func() { _, errs[i] = solve(context.Background(), p) })
	}
	waitStarted(t, g, 2)
	ctx, cancel := context.WithCancel(context.Background())
	fifth := make(chan struct{})
	go func() { _, errs[4] = solve(ctx, p); close(fifth) }()
	time.Sleep(20 * time.Millisecond) // the fifth waits for a slot now, behind two others
	cancel()
	select {
	case <-fifth:
	case <-time.After(10 * time.Second):
		t.Fatal("a cancelled call still waits for a slot")
	}
	close(g.release)
	wg.Wait()

	if g.peak.Load() != 2 {
		t.Errorf("at most %d solves at once, want 2", g.peak.Load())
	}
	if g.calls.Load() != 4 {
		t.Errorf("%d solves ran, want 4: the cancelled one must not run", g.calls.Load())
	}
	for i, err := range errs[:4] {
		if err != nil {
			t.Errorf("solve %d: %v", i, err)
		}
	}
	if !errors.Is(errs[4], context.Canceled) {
		t.Errorf("cancelled while waiting: %v, want context.Canceled", errs[4])
	}
}

// TestBudgetBackground: solves nobody waits for leave a slot free, so a
// page's solve runs at once while a crawler has queued many.
func TestBudgetBackground(t *testing.T) {
	g := newGate()
	b := NewBudget(3)
	bg, fg := b.Background(g.solve), b.Limit(g.solve)
	p := puzzle.MustParse(tutorialPuzzle)
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() { bg(context.Background(), p) })
	}
	waitStarted(t, g, 2)
	time.Sleep(20 * time.Millisecond)
	if n := g.running.Load(); n != 2 {
		t.Fatalf("%d background solves run, want 2 of 3 slots", n)
	}
	page := make(chan error)
	go func() { _, err := fg(context.Background(), p); page <- err }()
	waitStarted(t, g, 1) // at once, beside the two
	if n := g.running.Load(); n != 3 {
		t.Fatalf("%d solves run with the page's, want 3", n)
	}
	close(g.release)
	if err := <-page; err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if g.peak.Load() != 3 || g.calls.Load() != 7 {
		t.Errorf("peak %d, calls %d; want 3 and 7", g.peak.Load(), g.calls.Load())
	}
}

// TestPageSolveCancelled: a visitor who leaves while the page is solved
// cancels the solve, and gets no page.
func TestPageSolveCancelled(t *testing.T) {
	g := newGate()
	h := Handler(NewBudget(1).Limit(g.solve), nil, nil, nil, nil)
	for _, path := range []string{
		"/play?" + url.Values{"p": {tutorialPuzzle}}.Encode(),
		"/?n=6&seed=12",
		"/anleitung",
	} {
		ctx, cancel := context.WithCancel(context.Background())
		rec := httptest.NewRecorder()
		done := make(chan struct{})
		go func() {
			h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil).WithContext(ctx))
			close(done)
		}()
		waitStarted(t, g, 1)
		aborted := g.aborted.Load()
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: handler still busy after its request was cancelled", path)
		}
		if g.aborted.Load() != aborted+1 {
			t.Errorf("%s: the solve was not cancelled", path)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("%s: wrote %d bytes for a request that is gone", path, rec.Body.Len())
		}
	}
}
