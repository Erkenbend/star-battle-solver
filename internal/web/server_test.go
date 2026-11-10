package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
	h := Handler(testSolve, nil, nil, nil)

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
		} else if !strings.Contains(rec.Body.String(), `href="/">Zur Startseite`) {
			t.Errorf("%s: no way to the start page", bad)
		}
	}
}

// waitReady waits until the pool has an n×n puzzle ready.
func waitReady(t *testing.T, pl *Pool, n int) {
	t.Helper()
	for range 500 {
		pl.mu.Lock()
		ok := pl.ready[n] != nil
		pl.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no %d×%d puzzle ready", n, n)
}

func TestPool(t *testing.T) {
	var onRequest atomic.Int32 // solver calls by the handler itself
	pl := NewPool(testSolve, []int{5, 6})
	h := Handler(func(p *puzzle.Puzzle) (solver.Result, error) {
		onRequest.Add(1)
		return testSolve(p)
	}, pl, nil, nil)
	plain := Handler(testSolve, nil, nil, nil)

	for _, n := range []int{5, 6} {
		waitReady(t, pl, n)
		pl.mu.Lock()
		want := pl.ready[n].seed
		pl.mu.Unlock()

		// "New puzzle" hands out the ready one's seed ...
		loc := get(h, fmt.Sprintf("/?n=%d", n)).Header().Get("Location")
		if loc != fmt.Sprintf("/?n=%d&seed=%d", n, want) {
			t.Fatalf("n=%d: redirect to %q, want seed %d", n, loc, want)
		}
		// ... and its page comes from the pool, without solving on request,
		// identical to the page the seed gives without a pool.
		got := pageData(t, get(h, loc).Body.String())
		if c := onRequest.Load(); c != 0 {
			t.Fatalf("n=%d: handler solved %d times", n, c)
		}
		ref := pageData(t, get(plain, loc).Body.String())
		if got.Text != ref.Text || got.Title != ref.Title || len(got.Steps) != len(ref.Steps) || !got.Served {
			t.Fatalf("n=%d: pooled page differs from generated one:\n%s\n%s", n, got.Text, ref.Text)
		}

		// The size is refilled.
		waitReady(t, pl, n)
	}

	// Sizes outside the pool and other seeds are generated on request.
	if rec := get(h, "/?n=7&seed=3"); rec.Code != http.StatusOK || onRequest.Load() != 1 {
		t.Fatalf("unpooled: %d, %d solver calls", rec.Code, onRequest.Load())
	}
	if loc := get(h, "/?n=7").Header().Get("Location"); !strings.HasPrefix(loc, "/?n=7&seed=") {
		t.Fatalf("unpooled size: %q", loc)
	}
}
