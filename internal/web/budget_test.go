package web

import (
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
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
	// A cancelled solve is not kept: the next visitor's page is solved.
	close(g.release)
	calls := g.calls.Load()
	if rec := get(h, "/?n=6&seed=12"); rec.Code != 200 || g.calls.Load() != calls+1 {
		t.Errorf("after a cancelled solve: status %d, %d solves, want 200 and 1", rec.Code, g.calls.Load()-calls)
	}
}

// TestResultsKept: a reload, a shared link written another way and every
// visit of the tutorial cost no solve; a failed solve is not kept.
func TestResultsKept(t *testing.T) {
	var calls atomic.Int32
	fail := true
	h := Handler(func(ctx context.Context, p *puzzle.Puzzle) (solver.Result, error) {
		calls.Add(1)
		if fail {
			return solver.Result{}, errors.New("kaputt")
		}
		return testSolve(ctx, p)
	}, nil, nil, nil, nil)
	check := func(path string, wantCode int, wantCalls int32) string {
		t.Helper()
		rec := get(h, path)
		if rec.Code != wantCode || calls.Load() != wantCalls {
			t.Fatalf("%s: status %d after %d solves, want %d after %d", path, rec.Code, calls.Load(), wantCode, wantCalls)
		}
		return rec.Body.String()
	}
	check("/?n=7&seed=5", 500, 1)
	fail = false
	first := check("/?n=7&seed=5", 200, 2)
	if check("/?n=7&seed=5", 200, 2) != first {
		t.Error("the kept page differs from the solved one")
	}
	check("/play?p=AAB/ABB/CCC", 400, 2) // too small: no solve
	text := "BBBCC/BBCCC/ADDDC/DDDEE/DEEEE"
	check("/play?p="+text, 200, 3)
	check("/play?p="+strings.ReplaceAll(text, "/", ","), 200, 3)
	for range 3 {
		check("/anleitung", 200, 4)
	}
}

// TestResultsEvict: the least recently used entry goes first.
func TestResultsEvict(t *testing.T) {
	c := newResults(2)
	a, b, d := &served{}, &served{}, &served{}
	c.put("a", a)
	c.put("b", b)
	c.get("a")
	c.put("d", d)
	if c.get("a") != a || c.get("d") != d || c.get("b") != nil {
		t.Error("want a and d kept, b dropped")
	}
	var none *results
	none.put("a", a)
	if none.get("a") != nil {
		t.Error("a nil cache keeps something")
	}
}
