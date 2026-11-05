package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"starbattle/internal/board"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
	"starbattle/internal/solver/observers"
)

func testSolve(p *puzzle.Puzzle) (solver.Result, error) {
	return solver.Solve(board.New(p), observers.All(p), solver.Options{Lookahead: true, Backtrack: true}), nil
}

// get requests path from h and returns the response.
func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

// pageData extracts the JSON data of a served page.
func pageData(t *testing.T, body string) Data {
	t.Helper()
	m := regexp.MustCompile(`(?s)<script id="data" type="application/json">(.*?)</script>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no data in page")
	}
	var d Data
	if err := json.Unmarshal([]byte(m[1]), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestServer(t *testing.T) {
	h := Handler(testSolve)

	// Without a seed: redirect to a stable URL with a fresh seed.
	rec := get(h, "/?n=6")
	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusFound || !regexp.MustCompile(`^/\?n=6&seed=\d+$`).MatchString(loc) {
		t.Fatalf("redirect: %d %q", rec.Code, loc)
	}
	if loc := get(h, "/").Header().Get("Location"); !strings.HasPrefix(loc, "/?n=8&seed=") {
		t.Fatalf("default size: %q", loc)
	}

	// A seed always gives the same puzzle, solved and marked as served.
	rec = get(h, "/?n=7&seed=42")
	if rec.Code != http.StatusOK {
		t.Fatalf("generate: %d %s", rec.Code, rec.Body)
	}
	d := pageData(t, rec.Body.String())
	if !d.Served || d.N != 7 || len(d.Solution) != 7 || d.Status != "solved" || !strings.Contains(d.Title, "Nr. 42") {
		t.Fatalf("generated page: served=%v n=%d solution=%v status=%s title=%q", d.Served, d.N, d.Solution, d.Status, d.Title)
	}
	if d2 := pageData(t, get(h, "/?n=7&seed=42").Body.String()); d2.Text != d.Text {
		t.Fatalf("same seed, different puzzles:\n%s\n%s", d.Text, d2.Text)
	}

	// The puzzle text pasted back (with '/' as row separator) gives the same puzzle.
	rec = get(h, "/play?p="+url.QueryEscape(strings.ReplaceAll(strings.TrimSpace(d.Text), "\n", "/")))
	if rec.Code != http.StatusOK {
		t.Fatalf("play: %d %s", rec.Code, rec.Body)
	}
	if pd := pageData(t, rec.Body.String()); pd.Text != d.Text || len(pd.Solution) != 7 {
		t.Fatalf("pasted puzzle differs: %q vs %q", pd.Text, d.Text)
	}

	// Several solutions: served without a solution.
	if pd := pageData(t, get(h, "/play?p=AAAA,BBBB,CCCC,DDDD").Body.String()); pd.Solution != nil || pd.N != 4 {
		t.Fatalf("puzzle with several solutions: n=%d solution=%v", pd.N, pd.Solution)
	}

	for _, bad := range []string{
		"/?n=3", "/?n=99", "/?n=x", "/?n=6&seed=-1",
		"/play?p=" + url.QueryEscape("AB/AB"),               // too small
		"/play?p=" + url.QueryEscape("AAB/ABB/ABB"),         // too small
		"/play?p=" + url.QueryEscape("AAAA/BBBB/CCCC"),      // not square
		"/play?p=" + url.QueryEscape("AAAB/CCAB/CDDB/CCCB"), // no solution
	} {
		if rec := get(h, bad); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", bad, rec.Code)
		}
	}
}
