package generator

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/bruteforce"
)

// parsePattern reads a pattern: '.' is free, letters are regions (A = 0).
func parsePattern(s string) [][]int {
	var out [][]int
	for _, line := range strings.Fields(strings.ReplaceAll(s, "\\n", "\n")) {
		row := make([]int, len(line))
		for i, ch := range line {
			if ch == '.' {
				row[i] = Free
			} else {
				row[i] = int(ch - 'A')
			}
		}
		out = append(out, row)
	}
	return out
}

func checkCompletion(t *testing.T, pattern [][]int, seed uint64) {
	t.Helper()
	n := len(pattern)
	p, sol, err := Complete(pattern, rand.New(rand.NewPCG(seed, uint64(n))), Options{MinRegionSize: DefaultMinRegionSize(n)})
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("seed %d: invalid layout: %v\n%s", seed, err, p)
	}
	for r, row := range pattern {
		for c, reg := range row {
			if reg != Free && p.Regions[r][c] != reg {
				t.Fatalf("seed %d: cell (%d,%d) is %d, pattern says %d\n%s", seed, r, c, p.Regions[r][c], reg, p)
			}
		}
	}
	if !board.FromSolution(p, sol).Solved() {
		t.Fatalf("seed %d: returned solution is invalid\n%s", seed, p)
	}
	if got := bruteforce.CountSolutions(p, 2); got != 1 {
		t.Fatalf("seed %d: %d solutions\n%s", seed, got, p)
	}
}

func TestCompletePatterns(t *testing.T) {
	patterns := map[string]string{
		"empty": `
			........
			........
			........
			........
			........
			........
			........
			........`,
		// A heart in one color, the rest free.
		"heart": `
			..........
			..AA..AA..
			.AAAAAAAA.
			.AAAAAAAA.
			..AAAAAA..
			...AAAA...
			....AA....
			..........
			.B......C.
			BBB....CCC`,
		// Regions in several parts that have to be joined.
		"parts": `
			B.......B
			.........
			..A...A..
			.........
			.........
			.........
			..A...A..
			.........
			C.......C`,
	}
	for name, s := range patterns {
		t.Run(name, func(t *testing.T) {
			pattern := parsePattern(s)
			if err := CheckPattern(pattern, DefaultMinRegionSize(len(pattern)), false); err != nil {
				t.Fatal(err)
			}
			for seed := range uint64(5) {
				checkCompletion(t, pattern, seed)
			}
		})
	}
}

// discPattern keeps the cells of a generated puzzle inside a random disc
// covering about frac of the board, like a drawing of one area.
func discPattern(n int, frac float64, rng *rand.Rand) [][]int {
	p, _, err := Generate(n, rng, Options{MinRegionSize: DefaultMinRegionSize(n)})
	if err != nil {
		panic(err)
	}
	cr, cc := rng.Float64()*float64(n), rng.Float64()*float64(n)
	rad2 := frac * float64(n*n) / 3.14159
	pattern := make([][]int, n)
	for r := range n {
		pattern[r] = make([]int, n)
		for c := range n {
			dr, dc := float64(r)+0.5-cr, float64(c)+0.5-cc
			pattern[r][c] = Free
			if dr*dr+dc*dc < rad2 {
				pattern[r][c] = p.Regions[r][c]
			}
		}
	}
	return pattern
}

func TestCompleteDiscs(t *testing.T) {
	maxN := 12
	if testing.Short() {
		maxN = 9
	}
	for n := 6; n <= maxN; n++ {
		for i, frac := range []float64{0.2, 0.4, 0.6, 0.8} {
			seed := uint64(i)
			pattern := discPattern(n, frac, rand.New(rand.NewPCG(seed, 200+uint64(n))))
			if err := CheckPattern(pattern, DefaultMinRegionSize(n), false); err != nil {
				t.Fatalf("n=%d frac=%.1f: part of a valid puzzle rejected: %v", n, frac, err)
			}
			checkCompletion(t, pattern, seed)
		}
	}
}

func TestCheckPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		minSize int
		want    PatternProblem // -1: no problem
		region  int
		strict  bool
	}{
		{"empty", "....\n....\n....\n....", 1, -1, 0, false},
		{"two parts joinable", "A..A\n....\n....\n....", 1, -1, 0, false},
		{"part walled in", "AB..\nB...\n....\n...A", 1, Unreachable, 0, false},
		{"enclosed single cell", "AB...\nB....\n.....\n.....\n.....", 2, TooSmall, 0, false},
		{"enclosed single cell allowed", "AB...\nB....\n.....\n.....\n.....", 1, -1, 0, false},
		{"two whole rows", "AAAA\n....\nAAAA\n....", 1, NoStars, -1, false},
		// Row 1 is all C, so A and B are enclosed in row 0: both stars there.
		{"enclosed regions in one row", "AABB\nCCCC\n....\n....", 1, NoStars, -1, false},
		{"adjacent full rows of one color pair", "AAAA\nAAAA\n....\n....", 1, NoStars, -1, false},
		// Two regions enclosed in the same row: both stars in row 0.
		{"two enclosed in one row", "AXBX.\nXXXX.\n.....\n.....\n.....", 1, NoStars, -1, false},
		// Every color drawn: the stars can sit on drawn cells alone in more
		// than one way, so no completion has a unique solution.
		{"ambiguous", "AAAB..CC\nAA.B..CC\nA..BB..C\nDD...EEE\nDD..FF.E\nG...FF..\nGG.HHH..\nGGG.HH..", 1, Ambiguous, -1, false},
		// Strict: drawn regions are final.
		{"strict ok", "AA..\nAA..\n....\n..BB", 1, -1, 0, true},
		{"strict parts", "A..A\n....\n....\n....", 1, Unreachable, 0, true},
		{"strict small", "A....\n.....\n.....\n.....\n.....", 2, TooSmall, 0, true},
		// A wall of B and C splits the free cells in two, D is the only
		// region left.
		{"strict leftover", "..B.\n..B.\nCCC.\n...A", 1, Leftover, -1, true},
		{"not strict, same pattern", "..B.\n..B.\nCCC.\n...A", 1, -1, 0, false},
		// Strict: A's star must be on its own cells, all in row 0 next to
		// B's, which is enclosed in row 0 as well.
		{"strict stars", "AABB\n....\n....\n....", 1, NoStars, -1, true},
		// Strict: one free area, one missing region: the puzzle is fixed
		// (AABB AABB DDCC DDCC) and has two solutions.
		{"strict fixed", "AABB\nAABB\n..CC\n..CC", 1, Fixed, -1, true},
		{"not strict, same pattern", "AABB\nAABB\n..CC\n..CC", 1, -1, 0, false},
		{"strict small area", "A.A.\nAAA.\n....\n....", 2, TooSmall, -1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := parsePattern(strings.ReplaceAll(tt.pattern, "X", "C"))
			err := CheckPattern(pattern, tt.minSize, tt.strict)
			if tt.want < 0 {
				if err != nil {
					t.Fatalf("unexpected problem: %v", err)
				}
				return
			}
			pe, ok := err.(*PatternError)
			if !ok {
				t.Fatalf("got %v, want problem %d", err, tt.want)
			}
			if pe.Problem != tt.want || pe.Region != tt.region {
				t.Fatalf("got %v (problem %d, region %d), want problem %d, region %d", pe, pe.Problem, pe.Region, tt.want, tt.region)
			}
		})
	}
}

// checkStrict checks a strict completion: drawn regions are exactly their
// drawn cells.
func checkStrict(t *testing.T, pattern [][]int, seed uint64) {
	t.Helper()
	n := len(pattern)
	p, _, err := Complete(pattern, rand.New(rand.NewPCG(seed, uint64(n))), Options{MinRegionSize: DefaultMinRegionSize(n), Strict: true})
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	if bruteforce.CountSolutions(p, 2) != 1 {
		t.Fatalf("seed %d: not unique\n%s", seed, p)
	}
	drawn := map[int]bool{}
	for _, row := range pattern {
		for _, reg := range row {
			drawn[reg] = true
		}
	}
	for r, row := range pattern {
		for c, reg := range row {
			if got := p.Regions[r][c]; (reg != Free && got != reg) || (reg == Free && drawn[got]) {
				t.Fatalf("seed %d: cell (%d,%d) is %d, pattern %d\n%s", seed, r, c, got, reg, p)
			}
		}
	}
}

func TestCompleteStrict(t *testing.T) {
	for name, rows := range map[string]string{
		"heart": `
			..........
			..AA..AA..
			.AAAAAAAA.
			.AAAAAAAA.
			..AAAAAA..
			...AAAA...
			....AA....
			..........
			..........
			..........`,
		"blocks": `
			AA......
			AA......
			........
			...BB...
			...BB...
			........
			......CC
			......CC`,
	} {
		t.Run(name, func(t *testing.T) {
			for seed := range uint64(5) {
				checkStrict(t, parsePattern(rows), seed)
			}
		})
	}
}

// strictOK lists the designs whose patterns also complete strictly.
var strictOK = map[string]bool{"n08_lines.txt": true, "n08_sad_smiley.txt": true, "n08_smileyface.txt": true, "n09_lul.txt": true, "n10_confused_smiley.txt": true, "n12_inseln.txt": true}

// TestCompleteDesigns completes the patterns of the puzzles in designs/
// again, and strictly where noted in strictOK.
func TestCompleteDesigns(t *testing.T) {
	files, _ := filepath.Glob("../../designs/*.txt")
	if len(files) == 0 {
		t.Fatal("no designs")
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			text, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			var rows []string
			for _, line := range strings.Split(string(text), "\n") {
				if rest, ok := strings.CutPrefix(line, "#   "); ok {
					rows = append(rows, rest)
				}
			}
			pattern := parsePattern(strings.Join(rows, "\n"))
			n := len(pattern)
			if testing.Short() && n > 10 {
				t.Skip("large")
			}
			checkCompletion(t, pattern, 1)
			// Strictly, the check cannot see everything (see CheckPattern),
			// so these are only expected to work where noted.
			if strictOK[filepath.Base(f)] {
				checkStrict(t, pattern, 1)
			}
		})
	}
}

func TestCompleteRejectsInvalid(t *testing.T) {
	pattern := parsePattern("AAAA\n....\nAAAA\n....")
	_, _, err := Complete(pattern, rand.New(rand.NewPCG(1, 1)), Options{})
	if _, ok := err.(*PatternError); !ok {
		t.Fatalf("got %v, want a *PatternError", err)
	}
}

func TestCompleteKeepsShapes(t *testing.T) {
	// The drawn heart grows only where it must: most free cells go to the
	// regions that are not drawn.
	pattern := parsePattern(`
		..........
		..AA..AA..
		.AAAAAAAA.
		.AAAAAAAA.
		..AAAAAA..
		...AAAA...
		....AA....
		..........
		..........
		..........`)
	grown := 0
	for seed := range uint64(10) {
		p, _, err := Complete(pattern, rand.New(rand.NewPCG(seed, 10)), Options{MinRegionSize: 2})
		if err != nil {
			t.Fatal(err)
		}
		for r, row := range pattern {
			for c, reg := range row {
				if reg == Free && p.Regions[r][c] == 0 {
					grown++
				}
			}
		}
	}
	// 64 free cells per puzzle; a few must join the heart (e.g. the dip
	// between the lobes, or a path to its star).
	if grown > 10*8 {
		t.Fatalf("heart grew by %d cells in 10 puzzles", grown)
	}
}

func TestCompleteDeterministic(t *testing.T) {
	pattern := parsePattern(`
		.......
		.AA.BB.
		.AA.BB.
		.......
		.C...C.
		..CCC..
		.......`)
	a, _, err := Complete(pattern, rand.New(rand.NewPCG(3, 7)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := Complete(pattern, rand.New(rand.NewPCG(3, 7)), Options{Workers: 1})
	if a.String() != b.String() {
		t.Fatal("same seed produced different puzzles")
	}
}

func TestCompleteCancel(t *testing.T) {
	done := make(chan struct{})
	close(done)
	pattern := parsePattern("........\n........\n........\n........\n........\n........\n........\n........")
	if _, _, err := Complete(pattern, rand.New(rand.NewPCG(1, 1)), Options{Done: done}); err != ErrCancelled {
		t.Fatalf("got %v, want ErrCancelled", err)
	}
}
