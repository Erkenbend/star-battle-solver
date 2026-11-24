package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"starbattle/internal/generator"
	"starbattle/internal/party"
	"starbattle/internal/puzzle"
	"starbattle/internal/solver"
)

// testClock is a settable clock, safe for the preparation goroutines.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

var (
	testPuzzlesMu sync.Mutex
	testPuzzles   = map[int]*party.Puzzle{}
)

// testPuzzle returns a generated and solved n×n puzzle, the same for all tests.
func testPuzzle(t *testing.T, n int) *party.Puzzle {
	testPuzzlesMu.Lock()
	defer testPuzzlesMu.Unlock()
	if rp := testPuzzles[n]; rp != nil {
		return rp
	}
	for seed := uint64(1); ; seed++ {
		p, sol, err := generator.ForSeed(n, seed, nil)
		if err != nil || sol == nil {
			continue
		}
		res, _ := testSolve(context.Background(), p)
		rp := &party.Puzzle{N: n, Seed: seed, P: p, Solution: sol, Res: res}
		testPuzzles[n] = rp
		return rp
	}
}

// newTestParties returns a party manager with a settable clock, whose rounds
// do not end on time by themselves, and that prepares testPuzzle.
func newTestParties(t *testing.T) (*party.Manager, *testClock) {
	t.Helper()
	clk := &testClock{t: time.Date(2026, 11, 7, 20, 0, 0, 0, time.UTC)}
	pm := party.New(party.Config{
		MinN:       MinN,
		MaxN:       MaxN,
		Prepare:    func(n int) (*party.Puzzle, error) { return testPuzzle(t, n), nil },
		Now:        clk.now,
		RetryDelay: time.Nanosecond,
		After:      func(time.Duration, func()) {},
	})
	return pm, clk
}

// waitPrepared waits until the party has its next puzzle ready.
func waitPrepared(t *testing.T, pm *party.Manager, code string) {
	t.Helper()
	waitFor(t, func() bool { return !snap(t, pm, code, "").Preparing })
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !cond(); {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(time.Millisecond)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustJoin(t *testing.T, pm *party.Manager, code, name string) string {
	t.Helper()
	tok, _, err := pm.Join(code, name, "")
	must(t, err)
	return tok
}

func snap(t *testing.T, pm *party.Manager, code, tok string) party.Snapshot {
	t.Helper()
	s, err := pm.Snapshot(code, tok)
	must(t, err)
	return s
}

// solutionCells returns the cells of the stars of rp's solution.
func solutionCells(rp *party.Puzzle) []int {
	var out []int
	for r, c := range rp.Solution {
		out = append(out, r*rp.N+c)
	}
	return out
}

// TestPartyPuzzle: the puzzles for parties come from the pool or are
// generated and solved.
func TestPartyPuzzle(t *testing.T) {
	rp, err := partyPuzzle(testSolve, nil)(5)
	if err != nil || rp.P.N != 5 || len(rp.Solution) != 5 || rp.Res.Status != solver.Solved {
		t.Fatalf("got %+v, %v", rp, err)
	}
	failing := func(context.Context, *puzzle.Puzzle) (solver.Result, error) {
		return solver.Result{}, errors.New("kaputt")
	}
	if _, err := partyPuzzle(failing, nil)(5); err == nil || err.Error() != "kaputt" {
		t.Fatalf("solve error: got %v", err)
	}
}

func TestErrorStatus(t *testing.T) {
	for err, want := range map[error]int{
		&party.Error{Kind: party.NotFound}:  http.StatusNotFound,
		&party.Error{Kind: party.Invalid}:   http.StatusBadRequest,
		&party.Error{Kind: party.Forbidden}: http.StatusForbidden,
		&party.Error{Kind: party.Conflict}:  http.StatusConflict,
		&party.Error{Kind: party.Busy}:      http.StatusServiceUnavailable,
		errors.New("other"):                 http.StatusInternalServerError,
	} {
		if got := errorStatus(err); got != want {
			t.Errorf("errorStatus(%v) = %d, want %d", err, got, want)
		}
	}
}

// postJSON posts body as JSON to h and decodes the reply into out (may be nil).
func postJSON(t *testing.T, h http.Handler, path string, body any, out any) int {
	t.Helper()
	data, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", path, strings.NewReader(string(data))))
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s: %v: %s", path, err, rec.Body)
		}
	}
	return rec.Code
}

func TestPartyServer(t *testing.T) {
	pm, clk := newTestParties(t)
	h := Handler(testSolve, nil, pm, nil, nil)

	if rec := get(h, "/party"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="create"`) {
		t.Fatalf("start page: %d", rec.Code)
	}
	if rec := get(Handler(testSolve, nil, nil, nil, nil), "/party"); rec.Code != http.StatusNotFound {
		t.Fatalf("party mode without parties: %d", rec.Code)
	}

	// Creating redirects to the lobby.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/party", strings.NewReader(url.Values{"n": {"5", "x"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	loc := rec.Header().Get("Location")
	m := regexp.MustCompile(`^/party/([A-Z]{4})$`).FindStringSubmatch(loc)
	if rec.Code != http.StatusSeeOther || m == nil {
		t.Fatalf("create: %d %q", rec.Code, loc)
	}
	code := m[1]
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/party", strings.NewReader("n=99"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create without valid sizes: %d", rec.Code)
	}

	if rec := get(h, loc); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) {
		t.Fatalf("lobby: %d", rec.Code)
	}
	if rec := get(h, "/party/"+strings.ToLower(code)); rec.Code != http.StatusFound || rec.Header().Get("Location") != loc {
		t.Fatalf("lower-case code: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	// The link in upper case, as the lobby's QR code has it.
	if rec := get(h, "/PARTY/"+code); rec.Code != http.StatusFound || rec.Header().Get("Location") != loc {
		t.Fatalf("upper-case link: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := get(h, "/party/QQQQ"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown party: %d", rec.Code)
	}
	// The round of a party that is gone: an error page with the way home.
	rec = get(h, "/party/QQQQ/round/1")
	if body := rec.Body.String(); rec.Code != http.StatusNotFound || !strings.Contains(body, "Diese Party gibt es nicht (mehr).") ||
		!strings.Contains(body, `<a class="button primary" href="/">Zur Startseite</a>`) || !strings.Contains(body, `href="/party">Neue Party`) {
		t.Fatalf("round of an unknown party: %d %s", rec.Code, body)
	}

	// Join, ready: the round starts.
	var joined struct {
		Token    string         `json:"token"`
		ID       string         `json:"id"`
		Snapshot party.Snapshot `json:"snapshot"`
	}
	if c := postJSON(t, h, loc+"/join", map[string]any{"name": "Anna"}, &joined); c != http.StatusOK || joined.Token == "" || joined.Snapshot.You != joined.ID {
		t.Fatalf("join: %d %+v", c, joined)
	}
	var errReply struct{ Error string }
	if c := postJSON(t, h, loc+"/ready", map[string]any{"token": "bogus", "ready": true}, &errReply); c != http.StatusForbidden || errReply.Error == "" {
		t.Fatalf("ready with a bad token: %d %+v", c, errReply)
	}
	if c := postJSON(t, h, loc+"/dance", map[string]any{}, nil); c != http.StatusNotFound {
		t.Fatalf("unknown action: %d", c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", loc+"/ready", strings.NewReader("{")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad JSON: %d", rec.Code)
	}
	if rec := get(h, loc+"/round/1"); rec.Code != http.StatusNotFound {
		t.Fatalf("round page before the round: %d", rec.Code)
	}
	waitPrepared(t, pm, code)
	var ready struct{ Snapshot party.Snapshot }
	if c := postJSON(t, h, loc+"/ready", map[string]any{"token": joined.Token, "ready": true}, &ready); c != http.StatusOK || ready.Snapshot.Phase != party.PhasePlaying {
		t.Fatalf("ready: %d %+v", c, ready.Snapshot)
	}

	// The round's page carries the party; the next round's does not exist yet.
	rec = get(h, loc+"/round/1")
	if rec.Code != http.StatusOK {
		t.Fatalf("round page: %d", rec.Code)
	}
	if d := pageData(t, rec.Body.String()); d.Party == nil || d.Party.Code != code || d.Party.Round != 1 || !strings.Contains(rec.Body.String(), "const Party =") {
		t.Fatalf("round page data: %+v", d.Party)
	}
	if strings.Contains(get(h, "/?n=5&seed=1").Body.String(), "const Party =") {
		t.Fatal("solo page has the party script")
	}
	for _, p := range []string{"/round/2", "/round/x"} {
		if rec := get(h, loc+p); rec.Code != http.StatusNotFound {
			t.Fatalf("%s: %d", p, rec.Code)
		}
	}

	clk.add(party.Countdown)
	if c := postJSON(t, h, loc+"/progress", map[string]any{"token": joined.Token, "round": 1, "count": 2, "hints": 1}, nil); c != http.StatusOK {
		t.Fatalf("progress: %d", c)
	}
	if c := postJSON(t, h, loc+"/solve", map[string]any{"token": joined.Token, "round": 1, "stars": []int{0, 0, 0, 0, 0}}, nil); c != http.StatusBadRequest {
		t.Fatalf("wrong solution: %d", c)
	}
	var solved struct {
		Result   party.Result
		Snapshot party.Snapshot
	}
	sol := testPuzzle(t, 5).Solution
	if c := postJSON(t, h, loc+"/solve", map[string]any{"token": joined.Token, "round": 1, "stars": sol}, &solved); c != http.StatusOK || !solved.Result.Solved || solved.Result.Hints != 1 || solved.Snapshot.Phase != party.PhaseResults {
		t.Fatalf("solve: %d %+v", c, solved)
	}
	var left struct{ Snapshot party.Snapshot }
	if c := postJSON(t, h, loc+"/leave", map[string]any{"token": joined.Token}, &left); c != http.StatusOK || left.Snapshot.You != "" || len(left.Snapshot.Players) != 0 {
		t.Fatalf("leave: %d %+v", c, left.Snapshot)
	}
}

func TestPartyEvents(t *testing.T) {
	pm, _ := newTestParties(t)
	srv := httptest.NewServer(Handler(testSolve, nil, pm, nil, nil))
	defer srv.Close()
	code, err := pm.Create(party.Options{Sizes: []int{5}, Mode: ""})
	must(t, err)
	tok := mustJoin(t, pm, code, "Anna")

	if res, err := http.Get(srv.URL + "/party/QQQQ/events"); err != nil || res.StatusCode != http.StatusNotFound {
		t.Fatalf("events of an unknown party: %v %v", res.StatusCode, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/party/"+code+"/events?token="+tok, nil)
	res, err := http.DefaultClient.Do(req)
	must(t, err)
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	sc := bufio.NewScanner(res.Body)
	next := func() party.Snapshot {
		t.Helper()
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				var s party.Snapshot
				must(t, json.Unmarshal([]byte(data), &s))
				return s
			}
		}
		t.Fatalf("stream ended: %v", sc.Err())
		return party.Snapshot{}
	}
	if s := next(); s.Code != code || s.You == "" || len(s.Players) != 1 {
		t.Fatalf("first event %+v", s)
	}
	mustJoin(t, pm, code, "Ben")
	for s := next(); len(s.Players) != 2; s = next() {
	}

	// Closing the stream unsubscribes it (party.Manager.Subscribe tests that
	// the player's connection count goes with it).
	cancel()
	waitFor(t, func() bool { return pm.Subscribers(code) == 0 })
}

func TestCoopServer(t *testing.T) {
	pm, clk := newTestParties(t)
	h := Handler(testSolve, nil, pm, nil, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/party", strings.NewReader("n=5&mode=coop"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	loc := rec.Header().Get("Location")
	code := strings.TrimPrefix(loc, "/party/")
	if rec.Code != http.StatusSeeOther || snap(t, pm, code, "").Mode != party.ModeCoop {
		t.Fatalf("create: %d %q", rec.Code, loc)
	}
	waitPrepared(t, pm, code)
	var joined struct{ Token string }
	postJSON(t, h, loc+"/join", map[string]any{"name": "Anna"}, &joined)
	postJSON(t, h, loc+"/ready", map[string]any{"token": joined.Token, "ready": true}, nil)
	if d := pageData(t, get(h, loc+"/round/1").Body.String()); d.Party == nil || d.Party.Mode != party.ModeCoop {
		t.Fatalf("round page data: %+v", d.Party)
	}
	clk.add(party.Countdown)

	var marked struct {
		Rejected []int
		Snapshot party.Snapshot
	}
	ops := [][4]int{{1, 0, 0, 1}, {2, 0, 2, 0}}
	if c := postJSON(t, h, loc+"/mark", map[string]any{"token": joined.Token, "round": 1, "ops": ops}, &marked); c != http.StatusOK ||
		len(marked.Rejected) != 1 || marked.Rejected[0] != 2 || marked.Snapshot.Coop == nil || marked.Snapshot.Coop.Board[0] != '1' || marked.Snapshot.Coop.Acked != 2 {
		t.Fatalf("mark: %d %+v %+v", c, marked, marked.Snapshot.Coop)
	}
	// A whole board's worth of operations fits into one request.
	big := make([][4]int, 50)
	for i := range big {
		big[i] = [4]int{100 + i, i % 25, 0, 1}
	}
	if c := postJSON(t, h, loc+"/mark", map[string]any{"token": joined.Token, "round": 1, "ops": big}, nil); c != http.StatusOK {
		t.Fatalf("big mark: %d", c)
	}
	var voted struct{ Snapshot party.Snapshot }
	if c := postJSON(t, h, loc+"/vote", map[string]any{"token": joined.Token, "round": 1, "giveUp": true}, &voted); c != http.StatusOK || voted.Snapshot.Phase != party.PhaseResults {
		t.Fatalf("vote: %d %+v", c, voted.Snapshot)
	}
}

func TestClockServer(t *testing.T) {
	pm, clk := newTestParties(t) // the round does not end on time by itself
	h := Handler(testSolve, nil, pm, nil, nil)

	form := url.Values{"n": {"5"}, "mode": {"clock"}, "minutes": {"2"}, "skips": {"-1"}}
	req := httptest.NewRequest("POST", "/party", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	code := strings.TrimPrefix(rec.Header().Get("Location"), "/party/")
	s := snap(t, pm, code, "")
	if s.Mode != party.ModeClock || s.Clock.Minutes != 2 || s.Clock.Skips != -1 {
		t.Fatalf("created %+v, clock %+v", s, s.Clock)
	}
	waitPrepared(t, pm, code)

	var joined struct {
		Token string `json:"token"`
	}
	if st := postJSON(t, h, "/party/"+code+"/join", map[string]any{"name": "Ada"}, &joined); st != http.StatusOK {
		t.Fatalf("join: %d", st)
	}
	tok := joined.Token
	if st := postJSON(t, h, "/party/"+code+"/ready", map[string]any{"token": tok, "ready": true}, nil); st != http.StatusOK {
		t.Fatalf("ready: %d", st)
	}
	clk.add(party.Countdown)

	get := func(path string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec.Code
	}
	for path, want := range map[string]int{
		"/party/" + code + "/round/1":   http.StatusOK,
		"/party/" + code + "/round/1/1": http.StatusOK,
		"/party/" + code + "/round/1/2": http.StatusNotFound,
		"/party/" + code + "/round/1/0": http.StatusNotFound,
		"/party/" + code + "/round/1/x": http.StatusNotFound,
	} {
		if got := get(path); got != want {
			t.Errorf("GET %s: %d, want %d", path, got, want)
		}
	}

	// The page shows the puzzle, but neither its solution nor the trace.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/party/"+code+"/round/1/1", nil))
	d := pageData(t, rec.Body.String())
	if d.Solution != nil || len(d.Steps) != 0 || d.Status != "pending" || d.Party.Puzzle != 1 || d.Party.Mode != party.ModeClock {
		t.Fatalf("page data %+v, party %+v", d, d.Party)
	}
	if !strings.Contains(d.Title, "Puzzle 1") {
		t.Errorf("title %q", d.Title)
	}

	rp := testPuzzle(t, 5)
	var out struct {
		Snapshot party.Snapshot `json:"snapshot"`
	}
	if st := postJSON(t, h, "/party/"+code+"/progress", map[string]any{"token": tok, "round": 1, "puzzle": 1, "cells": solutionCells(rp)[:3]}, &out); st != http.StatusOK {
		t.Fatalf("progress: %d", st)
	}
	if out.Snapshot.Ranking[0].Stars != 3 {
		t.Fatalf("stars %+v", out.Snapshot.Ranking[0])
	}
	if st := postJSON(t, h, "/party/"+code+"/solve", map[string]any{"token": tok, "round": 1, "puzzle": 1, "stars": rp.Solution}, &out); st != http.StatusOK {
		t.Fatalf("solve: %d", st)
	}
	waitFor(t, func() bool { return len(snap(t, pm, code, "").Clock.Sizes) == 2 })
	if st := postJSON(t, h, "/party/"+code+"/skip", map[string]any{"token": tok, "round": 1, "puzzle": 2}, &out); st != http.StatusOK {
		t.Fatalf("skip: %d", st)
	}
	if e := out.Snapshot.Ranking[0]; e.Solved != 1 || e.Skips != 1 || e.Puzzle != 3 {
		t.Fatalf("after solve and skip: %+v", e)
	}
	if got := get("/party/" + code + "/round/1/2"); got != http.StatusOK {
		t.Fatalf("puzzle 2: %d", got)
	}
}

func TestLivesServer(t *testing.T) {
	pm, clk := newTestParties(t)
	h := Handler(testSolve, nil, pm, nil, nil)

	form := url.Values{"n": {"5"}, "mode": {"versus"}, "lives": {"2"}}
	req := httptest.NewRequest("POST", "/party", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	loc := rec.Header().Get("Location")
	code := strings.TrimPrefix(loc, "/party/")
	if s := snap(t, pm, code, ""); s.Lives != 2 {
		t.Fatalf("lives %d, want 2", s.Lives)
	}
	waitPrepared(t, pm, code)
	var joined struct {
		Token string `json:"token"`
	}
	if c := postJSON(t, h, loc+"/join", map[string]any{"name": "Ada"}, &joined); c != http.StatusOK {
		t.Fatalf("join: %d", c)
	}
	if c := postJSON(t, h, loc+"/ready", map[string]any{"token": joined.Token, "ready": true}, nil); c != http.StatusOK {
		t.Fatalf("ready: %d", c)
	}
	clk.add(party.Countdown)
	if d := pageData(t, get(h, loc+"/round/1").Body.String()); d.Party == nil || d.Party.Lives != 2 {
		t.Fatalf("round page data: %+v", d.Party)
	}
	var out struct{ Snapshot party.Snapshot }
	if c := postJSON(t, h, loc+"/progress", map[string]any{"token": joined.Token, "round": 1, "count": 1, "errors": 2}, &out); c != http.StatusOK {
		t.Fatalf("progress: %d", c)
	}
	if r := out.Snapshot.Ranking; len(r) != 1 || r[0].Status != "out" || r[0].Errors != 2 || out.Snapshot.Phase != party.PhaseResults {
		t.Fatalf("after the last life: %+v", out.Snapshot)
	}
}

// /party leads a member back to their party's lobby (PartyCookie); anyone
// else, a wrong token, a party that is gone, or garbage gets the page to
// create or join one, and so does a player after leaving.
func TestPartyCookie(t *testing.T) {
	pm, _ := newTestParties(t)
	h := Handler(testSolve, nil, pm, nil, nil)
	code, err := pm.Create(party.Options{Sizes: []int{5}})
	must(t, err)
	tok := mustJoin(t, pm, code, "Anna")
	at := func(path, cookie string) (int, string) {
		req := httptest.NewRequest("GET", path, nil)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: PartyCookie, Value: cookie})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code, rec.Header().Get("Location")
	}
	if st, loc := at("/party", code+"."+tok); st != http.StatusFound || loc != "/party/"+code {
		t.Errorf("member: %d to %q, want the lobby", st, loc)
	}
	for _, cookie := range []string{"", code + ".", code + ".nope", "ZZZZ." + tok, strings.ToLower(code) + "." + tok, code + tok, "%zz", "." + tok} {
		if st, loc := at("/party", cookie); st != http.StatusOK || loc != "" {
			t.Errorf("cookie %q: %d to %q, want the party page", cookie, st, loc)
		}
	}
	must(t, pm.Leave(code, tok))
	if st, loc := at("/party", code+"."+tok); st != http.StatusOK || loc != "" {
		t.Errorf("after leaving: %d to %q, want the party page", st, loc)
	}
}
