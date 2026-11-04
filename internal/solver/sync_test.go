package solver

import (
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
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
