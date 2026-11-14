package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// grid returns an n×n pattern of free cells with the given rows on top.
func grid(n int, top ...string) [][]int {
	rows := make([]string, n)
	for r := range rows {
		rows[r] = strings.Repeat(".", n)
		if r < len(top) {
			rows[r] = top[r] + rows[r][len(top[r]):]
		}
	}
	return patternOf(rows...)
}

func TestCheckPatternMessages(t *testing.T) {
	for _, tt := range []struct {
		name    string
		pattern [][]int
		strict  bool
		want    string
	}{
		{"unreachable", patternOf("AB..", "B...", "....", "...A"), false, "über freie Felder nicht"},
		{"strict unreachable", patternOf("A..A", "....", "....", "...."), true, "zusammenhängend gezeichnet"},
		{"enclosed too small", grid(10, "AB", "B"), false, "ringsum eingeschlossen"},
		{"strict too small", grid(10, "A"), true, "wächst sie nicht mehr"},
		{"strict small free area", grid(10, ".A", "AA"), true, "freie Fläche hat nur 1 Feld"},
		{"strict leftover", patternOf("..B.", "..B.", "CCC.", "...A"), true, "getrennte freie Flächen"},
		{"strict fixed", patternOf("AABB", "AABB", "..CC", "..CC"), true, "steht das Puzzle hier schon fest"},
		{"ambiguous", patternOf("AAAB..CC", "AA.B..CC", "A..BB..C", "DD...EEE", "DD..FF.E", "G...FF..", "GG.HHH..", "GGG.HH.."), false, "auf mehrere Arten"},
		{"no stars", patternOf("AAAA", "....", "AAAA", "...."), false, "keine Verteilung der Sterne"},
		{"strict no stars", patternOf("AABB", "....", "....", "...."), true, "eigenen Feldern"},
		{"invalid", patternOf("....", "...", "....", "...."), false, "Ungültiges Muster"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := checkPattern(tt.pattern, tt.strict)
			if p == nil || !strings.Contains(p.Message, tt.want) {
				t.Fatalf("got %+v, want a message with %q", p, tt.want)
			}
		})
	}
}

func TestFreeWhenStuck(t *testing.T) {
	p := puzzle.MustParse("AAB\nACB\nCCB\n") // layout only
	mark := func(r, c int, k board.Cell) solver.Mark {
		return solver.Mark{Pos: puzzle.Pos{Row: r, Col: c}, Kind: k}
	}
	rules := solver.Step{Marks: []solver.Mark{mark(0, 0, board.Star), mark(0, 1, board.Cross), mark(1, 0, board.Cross)}}
	if got := freeWhenStuck(p, []solver.Step{rules}); got != 0 {
		t.Fatalf("without search: got %d, want 0", got)
	}
	search := solver.Step{Source: solver.HypothesisSource, Marks: []solver.Mark{mark(2, 2, board.Cross)}}
	if got := freeWhenStuck(p, []solver.Step{rules, search}); got != 6 {
		t.Fatalf("with search: got %d, want 6", got)
	}
}

func TestSaveDesignErrors(t *testing.T) {
	dir := t.TempDir()
	square := [][]int{{0, 0, 1, 1}, {0, 0, 1, 1}, {2, 2, 3, 3}, {2, 2, 3, 3}}
	free4 := grid(4)
	for _, tt := range []struct {
		name string
		req  designRequest
		want string
	}{
		{"sizes differ", designRequest{Pattern: free4, Regions: square[:3]}, "verschieden groß"},
		{"regions ragged", designRequest{Pattern: free4, Regions: [][]int{{0, 0, 1, 1}, {0, 0, 1}, {2, 2, 3, 3}, {2, 2, 3, 3}}}, "Puzzle ist nicht quadratisch"},
		{"pattern ragged", designRequest{Pattern: patternOf("....", "...", "....", "...."), Regions: square}, "Muster ist nicht quadratisch"},
		{"strict grows", designRequest{Pattern: patternOf("A...", "....", "....", "...."), Regions: square, Strict: true}, "darf eine gezeichnete Farbe nicht wachsen"},
		{"invalid puzzle", designRequest{Pattern: free4, Regions: [][]int{{0, 1, 0, 1}, {2, 2, 3, 3}, {2, 2, 3, 3}, {2, 2, 3, 3}}}, "Puzzle ungültig"},
		{"not unique", designRequest{Pattern: free4, Regions: square}, "keine eindeutige Lösung"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := saveDesign(dir, tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestDesignerBadRequests(t *testing.T) {
	h := DesignerHandler(t.TempDir(), testSolve, nil)
	for _, path := range []string{"/api/check", "/api/complete", "/api/save"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{kaputt"))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "nicht lesbar") {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body)
		}
	}
}
