package board

import (
	"slices"
	"strings"
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

func TestTouchingStarsOnly(t *testing.T) {
	// (0,1) in B and (1,0) in D: different row, column and region, so only the
	// touching check can catch them.
	s := New(puzzle.MustParse(p5))
	s.Set(0, 1, Star)
	s.Set(1, 0, Star)
	err := s.Contradiction()
	if err == nil || !strings.Contains(err.Error(), "berühren") {
		t.Fatalf("got %v, want touching stars", err)
	}
}

func TestResetToFree(t *testing.T) {
	s := New(puzzle.MustParse(p5))
	s.Set(0, 0, Cross)
	if _, err := s.Set(0, 0, Free); err == nil {
		t.Fatal("resetting a decided cell to free must fail")
	}
	if s.At(0, 0) != Cross {
		t.Fatal("failed Set changed the cell")
	}
}

func TestParseString(t *testing.T) {
	p := puzzle.MustParse(p5)
	s, err := Parse(p, "M X . . .\nx....\n.....\n..*..\n.....\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "MX...\nX....\n.....\n..M..\n.....\n"
	if got := s.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if got := s.StarCols(); !slices.Equal(got, []int{0, -1, -1, 2, -1}) {
		t.Fatalf("StarCols() = %v", got)
	}
	if again := MustParse(p, s.String()); again.String() != want {
		t.Fatal("String/Parse round trip differs")
	}
	for name, text := range map[string]string{
		"short line":    "....\n.....\n.....\n.....\n.....\n",
		"too many rows": strings.Repeat(".....\n", 6),
		"too few rows":  strings.Repeat(".....\n", 4),
		"bad symbol":    ".....\n..?..\n.....\n.....\n.....\n",
	} {
		if _, err := Parse(p, text); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("MustParse did not panic")
		}
	}()
	MustParse(p, "?")
}
