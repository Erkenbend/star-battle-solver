package solver

import (
	"fmt"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/words"
)

var p5 = puzzle.MustParse(`
DBBBB
DDBCC
DDBAA
EDDAA
EAAAA
`)

func TestApplyStarImpliesCrosses(t *testing.T) {
	s := board.New(p5)
	delta, err := Apply(s, Star(puzzle.Pos{Row: 0, Col: 0}, "test"))
	if err != nil {
		t.Fatal(err)
	}
	if len(delta) != 1+12 {
		t.Fatalf("got %d changed cells, want 13", len(delta))
	}
	if again, _ := Apply(s, Star(puzzle.Pos{Row: 0, Col: 0}, "test")); len(again) != 0 {
		t.Fatal("duplicate mark should change nothing")
	}
	if _, err := Apply(s, Star(puzzle.Pos{Row: 0, Col: 1}, "test")); err == nil {
		t.Fatal("expected conflict")
	}
}

// scripted is a fake observer that emits a fixed list of marks once.
type scripted struct {
	marks []Mark
	fired *bool
}

func (o scripted) Name() string               { return "scripted" }
func (o scripted) Done(*board.State) bool     { return *o.fired }
func (o scripted) Deduce(*board.State) []Mark { *o.fired = true; return o.marks }

func TestRunSyncSolvesWithScriptedStars(t *testing.T) {
	// Solution of p5 found by brute force: columns per row.
	sol := []int{2, 4, 1, 3, 0}
	var marks []Mark
	for r, c := range sol {
		marks = append(marks, Star(puzzle.Pos{Row: r, Col: c}, ""))
	}
	fired := false
	res := RunSync(board.New(p5), []Observer{scripted{marks, &fired}})
	if res.Status != Solved {
		t.Fatalf("status %v (%v)\n%s", res.Status, res.Err, res.State)
	}
}

func TestRunSyncDetectsContradiction(t *testing.T) {
	fired := false
	bad := []Mark{Star(puzzle.Pos{Row: 0, Col: 0}, ""), Star(puzzle.Pos{Row: 1, Col: 1}, "")}
	res := RunSync(board.New(p5), []Observer{scripted{bad, &fired}})
	if res.Status != Contradiction {
		t.Fatalf("status %v, want contradiction", res.Status)
	}
}

// rowCrosser is a fake observer that crosses the first free cell of row 0,
// one step per call: after 5 steps p5's row 0 has no room for its star.
type rowCrosser struct{}

func (rowCrosser) Name() string           { return "rowCrosser" }
func (rowCrosser) Done(*board.State) bool { return false }
func (rowCrosser) Deduce(s *board.State) []Mark {
	for c := range s.P.N {
		if s.At(0, c) == board.Free {
			text := fmt.Sprintf("cross %d", c) // a reason of its own: a step of its own
			return []Mark{Cross(puzzle.Pos{Row: 0, Col: c}, "").Why(func(*words.Namer) string { return text })}
		}
	}
	return nil
}

// TestRunSyncStepLimit: a run within the limit is the same as RunSync's, a
// run longer than the limit is given up as soon as it exceeds it.
func TestRunSyncStepLimit(t *testing.T) {
	obs := []Observer{rowCrosser{}}
	full := RunSync(board.New(p5), obs)
	if full.Status != Contradiction || len(full.Trace) != 5 {
		t.Fatalf("RunSync: status %v with %d steps, want contradiction with 5", full.Status, len(full.Trace))
	}
	limit := func(n int) func() int { return func() int { return n } }
	if res := runSync(board.New(p5), obs, limit(5)); res.Status != Contradiction || len(res.Trace) != 5 || res.Err.Error() != full.Err.Error() {
		t.Errorf("limit 5: status %v with %d steps (%v), want RunSync's result", res.Status, len(res.Trace), res.Err)
	}
	for _, n := range []int{4, 2, 0} {
		if res := runSync(board.New(p5), obs, limit(n)); res.Status != Stuck || len(res.Trace) != n+1 {
			t.Errorf("limit %d: status %v with %d steps, want stuck after %d", n, res.Status, len(res.Trace), n+1)
		}
	}
}
