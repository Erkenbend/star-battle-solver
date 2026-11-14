package generator

import (
	"math/rand/v2"
	"testing"

	"starbattle/internal/puzzle"
)

func TestGenerateSmallAndLarge(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 1))
	p, sol, err := Generate(1, rng, Options{})
	if err != nil || p.N != 1 || len(sol) != 1 || sol[0] != 0 {
		t.Fatalf("n=1: got %v, %v, %v", p, sol, err)
	}
	for _, n := range []int{2, 3, puzzle.MaxN + 1} {
		if _, _, err := Generate(n, rng, Options{}); err == nil {
			t.Errorf("n=%d: expected error", n)
		}
	}
}

func TestPatternErrorMessages(t *testing.T) {
	for _, tt := range []struct {
		err  PatternError
		want string
	}{
		{PatternError{Problem: Unreachable, Region: 1}, "region B: parts cannot be connected"},
		{PatternError{Problem: TooSmall, Region: 2}, "region C: enclosed and too small"},
		{PatternError{Problem: Ambiguous, Region: -1}, "two star placements lie on drawn cells alone"},
		{PatternError{Problem: Fixed, Region: -1}, "the pattern fixes the puzzle, which has several solutions"},
		{PatternError{Problem: Leftover, Region: -1, Areas: 3, Missing: 1}, "3 areas of free cells, but only 1 regions left"},
		{PatternError{Problem: NoStars, Region: -1}, "no star placement fits the pattern"},
	} {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("problem %d: got %q, want %q", tt.err.Problem, got, tt.want)
		}
	}
}

func TestCheckPatternShape(t *testing.T) {
	for name, pattern := range map[string][][]int{
		"too small":      parsePattern("...\n...\n..."),
		"ragged":         parsePattern("....\n...\n....\n...."),
		"region too big": parsePattern("....\n.E..\n....\n...."),
		"below free":     {{Free - 1, Free, Free, Free}, {Free, Free, Free, Free}, {Free, Free, Free, Free}, {Free, Free, Free, Free}},
	} {
		err := CheckPattern(pattern, 1, false)
		if err == nil {
			t.Errorf("%s: expected error", name)
			continue
		}
		if _, ok := err.(*PatternError); ok {
			t.Errorf("%s: got a *PatternError (%v), want a plain error", name, err)
		}
	}
}
