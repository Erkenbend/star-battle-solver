package designer

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"starbattle/internal/collection"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/web"
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

// TestDesignerKeepsHardest: of all puzzles a search finds, the page gets
// the hardest, and the solver searches only where the rules get stuck.
func TestDesignerKeepsHardest(t *testing.T) {
	var mu sync.Mutex
	rated := map[string]int{} // puzzle → free cells after the rules
	searched := 0
	rules := func(p *puzzle.Puzzle) (solver.Result, error) {
		res, err := testRules(p)
		mu.Lock()
		defer mu.Unlock()
		if _, ok := rated[collection.ID(p)]; ok {
			t.Error("rated twice")
		}
		rated[collection.ID(p)] = freeCells(res.State)
		return res, err
	}
	solve := func(p *puzzle.Puzzle) (solver.Result, error) {
		mu.Lock()
		if rated[collection.ID(p)] == 0 {
			t.Error("searched although the rules solve it")
		}
		searched++
		mu.Unlock()
		return testSolve(p)
	}
	h := Handler(t.TempDir(), solve, rules, nil)
	cs, done := candidates(t, designEvents(t, postDesign(t, h, "/api/complete", designRequest{Pattern: grid(12), Candidates: 2, Tries: 150})))
	slices.SortFunc(cs, func(a, b *Candidate) int {
		if harder(a, b) {
			return -1
		}
		return 1
	})
	hardest := 0
	for _, free := range rated {
		hardest = max(hardest, free)
	}
	t.Logf("%d candidates sent, %d puzzles rated, %d searched, counters %+v", len(cs), len(rated), searched, done)
	if done.Accepted != int64(len(rated)) || len(cs) < min(2, len(rated)) || cs[0].RulesOut != hardest {
		t.Fatalf("hardest %d free cells, first candidate %d; %d sent, %d rated, counters %+v", hardest, cs[0].RulesOut, len(cs), len(rated), done)
	}
	for _, c := range cs {
		if (c.RulesOut == 0) != (c.Stats == web.Stats{}) || c.Status != solver.Solved.String() {
			t.Errorf("candidate %d: %+v", c.Seed, c)
		}
	}
}

// TestSearchPageAgrees: the page keeps the first k of the candidates it got,
// sorted by harder; whatever order they arrived in, those are the search's
// k hardest.
func TestSearchPageAgrees(t *testing.T) {
	s := &search{pattern: grid(10), k: 3, tries: 300, solve: testSolve, rules: testRules}
	var got []*Candidate
	for c := range s.run(context.Background()) {
		got = append(got, c)
	}
	slices.Reverse(got) // any order
	slices.SortFunc(got, func(a, b *Candidate) int {
		if harder(a, b) {
			return -1
		}
		return 1
	})
	page := got[:min(len(got), s.k)]
	if len(page) != len(s.best) {
		t.Fatalf("page %d, search %d", len(page), len(s.best))
	}
	for i := range page {
		if page[i] != s.best[i] {
			t.Fatalf("place %d: page %+v, search %+v", i, page[i], s.best[i])
		}
	}
	if s.entered != len(got) {
		t.Fatalf("%d entered, %d sent", s.entered, len(got))
	}
}

// TestDesignerCancelled: a request that is gone stops the search at once.
func TestDesignerCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b, _ := json.Marshal(designRequest{Pattern: heart, Candidates: 2})
	rec := httptest.NewRecorder()
	Handler(t.TempDir(), testSolve, testRules, nil).ServeHTTP(rec, httptest.NewRequest("POST", "/api/complete", bytes.NewReader(b)).WithContext(ctx))
	if _, done := candidates(t, designEvents(t, rec)); done.Tried != 0 {
		t.Fatalf("counters %+v", done)
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
	h := Handler(t.TempDir(), testSolve, testRules, nil)
	for _, path := range []string{"/api/check", "/api/complete", "/api/save"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{kaputt"))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "nicht lesbar") {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body)
		}
	}
}
