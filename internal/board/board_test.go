package board

import (
	"testing"

	"starbattle/internal/bruteforce"
	"starbattle/internal/puzzle"
)

// A uniquely solvable 5x5 puzzle (generate -n 5 -seed 3).
const p5 = `
DBBBB
DDBCC
DDBAA
EDDAA
EAAAA
`

func TestBruteforceKnown(t *testing.T) {
	p := puzzle.MustParse(p5)
	sols := bruteforce.Solve(p, 3)
	if len(sols) != 1 {
		t.Fatalf("got %d solutions: %v", len(sols), sols)
	}
	if !FromSolution(p, sols[0]).Solved() {
		t.Fatal("solution does not validate")
	}
}

func TestSetConflict(t *testing.T) {
	s := New(puzzle.MustParse(p5))
	if ch, err := s.Set(0, 0, Cross); !ch || err != nil {
		t.Fatal("first set should change")
	}
	if ch, err := s.Set(0, 0, Cross); ch || err != nil {
		t.Fatal("repeated set should be a no-op")
	}
	if _, err := s.Set(0, 0, Star); err == nil {
		t.Fatal("expected conflict")
	}
}

func TestContradiction(t *testing.T) {
	p := puzzle.MustParse(p5)
	s := New(p)
	s.Set(0, 0, Star)
	s.Set(1, 1, Star)
	if s.Contradiction() == nil {
		t.Fatal("touching stars not detected")
	}
	s = New(p)
	for c := range p.N {
		s.Set(2, c, Cross)
	}
	if s.Contradiction() == nil {
		t.Fatal("empty row not detected")
	}
}

func TestImplied(t *testing.T) {
	p := puzzle.MustParse(p5)
	// Star at (0,0): row 0 (4), column 0 (4), region D adds (1,1),(2,1),(3,1),(3,2).
	got := map[puzzle.Pos]bool{}
	for _, q := range Implied(p, 0, 0) {
		got[q] = true
	}
	for _, q := range []puzzle.Pos{{Row: 0, Col: 4}, {Row: 4, Col: 0}, {Row: 1, Col: 1}, {Row: 3, Col: 2}} {
		if !got[q] {
			t.Errorf("missing %v", q)
		}
	}
	if got[puzzle.Pos{Row: 0, Col: 0}] || len(got) != 12 {
		t.Errorf("got %d cells: %v", len(got), got)
	}
}
