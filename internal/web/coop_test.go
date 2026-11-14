package web

import (
	"net/http"
	"slices"
	"testing"
	"time"
)

// solutionOps returns operations setting the stars of the solution, numbered
// from first.
func solutionOps(rp *RoundPuzzle, first int) [][4]int {
	var ops [][4]int
	for r, c := range rp.Solution {
		ops = append(ops, [4]int{first + r, r*rp.N + c, cellFree, cellStar})
	}
	return ops
}

func TestCoopModeDefault(t *testing.T) {
	pm, _ := newTestParties(t)
	code, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: "chess"})
	must(t, err)
	if s := snap(t, pm, code, ""); s.Mode != ModeVersus || s.Coop != nil {
		t.Fatalf("mode %q, coop %v; want versus without board", s.Mode, s.Coop)
	}
	// Marks belong to coop parties only.
	pm2, clk := newTestParties(t)
	code, toks := startRound(t, pm2, clk, "Ann")
	_, err = pm2.Mark(code, toks[0], 1, [][4]int{{1, 0, cellFree, cellCross}})
	wantStatus(t, err, http.StatusConflict)
}

func TestCoopBoard(t *testing.T) {
	pm, clk := newTestParties(t)
	code, err := pm.Create(PartyOptions{Sizes: []int{5}, Mode: ModeCoop})
	must(t, err)
	waitPrepared(t, pm, code)
	ann, bob := mustJoin(t, pm, code, "Ann"), mustJoin(t, pm, code, "Bob")
	must(t, pm.SetReady(code, ann, true))
	must(t, pm.SetReady(code, bob, true))

	s := snap(t, pm, code, ann)
	if s.Mode != ModeCoop || s.Coop == nil || s.Coop.Board != "0000000000000000000000000" {
		t.Fatalf("coop snapshot %+v", s.Coop)
	}
	// Not before the start.
	_, err = pm.Mark(code, ann, 1, [][4]int{{1, 0, cellFree, cellCross}})
	wantStatus(t, err, http.StatusConflict)
	clk.add(partyCountdown)

	// Both saw cell 0 free: the first change wins, the second is rejected.
	rej, err := pm.Mark(code, ann, 1, [][4]int{{1, 0, cellFree, cellCross}})
	must(t, err)
	if len(rej) != 0 {
		t.Fatalf("rejected %v", rej)
	}
	rej, err = pm.Mark(code, bob, 1, [][4]int{{1, 0, cellFree, cellStar}, {2, 1, cellFree, cellCross}})
	must(t, err)
	if !slices.Equal(rej, []int{1}) {
		t.Fatalf("rejected %v, want [1]", rej)
	}
	// The same change twice is fine.
	rej, err = pm.Mark(code, ann, 1, [][4]int{{2, 1, cellFree, cellCross}})
	must(t, err)
	if len(rej) != 0 {
		t.Fatalf("rejected %v", rej)
	}
	s = snap(t, pm, code, bob)
	if s.Coop.Board[:3] != "110" || s.Coop.Seq != 2 || s.Coop.Acked != 2 {
		t.Fatalf("board %s seq %d acked %d", s.Coop.Board, s.Coop.Seq, s.Coop.Acked)
	}

	// A repeated request changes nothing (its numbers are acked).
	rej, err = pm.Mark(code, bob, 1, [][4]int{{1, 0, cellFree, cellStar}, {2, 1, cellFree, cellCross}})
	must(t, err)
	if len(rej) != 0 || snap(t, pm, code, "").Coop.Seq != 2 {
		t.Fatalf("repeat: rejected %v", rej)
	}

	// Undo after somebody else changed the cell: rejected, their mark stays.
	_, err = pm.Mark(code, bob, 1, [][4]int{{3, 0, cellCross, cellStar}})
	must(t, err)
	rej, err = pm.Mark(code, ann, 1, [][4]int{{3, 0, cellCross, cellFree}})
	must(t, err)
	if !slices.Equal(rej, []int{3}) || snap(t, pm, code, "").Coop.Board[0] != '2' {
		t.Fatalf("undo: rejected %v", rej)
	}

	// Contributions count changes only.
	moves := map[string]int{}
	for _, e := range snap(t, pm, code, "").Ranking {
		moves[e.Name] = e.Moves
	}
	if moves["Ann"] != 1 || moves["Bob"] != 2 {
		t.Fatalf("moves %v", moves)
	}

	// Invalid operations.
	for _, op := range [][4]int{{0, 0, 0, 1}, {9, -1, 0, 1}, {9, 25, 0, 1}, {9, 0, 0, 3}, {9, 0, 1, 1}} {
		_, err = pm.Mark(code, ann, 1, [][4]int{op})
		wantStatus(t, err, http.StatusBadRequest)
	}
	_, err = pm.Mark(code, ann, 1, make([][4]int, 51))
	wantStatus(t, err, http.StatusBadRequest)
	_, err = pm.Mark(code, ann, 2, [][4]int{{9, 0, 0, 1}})
	wantStatus(t, err, http.StatusConflict)
	_, err = pm.Mark(code, "nope", 1, [][4]int{{9, 0, 0, 1}})
	wantStatus(t, err, http.StatusForbidden)
}

func TestCoopSolve(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRoundMode(t, pm, clk, ModeCoop, "Ann", "Bob")
	rp := testPuzzle(t, 5)
	clk.add(42 * time.Second)

	// Bob sets all but the last star, Ann the last.
	ops := solutionOps(rp, 1)
	_, err := pm.Mark(code, toks[1], 1, ops[:4])
	must(t, err)
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying || s.Coop.Solved {
		t.Fatalf("phase %s solved %v before the last star", s.Phase, s.Coop.Solved)
	}
	_, err = pm.Mark(code, toks[0], 1, [][4]int{ops[4]})
	must(t, err)
	s := snap(t, pm, code, "")
	if s.Phase != PhaseResults || !s.Coop.Solved {
		t.Fatalf("phase %s solved %v", s.Phase, s.Coop.Solved)
	}
	for _, e := range s.Ranking {
		if e.Status != "solved" || e.TimeMs != 42000 || e.Rank != 0 {
			t.Fatalf("entry %+v", e)
		}
	}
	_, err = pm.Mark(code, toks[0], 1, [][4]int{{9, 0, cellFree, cellCross}})
	wantStatus(t, err, http.StatusConflict)
}

func TestCoopWrongStarsNotSolved(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRoundMode(t, pm, clk, ModeCoop, "Ann")
	rp := testPuzzle(t, 5)
	ops := solutionOps(rp, 1)
	// One star too many in the first row.
	extra := (rp.Solution[0] + 2) % 5
	ops = append(ops, [4]int{6, extra, cellFree, cellStar})
	_, err := pm.Mark(code, toks[0], 1, ops)
	must(t, err)
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying {
		t.Fatalf("phase %s with an extra star", s.Phase)
	}
	_, err = pm.Mark(code, toks[0], 1, [][4]int{{7, extra, cellStar, cellFree}})
	must(t, err)
	if s := snap(t, pm, code, ""); s.Phase != PhaseResults {
		t.Fatalf("phase %s after removing the extra star", s.Phase)
	}
}

func TestCoopVote(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRoundMode(t, pm, clk, ModeCoop, "Ann", "Bob", "Cid")
	must(t, pm.Vote(code, toks[0], 1, true))
	must(t, pm.Vote(code, toks[1], 1, true))
	s := snap(t, pm, code, "")
	if s.Phase != PhasePlaying || len(s.Coop.Votes) != 2 {
		t.Fatalf("phase %s votes %v", s.Phase, s.Coop.Votes)
	}
	// Withdrawn and given again.
	must(t, pm.Vote(code, toks[1], 1, false))
	must(t, pm.Vote(code, toks[1], 1, true))

	// Cid goes away: the present ones decide.
	for _, tok := range toks[:2] { // Ann and Bob keep their event streams
		_, cancel, err := pm.Subscribe(code, tok)
		must(t, err)
		defer cancel()
	}
	pm.Tick()
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying {
		t.Fatalf("phase %s while Cid is present", s.Phase)
	}
	clk.add(partyAbsentAfter + time.Second)
	pm.Tick()
	s = snap(t, pm, code, "")
	if s.Phase != PhaseResults {
		t.Fatalf("phase %s, want results", s.Phase)
	}
	for _, e := range s.Ranking {
		if e.Status != "gaveup" {
			t.Fatalf("entry %+v", e)
		}
	}
	wantStatus(t, pm.Vote(code, toks[0], 1, true), http.StatusConflict)
}

func TestCoopLateJoin(t *testing.T) {
	pm, clk := newTestParties(t)
	code, toks := startRoundMode(t, pm, clk, ModeCoop, "Ann")
	clk.add(time.Minute)
	dan, _, err := pm.Join(code, "Dan", "")
	must(t, err)
	s := snap(t, pm, code, dan)
	if s.Phase != PhasePlaying || len(s.Ranking) != 2 {
		t.Fatalf("phase %s ranking %+v", s.Phase, s.Ranking)
	}
	for _, p := range s.Players {
		if !p.Playing {
			t.Fatalf("%s not playing", p.Name)
		}
	}
	_, err = pm.Mark(code, dan, 1, [][4]int{{1, 0, cellFree, cellCross}})
	must(t, err)
	// Ann alone voting does not end the round, Dan counts.
	must(t, pm.Vote(code, toks[0], 1, true))
	if s := snap(t, pm, code, ""); s.Phase != PhasePlaying {
		t.Fatalf("phase %s", s.Phase)
	}
}

func TestCoopRoundData(t *testing.T) {
	pm, clk := newTestParties(t)
	code, _ := startRoundMode(t, pm, clk, ModeCoop, "Ann")
	d, err := pm.RoundData(code, 1, 0)
	must(t, err)
	if d.Party == nil || d.Party.Mode != ModeCoop {
		t.Fatalf("party info %+v", d.Party)
	}
}
