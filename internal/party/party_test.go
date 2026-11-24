package party

import (
	"errors"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"starbattle/internal/board"
	"starbattle/internal/generator"
	"starbattle/internal/solver"
	"starbattle/internal/solver/observers"
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
	testPuzzles   = map[int]*Puzzle{}
)

// testPuzzle returns a generated and solved n×n puzzle, the same for all tests.
func testPuzzle(t *testing.T, n int) *Puzzle {
	testPuzzlesMu.Lock()
	defer testPuzzlesMu.Unlock()
	if rp := testPuzzles[n]; rp != nil {
		return rp
	}
	for seed := uint64(1); ; seed++ {
		p, sol, err := generator.Generate(n, rand.New(rand.NewPCG(seed, uint64(n))), generator.Options{MinRegionSize: generator.DefaultMinRegionSize(n)})
		if err != nil || sol == nil {
			continue
		}
		res := solver.Solve(board.New(p), observers.All(p), solver.Options{Lookahead: true, Backtrack: true})
		rp := &Puzzle{n, seed, p, sol, res}
		testPuzzles[n] = rp
		return rp
	}
}

func newTestParties(t *testing.T) (*Manager, *testClock) {
	t.Helper()
	clk := &testClock{t: time.Date(2026, 11, 7, 20, 0, 0, 0, time.UTC)}
	pm := New(Config{
		MinN:       4,
		MaxN:       14,
		Prepare:    func(n int) (*Puzzle, error) { return testPuzzle(t, n), nil },
		Now:        clk.now,
		RetryDelay: time.Nanosecond,
	})
	return pm, clk
}

// waitPrepared waits until the party has its next puzzle ready.
func waitPrepared(t *testing.T, pm *Manager, code string) {
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

func mustJoin(t *testing.T, pm *Manager, code, name string) string {
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

func wantKind(t *testing.T, err error, kind Kind) {
	t.Helper()
	if pe, ok := errors.AsType[*Error](err); !ok || pe.Kind != kind {
		t.Fatalf("got error %v, want kind %d", err, kind)
	}
}

func snap(t *testing.T, pm *Manager, code, tok string) Snapshot {
	t.Helper()
	s, err := pm.Snapshot(code, tok)
	must(t, err)
	return s
}

// startRound creates a 5×5 party with the named players, all ready, and
// moves the clock past the countdown.
func startRound(t *testing.T, pm *Manager, clk *testClock, names ...string) (string, []string) {
	t.Helper()
	return startRoundMode(t, pm, clk, ModeVersus, names...)
}

// startRoundMode is startRound for a party in the given mode.
func startRoundMode(t *testing.T, pm *Manager, clk *testClock, mode string, names ...string) (string, []string) {
	t.Helper()
	return startRoundWith(t, pm, clk, Options{Sizes: []int{5}, Mode: mode}, names...)
}

// startRoundWith is startRound for a party with the given options.
func startRoundWith(t *testing.T, pm *Manager, clk *testClock, o Options, names ...string) (string, []string) {
	t.Helper()
	code, err := pm.Create(o)
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
	clk.add(Countdown)
	return code, toks
}

func TestPartyRound(t *testing.T) {
	pm, clk := newTestParties(t)
	code, err := pm.Create(Options{Sizes: []int{5, 5, 3, 99}, Mode: ""})
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
	if s.Phase != PhasePlaying || s.Round != 1 || s.N != 5 || s.StartInMs != Countdown.Milliseconds() {
		t.Fatalf("round snapshot %+v", s)
	}
	wantKind(t, pm.SetReady(code, a, true), Conflict)

	sol := testPuzzle(t, 5).Solution
	_, err = pm.Solve(code, a, 1, sol, 0, 0)
	wantKind(t, err, Conflict) // countdown still running

	clk.add(Countdown + time.Second)
	if s := snap(t, pm, code, ""); s.ElapsedMs != 1000 || s.StartInMs != 0 {
		t.Fatalf("elapsed %d, start in %d", s.ElapsedMs, s.StartInMs)
	}
	wrong := append([]int(nil), sol...)
	wrong[0], wrong[1] = wrong[1], wrong[0]
	_, err = pm.Solve(code, a, 1, wrong, 0, 0)
	wantKind(t, err, Invalid)
	_, err = pm.Solve(code, a, 2, sol, 0, 0)
	wantKind(t, err, Conflict)

	res, err := pm.Solve(code, a, 1, sol, 1, 0)
	must(t, err)
	if !res.Solved || res.Hints != 1 || res.TimeMs != 1000 {
		t.Fatalf("result %+v", res)
	}
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying || s.Ranking[1].Status != "playing" {
		t.Fatalf("round ended while Ben still plays: %+v", s)
	}
	must(t, pm.Progress(code, b, 1, 3, 0, 0))
	clk.add(2 * time.Second)
	_, err = pm.Solve(code, b, 1, sol, 0, 0)
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
	if res2, err := pm.Solve(code, a, 1, sol, 5, 0); err != nil || res2 != res {
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
	must(t, pm.GiveUp(code, toks[0], 1, 2, 0))
	must(t, pm.GiveUp(code, toks[0], 1, 2, 0)) // repeated: fine
	_, err := pm.Solve(code, toks[0], 1, sol, 0, 0)
	wantKind(t, err, Conflict)
	clk.add(30 * time.Second)
	_, err = pm.Solve(code, toks[1], 1, sol, 4, 0)
	must(t, err)
	wantKind(t, pm.GiveUp(code, toks[1], 1, 0, 0), Conflict)
	must(t, pm.GiveUp(code, toks[2], 1, 0, 0))
	s := snap(t, pm, code, "")
	if s.Phase != PhaseResults {
		t.Fatal("round did not end")
	}
	r := s.Ranking
	if r[0].Name != "B" || r[0].Rank != 1 || r[1].Status != "gaveup" || r[1].Rank != 2 || r[2].Status != "gaveup" || r[2].Rank != 3 {
		t.Fatalf("ranking %+v", r)
	}
}

func TestPartyLives(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRoundWith(t, pm, clk, Options{Sizes: []int{5}, Lives: 2}, "A", "B", "C", "D", "E")
	if s := snap(t, pm, code, ""); s.Lives != 2 {
		t.Fatalf("lives %d, want 2", s.Lives)
	}
	sol := testPuzzle(t, 5).Solution
	status := func(name string) RankEntry {
		for _, e := range snap(t, pm, code, "").Ranking {
			if e.Name == name {
				return e
			}
		}
		t.Fatalf("%s not in the ranking", name)
		return RankEntry{}
	}

	// A breaks the rules once, then twice: out.
	must(t, pm.Progress(code, toks[0], 1, 1, 0, 1))
	must(t, pm.Progress(code, toks[0], 1, 1, 0, 0)) // errors never decrease
	if e := status("A"); e.Status != "playing" || e.Errors != 1 {
		t.Fatalf("A after one error: %+v", e)
	}
	clk.add(time.Second)
	must(t, pm.Progress(code, toks[0], 1, 2, 1, 2))
	if e := status("A"); e.Status != "out" || e.Errors != 2 || e.Hints != 1 || e.TimeMs != 1000 {
		t.Fatalf("A after two errors: %+v", e)
	}
	_, err := pm.Solve(code, toks[0], 1, sol, 1, 2)
	wantKind(t, err, Conflict)

	// B solves, but reports too many errors with it (the progress got lost).
	_, err = pm.Solve(code, toks[1], 1, sol, 0, 3)
	wantKind(t, err, Conflict)
	if e := status("B"); e.Status != "out" {
		t.Fatalf("B: %+v", e)
	}
	// C's page gives up by itself on the last life.
	must(t, pm.GiveUp(code, toks[2], 1, 0, 2))
	// D solves with one error to spare.
	res, err := pm.Solve(code, toks[3], 1, sol, 0, 1)
	must(t, err)
	if !res.Solved || res.Errors != 1 {
		t.Fatalf("D: %+v", res)
	}

	// E solves later and with hints, but without errors: ahead of D.
	clk.add(time.Second)
	_, err = pm.Solve(code, toks[4], 1, sol, 2, 0)
	must(t, err)

	s := snap(t, pm, code, "")
	if s.Phase != PhaseResults {
		t.Fatal("round did not end")
	}
	r := s.Ranking
	if r[0].Name != "E" || r[0].Rank != 1 || r[1].Name != "D" || r[1].Rank != 2 || r[1].Errors != 1 || r[2].Status != "out" || r[2].Rank != 3 || r[4].Status != "out" || r[4].Rank != 5 {
		t.Fatalf("ranking %+v", r)
	}

	// The next round starts with all lives.
	waitPrepared(t, pm, code)
	for _, tok := range toks {
		must(t, pm.SetReady(code, tok, true))
	}
	clk.add(Countdown)
	if e := status("A"); e.Status != "playing" || e.Errors != 0 {
		t.Fatalf("A in round 2: %+v", e)
	}
}

func TestPartyLivesOptions(t *testing.T) {
	pm, _ := newTestParties(t)
	for _, c := range []struct {
		o    Options
		want int
	}{
		{Options{Sizes: []int{5}}, 0},
		{Options{Sizes: []int{5}, Lives: 3}, 3},
		{Options{Sizes: []int{5}, Lives: -1}, 0},
		{Options{Sizes: []int{5}, Lives: 99}, MaxLives},
		{Options{Sizes: []int{5}, Mode: ModeCoop, Lives: 3}, 0},
		{Options{Sizes: []int{5}, Mode: ModeClock, Lives: 3}, 0},
	} {
		code, err := pm.Create(c.o)
		must(t, err)
		if got := snap(t, pm, code, "").Lives; got != c.want {
			t.Errorf("%+v: lives %d, want %d", c.o, got, c.want)
		}
	}
}

func TestPartyHintsNeverDecrease(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRound(t, pm, clk, "A", "B")
	must(t, pm.Progress(code, toks[0], 1, 1, 3, 0))
	must(t, pm.Progress(code, toks[0], 1, 2, 1, 0)) // e.g. after cleared storage
	if h := snap(t, pm, code, "").Ranking[0].Hints; h != 3 {
		t.Fatalf("hints %d, want 3", h)
	}
	res, err := pm.Solve(code, toks[0], 1, testPuzzle(t, 5).Solution, 0, 0)
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
	clk.add(absentAfter + time.Second)
	pm.Tick()
	s := snap(t, pm, code, "")
	if !s.Players[0].Present || s.Players[1].Present || s.Ranking[1].Status != "absent" {
		t.Fatalf("presence %+v", s)
	}
	sol := testPuzzle(t, 5).Solution
	_, err = pm.Solve(code, toks[0], 1, sol, 0, 0)
	must(t, err)
	if s := snap(t, pm, code, ""); s.Phase != PhaseResults {
		t.Fatal("absent player blocks the end of the round")
	}
	_, err = pm.Solve(code, toks[1], 1, sol, 0, 0)
	wantKind(t, err, Conflict)

	// The absent player does not block the start either, until back.
	waitPrepared(t, pm, code)
	must(t, pm.Progress(code, toks[1], 1, 0, 0, 0)) // B is back (any request)
	must(t, pm.SetReady(code, toks[0], true))
	if s := snap(t, pm, code, ""); s.Phase != PhaseResults {
		t.Fatal("started without B being ready")
	}
	clk.add(absentAfter + time.Second)
	pm.Tick()
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying || len(s.Ranking) != 1 {
		t.Fatalf("round 2 should start with A alone: %+v", s)
	}
}

func TestPartyAllAbsent(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRound(t, pm, clk, "A")
	clk.add(absentAfter + time.Second)
	pm.Tick()
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying {
		t.Fatal("round ended with nobody there")
	}
	must(t, pm.Progress(code, toks[0], 1, 1, 0, 0))
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying || !s.Players[0].Present {
		t.Fatalf("after return: %+v", s)
	}
	_, err := pm.Solve(code, toks[0], 1, testPuzzle(t, 5).Solution, 0, 0)
	must(t, err)
	if s := snap(t, pm, code, ""); s.Phase != PhaseResults {
		t.Fatal("round did not end")
	}
}

func TestPartyLateJoin(t *testing.T) {
	pm, clk := newTestParties(t)
	code, err := pm.Create(Options{Sizes: []int{5}, Mode: ""})
	must(t, err)
	waitPrepared(t, pm, code)
	a := mustJoin(t, pm, code, "A")
	must(t, pm.SetReady(code, a, true))
	clk.add(time.Second)
	b := mustJoin(t, pm, code, "B") // during the countdown: plays along
	clk.add(Countdown)
	c := mustJoin(t, pm, code, "C") // too late: waits for the next round
	s := snap(t, pm, code, "")
	if len(s.Ranking) != 2 || !s.Players[1].Playing || s.Players[2].Playing {
		t.Fatalf("participants %+v", s)
	}
	wantKind(t, pm.Progress(code, c, 1, 1, 0, 0), Conflict)
	sol := testPuzzle(t, 5).Solution
	for _, tok := range []string{a, b} {
		_, err := pm.Solve(code, tok, 1, sol, 0, 0)
		must(t, err)
	}
	if s := snap(t, pm, code, ""); s.Phase != PhaseResults {
		t.Fatal("C blocks the end of the round")
	}
}

func TestPartyOnlyLateJoinerPresent(t *testing.T) {
	pm, clk := newTestParties(t)
	code, _ := startRound(t, pm, clk, "A")
	clk.add(absentAfter + time.Second)
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
	wantKind(t, pm.Leave(code, toks[1]), Forbidden)
	_, err := pm.Solve(code, toks[0], 1, testPuzzle(t, 5).Solution, 0, 0)
	must(t, err)
	s := snap(t, pm, code, "")
	if s.Phase != PhaseResults || len(s.Players) != 1 || s.Ranking[1].Status != "left" || s.Ranking[1].Name != "B" {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestPartyRejoinAndLimits(t *testing.T) {
	pm, _ := newTestParties(t)
	_, err := pm.Create(Options{Sizes: []int{2, 15}, Mode: ""})
	wantKind(t, err, Invalid)
	code, err := pm.Create(Options{Sizes: []int{6}, Mode: ""})
	must(t, err)
	_, err = pm.Snapshot("nope", "")
	wantKind(t, err, NotFound)
	_, _, err = pm.Join(code, "   ", "")
	wantKind(t, err, Invalid)

	tok, id, err := pm.Join(strings.ToLower(code), strings.Repeat("x", 30), "")
	must(t, err)
	tok2, id2, err := pm.Join(code, "", tok)
	must(t, err)
	if tok2 != tok || id2 != id {
		t.Fatal("rejoin gave a new player")
	}
	s := snap(t, pm, code, tok)
	if len(s.Players) != 1 || len([]rune(s.Players[0].Name)) != MaxName {
		t.Fatalf("players %+v", s.Players)
	}
	for i := 1; i < MaxPlayers; i++ {
		mustJoin(t, pm, code, "P")
	}
	_, _, err = pm.Join(code, "one too many", "")
	wantKind(t, err, Conflict)
	wantKind(t, pm.SetReady(code, "bogus", true), Forbidden)
}

func TestPartyExpiry(t *testing.T) {
	pm, clk := newTestParties(t)
	code, err := pm.Create(Options{Sizes: []int{5}, Mode: ""})
	must(t, err)
	tok := mustJoin(t, pm, code, "A")
	ch, cancel, err := pm.Subscribe(code, tok)
	must(t, err)
	clk.add(idleExpire + time.Minute)
	pm.Tick()
	if _, err := pm.Snapshot(code, ""); err != nil {
		t.Fatal("party with an open stream expired")
	}
	cancel()
	pm.Tick()
	_, err = pm.Snapshot(code, "")
	wantKind(t, err, NotFound)
	<-ch // the stream was told
}

// TestPartyWordCodes: with Config.Words every second party gets the next
// word of a shuffled order, and a random code when that word is taken;
// without, only random codes of codeLetters.
func TestPartyWordCodes(t *testing.T) {
	newManager := func(seed uint64, words []string) *Manager {
		var mu sync.Mutex // the preparations pick sizes in the background
		r := rand.New(rand.NewPCG(seed, 1))
		return New(Config{
			MinN:    4,
			MaxN:    14,
			Prepare: func(n int) (*Puzzle, error) { return testPuzzle(t, n), nil },
			Pick: func(k int) int {
				mu.Lock()
				defer mu.Unlock()
				return r.IntN(k)
			},
			Words: words,
		})
	}
	create := func(pm *Manager) string {
		t.Helper()
		code, err := pm.Create(Options{Sizes: []int{5}})
		must(t, err)
		if _, err := pm.Snapshot(NormalizeCode(strings.ToLower(code)), ""); err != nil {
			t.Fatalf("party %s not found by its code typed in lower case: %v", code, err)
		}
		return code
	}
	random := func(code string) bool {
		return len(code) == 4 && strings.Trim(code, codeLetters) == ""
	}
	isWord := func(code string) bool { return slices.Contains(CodeWords, code) }

	for _, w := range CodeWords {
		if len(w) != 4 || strings.Trim(w, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
			t.Errorf("word %q is not four capital letters", w)
		}
	}
	order := func(seed uint64) []string {
		pm := newManager(seed, CodeWords)
		var words []string
		for range CodeWords {
			code := create(pm)
			if !isWord(code) {
				t.Fatalf("seed %d: code %s on a word's turn", seed, code)
			}
			words = append(words, code)
			if code := create(pm); !random(code) {
				t.Fatalf("seed %d: code %s on a random code's turn", seed, code)
			}
		}
		if got, want := slices.Sorted(slices.Values(words)), slices.Sorted(slices.Values(CodeWords)); !slices.Equal(got, want) {
			t.Fatalf("seed %d: words %v, want each of %v once", seed, words, CodeWords)
		}
		// All words are taken now: the next word's turn falls back to a
		// random code, and once that party is gone the word comes again.
		if code := create(pm); !random(code) || isWord(code) {
			t.Fatalf("seed %d: code %s, want a random one while all words are taken", seed, code)
		}
		pm.mu.Lock()
		delete(pm.parties, words[2])
		pm.mu.Unlock()
		create(pm) // random
		create(pm) // words[1], taken: random
		create(pm) // random
		if code := create(pm); code != words[2] {
			t.Fatalf("seed %d: code %s, want the freed word %s", seed, code, words[2])
		}
		return words
	}
	if a, b := order(1), order(2); slices.Equal(a, b) || slices.Equal(a, CodeWords) {
		t.Errorf("words not shuffled: %v and %v", a, b)
	}

	pm := newManager(3, nil)
	for range 10 {
		if code := create(pm); !random(code) {
			t.Fatalf("code %s without words", code)
		}
	}
}

func TestPartyPrepareAfterReady(t *testing.T) {
	pm, _ := newTestParties(t)
	gate := make(chan struct{})
	pm.prepare = func(n int) (*Puzzle, error) {
		<-gate
		return testPuzzle(t, n), nil
	}
	code, err := pm.Create(Options{Sizes: []int{5}, Mode: ""})
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
	pm.prepare = func(n int) (*Puzzle, error) {
		mu.Lock()
		defer mu.Unlock()
		if fails > 0 {
			fails--
			return nil, errors.New("boom")
		}
		return testPuzzle(t, n), nil
	}
	code, err := pm.Create(Options{Sizes: []int{5}, Mode: ""})
	must(t, err)
	waitPrepared(t, pm, code)
	if s := snap(t, pm, code, ""); s.PrepErr != "" || s.Preparing {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestPartySubscribe(t *testing.T) {
	pm, _ := newTestParties(t)
	code, err := pm.Create(Options{Sizes: []int{5}, Mode: ""})
	must(t, err)
	ch, cancel, err := pm.Subscribe(code, "")
	must(t, err)
	tok := mustJoin(t, pm, code, "A")
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("no signal after a join")
	}
	_, cancelA, err := pm.Subscribe(code, tok) // A's own stream
	must(t, err)
	if n := pm.Subscribers(code); n != 2 {
		t.Fatalf("%d subscribers, want 2", n)
	}
	cancel()
	cancelA()
	pm.mu.Lock()
	conns := pm.parties[code].players[0].conns
	pm.mu.Unlock()
	if n := pm.Subscribers(code); n != 0 || conns != 0 {
		t.Fatalf("after cancel: %d subscribers, A has %d streams", n, conns)
	}
	if n := pm.Subscribers("QQQQ"); n != 0 {
		t.Fatalf("unknown party: %d subscribers", n)
	}
}

func TestPartyPage(t *testing.T) {
	pm, clk := newTestParties(t)
	code, _ := startRound(t, pm, clk, "A")
	_, err := pm.Page(code, 2, 0)
	wantKind(t, err, NotFound)
	pg, err := pm.Page(code, 1, 0)
	must(t, err)
	if pg.Code != code || pg.Round != 1 || pg.Mode != ModeVersus || pg.K != 0 || pg.Puzzle != testPuzzle(t, 5) {
		t.Fatalf("page %+v", pg)
	}
}
