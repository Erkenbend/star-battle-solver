package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"starbattle/internal/bruteforce"
	"starbattle/internal/generator"
	"starbattle/internal/puzzle"
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
	h := DesignerHandler(t.TempDir(), testSolve)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="canvas"`) {
		t.Fatalf("status %d", rec.Code)
	}
	// Not part of serve.
	rec = httptest.NewRecorder()
	Handler(testSolve, nil, nil, nil).ServeHTTP(rec, httptest.NewRequest("POST", "/api/complete", nil))
	if rec.Code == http.StatusOK {
		t.Fatal("serve answers /api/complete")
	}
}

func TestDesignerCheck(t *testing.T) {
	h := DesignerHandler(t.TempDir(), testSolve)
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

func TestDesignerCompleteAndSave(t *testing.T) {
	dir := t.TempDir()
	h := DesignerHandler(dir, testSolve)
	rec := postDesign(t, h, "/api/complete", designRequest{Pattern: heart, Candidates: 3})
	evs := designEvents(t, rec)
	last := evs[len(evs)-1]
	if last.Done == nil || last.Done.Found != len(evs)-1 || last.Done.Found+last.Done.Failed+last.Done.Duplicates != 3 || last.Done.Found == 0 {
		t.Fatalf("got %s", rec.Body)
	}
	for _, ev := range evs[:len(evs)-1] {
		if ev.Candidate == nil {
			t.Fatalf("got %s", rec.Body)
		}
	}
	c := evs[0].Candidate
	for r, row := range heart {
		for col, reg := range row {
			if reg != generator.Free && c.Regions[r][col] != reg {
				t.Fatalf("cell (%d,%d) lost its color", r, col)
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
	if !strings.Contains(string(text), "#   .AA..AA.") {
		t.Errorf("pattern missing in the comments:\n%s", text)
	}
	// Never overwrite.
	if rec = postDesign(t, h, "/api/save", save); rec.Code != http.StatusBadRequest {
		t.Fatalf("second save: %d", rec.Code)
	}
	// Without a name: numbered.
	save.Name = ""
	if rec = postDesign(t, h, "/api/save", save); !strings.Contains(rec.Body.String(), "n08_d001.txt") {
		t.Fatalf("unnamed save: %s", rec.Body)
	}
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
	h := DesignerHandler(t.TempDir(), testSolve)
	rec := postDesign(t, h, "/api/complete", designRequest{Pattern: patternOf("AAAA", "....", "AAAA", "...."), Candidates: 2})
	evs := designEvents(t, rec)
	if len(evs) != 1 || evs[0].Problem == nil {
		t.Fatalf("got %s", rec.Body)
	}
}
