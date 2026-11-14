package web

import (
	"cmp"
	"fmt"
	"net/http"
	"slices"
	"time"

	"starbattle/internal/solver"
)

// Rounds against the clock ("Gegen die Uhr"): every participant solves as
// many puzzles as possible before the time is up. The puzzles of a round
// form one sequence, the same for all: whoever is at puzzle k plays the k-th
// one. The sequence grows from the party's prepared puzzles when somebody
// reaches its end, and the party keeps a few prepared (clockAhead), so fast
// players rarely wait for the generator. Whoever does waits until the next
// puzzle is ready, with the clock running.
//
// Solving (checked by the server) or skipping (as often as the rules allow)
// moves a participant on to the next puzzle. There are no hints; checking
// the board only tells whether everything so far fits the solution, not
// where it does not, and the page never has the solution. The ranking
// goes by puzzles solved, then by the stars on the current puzzle: each one
// in its place counts one, each wrong one minus one, so stars everywhere do
// not pay. Clients report their stars as they go; reports and solutions that
// arrive shortly after the deadline (clockGrace) still count.

// Rules and timing against the clock.
const (
	ClockDefaultMinutes = 5
	ClockDefaultSkips   = 2
	ClockMaxSkips       = 99

	clockAhead = 3               // puzzles kept prepared
	clockGrace = 3 * time.Second // after the deadline: late reports still count
)

// ClockMinutes are the round lengths a creator can choose from.
var ClockMinutes = []int{2, 5, 10}

// ClockRules are the settings of a party against the clock.
type ClockRules struct {
	Minutes int `json:"minutes"`
	Skips   int `json:"skips"` // per participant and round, -1: any number
}

// normalize returns the rules a party in mode plays by: none outside clock
// mode, otherwise a known length (the default if not) and the skips, any
// number if negative.
func (c ClockRules) normalize(mode string) ClockRules {
	if mode != ModeClock {
		return ClockRules{}
	}
	if !slices.Contains(ClockMinutes, c.Minutes) {
		c.Minutes = ClockDefaultMinutes
	}
	if c.Skips < 0 {
		c.Skips = -1
	}
	c.Skips = min(c.Skips, ClockMaxSkips)
	return c
}

func (c ClockRules) duration() time.Duration { return time.Duration(c.Minutes) * time.Minute }

// clockRound is the state of a round against the clock. It is guarded by
// Parties.mu like the rest of the party.
type clockRound struct {
	rules    ClockRules
	deadline time.Time
	seq      []*RoundPuzzle        // the round's puzzles so far, in order
	data     map[int]*Data         // page data per index into seq, built on first request
	players  map[string]*clockTurn // participant ID → their state
}

// clockTurn is where a participant stands in a round against the clock.
type clockTurn struct {
	idx    int // current puzzle, index into seq; len(seq) while it is prepared
	solved int
	skips  int
	stars  int // right minus wrong stars on the current puzzle, as reported
}

func newClockRound(rules ClockRules, start time.Time, first *RoundPuzzle) *clockRound {
	return &clockRound{
		rules:    rules,
		deadline: start.Add(rules.duration()),
		seq:      []*RoundPuzzle{first},
		data:     map[int]*Data{},
		players:  map[string]*clockTurn{},
	}
}

// turn returns the state of participant id, new ones at the first puzzle.
func (c *clockRound) turn(id string) *clockTurn {
	t := c.players[id]
	if t == nil {
		t = &clockTurn{}
		c.players[id] = t
	}
	return t
}

func (t *clockTurn) advance() {
	t.idx++
	t.stars = 0
}

// clockFeed extends the running round's sequence from the prepared puzzles
// as far as somebody has got; pm.mu must be held.
func (p *Party) clockFeed() {
	r := p.Round
	if p.Phase != PhasePlaying || r == nil || r.clock == nil {
		return
	}
	c := r.clock
	need := 0
	for _, t := range c.players {
		need = max(need, t.idx+1)
	}
	for len(c.seq) < need && len(p.ready) > 0 {
		c.seq = append(c.seq, p.ready[0])
		p.ready = slices.Delete(p.ready, 0, 1)
	}
}

// clockAt returns the round against the clock num and the state of the
// player with this token, who must be at puzzle k (from 1) of it, or past it
// (past is true then). The round must have started, and the deadline may be
// at most grace ago. pm.mu must be held.
func (pm *Parties) clockAt(code, token string, num, k int, grace time.Duration) (p *Party, r *Round, t *clockTurn, past bool, err error) {
	p, pl, err := pm.player(code, token)
	if err != nil {
		return nil, nil, nil, false, err
	}
	if r, err = p.participant(pl, num); err != nil {
		return nil, nil, nil, false, err
	}
	c := r.clock
	if c == nil {
		return nil, nil, nil, false, partyErr(http.StatusConflict, "In dieser Party wird nicht gegen die Uhr gespielt.")
	}
	now := pm.now()
	if now.Before(r.Start) {
		return nil, nil, nil, false, partyErr(http.StatusConflict, "Die Runde hat noch nicht begonnen.")
	}
	if !now.Before(c.deadline.Add(grace)) {
		return nil, nil, nil, false, partyErr(http.StatusConflict, "Die Zeit ist um.")
	}
	t = c.turn(pl.ID)
	switch {
	case k-1 < t.idx:
		return p, r, t, true, nil // e.g. a repeated request
	case k-1 > t.idx:
		return nil, nil, nil, false, partyErr(http.StatusConflict, "Bei Puzzle %d bist du noch nicht.", k)
	case t.idx >= len(c.seq):
		return nil, nil, nil, false, partyErr(http.StatusConflict, "Puzzle %d wird noch vorbereitet.", k)
	}
	return p, r, t, false, nil
}

// ClockSolve records the player's solution (star column per row) of puzzle
// k of round num and moves them on to the next one. Repeating it is harmless.
func (pm *Parties) ClockSolve(code, token string, num, k int, stars []int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, r, t, past, err := pm.clockAt(code, token, num, k, clockGrace)
	if err != nil || past {
		return err
	}
	if !slices.Equal(stars, r.clock.seq[t.idx].Solution) {
		return partyErr(http.StatusBadRequest, "Das ist nicht die Lösung.")
	}
	t.solved++
	t.advance()
	pm.update(p)
	return nil
}

// ClockSkip moves the player on from puzzle k of round num without solving
// it, if they have skips left.
func (pm *Parties) ClockSkip(code, token string, num, k int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, r, t, past, err := pm.clockAt(code, token, num, k, 0)
	if err != nil || past {
		return err
	}
	if lim := r.clock.rules.Skips; lim >= 0 && t.skips >= lim {
		return partyErr(http.StatusConflict, "Du kannst nicht mehr überspringen.")
	}
	t.skips++
	t.advance()
	pm.update(p)
	return nil
}

// ClockCheck tells whether the player's board of puzzle k of round num (one
// digit per cell, row-major: 0 free, 1 cross, 2 star) fits the solution so
// far: no star off the solution and no cross on it.
func (pm *Parties) ClockCheck(code, token string, num, k int, board string) (bool, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	_, r, t, past, err := pm.clockAt(code, token, num, k, 0)
	if err != nil {
		return false, err
	}
	if past {
		return false, partyErr(http.StatusConflict, "Mit Puzzle %d bist du schon fertig.", k)
	}
	rp := r.clock.seq[t.idx]
	n := rp.N
	if len(board) != n*n {
		return false, partyErr(http.StatusBadRequest, "Ungültiges Brett.")
	}
	ok := true
	for i := range n * n {
		v := board[i]
		if v < '0'+cellFree || v > '0'+cellStar {
			return false, partyErr(http.StatusBadRequest, "Ungültiges Brett.")
		}
		star := rp.Solution[i/n] == i%n
		if (v == '0'+cellStar && !star) || (v == '0'+cellCross && star) {
			ok = false
		}
	}
	return ok, nil
}

// ClockProgress records the stars the player has set on puzzle k of round
// num, as cell indices (row-major).
func (pm *Parties) ClockProgress(code, token string, num, k int, cells []int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, r, t, past, err := pm.clockAt(code, token, num, k, clockGrace)
	if err != nil || past {
		return err
	}
	rp := r.clock.seq[t.idx]
	n := rp.N
	if len(cells) > n*n {
		return partyErr(http.StatusBadRequest, "Zu viele Sterne.")
	}
	seen := map[int]bool{}
	score := 0
	for _, i := range cells {
		if i < 0 || i >= n*n {
			return partyErr(http.StatusBadRequest, "Ungültiges Feld.")
		}
		if seen[i] {
			continue
		}
		seen[i] = true
		if rp.Solution[i/n] == i%n {
			score++
		} else {
			score--
		}
	}
	if score != t.stars {
		t.stars = score
		pm.update(p)
	}
	return nil
}

// clockData returns the page data of puzzle k (from 1) of round r: without
// solution and solving trace, there is no help against the clock.
func (p *Party) clockData(r *Round, k int) (Data, error) {
	c := r.clock
	if k > len(c.seq) {
		return Data{}, partyErr(http.StatusNotFound, "Puzzle %d von Runde %d gibt es (noch) nicht.", k, r.Num)
	}
	if c.data[k-1] == nil {
		rp := c.seq[k-1]
		d := Build(fmt.Sprintf("Party %s · Runde %d · Puzzle %d · %d×%d", p.Code, r.Num, k, rp.N, rp.N), rp.P, nil, solver.Result{})
		d.Status, d.Steps = "pending", []Step{}
		d.Served = true
		d.Text = rp.P.String()
		c.data[k-1] = &d
	}
	d := *c.data[k-1]
	d.Party = &PartyInfo{Code: p.Code, Round: r.Num, Mode: p.Mode, Puzzle: k}
	return d, nil
}

// ClockSnap is a party's play against the clock as sent to the players.
type ClockSnap struct {
	ClockRules
	Sizes []int `json:"sizes"` // size of each puzzle of the current or last round so far
}

func (p *Party) clockSnap() *ClockSnap {
	s := &ClockSnap{ClockRules: p.Clock, Sizes: []int{}}
	if r := p.Round; r != nil && r.clock != nil {
		for _, rp := range r.clock.seq {
			s.Sizes = append(s.Sizes, rp.N)
		}
	}
	return s
}

// clockRanking orders the participants of the current round against the
// clock by puzzles solved, then stars on the current puzzle. Equal scores
// share a rank. The status is playing, absent or left during the round, and
// done or left after it.
func (p *Party) clockRanking() []RankEntry {
	r := p.Round
	c := r.clock
	byID := map[string]*Player{}
	for _, pl := range p.players {
		byID[pl.ID] = pl
	}
	out := []RankEntry{}
	for _, id := range r.Participants {
		t := c.players[id]
		if t == nil {
			t = &clockTurn{}
		}
		e := RankEntry{ID: id, Name: r.names[id], Stars: t.stars, Solved: t.solved, Skips: t.skips, Puzzle: t.idx + 1}
		pl := byID[id]
		switch {
		case pl == nil:
			e.Status = "left"
		case p.Phase != PhasePlaying:
			e.Status, e.Name = "done", pl.Name
		case !pl.present:
			e.Status, e.Name = "absent", pl.Name
		default:
			e.Status, e.Name = "playing", pl.Name
		}
		out = append(out, e)
	}
	score := func(a, b RankEntry) int {
		return cmp.Or(cmp.Compare(b.Solved, a.Solved), cmp.Compare(b.Stars, a.Stars))
	}
	slices.SortStableFunc(out, score)
	for i := range out {
		out[i].Rank = i + 1
		if i > 0 && score(out[i-1], out[i]) == 0 {
			out[i].Rank = out[i-1].Rank
		}
	}
	return out
}
