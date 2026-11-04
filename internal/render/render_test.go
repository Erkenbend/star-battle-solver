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
