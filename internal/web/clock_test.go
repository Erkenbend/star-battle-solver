package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// startClock starts a 5×5 round against the clock with the named players,
// all ready, and moves the clock past the countdown. It returns the code,
// the tokens and the function that ends the round on time.
func startClock(t *testing.T, pm *Parties, clk *testClock, rules ClockRules, names ...string) (string, []string, func()) {
	t.Helper()
	var mu sync.Mutex
	var onTime func()
	var wait time.Duration
	pm.after = func(d time.Duration, f func()) {
		mu.Lock()
		defer mu.Unlock()
		wait, onTime = d, f
	}
	code, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: ModeClock, Clock: rules})
	must(t, err)
	waitPrepared(t, pm, code)
	var toks []string
	for _, name := range names {
		toks = append(toks, mustJoin(t, pm, code, name))
	}
	for _, tok := range toks {
		must(t, pm.SetReady(code, tok, true))
	}
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying {
		t.Fatalf("phase %s, want playing", s.Phase)
	}
	mu.Lock()
	if want := partyCountdown + time.Duration(rules.Minutes)*time.Minute; wait != want || onTime == nil {
		t.Fatalf("round ends after %v, want %v", wait, want)
	}
	f := onTime
	mu.Unlock()
	clk.add(partyCountdown)
	return code, toks, f
}

// solutionCells returns the cells of the stars of rp's solution.
func solutionCells(rp *RoundPuzzle) []int {
	var out []int
	for r, c := range rp.Solution {
		out = append(out, r*rp.N+c)
	}
	return out
}

func entry(t *testing.T, s Snapshot, id string) RankEntry {
	t.Helper()
	for _, e := range s.Ranking {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("%s not in the ranking %+v", id, s.Ranking)
	return RankEntry{}
}

func TestClockRules(t *testing.T) {
	for _, c := range []struct {
		in   ClockRules
		mode string
		want ClockRules
	}{
		{ClockRules{2, 1}, ModeClock, ClockRules{2, 1}},
		{ClockRules{10, -1}, ModeClock, ClockRules{10, -1}},
		{ClockRules{7, -5}, ModeClock, ClockRules{ClockDefaultMinutes, -1}},
		{ClockRules{0, 500}, ModeClock, ClockRules{ClockDefaultMinutes, ClockMaxSkips}},
		{ClockRules{5, 2}, ModeVersus, ClockRules{}},
	} {
		if got := c.in.normalize(c.mode); got != c.want {
			t.Errorf("%+v in %s: got %+v, want %+v", c.in, c.mode, got, c.want)
		}
	}

	pm, _ := newTestParties(t)
	code, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: ModeClock, Clock: ClockRules{10, 3}})
	must(t, err)
	s := snap(t, pm, code, "")
	if s.Mode != ModeClock || s.Clock == nil || s.Clock.Minutes != 10 || s.Clock.Skips != 3 || len(s.Clock.Sizes) != 0 {
		t.Fatalf("snapshot %+v, clock %+v", s, s.Clock)
	}
	// A few puzzles are prepared ahead, not just one.
	waitFor(t, func() bool {
		pm.mu.Lock()
		defer pm.mu.Unlock()
		p := pm.parties[code]
		return len(p.ready) == clockAhead && !p.prepping
	})
	if s := snap(t, pm, code, ""); s.Clock == nil {
		t.Fatal("no rules in the snapshot")
	}
	versus, err := pm.Create(PartyOptions{Sizes: []int{5}})
	must(t, err)
	if s := snap(t, pm, versus, ""); s.Clock != nil {
		t.Fatalf("versus party with clock %+v", s.Clock)
	}
}

func TestClockRound(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks, onTime := startClock(t, pm, clk, ClockRules{2, 1}, "Ada", "Bob")
	a, b := toks[0], toks[1]
	s := snap(t, pm, code, a)
	ida := s.You
	idb := snap(t, pm, code, b).You
	rp := testPuzzle(t, 5)
	if e := entry(t, s, ida); e.Puzzle != 1 || e.Solved != 0 || e.Status != "playing" {
		t.Fatalf("at the start: %+v", e)
	}
	if len(s.Clock.Sizes) != 1 || s.Clock.Sizes[0] != 5 {
		t.Fatalf("puzzles %v", s.Clock.Sizes)
	}

	// The page shows the puzzle, but neither its solution nor the trace.
	d, err := pm.RoundData(code, 1, 1)
	must(t, err)
	if d.Solution != nil || len(d.Steps) != 0 || d.Status != "pending" || d.Party.Puzzle != 1 || d.Party.Mode != ModeClock {
		t.Fatalf("page data %+v, party %+v", d, d.Party)
	}
	if !strings.Contains(d.Title, "Puzzle 1") {
		t.Errorf("title %q", d.Title)
	}
	if _, err := pm.RoundData(code, 1, 2); err == nil {
		t.Fatal("puzzle 2 before anybody got there")
	}

	// Versus actions do not apply.
	wantStatus(t, pm.Progress(code, a, 1, 2, 0), http.StatusConflict)
	_, err = pm.Solve(code, a, 1, rp.Solution, 0)
	wantStatus(t, err, http.StatusConflict)
	wantStatus(t, pm.GiveUp(code, a, 1, 0), http.StatusConflict)

	// Ada solves the first puzzle and gets the second.
	wrong := append([]int{}, rp.Solution...)
	wrong[0], wrong[1] = wrong[1], wrong[0]
	wantStatus(t, pm.ClockSolve(code, a, 1, 1, wrong), http.StatusBadRequest)
	must(t, pm.ClockSolve(code, a, 1, 1, rp.Solution))
	must(t, pm.ClockSolve(code, a, 1, 1, rp.Solution)) // repeated: harmless
	wantStatus(t, pm.ClockSolve(code, a, 1, 3, rp.Solution), http.StatusConflict)
	s = snap(t, pm, code, a)
	if e := entry(t, s, ida); e.Puzzle != 2 || e.Solved != 1 {
		t.Fatalf("after solving: %+v", e)
	}
	if len(s.Clock.Sizes) != 2 {
		t.Fatalf("puzzles %v, want 2", s.Clock.Sizes)
	}
	if _, err := pm.RoundData(code, 1, 2); err != nil {
		t.Fatal(err)
	}

	// Bob's stars: two right, one wrong.
	sol := solutionCells(rp)
	bad := sol[0] + 1
	if rp.Solution[0] == rp.N-1 {
		bad = sol[0] - 1
	}
	must(t, pm.ClockProgress(code, b, 1, 1, []int{sol[0], sol[1], bad, sol[1]}))
	wantStatus(t, pm.ClockProgress(code, b, 1, 1, []int{25}), http.StatusBadRequest)
	if e := entry(t, snap(t, pm, code, b), idb); e.Stars != 1 {
		t.Fatalf("Bob's stars %d, want 1", e.Stars)
	}
	// Ada leads with a solved puzzle; Bob's stars do not beat that.
	s = snap(t, pm, code, b)
	if s.Ranking[0].ID != ida || s.Ranking[0].Rank != 1 || s.Ranking[1].Rank != 2 {
		t.Fatalf("ranking %+v", s.Ranking)
	}

	// One skip each.
	must(t, pm.ClockSkip(code, b, 1, 1))
	wantStatus(t, pm.ClockSkip(code, b, 1, 2), http.StatusConflict)
	if e := entry(t, snap(t, pm, code, b), idb); e.Puzzle != 2 || e.Skips != 1 || e.Stars != 0 {
		t.Fatalf("Bob after skipping: %+v", e)
	}

	// Ada sets stars on puzzle 2 and the time runs out.
	must(t, pm.ClockProgress(code, a, 1, 2, sol[:1]))
	clk.add(2 * time.Minute)
	onTime()
	s = snap(t, pm, code, a)
	if s.Phase != PhaseResults {
		t.Fatalf("phase %s after the deadline", s.Phase)
	}
	// Reports arriving a little late still count, skips do not.
	must(t, pm.ClockProgress(code, a, 1, 2, sol[:2]))
	wantStatus(t, pm.ClockSkip(code, a, 1, 2), http.StatusConflict)
	s = snap(t, pm, code, a)
	if e := s.Ranking[0]; e.ID != ida || e.Status != "done" || e.Solved != 1 || e.Stars != 2 {
		t.Fatalf("winner %+v", e)
	}
	clk.add(clockGrace)
	wantStatus(t, pm.ClockProgress(code, a, 1, 2, sol), http.StatusConflict)

	// Ready for the next round, as in the other modes (Bob first: by now he
	// counts as absent, and Ada alone would start without him).
	must(t, pm.SetReady(code, b, true))
	must(t, pm.SetReady(code, a, true))
	if s := snap(t, pm, code, a); s.Phase != PhasePlaying || s.Round != 2 || len(s.Clock.Sizes) != 1 {
		t.Fatalf("round 2: phase %s, round %d, clock %+v", s.Phase, s.Round, s.Clock)
	}
}

func TestClockTies(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks, _ := startClock(t, pm, clk, ClockRules{5, -1}, "Ada", "Bob", "Cy")
	rp := testPuzzle(t, 5)
	sol := solutionCells(rp)
	must(t, pm.ClockProgress(code, toks[0], 1, 1, sol[:2]))
	must(t, pm.ClockProgress(code, toks[1], 1, 1, sol[3:]))
	for k := 1; k <= 5; k++ { // any number of skips
		waitFor(t, func() bool { return len(snap(t, pm, code, "").Clock.Sizes) >= k })
		must(t, pm.ClockSkip(code, toks[2], 1, k))
	}
	s := snap(t, pm, code, "")
	var ranks []int
	for _, e := range s.Ranking {
		ranks = append(ranks, e.Rank)
	}
	if len(ranks) != 3 || ranks[0] != 1 || ranks[1] != 1 || ranks[2] != 3 || s.Ranking[2].Skips != 5 {
		t.Fatalf("ranking %+v", s.Ranking)
	}
}

// A player faster than the generator waits for the next puzzle.
func TestClockWaitsForPuzzle(t *testing.T) {
	pm, clk := newTestParties(t)
	gate := make(chan struct{})
	defer close(gate)
	pm.prepare = func(n int) (*RoundPuzzle, error) {
		<-gate
		return testPuzzle(t, n), nil
	}
	pm.after = func(time.Duration, func()) {}
	code, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: ModeClock, Clock: ClockRules{5, 0}})
	must(t, err)
	gate <- struct{}{}
	waitPrepared(t, pm, code)
	a := mustJoin(t, pm, code, "Ada")
	must(t, pm.SetReady(code, a, true))
	clk.add(partyCountdown)
	rp := testPuzzle(t, 5)
	must(t, pm.ClockSolve(code, a, 1, 1, rp.Solution))
	s := snap(t, pm, code, a)
	if e := entry(t, s, s.You); e.Puzzle != 2 || len(s.Clock.Sizes) != 1 {
		t.Fatalf("waiting: %+v, puzzles %v", e, s.Clock.Sizes)
	}
	wantStatus(t, pm.ClockSolve(code, a, 1, 2, rp.Solution), http.StatusConflict)
	wantStatus(t, pm.ClockSkip(code, a, 1, 2), http.StatusConflict)
	gate <- struct{}{}
	waitFor(t, func() bool { return len(snap(t, pm, code, a).Clock.Sizes) == 2 })
	must(t, pm.ClockSolve(code, a, 1, 2, rp.Solution))
}

func TestClockLateJoin(t *testing.T) {
	pm, clk := newTestParties(t)
	code, _, _ := startClock(t, pm, clk, ClockRules{5, 0}, "Ada")
	clk.add(time.Minute)
	c := mustJoin(t, pm, code, "Cy")
	s := snap(t, pm, code, c)
	if e := entry(t, s, s.You); e.Status != "playing" || e.Puzzle != 1 {
		t.Fatalf("late joiner %+v", e)
	}
	must(t, pm.ClockSolve(code, c, 1, 1, testPuzzle(t, 5).Solution))
}

func TestClockOnlyInClockMode(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRound(t, pm, clk, "Ada")
	wantStatus(t, pm.ClockSolve(code, toks[0], 1, 1, testPuzzle(t, 5).Solution), http.StatusConflict)
	wantStatus(t, pm.ClockSkip(code, toks[0], 1, 1), http.StatusConflict)
}

func TestClockServer(t *testing.T) {
	pm, clk := newTestParties(t)
	pm.after = func(time.Duration, func()) {}
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
	if s.Mode != ModeClock || s.Clock.Minutes != 2 || s.Clock.Skips != -1 {
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
	clk.add(partyCountdown)

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

	rp := testPuzzle(t, 5)
	var out struct {
		Snapshot Snapshot `json:"snapshot"`
	}
	if st := postJSON(t, h, "/party/"+code+"/progress", map[string]any{"token": tok, "round": 1, "puzzle": 1, "cells": solutionCells(rp)[:3]}, &out); st != http.StatusOK {
		t.Fatalf("progress: %d", st)
	}
	if out.Snapshot.Ranking[0].Stars != 3 {
		t.Fatalf("stars %+v", out.Snapshot.Ranking[0])
	}
	var checked struct {
		Correct bool `json:"correct"`
	}
	if st := postJSON(t, h, "/party/"+code+"/check", map[string]any{"token": tok, "round": 1, "puzzle": 1, "board": boardOf(rp, solutionCells(rp)[:3], nil)}, &checked); st != http.StatusOK || !checked.Correct {
		t.Fatalf("check: %d, %+v", st, checked)
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

// boardOf returns rp's board with the given stars and crosses, as sent by "Prüfen".
func boardOf(rp *RoundPuzzle, stars, crosses []int) string {
	b := []byte(strings.Repeat("0", rp.N*rp.N))
	for _, i := range stars {
		b[i] = '2'
	}
	for _, i := range crosses {
		b[i] = '1'
	}
	return string(b)
}

func TestClockCheck(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks, onTime := startClock(t, pm, clk, ClockRules{2, 0}, "Ada")
	a := toks[0]
	rp := testPuzzle(t, 5)
	sol := solutionCells(rp)
	other := (sol[0] + 1) % 5 // a cell of row 0 without a star

	for _, c := range []struct {
		name           string
		stars, crosses []int
		want           bool
	}{
		{"empty", nil, nil, true},
		{"right star and cross", sol[:2], []int{other}, true},
		{"star off the solution", []int{other}, nil, false},
		{"cross on a star", nil, sol[:1], false},
		{"solved", sol, nil, true},
	} {
		ok, err := pm.ClockCheck(code, a, 1, 1, boardOf(rp, c.stars, c.crosses))
		must(t, err)
		if ok != c.want {
			t.Errorf("%s: got %v, want %v", c.name, ok, c.want)
		}
	}
	for _, bad := range []string{"", "0", strings.Repeat("0", 24) + "3"} {
		_, err := pm.ClockCheck(code, a, 1, 1, bad)
		wantStatus(t, err, http.StatusBadRequest)
	}
	_, err := pm.ClockCheck(code, a, 1, 2, boardOf(rp, nil, nil))
	wantStatus(t, err, http.StatusConflict) // not there yet

	must(t, pm.ClockSolve(code, a, 1, 1, rp.Solution))
	_, err = pm.ClockCheck(code, a, 1, 1, boardOf(rp, nil, nil))
	wantStatus(t, err, http.StatusConflict) // done with it

	clk.add(2 * time.Minute)
	onTime()
	_, err = pm.ClockCheck(code, a, 1, 2, boardOf(rp, nil, nil))
	wantStatus(t, err, http.StatusConflict) // time is up, no grace for checking

	// Only against the clock.
	vcode, vtoks := startRound(t, pm, clk, "Bob")
	_, err = pm.ClockCheck(vcode, vtoks[0], 1, 1, boardOf(rp, nil, nil))
	wantStatus(t, err, http.StatusConflict)
}
