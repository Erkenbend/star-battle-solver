package web

import (
	"net/http"
	"slices"
	"strings"
)

// Cooperative rounds ("Miteinander"): all participants mark the same board,
// which lives on the server. Clients apply their own changes at once and send
// them as operations; the server orders them by arrival:
//
//   - An operation [num, i, from, to] says "cell i, which I saw as from,
//     becomes to". It is applied if the cell still is from, accepted without
//     change if it already is to, and rejected otherwise: a change based on a
//     stale view never overwrites what somebody else did in the meantime.
//   - num counts up per player. The server remembers the highest one it
//     processed (acked) and skips operations up to it, so resending after a
//     lost reply is harmless.
//   - Snapshots carry the whole board, its sequence number and the viewer's
//     acked number. A client shows the board plus its operations not yet
//     acked, replayed with the same rule.
//
// The server notices the solution itself; giving up takes the votes of all
// present participants.

// Cell values, as in app.js.
const (
	cellFree  = 0
	cellCross = 1
	cellStar  = 2
)

// coopBoard is the shared board of a cooperative round. It is guarded by
// Parties.mu like the rest of the party.
type coopBoard struct {
	cells  []uint8
	seq    int             // counts the changes of cells
	acked  map[string]int  // player ID → highest operation number processed
	moves  map[string]int  // player ID → operations that changed a cell
	votes  map[string]bool // player ID → wants to give up
	solved bool
}

func newCoopBoard(n int) *coopBoard {
	return &coopBoard{
		cells: make([]uint8, n*n),
		acked: map[string]int{},
		moves: map[string]int{},
		votes: map[string]bool{},
	}
}

// apply processes the operations of a player in order and returns the
// numbers of those rejected. Invalid operations reject the whole request.
func (b *coopBoard) apply(player string, ops [][4]int) ([]int, error) {
	if len(ops) > 2*len(b.cells) {
		return nil, partyErr(http.StatusBadRequest, "Zu viele Änderungen auf einmal.")
	}
	for _, op := range ops {
		num, i, from, to := op[0], op[1], op[2], op[3]
		if num <= 0 || i < 0 || i >= len(b.cells) || from < cellFree || from > cellStar || to < cellFree || to > cellStar || from == to {
			return nil, partyErr(http.StatusBadRequest, "Ungültige Änderung.")
		}
	}
	rejected := []int{}
	for _, op := range ops {
		num, i, from, to := op[0], op[1], uint8(op[2]), uint8(op[3])
		if num <= b.acked[player] {
			continue // processed before: a repeated request
		}
		b.acked[player] = num
		switch b.cells[i] {
		case from:
			b.cells[i] = to
			b.seq++
			b.moves[player]++
		case to:
			// somebody else did the same
		default:
			rejected = append(rejected, num)
		}
	}
	return rejected, nil
}

// starColumns returns the star column per row, -1 for none and -2 for more
// than one.
func (b *coopBoard) starColumns(n int) []int {
	out := make([]int, n)
	for r := range n {
		out[r] = -1
		for c := range n {
			if b.cells[r*n+c] == cellStar {
				if out[r] >= 0 {
					out[r] = -2
					break
				}
				out[r] = c
			}
		}
	}
	return out
}

func (b *coopBoard) String() string {
	var sb strings.Builder
	for _, v := range b.cells {
		sb.WriteByte('0' + v)
	}
	return sb.String()
}

// coopRound returns the running cooperative round num if pl takes part.
func (p *Party) coopRound(pl *Player, num int) (*Round, error) {
	r, err := p.participant(pl, num)
	if err != nil {
		return nil, err
	}
	if r.coop == nil {
		return nil, partyErr(http.StatusConflict, "In dieser Party spielt jeder für sich.")
	}
	return r, nil
}

// Mark applies the player's operations to the board of round num and
// returns the numbers of those rejected.
func (pm *Parties) Mark(code, token string, num int, ops [][4]int) ([]int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, pl, err := pm.player(code, token)
	if err != nil {
		return nil, err
	}
	r, err := p.coopRound(pl, num)
	if err != nil {
		return nil, err
	}
	if p.Phase != PhasePlaying {
		return nil, partyErr(http.StatusConflict, "Die Runde ist schon vorbei.")
	}
	now := pm.now()
	if now.Before(r.Start) {
		return nil, partyErr(http.StatusConflict, "Die Runde hat noch nicht begonnen.")
	}
	b := r.coop
	rejected, err := b.apply(pl.ID, ops)
	if err != nil {
		return nil, err
	}
	if slices.Equal(b.starColumns(r.Puzzle.N), r.Puzzle.Solution) {
		b.solved = true
		t := now.Sub(r.Start)
		for _, id := range r.Participants {
			r.Results[id] = &Result{Solved: true, Time: t, TimeMs: t.Milliseconds()}
		}
	}
	pm.update(p)
	return rejected, nil
}

// Vote sets or withdraws the player's vote to give up round num.
func (pm *Parties) Vote(code, token string, num int, giveUp bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, pl, err := pm.player(code, token)
	if err != nil {
		return err
	}
	r, err := p.coopRound(pl, num)
	if err != nil {
		return err
	}
	if p.Phase != PhasePlaying {
		return partyErr(http.StatusConflict, "Die Runde ist schon vorbei.")
	}
	if giveUp {
		r.coop.votes[pl.ID] = true
	} else {
		delete(r.coop.votes, pl.ID)
	}
	pm.update(p)
	return nil
}

// coopEnroll makes every present player a participant of the running
// cooperative round: joining in late is welcome; pm.mu must be held.
func (p *Party) coopEnroll() {
	r := p.Round
	if p.Phase != PhasePlaying || r == nil || r.coop == nil {
		return
	}
	for _, pl := range p.players {
		if pl.present && !slices.Contains(r.Participants, pl.ID) {
			r.Participants = append(r.Participants, pl.ID)
			r.names[pl.ID] = pl.Name
		}
	}
}

// coopMaybeEnd ends the cooperative round once it is solved, or when every
// present participant (at least one) votes to give up; pm.mu must be held.
func (pm *Parties) coopMaybeEnd(p *Party) {
	r := p.Round
	if r.coop.solved {
		p.Phase = PhaseResults
		return
	}
	voters := 0
	for _, pl := range p.players {
		if !pl.present || !slices.Contains(r.Participants, pl.ID) {
			continue
		}
		if !r.coop.votes[pl.ID] {
			return
		}
		voters++
	}
	if voters == 0 {
		return
	}
	t := pm.now().Sub(r.Start)
	for _, id := range r.Participants {
		r.Results[id] = &Result{Time: t, TimeMs: t.Milliseconds()}
	}
	p.Phase = PhaseResults
}

// CoopSnap is the shared board of a cooperative round as sent to a viewer.
type CoopSnap struct {
	Board  string   `json:"board"` // one digit per cell, row-major: 0 free, 1 cross, 2 star
	Seq    int      `json:"seq"`
	Acked  int      `json:"acked"` // the viewer's highest processed operation
	Solved bool     `json:"solved"`
	Votes  []string `json:"votes"` // IDs of participants who want to give up
}

func (r *Round) coopSnap(viewer string) *CoopSnap {
	b := r.coop
	s := &CoopSnap{Board: b.String(), Seq: b.seq, Acked: b.acked[viewer], Solved: b.solved, Votes: []string{}}
	for _, id := range r.Participants {
		if b.votes[id] {
			s.Votes = append(s.Votes, id)
		}
	}
	return s
}
