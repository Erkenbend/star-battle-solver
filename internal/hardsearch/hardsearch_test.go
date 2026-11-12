package hardsearch

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"starbattle"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
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
