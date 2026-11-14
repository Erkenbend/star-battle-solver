package web

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

func TestPartiesUnknownCode(t *testing.T) {
	pm, _ := newTestParties(t)
	for name, err := range map[string]error{
		"join":     func() error { _, _, err := pm.Join("ZZZZ", "Ada", ""); return err }(),
		"progress": pm.Progress("ZZZZ", "tok", 1, 0, 0),
		"solve":    func() error { _, err := pm.Solve("ZZZZ", "tok", 1, nil, 0); return err }(),
		"giveup":   pm.GiveUp("ZZZZ", "tok", 1, 0),
	} {
		if got := errorStatus(err); got != http.StatusNotFound {
			t.Errorf("%s: status %d (%v), want 404", name, got, err)
		}
	}
	if got := errorStatus(errors.New("other")); got != http.StatusInternalServerError {
		t.Errorf("plain error: status %d, want 500", got)
	}
}

func TestPartiesLimit(t *testing.T) {
	pm, _ := newTestParties(t)
	for i := range PartyMaxParties {
		if _, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: ""}); err != nil {
			t.Fatalf("party %d: %v", i, err)
		}
	}
	_, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: ""})
	if got := errorStatus(err); got != http.StatusServiceUnavailable {
		t.Fatalf("one too many: status %d (%v), want 503", got, err)
	}
}

// The default prepare generates and solves a puzzle when the pool has none.
func TestPartiesPrepare(t *testing.T) {
	rp, err := NewParties(testSolve, nil).prepare(5)
	if err != nil || rp.P.N != 5 || len(rp.Solution) != 5 || rp.Res.Status != solver.Solved {
		t.Fatalf("got %+v, %v", rp, err)
	}
	failing := func(*puzzle.Puzzle) (solver.Result, error) { return solver.Result{}, errors.New("kaputt") }
	if _, err := NewParties(failing, nil).prepare(5); err == nil || err.Error() != "kaputt" {
		t.Fatalf("solve error: got %v", err)
	}
}

func TestPartiesRun(t *testing.T) {
	pm, _ := newTestParties(t)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		pm.Run(time.Millisecond, stop)
		close(done)
	}()
	time.Sleep(5 * time.Millisecond) // a few ticks
	close(stop)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after stop")
	}
}
