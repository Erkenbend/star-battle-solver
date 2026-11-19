package party

import (
	"errors"
	"testing"
	"time"
)

func TestPartiesUnknownCode(t *testing.T) {
	pm, _ := newTestParties(t)
	for name, err := range map[string]error{
		"join":     func() error { _, _, err := pm.Join("ZZZZ", "Ada", ""); return err }(),
		"progress": pm.Progress("ZZZZ", "tok", 1, 0, 0, 0),
		"solve":    func() error { _, err := pm.Solve("ZZZZ", "tok", 1, nil, 0, 0); return err }(),
		"giveup":   pm.GiveUp("ZZZZ", "tok", 1, 0, 0),
	} {
		if pe, ok := errors.AsType[*Error](err); !ok || pe.Kind != NotFound {
			t.Errorf("%s: %v, want NotFound", name, err)
		}
	}
}

func TestPartiesLimit(t *testing.T) {
	pm, _ := newTestParties(t)
	for i := range MaxParties {
		if _, err := pm.Create(Options{Sizes: []int{5}, Mode: ""}); err != nil {
			t.Fatalf("party %d: %v", i, err)
		}
	}
	_, err := pm.Create(Options{Sizes: []int{5}, Mode: ""})
	wantKind(t, err, Busy)
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
