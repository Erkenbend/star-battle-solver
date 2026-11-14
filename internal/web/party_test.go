package web

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
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
	testPuzzles   = map[int]*RoundPuzzle{}
)

// testPuzzle returns a generated and solved n×n puzzle, the same for all tests.
func testPuzzle(t *testing.T, n int) *RoundPuzzle {
	testPuzzlesMu.Lock()
	defer testPuzzlesMu.Unlock()
	if rp := testPuzzles[n]; rp != nil {
		return rp
	}
	for seed := uint64(1); ; seed++ {
		p, sol, err := generate(n, seed)
		if err != nil || sol == nil {
			continue
		}
		res, _ := testSolve(p)
		rp := &RoundPuzzle{n, seed, p, sol, res}
		testPuzzles[n] = rp
		return rp
	}
}

func newTestParties(t *testing.T) (*Parties, *testClock) {
	t.Helper()
	clk := &testClock{t: time.Date(2026, 11, 7, 20, 0, 0, 0, time.UTC)}
	pm := NewParties(testSolve, nil)
	pm.now = clk.now
	pm.retryDelay = 0
	pm.prepare = func(n int) (*RoundPuzzle, error) { return testPuzzle(t, n), nil }
	return pm, clk
}

// waitPrepared waits until the party has its next puzzle ready.
func waitPrepared(t *testing.T, pm *Parties, code string) {
	t.Helper()
	waitFor(t, func() bool {
		pm.mu.Lock()
		defer pm.mu.Unlock()
		return len(pm.parties[code].ready) > 0
	})
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

func mustJoin(t *testing.T, pm *Parties, code, name string) string {
	t.Helper()
	tok, _, err := pm.Join(code, name, "")
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func wantStatus(t *testing.T, err error, status int) {
	t.Helper()
	var pe *PartyError
	if !errors.As(err, &pe) || pe.Status != status {
		t.Fatalf("got error %v, want status %d", err, status)
	}
}

func snap(t *testing.T, pm *Parties, code, tok string) Snapshot {
	t.Helper()
	s, err := pm.Snapshot(code, tok)
	must(t, err)
	return s
}

// startRound creates a 5×5 party with the named players, all ready, and
// moves the clock past the countdown.
func startRound(t *testing.T, pm *Parties, clk *testClock, names ...string) (string, []string) {
	t.Helper()
	return startRoundMode(t, pm, clk, ModeVersus, names...)
}

// startRoundMode is startRound for a party in the given mode.
func startRoundMode(t *testing.T, pm *Parties, clk *testClock, mode string, names ...string) (string, []string) {
	t.Helper()
	code, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: mode})
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
	clk.add(partyCountdown)
	return code, toks
}

func TestPartyRound(t *testing.T) {
	pm, clk := newTestParties(t)
	code, err := pm.Create(PartyOptions{Sizes: []int{5, 5, 3, 99}, Mode: ""})
	must(t, err)
	waitPrepared(t, pm, code)
	a := mustJoin(t, pm, code, "  Anna   Lena ")
	b := mustJoin(t, pm, code, "Ben")
	s := snap(t, pm, code, a)
	if s.Phase != PhaseLobby || len(s.Players) != 2 || s.Players[0].Name != "Anna Lena" || s.You != s.Players[0].ID || len(s.Sizes) != 1 {
		t.Fatalf("lobby snapshot %+v", s)
	}

	must(t, pm.SetReady(code, a, true))
	if s := snap(t, pm, code, ""); s.Phase != PhaseLobby {
		t.Fatal("started before everybody was ready")
	}
	must(t, pm.SetReady(code, b, true))
	s = snap(t, pm, code, "")
	if s.Phase != PhasePlaying || s.Round != 1 || s.N != 5 || s.StartInMs != partyCountdown.Milliseconds() {
		t.Fatalf("round snapshot %+v", s)
	}
	wantStatus(t, pm.SetReady(code, a, true), http.StatusConflict)

	sol := testPuzzle(t, 5).Solution
	_, err = pm.Solve(code, a, 1, sol, 0)
	wantStatus(t, err, http.StatusConflict) // countdown still running

	clk.add(partyCountdown + time.Second)
	if s := snap(t, pm, code, ""); s.ElapsedMs != 1000 || s.StartInMs != 0 {
		t.Fatalf("elapsed %d, start in %d", s.ElapsedMs, s.StartInMs)
	}
	wrong := append([]int(nil), sol...)
	wrong[0], wrong[1] = wrong[1], wrong[0]
	_, err = pm.Solve(code, a, 1, wrong, 0)
	wantStatus(t, err, http.StatusBadRequest)
	_, err = pm.Solve(code, a, 2, sol, 0)
	wantStatus(t, err, http.StatusConflict)

	res, err := pm.Solve(code, a, 1, sol, 1)
	must(t, err)
	if !res.Solved || res.Hints != 1 || res.TimeMs != 1000 {
		t.Fatalf("result %+v", res)
	}
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying || s.Ranking[1].Status != "playing" {
		t.Fatalf("round ended while Ben still plays: %+v", s)
	}
	must(t, pm.Progress(code, b, 1, 3, 0))
	clk.add(2 * time.Second)
	_, err = pm.Solve(code, b, 1, sol, 0)
	must(t, err)

	s = snap(t, pm, code, "")
	if s.Phase != PhaseResults {
		t.Fatalf("phase %s, want results", s.Phase)
	}
	// Fewer hints beat a faster time.
	if r := s.Ranking; r[0].Name != "Ben" || r[0].Rank != 1 || r[0].TimeMs != 3000 || r[1].Name != "Anna Lena" || r[1].Rank != 2 {
		t.Fatalf("ranking %+v", r)
	}
	// Repeating a solve returns the recorded result.
	if res2, err := pm.Solve(code, a, 1, sol, 5); err != nil || res2 != res {
		t.Fatalf("repeated solve: %+v, %v", res2, err)
	}

	waitPrepared(t, pm, code)
	must(t, pm.SetReady(code, a, true))
	must(t, pm.SetReady(code, b, true))
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying || s.Round != 2 || len(s.Ranking) != 2 || s.Ranking[0].Status != "playing" {
		t.Fatalf("round 2: %+v", s)
	}
}

func TestPartyGiveUp(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRound(t, pm, clk, "A", "B", "C")
	sol := testPuzzle(t, 5).Solution
	must(t, pm.GiveUp(code, toks[0], 1, 2))
	must(t, pm.GiveUp(code, toks[0], 1, 2)) // repeated: fine
	_, err := pm.Solve(code, toks[0], 1, sol, 0)
	wantStatus(t, err, http.StatusConflict)
	clk.add(30 * time.Second)
	_, err = pm.Solve(code, toks[1], 1, sol, 4)
	must(t, err)
	wantStatus(t, pm.GiveUp(code, toks[1], 1, 0), http.StatusConflict)
	must(t, pm.GiveUp(code, toks[2], 1, 0))
	s := snap(t, pm, code, "")
	if s.Phase != PhaseResults {
		t.Fatal("round did not end")
	}
	r := s.Ranking
	if r[0].Name != "B" || r[0].Rank != 1 || r[1].Status != "gaveup" || r[1].Rank != 2 || r[2].Status != "gaveup" || r[2].Rank != 3 {
		t.Fatalf("ranking %+v", r)
	}
}

func TestPartyHintsNeverDecrease(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRound(t, pm, clk, "A", "B")
	must(t, pm.Progress(code, toks[0], 1, 1, 3))
	must(t, pm.Progress(code, toks[0], 1, 2, 1)) // e.g. after cleared storage
	if h := snap(t, pm, code, "").Ranking[0].Hints; h != 3 {
		t.Fatalf("hints %d, want 3", h)
	}
	res, err := pm.Solve(code, toks[0], 1, testPuzzle(t, 5).Solution, 0)
	must(t, err)
	if res.Hints != 3 {
		t.Fatalf("result hints %d, want 3", res.Hints)
	}
}

func TestPartyAbsence(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRound(t, pm, clk, "A", "B")
	_, cancel, err := pm.Subscribe(code, toks[0]) // A keeps a stream open
	must(t, err)
	defer cancel()
	clk.add(partyAbsentAfter + time.Second)
	pm.Tick()
	s := snap(t, pm, code, "")
	if !s.Players[0].Present || s.Players[1].Present || s.Ranking[1].Status != "absent" {
		t.Fatalf("presence %+v", s)
	}
	sol := testPuzzle(t, 5).Solution
	_, err = pm.Solve(code, toks[0], 1, sol, 0)
	must(t, err)
	if s := snap(t, pm, code, ""); s.Phase != PhaseResults {
		t.Fatal("absent player blocks the end of the round")
	}
	_, err = pm.Solve(code, toks[1], 1, sol, 0)
	wantStatus(t, err, http.StatusConflict)

	// The absent player does not block the start either, until back.
	waitPrepared(t, pm, code)
	must(t, pm.Progress(code, toks[1], 1, 0, 0)) // B is back (any request)
	must(t, pm.SetReady(code, toks[0], true))
	if s := snap(t, pm, code, ""); s.Phase != PhaseResults {
		t.Fatal("started without B being ready")
	}
	clk.add(partyAbsentAfter + time.Second)
	pm.Tick()
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying || len(s.Ranking) != 1 {
		t.Fatalf("round 2 should start with A alone: %+v", s)
	}
}

func TestPartyAllAbsent(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRound(t, pm, clk, "A")
	clk.add(partyAbsentAfter + time.Second)
	pm.Tick()
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying {
		t.Fatal("round ended with nobody there")
	}
	must(t, pm.Progress(code, toks[0], 1, 1, 0))
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying || !s.Players[0].Present {
		t.Fatalf("after return: %+v", s)
	}
	_, err := pm.Solve(code, toks[0], 1, testPuzzle(t, 5).Solution, 0)
	must(t, err)
	if s := snap(t, pm, code, ""); s.Phase != PhaseResults {
		t.Fatal("round did not end")
	}
}

func TestPartyLateJoin(t *testing.T) {
	pm, clk := newTestParties(t)
	code, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: ""})
	must(t, err)
	waitPrepared(t, pm, code)
	a := mustJoin(t, pm, code, "A")
	must(t, pm.SetReady(code, a, true))
	clk.add(time.Second)
	b := mustJoin(t, pm, code, "B") // during the countdown: plays along
	clk.add(partyCountdown)
	c := mustJoin(t, pm, code, "C") // too late: waits for the next round
	s := snap(t, pm, code, "")
	if len(s.Ranking) != 2 || !s.Players[1].Playing || s.Players[2].Playing {
		t.Fatalf("participants %+v", s)
	}
	wantStatus(t, pm.Progress(code, c, 1, 1, 0), http.StatusConflict)
	sol := testPuzzle(t, 5).Solution
	for _, tok := range []string{a, b} {
		_, err := pm.Solve(code, tok, 1, sol, 0)
		must(t, err)
	}
	if s := snap(t, pm, code, ""); s.Phase != PhaseResults {
		t.Fatal("C blocks the end of the round")
	}
}

func TestPartyOnlyLateJoinerPresent(t *testing.T) {
	pm, clk := newTestParties(t)
	code, _ := startRound(t, pm, clk, "A")
	clk.add(partyAbsentAfter + time.Second)
	c := mustJoin(t, pm, code, "C")
	pm.Tick()
	// A is away without a result; C should not wait forever.
	s := snap(t, pm, code, c)
	if s.Phase != PhaseResults || s.Ranking[0].Status != "absent" {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestPartyLeave(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRound(t, pm, clk, "A", "B")
	must(t, pm.Leave(code, toks[1]))
	wantStatus(t, pm.Leave(code, toks[1]), http.StatusForbidden)
	_, err := pm.Solve(code, toks[0], 1, testPuzzle(t, 5).Solution, 0)
	must(t, err)
	s := snap(t, pm, code, "")
	if s.Phase != PhaseResults || len(s.Players) != 1 || s.Ranking[1].Status != "left" || s.Ranking[1].Name != "B" {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestPartyRejoinAndLimits(t *testing.T) {
	pm, _ := newTestParties(t)
	_, err := pm.Create(PartyOptions{Sizes: []int{2, 15}, Mode: ""})
	wantStatus(t, err, http.StatusBadRequest)
	code, err := pm.Create(PartyOptions{Sizes: []int{6}, Mode: ""})
	must(t, err)
	_, err = pm.Snapshot("nope", "")
	wantStatus(t, err, http.StatusNotFound)
	_, _, err = pm.Join(code, "   ", "")
	wantStatus(t, err, http.StatusBadRequest)

	tok, id, err := pm.Join(strings.ToLower(code), strings.Repeat("x", 30), "")
	must(t, err)
	tok2, id2, err := pm.Join(code, "", tok)
	must(t, err)
	if tok2 != tok || id2 != id {
		t.Fatal("rejoin gave a new player")
	}
	s := snap(t, pm, code, tok)
	if len(s.Players) != 1 || len([]rune(s.Players[0].Name)) != PartyMaxName {
		t.Fatalf("players %+v", s.Players)
	}
	for i := 1; i < PartyMaxPlayers; i++ {
		mustJoin(t, pm, code, "P")
	}
	_, _, err = pm.Join(code, "one too many", "")
	wantStatus(t, err, http.StatusConflict)
	wantStatus(t, pm.SetReady(code, "bogus", true), http.StatusForbidden)
}

func TestPartyExpiry(t *testing.T) {
	pm, clk := newTestParties(t)
	code, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: ""})
	must(t, err)
	tok := mustJoin(t, pm, code, "A")
	ch, cancel, err := pm.Subscribe(code, tok)
	must(t, err)
	clk.add(partyIdleExpire + time.Minute)
	pm.Tick()
	if _, err := pm.Snapshot(code, ""); err != nil {
		t.Fatal("party with an open stream expired")
	}
	cancel()
	pm.Tick()
	_, err = pm.Snapshot(code, "")
	wantStatus(t, err, http.StatusNotFound)
	<-ch // the stream was told
}

func TestPartyPrepareAfterReady(t *testing.T) {
	pm, _ := newTestParties(t)
	gate := make(chan struct{})
	pm.prepare = func(n int) (*RoundPuzzle, error) {
		<-gate
		return testPuzzle(t, n), nil
	}
	code, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: ""})
	must(t, err)
	a := mustJoin(t, pm, code, "A")
	must(t, pm.SetReady(code, a, true))
	if s := snap(t, pm, code, ""); s.Phase != PhaseLobby || !s.Preparing {
		t.Fatalf("snapshot %+v", s)
	}
	gate <- struct{}{}
	waitFor(t, func() bool { return snap(t, pm, code, "").Phase == PhasePlaying })
	close(gate) // the preparation for round 2 may proceed
}

func TestPartyPrepareRetries(t *testing.T) {
	pm, _ := newTestParties(t)
	var mu sync.Mutex
	fails := 2
	pm.prepare = func(n int) (*RoundPuzzle, error) {
		mu.Lock()
		defer mu.Unlock()
		if fails > 0 {
			fails--
			return nil, errors.New("boom")
		}
		return testPuzzle(t, n), nil
	}
	code, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: ""})
	must(t, err)
	waitPrepared(t, pm, code)
	if s := snap(t, pm, code, ""); s.PrepErr != "" || s.Preparing {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestPartySubscribe(t *testing.T) {
	pm, _ := newTestParties(t)
	code, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: ""})
	must(t, err)
	ch, cancel, err := pm.Subscribe(code, "")
	must(t, err)
	mustJoin(t, pm, code, "A")
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("no signal after a join")
	}
	cancel()
	pm.mu.Lock()
	n := len(pm.parties[code].subs)
	pm.mu.Unlock()
	if n != 0 {
		t.Fatal("subscriber not removed")
	}
}

func TestPartyRoundData(t *testing.T) {
	pm, clk := newTestParties(t)
	code, _ := startRound(t, pm, clk, "A")
	_, err := pm.RoundData(code, 2, 0)
	wantStatus(t, err, http.StatusNotFound)
	d, err := pm.RoundData(code, 1, 0)
	must(t, err)
	if d.Party == nil || d.Party.Code != code || d.Party.Round != 1 || d.N != 5 || !d.Served || len(d.Solution) != 5 {
		t.Fatalf("data %+v", d)
	}
}
