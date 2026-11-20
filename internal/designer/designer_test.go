package designer

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"starbattle"
	"starbattle/internal/board"
	"starbattle/internal/bruteforce"
	"starbattle/internal/collection"
	"starbattle/internal/generator"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/solver/observers"
	"starbattle/internal/web"
)

// patternOf reads a pattern: '.' is free, letters are regions (A = 0).
func patternOf(rows ...string) [][]int {
	out := make([][]int, len(rows))
	for r, row := range rows {
		for _, ch := range row {
			v := generator.Free
			if ch != '.' {
				v = int(ch - 'A')
			}
			out[r] = append(out[r], v)
		}
	}
	return out
}

func testSolve(p *puzzle.Puzzle) (solver.Result, error) {
	return solver.Solve(board.New(p), observers.All(p), solver.Options{Lookahead: true, Backtrack: true}), nil
}

// get requests path from h and returns the response.
func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

// TestDesignerPlay: the designer's "try it" opens the play page, which links
// to the settings, so the designer serves them too.
func TestDesignerPlay(t *testing.T) {
	syms, err := web.LoadSymbols(starbattle.Symbols, "symbols")
	if err != nil {
		t.Fatal(err)
	}
	h := Handler(t.TempDir(), testSolve, syms)
	paths := []string{"/play?p=DBBBB/DDBCC/DDBAA/EDDAA/EAAAA", "/einstellungen", "/symbole.json"}
	for _, s := range syms.List() {
		if s.Kind == web.KindSprite {
			paths = append(paths, s.URL)
			break
		}
	}
	for _, p := range paths {
		if rec := get(h, p); rec.Code != http.StatusOK {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
	if rec := get(Handler(t.TempDir(), testSolve, nil), "/einstellungen"); rec.Code != http.StatusNotFound {
		t.Errorf("without symbols: /einstellungen %d", rec.Code)
	}
}

func postDesign(t *testing.T, h http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", path, bytes.NewReader(b)))
	return rec
}

// designEvents reads the NDJSON answer of /api/complete.
func designEvents(t *testing.T, rec *httptest.ResponseRecorder) []designEvent {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("Content-Type %q", ct)
	}
	var evs []designEvent
	for _, line := range strings.Split(strings.TrimSpace(rec.Body.String()), "\n") {
		var ev designEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("line %q: %v", line, err)
		}
		evs = append(evs, ev)
	}
	return evs
}

var heart = patternOf(
	"........",
	".AA..AA.",
	".AAAAAA.",
	"..AAAA..",
	"...AA...",
	"........",
	"........",
	"........",
)

func TestDesignerPage(t *testing.T) {
	h := Handler(t.TempDir(), testSolve, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="canvas"`) {
		t.Fatalf("status %d", rec.Code)
	}
	// Not part of serve.
	rec = httptest.NewRecorder()
	web.Handler(testSolve, nil, nil, nil, nil).ServeHTTP(rec, httptest.NewRequest("POST", "/api/complete", nil))
	if rec.Code == http.StatusOK {
		t.Fatal("serve answers /api/complete")
	}
}

func TestDesignerCheck(t *testing.T) {
	h := Handler(t.TempDir(), testSolve, nil)
	rec := postDesign(t, h, "/api/check", designRequest{Pattern: heart})
	if got := strings.TrimSpace(rec.Body.String()); got != "null" {
		t.Fatalf("valid pattern: got %s", got)
	}
	rec = postDesign(t, h, "/api/check", designRequest{Pattern: patternOf("AB..", "B...", "....", "...A")})
	var p patternProblem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || !strings.Contains(p.Message, "Farbe A") || len(p.Cells) != 1 {
		t.Fatalf("got %s", rec.Body)
	}
	rec = postDesign(t, h, "/api/check", designRequest{Pattern: patternOf("AAA", "...", "...")})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("3×3: status %d", rec.Code)
	}
}

// candidates returns the candidates of evs and the final counters, and
// checks that the answer ends with done.
func candidates(t *testing.T, evs []designEvent) ([]*Candidate, *designCounts) {
	t.Helper()
	last := evs[len(evs)-1]
	if last.Done == nil {
		t.Fatalf("no done at the end: %+v", last)
	}
	var cs []*Candidate
	for _, ev := range evs[:len(evs)-1] {
		switch {
		case ev.Candidate != nil:
			cs = append(cs, ev.Candidate)
		case ev.Progress == nil:
			t.Fatalf("unexpected event %+v", ev)
		}
	}
	return cs, last.Done
}

func TestDesignerCompleteAndSave(t *testing.T) {
	dir := t.TempDir()
	h := Handler(dir, testSolve, nil)
	rec := postDesign(t, h, "/api/complete", designRequest{Pattern: heart, Candidates: 3})
	cs, done := candidates(t, designEvents(t, rec))
	// The third puzzle ends the search.
	if len(cs) != 3 || done.Accepted != 3 {
		t.Fatalf("%d candidates, counters %+v", len(cs), done)
	}
	seeds := map[uint64]bool{}
	for _, c := range cs {
		if seeds[c.Seed] || c.Status != solver.Solved.String() {
			t.Fatalf("candidate %+v", c)
		}
		seeds[c.Seed] = true
	}
	c := cs[0]
	for _, c := range cs {
		for r, row := range heart {
			for col, reg := range row {
				if reg != generator.Free && c.Regions[r][col] != reg {
					t.Fatalf("cell (%d,%d) lost its color", r, col)
				}
			}
		}
	}

	save := designRequest{Pattern: heart, Regions: c.Regions, Seed: c.Seed, Name: "herz"}
	rec = postDesign(t, h, "/api/save", save)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "n08_herz.txt") {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	text, err := os.ReadFile(filepath.Join(dir, "n08_herz.txt"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := puzzle.Parse(bytes.NewReader(text))
	if err != nil || bruteforce.CountSolutions(p, 2) != 1 {
		t.Fatalf("saved file: %v\n%s", err, text)
	}
	if !strings.Contains(string(text), "#   .AA..AA.") || !strings.HasPrefix(string(text), collection.NameKey+" Herz\n") {
		t.Errorf("pattern or name missing in the comments:\n%s", text)
	}
	// Never overwrite.
	if rec = postDesign(t, h, "/api/save", save); rec.Code != http.StatusBadRequest {
		t.Fatalf("second save: %d", rec.Code)
	}
	// Nor save the same puzzle under another name: the library would refuse it.
	save.Name = "herz2"
	if rec = postDesign(t, h, "/api/save", save); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "schon als n08_herz.txt") {
		t.Fatalf("same puzzle again: %d %s", rec.Code, rec.Body)
	}
	// It is in the library's index, which loads.
	motifs, err := web.LoadMotifs(os.DirFS(dir), ".", testSolve)
	if err != nil || motifs.Len() != 1 {
		t.Fatalf("index: %v", err)
	}
	if entries, _ := collection.Load(os.DirFS(dir), "."); collection.Name(entries[0].File, entries[0].Text) != "Herz" {
		t.Errorf("motif name in %s", entries[0].File)
	}
	// Without a name: numbered.
	save.Name = ""
	if rec = postDesign(t, Handler(t.TempDir(), testSolve, nil), "/api/save", save); !strings.Contains(rec.Body.String(), "n08_d001.txt") {
		t.Fatalf("unnamed save: %s", rec.Body)
	}
	// A name is used once across sizes: n10_herz.txt would be "Herz" too.
	if err := os.WriteFile(filepath.Join(dir, "n10_herz.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "n08_herz.txt"), filepath.Join(dir, "n08_herz.bak")); err != nil {
		t.Fatal(err)
	}
	save.Name = "herz"
	if rec = postDesign(t, h, "/api/save", save); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "n10_herz.txt") {
		t.Fatalf("name of another size: %d %s", rec.Code, rec.Body)
	}
	save.Name = ""
	// A layout that does not keep the pattern is refused.
	save.Pattern = patternOf("........", "........", "........", "........", "........", "........", "........", "........")
	save.Pattern[7][7] = (c.Regions[7][7] + 1) % 8
	if rec = postDesign(t, h, "/api/save", save); rec.Code != http.StatusBadRequest {
		t.Fatalf("mismatching pattern saved: %d", rec.Code)
	}
	// Bad names are refused.
	save.Pattern, save.Name = heart, "../x"
	if rec = postDesign(t, h, "/api/save", save); rec.Code != http.StatusBadRequest {
		t.Fatalf("name ../x: %d", rec.Code)
	}
}

func TestDesignerCompleteInvalid(t *testing.T) {
	h := Handler(t.TempDir(), testSolve, nil)
	rec := postDesign(t, h, "/api/complete", designRequest{Pattern: patternOf("AAAA", "....", "AAAA", "...."), Candidates: 2})
	evs := designEvents(t, rec)
	if len(evs) != 1 || evs[0].Problem == nil {
		t.Fatalf("got %s", rec.Body)
	}
}

func TestDesignerStrict(t *testing.T) {
	dir := t.TempDir()
	h := Handler(dir, testSolve, nil)
	// Not connected as drawn: fine normally, not strictly.
	parts := patternOf("A..A....", "........", "........", "........", "........", "........", "........", "........")
	if rec := postDesign(t, h, "/api/check", designRequest{Pattern: parts}); strings.TrimSpace(rec.Body.String()) != "null" {
		t.Fatalf("normal: %s", rec.Body)
	}
	rec := postDesign(t, h, "/api/check", designRequest{Pattern: parts, Strict: true})
	if !strings.Contains(rec.Body.String(), "strengen Modus") {
		t.Fatalf("strict: %s", rec.Body)
	}

	cs, _ := candidates(t, designEvents(t, postDesign(t, h, "/api/complete", designRequest{Pattern: heart, Candidates: 2, Strict: true, Tries: 20})))
	if len(cs) == 0 {
		t.Fatal("no candidate")
	}
	c := cs[0]
	for r, row := range heart {
		for col, reg := range row {
			if (reg == generator.Free) == (c.Regions[r][col] == 0) {
				t.Fatalf("cell (%d,%d): heart not kept exactly", r, col)
			}
		}
	}
	save := designRequest{Pattern: heart, Regions: c.Regions, Seed: c.Seed, Name: "streng", Strict: true}
	if rec := postDesign(t, h, "/api/save", save); rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	text, _ := os.ReadFile(filepath.Join(dir, "n08_streng.txt"))
	if !strings.Contains(string(text), ", Strict)") {
		t.Errorf("mode missing:\n%s", text)
	}
	// A completion where the heart grew is refused in strict mode, before
	// anything else is checked.
	grown := make([][]int, 8)
	for r := range grown {
		grown[r] = append([]int(nil), c.Regions[r]...)
	}
	grown[0][0] = 0
	save.Regions, save.Name = grown, "gewachsen"
	if rec := postDesign(t, h, "/api/save", save); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "nicht wachsen") {
		t.Fatalf("grown heart: %d %s", rec.Code, rec.Body)
	}
}
