package hardsearch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"starbattle"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/solver/observers"
)

// TestReproducesLibrary: the seed in a library file name gives that puzzle
// again, with the verdict its header states ("5 lookahead steps" etc.).
func TestReproducesLibrary(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("../..", starbattle.HardDir, "n10_s453.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := puzzle.MustParse(string(text))
	p, err := Generate(10, 453, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.String() != want.String() {
		t.Fatalf("generate -n 10 -seed 453 is no longer n10_s453.txt:\n%s", p)
	}
	v, err := Classify(p)
	if err != nil {
		t.Fatal(err)
	}
	if v.Base != solver.Stuck || v.All != solver.Stuck || v.Lookahead != 4 || v.Guesses != 0 {
		t.Errorf("verdict %v", v)
	}
	if got, want := v.Rating.String(), "2 (4 lookahead steps, longest 5)"; got != want {
		t.Errorf("rating %q, want %q", got, want)
	}
	if !v.Hard(0) || !v.Hard(4) || v.Hard(5) {
		t.Errorf("Hard: %v %v %v", v.Hard(0), v.Hard(4), v.Hard(5))
	}
}

func TestSearch(t *testing.T) {
	var got []Found
	next := Search(context.Background(), Options{N: 10, From: 405, Count: 4, Workers: 3}, func(f Found) { got = append(got, f) })
	if next != 409 || len(got) != 4 {
		t.Fatalf("next %d, %d results", next, len(got))
	}
	for i, f := range got {
		if f.Seed != 405+uint64(i) || f.Err != nil || f.Puzzle == nil {
			t.Fatalf("result %d: seed %d, err %v", i, f.Seed, f.Err)
		}
	}
	// Seed 407 is one of the puzzles only the base rules get stuck on.
	if v := got[2].Verdict; v.Base != solver.Stuck || v.All != solver.Solved || !v.Hard(0) || v.Hard(1) {
		t.Errorf("seed 407: %v", v)
	}
	if v := got[0].Verdict; v.Base != solver.Solved || v.Hard(0) {
		t.Errorf("seed 405: %v", v)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n := 0
	if next := Search(ctx, Options{N: 10, From: 1, Count: 100}, func(Found) { n++ }); next != 1 || n != 0 {
		t.Errorf("cancelled search: next %d, %d results", next, n)
	}
}

func TestRate(t *testing.T) {
	chain := func(k int) *solver.Hypothesis { return &solver.Hypothesis{Steps: make([]solver.Step, k)} }
	la := func(k int) solver.Step { return solver.Step{Level: solver.LevelLookahead, Hypothesis: chain(k)} }
	rule := func(level int) solver.Step { return solver.Step{Level: level} }
	for _, c := range []struct {
		res  solver.Result
		want string
	}{
		{solver.Result{Trace: []solver.Step{rule(1), rule(observers.LevelDeadCells)}}, "1 (rules alone)"},
		{solver.Result{Trace: []solver.Step{rule(observers.LevelPigeonhole)}}, "2 (rules alone, with Pigeonhole)"},
		// Rules inside a thought experiment count too.
		{solver.Result{Trace: []solver.Step{{Level: solver.LevelLookahead, Hypothesis: &solver.Hypothesis{Steps: []solver.Step{rule(observers.LevelPigeonhole)}}}}, Stats: solver.Stats{Lookahead: 1}}, "2 (1 lookahead steps, longest 1)"},
		{solver.Result{Trace: []solver.Step{la(3), la(LongChain - 1)}, Stats: solver.Stats{Lookahead: 2}}, "2 (2 lookahead steps, longest 6)"},
		{solver.Result{Trace: []solver.Step{la(3), la(LongChain)}, Stats: solver.Stats{Lookahead: 2}}, "3 (2 lookahead steps, longest 7)"},
		{solver.Result{Trace: []solver.Step{la(3)}, Stats: solver.Stats{Lookahead: ManyLookahead}}, "3 (10 lookahead steps, longest 3)"},
		{solver.Result{Trace: []solver.Step{{Level: solver.LevelGuess}}, Stats: solver.Stats{Guesses: 1}}, "3 (0 lookahead steps, longest 0, 1 guesses)"},
	} {
		if got := Rate(c.res).String(); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

// TestLibraryRatings: the rating line of every hard puzzle the tests run is
// what Classify gives today. After a change to the rules or the search,
// `starbattle hard -rate -o testdata/hard` writes them anew.
func TestLibraryRatings(t *testing.T) {
	names, err := starbattle.HardTestSet()
	if err != nil {
		t.Fatal(err)
	}
	if testing.Short() {
		names = names[:3]
	}
	for _, name := range names {
		text, err := os.ReadFile(filepath.Join("../..", starbattle.HardDir, name))
		if err != nil {
			t.Fatal(err)
		}
		v, err := Classify(puzzle.MustParse(string(text)))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := starbattle.RatingKey + " " + v.Rating.String()
		if !strings.Contains(string(text), want+"\n") {
			t.Errorf("%s: rating line is not %q; run starbattle hard -rate -o testdata/hard", name, want)
		}
		if got := starbattle.FileRating(text); got != v.Rating.Stars {
			t.Errorf("%s: FileRating %d, want %d", name, got, v.Rating.Stars)
		}
	}
}
