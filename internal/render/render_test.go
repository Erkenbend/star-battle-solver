package render

import (
	"strings"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
)

func TestHighlight(t *testing.T) {
	p := puzzle.MustParse("AAB\nACB\nCCB\n") // layout only, need not be solvable
	s := board.New(p)
	s.Set(0, 0, board.Star)
	s.Set(0, 1, board.Cross)
	s.Set(2, 2, board.Cross)
	got := State(s, Options{NoColor: true, Highlight: map[puzzle.Pos]bool{{Row: 0, Col: 1}: true}})
	want := strings.Join([]string{
		"    " + "   " + " ▼ " + "   ",
		"     0  1  2 ",
		"▶ 0  ★ [×] B ",
		"  1  A  C  B ",
		"  2  C  C  · ",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestPuzzleNoColor(t *testing.T) {
	got := Puzzle(puzzle.MustParse("AAB\nACB\nCCB\n"), Options{NoColor: true})
	want := strings.Join([]string{
		"     0  1  2 ",
		"  0  A  A  B ",
		"  1  A  C  B ",
		"  2  C  C  B ",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestColorAndConflict(t *testing.T) {
	p := puzzle.MustParse("AAB\nACB\nCCB\n")
	s := board.New(p)
	s.Set(0, 0, board.Star)
	s.Set(1, 1, board.Star)
	s.Set(2, 2, board.Cross)
	conflict := map[puzzle.Pos]bool{{Row: 0, Col: 0}: true, {Row: 1, Col: 1}: true}

	plain := State(s, Options{NoColor: true, Conflict: conflict})
	if !strings.Contains(plain, "(★)") || !strings.Contains(plain, "▶ 1") {
		t.Fatalf("conflict cells not marked:\n%s", plain)
	}

	colored := State(s, Options{Conflict: conflict})
	for _, want := range []string{
		"\x1b[1;31m▼\x1b[0m",                     // arrow over a conflict column
		"\x1b[48;5;203m\x1b[1;31m(★)\x1b[0m",     // conflict star in region A, bold red
		"\x1b[48;5;215m\x1b[38;5;240m B \x1b[0m", // free cell of region B, gray letter
		"\x1b[48;5;215m\x1b[30m · \x1b[0m",       // cross, black
	} {
		if !strings.Contains(colored, want) {
			t.Errorf("missing %q in\n%q", want, colored)
		}
	}
}
